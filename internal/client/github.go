package client

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/bloomerab/convoy/config"
	"github.com/bloomerab/convoy/internal/model"
	"github.com/google/go-github/v72/github"
)

// GitHubPoller polls GitHub Actions workflow runs on an interval.
type GitHubPoller struct {
	client   *github.Client
	cfg      config.GitHubConfig
	mu       sync.RWMutex
	runs     []model.Resource
	username string // authenticated user's login
}

func NewGitHubPoller(cfg config.GitHubConfig) (*GitHubPoller, error) {
	token := cfg.Token
	if token == "" {
		token = ghAuthToken()
	}
	if token == "" {
		return nil, fmt.Errorf("no GitHub token: set github.token in config or run 'gh auth login'")
	}

	ghClient := github.NewClient(nil).WithAuthToken(token)

	p := &GitHubPoller{
		client: ghClient,
		cfg:    cfg,
	}

	return p, nil
}

// Username returns the authenticated GitHub username (resolved on first poll).
func (p *GitHubPoller) Username() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.username
}

// Start begins polling. Blocks until ctx is cancelled.
func (p *GitHubPoller) Start(ctx context.Context) error {
	// Resolve authenticated user
	user, _, err := p.client.Users.Get(ctx, "")
	if err != nil {
		return fmt.Errorf("get authenticated user: %w", err)
	}
	p.mu.Lock()
	p.username = user.GetLogin()
	p.mu.Unlock()

	// Initial poll
	p.poll(ctx)

	interval := 30 * time.Second
	if p.cfg.MaxRuns > 0 {
		// Don't spam the API
		interval = 30 * time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			p.poll(ctx)
		}
	}
}

func (p *GitHubPoller) poll(ctx context.Context) {
	repos, err := p.discoverRepos(ctx)
	if err != nil {
		log.Printf("github: discover repos: %v", err)
		return
	}

	p.mu.RLock()
	hasData := len(p.runs) > 0
	p.mu.RUnlock()

	var all []model.Resource
	for _, repo := range repos {
		if ctx.Err() != nil {
			return
		}
		runs, err := p.fetchRuns(ctx, repo)
		if err != nil {
			log.Printf("github: fetch runs for %s: %v", repo, err)
			continue
		}
		all = append(all, runs...)

		// On first load (no existing data), publish incrementally so
		// data appears fast. On subsequent polls, build full list first
		// then swap atomically to avoid rows disappearing.
		if !hasData {
			p.mu.Lock()
			snapshot := make([]model.Resource, len(all))
			copy(snapshot, all)
			p.runs = snapshot
			p.mu.Unlock()
		}
	}

	// Atomic swap of full result
	p.mu.Lock()
	p.runs = all
	p.mu.Unlock()
}

func (p *GitHubPoller) discoverRepos(ctx context.Context) ([]string, error) {
	if len(p.cfg.Repos) > 0 {
		result := make([]string, len(p.cfg.Repos))
		for i, r := range p.cfg.Repos {
			if strings.Contains(r, "/") {
				result[i] = r
			} else {
				result[i] = p.cfg.Org + "/" + r
			}
		}
		return result, nil
	}

	if p.cfg.Org == "" {
		return nil, nil
	}

	// List repos with recent activity (limit to 10 most recently pushed)
	opts := &github.RepositoryListByOrgOptions{
		Sort:        "pushed",
		Direction:   "desc",
		ListOptions: github.ListOptions{PerPage: 20},
	}
	repos, _, err := p.client.Repositories.ListByOrg(ctx, p.cfg.Org, opts)
	if err != nil {
		return nil, err
	}

	var result []string
	cutoff := time.Now().Add(-7 * 24 * time.Hour)
	for _, r := range repos {
		if r.PushedAt != nil && r.PushedAt.After(cutoff) {
			result = append(result, r.GetFullName())
		}
	}
	return result, nil
}

func (p *GitHubPoller) fetchRuns(ctx context.Context, fullName string) ([]model.Resource, error) {
	parts := strings.SplitN(fullName, "/", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid repo name: %s", fullName)
	}
	owner, repo := parts[0], parts[1]

	opts := &github.ListWorkflowRunsOptions{
		ListOptions: github.ListOptions{PerPage: p.cfg.MaxRuns},
	}
	result, _, err := p.client.Actions.ListRepositoryWorkflowRuns(ctx, owner, repo, opts)
	if err != nil {
		return nil, err
	}

	var resources []model.Resource
	for _, run := range result.WorkflowRuns {
		r := model.Resource{
			Cluster: "github",
			Kind:    model.KindWorkflowRun,
			Name:    run.GetName(),
			Repo:    fullName,
			Branch:  run.GetHeadBranch(),
			Health:  workflowHealth(run.GetStatus(), run.GetConclusion()),
			Message: fmt.Sprintf("%s #%d", run.GetHeadBranch(), run.GetRunNumber()),
			Revision: func() string {
				sha := run.GetHeadSHA()
				if len(sha) > 7 {
					return sha[:7]
				}
				return sha
			}(),
			Actor: run.GetActor().GetLogin(),
			RunID: run.GetID(),
			URL:   run.GetHTMLURL(),
		}
		if run.UpdatedAt != nil {
			r.LastTransition = run.UpdatedAt.Time
		}
		resources = append(resources, r)
	}

	return resources, nil
}

// Resources returns the cached workflow runs.
func (p *GitHubPoller) Resources() []model.Resource {
	p.mu.RLock()
	defer p.mu.RUnlock()
	result := make([]model.Resource, len(p.runs))
	copy(result, p.runs)
	return result
}

// RerunWorkflow re-runs a GitHub Actions workflow run.
// Uses rerun-failed-jobs for failed runs, full rerun otherwise.
func (p *GitHubPoller) RerunWorkflow(ctx context.Context, repo string, runID int64, failed bool) error {
	parts := strings.SplitN(repo, "/", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid repo: %s", repo)
	}
	owner, repoName := parts[0], parts[1]

	if failed {
		_, err := p.client.Actions.RerunFailedJobsByID(ctx, owner, repoName, runID)
		return err
	}
	_, err := p.client.Actions.RerunWorkflowByID(ctx, owner, repoName, runID)
	return err
}

// FetchRunJobs returns a summary of jobs and their steps for a workflow run.
func (p *GitHubPoller) FetchRunJobs(ctx context.Context, repo string, runID int64) (string, error) {
	parts := strings.SplitN(repo, "/", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid repo: %s", repo)
	}
	owner, repoName := parts[0], parts[1]

	jobs, _, err := p.client.Actions.ListWorkflowJobs(ctx, owner, repoName, runID, &github.ListWorkflowJobsOptions{
		Filter:      "latest",
		ListOptions: github.ListOptions{PerPage: 30},
	})
	if err != nil {
		return "", fmt.Errorf("list jobs: %w", err)
	}

	var b strings.Builder
	for _, job := range jobs.Jobs {
		icon := "✓"
		if job.GetConclusion() == "failure" {
			icon = "✗"
		} else if job.GetStatus() != "completed" {
			icon = "●"
		} else if job.GetConclusion() == "skipped" {
			icon = "◌"
		}

		fmt.Fprintf(&b, "%s %s", icon, job.GetName())
		if job.GetConclusion() != "" && job.GetConclusion() != "success" {
			fmt.Fprintf(&b, " (%s)", job.GetConclusion())
		}
		b.WriteString("\n")

		for _, step := range job.Steps {
			stepIcon := "  ✓"
			if step.GetConclusion() == "failure" {
				stepIcon = "  ✗"
			} else if step.GetConclusion() == "skipped" {
				stepIcon = "  ◌"
			} else if step.GetStatus() != "completed" {
				stepIcon = "  ●"
			}
			fmt.Fprintf(&b, "%s %s", stepIcon, step.GetName())
			if step.GetConclusion() == "failure" {
				b.WriteString(" [FAILED]")
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	return b.String(), nil
}

func workflowHealth(status, conclusion string) model.HealthStatus {
	switch status {
	case "in_progress", "queued", "waiting", "pending":
		return model.HealthProgressing
	}
	switch conclusion {
	case "success":
		return model.HealthReady
	case "failure", "timed_out":
		return model.HealthFailed
	case "cancelled", "skipped":
		return model.HealthSuspended
	default:
		return model.HealthUnknown
	}
}

func ghAuthToken() string {
	out, err := exec.Command("gh", "auth", "token").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
