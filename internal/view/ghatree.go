package view

import (
	"fmt"
	"sort"
	"strings"

	"github.com/bloomerab/convoy/internal/model"
	"github.com/bloomerab/convoy/internal/render"
	"github.com/rivo/tview"
)

// GHATreeView shows GitHub Actions as a tree: org → repo → runs
type GHATreeView struct {
	*tview.TreeView
}

// SelectedResource returns the resource for the currently selected tree node.
func (tv *GHATreeView) SelectedResource() *model.Resource {
	node := tv.GetCurrentNode()
	if node == nil {
		return nil
	}
	if r, ok := node.GetReference().(*model.Resource); ok {
		return r
	}
	return nil
}

func NewGHATreeView(resources []model.Resource) *GHATreeView {
	// Filter to GHA runs
	var runs []model.Resource
	for _, r := range resources {
		if r.Kind == model.KindWorkflowRun {
			runs = append(runs, r)
		}
	}

	// Group by org → repo → runs
	type repoRuns struct {
		repo string
		runs []model.Resource
	}

	orgRepos := make(map[string]map[string][]model.Resource) // org → repo → runs
	for _, r := range runs {
		parts := strings.SplitN(r.Repo, "/", 2)
		org := parts[0]
		repo := r.Repo
		if _, ok := orgRepos[org]; !ok {
			orgRepos[org] = make(map[string][]model.Resource)
		}
		orgRepos[org][repo] = append(orgRepos[org][repo], r)
	}

	// Root
	root := tview.NewTreeNode("[#FFFFFF::b]GitHub Actions[-::-]").
		SetSelectable(false)

	// Sort orgs
	var orgs []string
	for org := range orgRepos {
		orgs = append(orgs, org)
	}
	sort.Strings(orgs)

	for _, org := range orgs {
		repos := orgRepos[org]
		orgNode := tview.NewTreeNode(fmt.Sprintf("[#6EB5FF]%s[-]", org)).
			SetSelectable(false).
			SetExpanded(true)
		root.AddChild(orgNode)

		// Sort repos
		var repoNames []string
		for repo := range repos {
			repoNames = append(repoNames, repo)
		}
		sort.Strings(repoNames)

		for _, repoName := range repoNames {
			repoRuns := repos[repoName]

			// Sort runs: failed first, then by time
			sort.Slice(repoRuns, func(i, j int) bool {
				hi, ti := repoRuns[i].SortKey()
				hj, tj := repoRuns[j].SortKey()
				if hi != hj {
					return hi < hj
				}
				return ti < tj
			})

			// Determine repo health from latest run
			repoHealth := model.HealthReady
			for _, r := range repoRuns {
				if r.Health.IsFailed() {
					repoHealth = model.HealthFailed
					break
				}
				if r.Health == model.HealthProgressing {
					repoHealth = model.HealthProgressing
				}
			}

			shortRepo := repoName
			if idx := strings.LastIndex(shortRepo, "/"); idx >= 0 {
				shortRepo = shortRepo[idx+1:]
			}

			repoColor := healthColorHex(repoHealth)
			repoLabel := fmt.Sprintf("[%s]%s[-] %s [#9696B4](%d runs)[-]",
				repoColor, repoHealth.Symbol(), shortRepo, len(repoRuns))
			repoNode := tview.NewTreeNode(repoLabel).
				SetSelectable(true).
				SetExpanded(repoHealth.IsFailed() || repoHealth == model.HealthProgressing)

			for _, r := range repoRuns {
				color := healthColorHex(r.Health)
				age := render.FormatAge(r.LastTransition)

				label := fmt.Sprintf("[%s]%s[-] %s [#9696B4]%s  %s  %s[-]",
					color, r.Health.Symbol(), r.Name, r.Branch, r.Actor, age)

				if r.Health.IsFailed() && r.Message != "" {
					msg := r.Message
					if len(msg) > 40 {
						msg = msg[:37] + "..."
					}
					label += fmt.Sprintf(" [#FF5050]%s[-]", msg)
				}

				rCopy := r
				runNode := tview.NewTreeNode(label).
					SetSelectable(true).
					SetReference(&rCopy)
				repoNode.AddChild(runNode)
			}

			orgNode.AddChild(repoNode)
		}
	}

	tree := tview.NewTreeView().
		SetRoot(root).
		SetCurrentNode(root)
	tree.SetBorderPadding(0, 0, 1, 1)

	return &GHATreeView{TreeView: tree}
}

// Refresh rebuilds the tree.
func (tv *GHATreeView) Refresh(resources []model.Resource) {
	var selectedKey string
	if r := tv.SelectedResource(); r != nil {
		selectedKey = r.Repo + "/" + r.Name + "/" + r.Branch
	}

	fresh := NewGHATreeView(resources)
	tv.SetRoot(fresh.GetRoot())

	if selectedKey != "" {
		tv.GetRoot().Walk(func(node, parent *tview.TreeNode) bool {
			if r, ok := node.GetReference().(*model.Resource); ok {
				key := r.Repo + "/" + r.Name + "/" + r.Branch
				if key == selectedKey {
					tv.SetCurrentNode(node)
					return false
				}
			}
			return true
		})
	}
}

func healthColorHexGHA(h model.HealthStatus) string {
	return healthColorHex(h)
}
