package render

import (
	"fmt"
	"strings"
	"time"

	"github.com/bloomerab/convoy/internal/model"
)

// FluxHeader returns columns for Flux resources.
func FluxHeader() []string {
	return []string{"", "NAME", "KIND", "CLUSTER", "STATUS", "MESSAGE", "REVISION", "AGE"}
}

// FluxRow renders a Flux resource to table cells.
func FluxRow(r model.Resource) []string {
	return []string{
		r.Health.Symbol(),
		r.Name,
		kindShort(r.Kind),
		r.Cluster,
		r.Health.String(),
		truncate(r.Message, 60),
		truncate(r.Revision, 12),
		formatAge(r.LastTransition),
	}
}

// GHAHeader returns columns for GitHub Actions runs.
func GHAHeader() []string {
	return []string{"", "REPO", "WORKFLOW", "BRANCH", "STATUS", "ACTOR", "REVISION", "AGE"}
}

// GHARow renders a workflow run to table cells.
func GHARow(r model.Resource) []string {
	repo := r.Repo
	if idx := strings.LastIndex(repo, "/"); idx >= 0 {
		repo = repo[idx+1:]
	}

	return []string{
		r.Health.Symbol(),
		repo,
		r.Name,
		r.Branch,
		r.Health.String(),
		r.Actor,
		truncate(r.Revision, 7),
		formatAge(r.LastTransition),
	}
}

// FailureRow renders a failed resource as a compact one-liner.
func FailureRow(r model.Resource) []string {
	name := r.Name
	if r.Kind == model.KindWorkflowRun {
		repo := r.Repo
		if idx := strings.LastIndex(repo, "/"); idx >= 0 {
			repo = repo[idx+1:]
		}
		name = repo + "/" + r.Name
	}

	source := r.Cluster
	if r.Kind == model.KindWorkflowRun {
		source = r.Branch
	}

	return []string{
		r.Health.Symbol(),
		name,
		kindShort(r.Kind),
		source,
		truncate(r.Message, 50),
		formatAge(r.LastTransition),
	}
}

// FailureHeader returns columns for the failures section.
func FailureHeader() []string {
	return []string{"", "NAME", "KIND", "SOURCE", "MESSAGE", "AGE"}
}

func kindShort(k model.ResourceKind) string {
	switch k {
	case model.KindKustomization:
		return "Ks"
	case model.KindHelmRelease:
		return "Hr"
	case model.KindGitRepository:
		return "GitRepo"
	case model.KindWorkflowRun:
		return "GHA"
	default:
		return string(k)
	}
}

func truncate(s string, max int) string {
	if len(s) > max {
		return s[:max-3] + "..."
	}
	return s
}

func formatAge(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}
