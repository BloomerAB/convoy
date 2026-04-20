package view

import (
	"fmt"
	"sort"
	"strings"

	"github.com/bloomerab/convoy/internal/model"
	"github.com/rivo/tview"
)

// TreeView shows the Flux dependency tree for a cluster.
type TreeView struct {
	*tview.TreeView
	cluster string
}

// SelectedResource returns the resource for the currently selected tree node.
func (tv *TreeView) SelectedResource() *model.Resource {
	node := tv.GetCurrentNode()
	if node == nil {
		return nil
	}
	if r, ok := node.GetReference().(*model.Resource); ok {
		return r
	}
	return nil
}

// resKey returns a unique key for a resource: "Kind/namespace/name"
func resKey(r model.Resource) string {
	return string(r.Kind) + "/" + r.Namespace + "/" + r.Name
}

func NewFluxTreeView(resources []model.Resource, cluster string) *TreeView {
	var clusterRes []model.Resource
	for _, r := range resources {
		if r.Cluster == cluster && r.Kind != model.KindWorkflowRun {
			clusterRes = append(clusterRes, r)
		}
	}

	var sources []model.Resource
	var workloads []model.Resource
	for _, r := range clusterRes {
		if r.Kind == model.KindGitRepository || r.Kind == model.KindHelmRepository {
			sources = append(sources, r)
		} else {
			workloads = append(workloads, r)
		}
	}

	// Build parent→children map
	childrenOf := make(map[string][]model.Resource)
	hasParent := make(map[string]bool)

	managedByToKey := func(managedBy string) string {
		return string(model.KindKustomization) + "/" + managedBy
	}

	// Pass 1: Kustomizations → under their sourceRef GitRepo (primary ownership)
	for _, r := range workloads {
		rk := resKey(r)
		if r.Kind == model.KindKustomization && r.SourceRef != "" {
			childrenOf["source:"+r.SourceRef] = append(childrenOf["source:"+r.SourceRef], r)
			hasParent[rk] = true
		}
	}

	// Pass 2: ManagedBy label — HelmRepos under their Kustomization
	// GitRepos are always top-level roots, skip them here
	for _, r := range sources {
		rk := resKey(r)
		if r.Kind == model.KindGitRepository {
			continue
		}
		if !hasParent[rk] && r.ManagedBy != "" {
			pk := managedByToKey(r.ManagedBy)
			childrenOf[pk] = append(childrenOf[pk], r)
			hasParent[rk] = true
		}
	}
	for _, r := range workloads {
		rk := resKey(r)
		if !hasParent[rk] && r.ManagedBy != "" {
			pk := managedByToKey(r.ManagedBy)
			if pk == rk {
				continue
			}
			childrenOf[pk] = append(childrenOf[pk], r)
			hasParent[rk] = true
		}
	}

	// Pass 3: HelmReleases with dependsOn but no ManagedBy → under their dep
	byName := make(map[string][]model.Resource)
	for _, r := range workloads {
		byName[r.Name] = append(byName[r.Name], r)
	}

	resolveDep := func(dep string, fromNs string) string {
		name := dep
		if idx := strings.Index(dep, "/"); idx >= 0 {
			name = dep[idx+1:]
		}
		if matches := byName[name]; len(matches) == 1 {
			return resKey(matches[0])
		}
		for _, m := range byName[name] {
			if m.Namespace == fromNs {
				return resKey(m)
			}
		}
		return ""
	}

	for _, r := range workloads {
		rk := resKey(r)
		if !hasParent[rk] && len(r.DependsOn) > 0 {
			// Place under first resolvable dep
			for _, dep := range r.DependsOn {
				pk := resolveDep(dep, r.Namespace)
				if pk != "" {
					childrenOf[pk] = append(childrenOf[pk], r)
					hasParent[rk] = true
					break
				}
			}
		}
	}

	// Build dependsOn labels for display
	depsLabel := make(map[string]string)
	for _, r := range workloads {
		if len(r.DependsOn) > 0 {
			depsLabel[resKey(r)] = strings.Join(r.DependsOn, ", ")
		}
	}

	// Root
	root := tview.NewTreeNode(fmt.Sprintf("[#FFFFFF::b]%s[-::-]", cluster)).
		SetSelectable(false)

	added := make(map[string]bool)

	// GitRepositories as top-level
	var gitRepos []model.Resource
	for _, r := range sources {
		if r.Kind == model.KindGitRepository {
			gitRepos = append(gitRepos, r)
		}
	}
	sort.Slice(gitRepos, func(i, j int) bool {
		return gitRepos[i].Name < gitRepos[j].Name
	})

	for _, src := range gitRepos {
		rk := resKey(src)
		added[rk] = true

		srcNode := makeNode(src, "")
		root.AddChild(srcNode)

		sourceKey := "source:" + string(src.Kind) + "/" + src.Namespace + "/" + src.Name
		addSortedChildren(srcNode, childrenOf[sourceKey], childrenOf, added, depsLabel)
	}

	// Orphans
	allRes := append(sources, workloads...)
	for _, r := range allRes {
		rk := resKey(r)
		if !added[rk] {
			added[rk] = true
			node := makeNode(r, depsLabel[rk])
			addSortedChildren(node, childrenOf[rk], childrenOf, added, depsLabel)
			root.AddChild(node)
		}
	}

	tree := tview.NewTreeView().
		SetRoot(root).
		SetCurrentNode(root)
	tree.SetBorderPadding(0, 0, 1, 1)

	return &TreeView{TreeView: tree, cluster: cluster}
}

// Refresh rebuilds the tree with fresh data, preserving selection.
func (tv *TreeView) Refresh(resources []model.Resource) {
	// Remember selected resource
	var selectedKey string
	if r := tv.SelectedResource(); r != nil {
		selectedKey = resKey(*r)
	}

	fresh := NewFluxTreeView(resources, tv.cluster)
	newRoot := fresh.GetRoot()
	tv.SetRoot(newRoot)

	// Restore selection
	if selectedKey != "" {
		tv.GetRoot().Walk(func(node, parent *tview.TreeNode) bool {
			if r, ok := node.GetReference().(*model.Resource); ok {
				if resKey(*r) == selectedKey {
					tv.SetCurrentNode(node)
					return false
				}
			}
			return true
		})
	}
}

func addSortedChildren(parentNode *tview.TreeNode, children []model.Resource, childrenOf map[string][]model.Resource, added map[string]bool, depsLabel map[string]string) {
	sort.Slice(children, func(i, j int) bool {
		return children[i].Name < children[j].Name
	})

	for _, child := range children {
		rk := resKey(child)
		if added[rk] {
			continue
		}
		added[rk] = true
		childNode := makeNode(child, depsLabel[rk])
		addSortedChildren(childNode, childrenOf[rk], childrenOf, added, depsLabel)
		parentNode.AddChild(childNode)
	}
}

func makeNode(r model.Resource, deps string) *tview.TreeNode {
	color := healthColorHex(r.Health)
	label := fmt.Sprintf("[%s]%s[-] %s [#9696B4](%s)[-]",
		color, r.Health.Symbol(), r.Name, kindLabel(r.Kind))

	if deps != "" {
		label += fmt.Sprintf(" [#6EB5FF]→ %s[-]", deps)
	}

	if r.Health.IsFailed() && r.Message != "" {
		msg := r.Message
		if len(msg) > 40 {
			msg = msg[:37] + "..."
		}
		label += fmt.Sprintf(" [#FF5050]%s[-]", msg)
	}
	rCopy := r
	node := tview.NewTreeNode(label).
		SetSelectable(true).
		SetExpanded(true).
		SetReference(&rCopy)
	return node
}

func kindLabel(k model.ResourceKind) string {
	switch k {
	case model.KindKustomization:
		return "ks"
	case model.KindHelmRelease:
		return "helmrelease"
	case model.KindGitRepository:
		return "git"
	case model.KindHelmRepository:
		return "helmrepo"
	default:
		return strings.ToLower(string(k))
	}
}

func healthColorHex(h model.HealthStatus) string {
	switch h {
	case model.HealthReady:
		return "#64FF64"
	case model.HealthFailed:
		return "#FF5050"
	case model.HealthProgressing:
		return "#FFFF64"
	case model.HealthSuspended:
		return "#9696B4"
	default:
		return "#FFFFFF"
	}
}
