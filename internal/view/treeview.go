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
	// Filter to this cluster's Flux resources
	var clusterRes []model.Resource
	for _, r := range resources {
		if r.Cluster == cluster && r.Kind != model.KindWorkflowRun {
			clusterRes = append(clusterRes, r)
		}
	}

	// Build lookup: "namespace/name" → resource (for all non-source kinds)
	// and "Kind/namespace/name" → resource (for sourceRef matching)
	byNsName := make(map[string]model.Resource)
	byFullRef := make(map[string]model.Resource)
	for _, r := range clusterRes {
		byNsName[r.Namespace+"/"+r.Name] = r
		byFullRef[string(r.Kind)+"/"+r.Namespace+"/"+r.Name] = r
	}

	// Build parent→children map based on dependsOn (cross-kind)
	// A resource is a child of each thing it dependsOn
	childrenOf := make(map[string][]model.Resource) // parent key → children
	hasParent := make(map[string]bool)               // child key → true if it has a dependsOn parent

	for _, r := range clusterRes {
		if r.Kind == model.KindGitRepository || r.Kind == model.KindHelmRepository {
			continue
		}
		for _, dep := range r.DependsOn {
			childrenOf[dep] = append(childrenOf[dep], r)
			hasParent[r.Namespace+"/"+r.Name] = true
		}
	}

	// Resources that reference a source but have no dependsOn → children of that source
	sourceChildren := make(map[string][]model.Resource) // sourceRef → children
	for _, r := range clusterRes {
		if r.Kind == model.KindGitRepository || r.Kind == model.KindHelmRepository {
			continue
		}
		key := r.Namespace + "/" + r.Name
		if !hasParent[key] && r.SourceRef != "" {
			sourceChildren[r.SourceRef] = append(sourceChildren[r.SourceRef], r)
			hasParent[key] = true
		}
	}

	// Root node
	root := tview.NewTreeNode(fmt.Sprintf("[#FFFFFF::b]%s[-::-]", cluster)).
		SetSelectable(false)

	// Collect sources
	var sources []model.Resource
	for _, r := range clusterRes {
		if r.Kind == model.KindGitRepository || r.Kind == model.KindHelmRepository {
			sources = append(sources, r)
		}
	}
	sort.Slice(sources, func(i, j int) bool {
		return sources[i].Name < sources[j].Name
	})

	added := make(map[string]bool)

	for _, src := range sources {
		srcRef := string(src.Kind) + "/" + src.Namespace + "/" + src.Name

		// Get root-level children of this source (no dependsOn)
		rootChildren := sourceChildren[srcRef]
		if len(rootChildren) == 0 {
			// Source with no direct children — still show it but collapsed
			srcNode := makeNode(src)
			srcNode.SetExpanded(false)
			root.AddChild(srcNode)
			added[src.Namespace+"/"+src.Name] = true
			continue
		}

		srcNode := makeNode(src)
		root.AddChild(srcNode)
		added[src.Namespace+"/"+src.Name] = true

		sort.Slice(rootChildren, func(i, j int) bool {
			return rootChildren[i].Name < rootChildren[j].Name
		})

		for _, child := range rootChildren {
			childKey := child.Namespace + "/" + child.Name
			if added[childKey] {
				continue
			}
			added[childKey] = true
			childNode := makeNode(child)
			buildSubTree(childNode, child, childrenOf, added)
			srcNode.AddChild(childNode)
		}
	}

	// Orphans: resources not yet added (no source, no parent)
	for _, r := range clusterRes {
		if r.Kind == model.KindGitRepository || r.Kind == model.KindHelmRepository {
			continue
		}
		key := r.Namespace + "/" + r.Name
		if !added[key] {
			added[key] = true
			node := makeNode(r)
			buildSubTree(node, r, childrenOf, added)
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

// buildSubTree recursively adds dependents of a resource as children.
func buildSubTree(parentNode *tview.TreeNode, parent model.Resource, childrenOf map[string][]model.Resource, added map[string]bool) {
	parentKey := parent.Namespace + "/" + parent.Name
	deps := childrenOf[parentKey]

	sort.Slice(deps, func(i, j int) bool {
		return deps[i].Name < deps[j].Name
	})

	for _, dep := range deps {
		depKey := dep.Namespace + "/" + dep.Name
		if added[depKey] {
			continue
		}
		added[depKey] = true
		depNode := makeNode(dep)
		buildSubTree(depNode, dep, childrenOf, added)
		parentNode.AddChild(depNode)
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
