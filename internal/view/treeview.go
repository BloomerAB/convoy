package view

import (
	"fmt"
	"sort"
	"strings"

	"github.com/bloomerab/convoy/internal/model"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// TreeView shows the Flux dependency tree for a cluster.
type TreeView struct {
	*tview.TreeView
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

func NewFluxTreeView(resources []model.Resource, cluster string) *TreeView {
	var clusterRes []model.Resource
	for _, r := range resources {
		if r.Cluster == cluster && r.Kind != model.KindWorkflowRun {
			clusterRes = append(clusterRes, r)
		}
	}

	// Separate by type
	var sources []model.Resource
	var workloads []model.Resource
	for _, r := range clusterRes {
		if r.Kind == model.KindGitRepository || r.Kind == model.KindHelmRepository {
			sources = append(sources, r)
		} else {
			workloads = append(workloads, r)
		}
	}

	// Lookup for dependsOn resolution
	byName := make(map[string][]model.Resource)
	byNsName := make(map[string]model.Resource)
	for _, r := range workloads {
		byNsName[r.Namespace+"/"+r.Name] = r
		byName[r.Name] = append(byName[r.Name], r)
	}

	resolveDep := func(dep string, fromNs string) string {
		if _, ok := byNsName[dep]; ok {
			return dep
		}
		if _, ok := byNsName[fromNs+"/"+dep]; ok {
			return fromNs + "/" + dep
		}
		name := dep
		if idx := strings.Index(dep, "/"); idx >= 0 {
			name = dep[idx+1:]
		}
		if matches := byName[name]; len(matches) == 1 {
			return matches[0].Namespace + "/" + matches[0].Name
		}
		return ""
	}

	// Build parent→children based on:
	// 1. dependsOn (explicit Flux dependency)
	// 2. ManagedBy (Kustomization that created this resource)
	childrenOf := make(map[string][]model.Resource)
	hasParent := make(map[string]bool)

	// Pass 1: dependsOn (highest priority)
	for _, r := range workloads {
		for _, dep := range r.DependsOn {
			parentKey := resolveDep(dep, r.Namespace)
			if parentKey != "" {
				childrenOf[parentKey] = append(childrenOf[parentKey], r)
				hasParent[r.Namespace+"/"+r.Name] = true
			}
		}
	}

	// Pass 2: ManagedBy — HelmRepos and HelmReleases under their Kustomization
	for _, r := range sources {
		key := r.Namespace + "/" + r.Name
		if !hasParent[key] && r.ManagedBy != "" {
			childrenOf[r.ManagedBy] = append(childrenOf[r.ManagedBy], r)
			hasParent[key] = true
		}
	}
	for _, r := range workloads {
		key := r.Namespace + "/" + r.Name
		if !hasParent[key] && r.ManagedBy != "" {
			childrenOf[r.ManagedBy] = append(childrenOf[r.ManagedBy], r)
			hasParent[key] = true
		}
	}

	// Pass 3: Kustomizations with no parent → place under their sourceRef GitRepo
	for _, r := range workloads {
		key := r.Namespace + "/" + r.Name
		if !hasParent[key] && r.Kind == model.KindKustomization && r.SourceRef != "" {
			childrenOf["source:"+r.SourceRef] = append(childrenOf["source:"+r.SourceRef], r)
			hasParent[key] = true
		}
	}

	// Root
	root := tview.NewTreeNode(fmt.Sprintf("[#FFFFFF::b]%s[-::-]", cluster)).
		SetSelectable(false)

	added := make(map[string]bool)

	// GitRepositories as top-level roots
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
		srcKey := src.Namespace + "/" + src.Name
		added[srcKey] = true

		srcNode := makeNode(src)
		root.AddChild(srcNode)

		// Children linked via sourceRef
		sourceKey := "source:" + string(src.Kind) + "/" + src.Namespace + "/" + src.Name
		addSortedChildren(srcNode, childrenOf[sourceKey], childrenOf, added)
	}

	// Remaining orphans
	allRes := append(sources, workloads...)
	for _, r := range allRes {
		key := r.Namespace + "/" + r.Name
		if !added[key] {
			added[key] = true
			node := makeNode(r)
			addSortedChildren(node, childrenOf[key], childrenOf, added)
			root.AddChild(node)
		}
	}

	tree := tview.NewTreeView().
		SetRoot(root).
		SetCurrentNode(root)
	tree.SetBorder(true).
		SetTitle(fmt.Sprintf(" Flux Tree: %s ", cluster)).
		SetBorderColor(tcell.ColorCornflowerBlue)

	return &TreeView{TreeView: tree}
}

func addSortedChildren(parentNode *tview.TreeNode, children []model.Resource, childrenOf map[string][]model.Resource, added map[string]bool) {
	sort.Slice(children, func(i, j int) bool {
		// Sources before workloads, then alphabetical
		iSrc := children[i].Kind == model.KindGitRepository || children[i].Kind == model.KindHelmRepository
		jSrc := children[j].Kind == model.KindGitRepository || children[j].Kind == model.KindHelmRepository
		if iSrc != jSrc {
			return !iSrc // workloads (ks/hr) first, then sources
		}
		return children[i].Name < children[j].Name
	})

	for _, child := range children {
		childKey := child.Namespace + "/" + child.Name
		if added[childKey] {
			continue
		}
		added[childKey] = true
		childNode := makeNode(child)
		addSortedChildren(childNode, childrenOf[childKey], childrenOf, added)
		parentNode.AddChild(childNode)
	}
}

func makeNode(r model.Resource) *tview.TreeNode {
	color := healthColorHex(r.Health)
	label := fmt.Sprintf("[%s]%s[-] %s [#9696B4](%s)[-]",
		color, r.Health.Symbol(), r.Name, kindLabel(r.Kind))
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
		return "hr"
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
