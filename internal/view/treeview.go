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

func NewFluxTreeView(resources []model.Resource, cluster string) *TreeView {
	// Filter to this cluster's Flux resources
	var clusterRes []model.Resource
	for _, r := range resources {
		if r.Cluster == cluster && r.Kind != model.KindWorkflowRun {
			clusterRes = append(clusterRes, r)
		}
	}

	// Build lookup maps
	// key: "Kind/namespace/name" → resource
	byRef := make(map[string]model.Resource)
	// key: "namespace/name" → resource (for dependsOn lookups within same kind)
	ksByNsName := make(map[string]model.Resource)

	for _, r := range clusterRes {
		ref := string(r.Kind) + "/" + r.Namespace + "/" + r.Name
		byRef[ref] = r
		if r.Kind == model.KindKustomization {
			ksByNsName[r.Namespace+"/"+r.Name] = r
		}
	}

	// Find which resources are referenced (children) so we know the roots
	isChild := make(map[string]bool)
	for _, r := range clusterRes {
		if r.SourceRef != "" {
			isChild[r.SourceRef] = true
		}
		for _, dep := range r.DependsOn {
			isChild[string(r.Kind)+"/"+dep] = true
		}
	}

	// Root node
	root := tview.NewTreeNode(fmt.Sprintf("[#FFFFFF::b]%s[-::-]", cluster)).
		SetSelectable(false)

	// Sources (GitRepository, HelmRepository) are the tree roots
	var sources []model.Resource
	for _, r := range clusterRes {
		if r.Kind == model.KindGitRepository || r.Kind == model.KindHelmRepository {
			sources = append(sources, r)
		}
	}
	sort.Slice(sources, func(i, j int) bool {
		return sources[i].Name < sources[j].Name
	})

	for _, src := range sources {
		srcNode := makeNode(src)
		root.AddChild(srcNode)

		// Find Kustomizations/HelmReleases that reference this source
		srcRef := string(src.Kind) + "/" + src.Namespace + "/" + src.Name
		var children []model.Resource
		for _, r := range clusterRes {
			if r.SourceRef == srcRef {
				children = append(children, r)
			}
		}
		sort.Slice(children, func(i, j int) bool {
			return children[i].Name < children[j].Name
		})

		// Build dependency sub-trees for each child
		added := make(map[string]bool)
		for _, child := range children {
			if !hasDependencies(child) {
				childNode := makeNode(child)
				addDependents(childNode, child, clusterRes, added)
				srcNode.AddChild(childNode)
				added[child.Namespace+"/"+child.Name] = true
			}
		}
		// Add remaining children that have unresolved deps
		for _, child := range children {
			key := child.Namespace + "/" + child.Name
			if !added[key] {
				childNode := makeNode(child)
				addDependents(childNode, child, clusterRes, added)
				srcNode.AddChild(childNode)
				added[key] = true
			}
		}
	}

	// Add orphans (resources with no source ref and not a source themselves)
	for _, r := range clusterRes {
		if r.Kind == model.KindGitRepository || r.Kind == model.KindHelmRepository {
			continue
		}
		if r.SourceRef == "" {
			root.AddChild(makeNode(r))
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
	node := tview.NewTreeNode(label).
		SetSelectable(true).
		SetExpanded(true)
	return node
}

func addDependents(parentNode *tview.TreeNode, parent model.Resource, all []model.Resource, added map[string]bool) {
	// Find resources that dependOn this parent
	parentKey := parent.Namespace + "/" + parent.Name
	var deps []model.Resource
	for _, r := range all {
		if r.Kind != parent.Kind {
			continue
		}
		for _, d := range r.DependsOn {
			if d == parentKey {
				deps = append(deps, r)
				break
			}
		}
	}
	sort.Slice(deps, func(i, j int) bool {
		return deps[i].Name < deps[j].Name
	})
	for _, dep := range deps {
		key := dep.Namespace + "/" + dep.Name
		if added[key] {
			continue
		}
		added[key] = true
		depNode := makeNode(dep)
		addDependents(depNode, dep, all, added)
		parentNode.AddChild(depNode)
	}
}

func hasDependencies(r model.Resource) bool {
	return len(r.DependsOn) > 0
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
		return "helm"
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
