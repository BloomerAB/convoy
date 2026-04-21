package view

import (
	"fmt"
	"sort"
	"strings"

	"github.com/bloomerab/convoy/internal/model"
	"github.com/bloomerab/convoy/internal/render"
	"github.com/rivo/tview"
)

// GHATreeView shows GitHub Actions as a tree: org → repo → workflow → runs
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

	// Group: org → repo → workflow → runs
	type workflowRuns struct {
		name string
		runs []model.Resource
	}

	orgRepos := make(map[string]map[string]map[string][]model.Resource)
	for _, r := range runs {
		parts := strings.SplitN(r.Repo, "/", 2)
		org := parts[0]
		if _, ok := orgRepos[org]; !ok {
			orgRepos[org] = make(map[string]map[string][]model.Resource)
		}
		if _, ok := orgRepos[org][r.Repo]; !ok {
			orgRepos[org][r.Repo] = make(map[string][]model.Resource)
		}
		orgRepos[org][r.Repo][r.Name] = append(orgRepos[org][r.Repo][r.Name], r)
	}

	root := tview.NewTreeNode("[#FFFFFF::b]GitHub Actions[-::-]").
		SetSelectable(false)

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

		var repoNames []string
		for repo := range repos {
			repoNames = append(repoNames, repo)
		}
		sort.Strings(repoNames)

		for _, repoName := range repoNames {
			workflows := repos[repoName]

			// Repo health: worst of any workflow
			repoHealth := model.HealthReady
			for _, wfRuns := range workflows {
				for _, r := range wfRuns {
					if r.Health.IsFailed() {
						repoHealth = model.HealthFailed
					} else if r.Health == model.HealthProgressing && repoHealth != model.HealthFailed {
						repoHealth = model.HealthProgressing
					}
				}
			}

			shortRepo := repoName
			if idx := strings.LastIndex(shortRepo, "/"); idx >= 0 {
				shortRepo = shortRepo[idx+1:]
			}

			repoColor := healthColorHex(repoHealth)
			repoNode := tview.NewTreeNode(fmt.Sprintf("[%s]%s[-] %s", repoColor, repoHealth.Symbol(), shortRepo)).
				SetSelectable(true).
				SetExpanded(repoHealth.IsFailed() || repoHealth == model.HealthProgressing)
			orgNode.AddChild(repoNode)

			// Sort workflow names
			var wfNames []string
			for wf := range workflows {
				wfNames = append(wfNames, wf)
			}
			sort.Strings(wfNames)

			for _, wfName := range wfNames {
				wfRuns := workflows[wfName]

				// Sort runs by time (newest first)
				sort.Slice(wfRuns, func(i, j int) bool {
					return wfRuns[i].LastTransition.After(wfRuns[j].LastTransition)
				})

				// Workflow health from latest run
				wfHealth := model.HealthReady
				if len(wfRuns) > 0 {
					wfHealth = wfRuns[0].Health
				}

				wfColor := healthColorHex(wfHealth)
				wfNode := tview.NewTreeNode(fmt.Sprintf("[%s]%s[-] %s [#9696B4](%d)[-]",
					wfColor, wfHealth.Symbol(), wfName, len(wfRuns))).
					SetSelectable(true).
					SetExpanded(wfHealth.IsFailed() || wfHealth == model.HealthProgressing)

				for _, r := range wfRuns {
					color := healthColorHex(r.Health)
					age := render.FormatAge(r.LastTransition)

					label := fmt.Sprintf("[%s]%s[-] [#9696B4]#%d[-] %s [#9696B4]%s  %s[-]",
						color, r.Health.Symbol(), r.RunNumber, r.Branch, r.Actor, age)

					rCopy := r
					runNode := tview.NewTreeNode(label).
						SetSelectable(true).
						SetReference(&rCopy)
					wfNode.AddChild(runNode)
				}

				repoNode.AddChild(wfNode)
			}
		}
	}

	tree := tview.NewTreeView().
		SetRoot(root).
		SetCurrentNode(root)
	tree.SetBorderPadding(0, 0, 1, 1)

	return &GHATreeView{TreeView: tree}
}

// Refresh rebuilds the tree, preserving selection and expand state.
func (tv *GHATreeView) Refresh(resources []model.Resource) {
	// Save current node's path (index at each level)
	currentNode := tv.GetCurrentNode()
	var selectedPath []int
	if currentNode != nil {
		selectedPath = tv.findNodePath(tv.GetRoot(), currentNode)
	}

	// Save expand state by node text
	expandState := make(map[string]bool)
	tv.GetRoot().Walk(func(node, parent *tview.TreeNode) bool {
		if len(node.GetChildren()) > 0 {
			expandState[node.GetText()] = node.IsExpanded()
		}
		return true
	})

	fresh := NewGHATreeView(resources)
	newRoot := fresh.GetRoot()

	// Restore expand state
	newRoot.Walk(func(node, parent *tview.TreeNode) bool {
		if expanded, ok := expandState[node.GetText()]; ok {
			node.SetExpanded(expanded)
		}
		return true
	})

	tv.SetRoot(newRoot)

	// Restore selection by path
	if len(selectedPath) > 0 {
		node := tv.navigatePath(newRoot, selectedPath)
		if node != nil {
			tv.SetCurrentNode(node)
		}
	}
}

func (tv *GHATreeView) findNodePath(root, target *tview.TreeNode) []int {
	if root == target {
		return []int{}
	}
	for i, child := range root.GetChildren() {
		if child == target {
			return []int{i}
		}
		if path := tv.findNodePath(child, target); path != nil {
			return append([]int{i}, path...)
		}
	}
	return nil
}

func (tv *GHATreeView) navigatePath(root *tview.TreeNode, path []int) *tview.TreeNode {
	node := root
	for _, idx := range path {
		children := node.GetChildren()
		if idx >= len(children) {
			// Clamp to last child
			if len(children) == 0 {
				return node
			}
			return children[len(children)-1]
		}
		node = children[idx]
	}
	return node
}
