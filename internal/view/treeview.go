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

// ksKey returns the Kustomization key format: "namespace/name"
func ksKey(r model.Resource) string {
	return r.Namespace + "/" + r.Name
}

func NewFluxTreeView(resources []model.Resource, cluster string) *TreeView {
	var clusterRes []model.Resource
	for _, r := range resources {
		if r.Cluster == cluster && r.Kind != model.KindWorkflowRun {
			clusterRes = append(clusterRes, r)
		}
	}

	// Separate by type
	var sources []model.Resource   // GitRepo, HelmRepo
	var workloads []model.Resource // Kustomization, HelmRelease
	for _, r := range clusterRes {
		if r.Kind == model.KindGitRepository || r.Kind == model.KindHelmRepository {
			sources = append(sources, r)
		} else {
			workloads = append(workloads, r)
		}
	}

	// Lookup for dependsOn resolution (by name, cross-namespace)
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
		// Try same namespace
		for _, m := range byName[name] {
			if m.Namespace == fromNs {
				return resKey(m)
			}
		}
		return ""
	}

	// Build parent→children map using full resKey
	childrenOf := make(map[string][]model.Resource)
	hasParent := make(map[string]bool)

	// Pass 1: dependsOn (explicit Flux dependency — highest priority)
	for _, r := range workloads {
		for _, dep := range r.DependsOn {
			parentKey := resolveDep(dep, r.Namespace)
			if parentKey != "" {
				childrenOf[parentKey] = append(childrenOf[parentKey], r)
				hasParent[resKey(r)] = true
			}
		}
	}

	// Pass 2: Kustomizations with no dependsOn parent → under their sourceRef GitRepo
	for _, r := range workloads {
		rk := resKey(r)
		if !hasParent[rk] && r.Kind == model.KindKustomization && r.SourceRef != "" {
			childrenOf["source:"+r.SourceRef] = append(childrenOf["source:"+r.SourceRef], r)
			hasParent[rk] = true
		}
	}

	// Pass 3: ManagedBy label — HelmRepos, HelmReleases, and remaining workloads
	// under their managing Kustomization. Skip self-references.
	managedByToKey := func(managedBy string) string {
		return string(model.KindKustomization) + "/" + managedBy
	}

	for _, r := range sources {
		rk := resKey(r)
		if !hasParent[rk] && r.ManagedBy != "" {
			pk := managedByToKey(r.ManagedBy)
			childrenOf[pk] = append(childrenOf[pk], r)
			hasParent[rk] = true
		}
	}
	for _, r := range workloads {
		rk := resKey(r)
		if !hasParent[rk] && r.ManagedBy != "" {
			// Skip self-management (e.g. flux-system ks manages itself)
			pk := managedByToKey(r.ManagedBy)
			if pk == rk {
				continue
			}
			childrenOf[pk] = append(childrenOf[pk], r)
			hasParent[rk] = true
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
		rk := resKey(src)
		added[rk] = true

		srcNode := makeNode(src)
		root.AddChild(srcNode)

		// Kustomizations linked via sourceRef
		sourceKey := "source:" + string(src.Kind) + "/" + src.Namespace + "/" + src.Name
		addSortedChildren(srcNode, childrenOf[sourceKey], childrenOf, added)
	}

	// Remaining orphans
	allRes := append(sources, workloads...)
	for _, r := range allRes {
		rk := resKey(r)
		if !added[rk] {
			added[rk] = true
			node := makeNode(r)
			addSortedChildren(node, childrenOf[resKey(r)], childrenOf, added)
			root.AddChild(node)
		}
	}

	tree := tview.NewTreeView().
		SetRoot(root).
		SetCurrentNode(root)
	tree.SetBorderPadding(0, 0, 1, 1)

	return &TreeView{TreeView: tree}
}

func addSortedChildren(parentNode *tview.TreeNode, children []model.Resource, childrenOf map[string][]model.Resource, added map[string]bool) {
	sort.Slice(children, func(i, j int) bool {
		return children[i].Name < children[j].Name
	})

	for _, child := range children {
		rk := resKey(child)
		if added[rk] {
			continue
		}
		added[rk] = true
		childNode := makeNode(child)
		// Recurse: this child's own children
		addSortedChildren(childNode, childrenOf[rk], childrenOf, added)
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
