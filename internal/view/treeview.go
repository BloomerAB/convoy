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
	// Build repo→latest GHA run lookup
	ghaByRepo := make(map[string]model.Resource) // repo → most recent run
	for _, r := range resources {
		if r.Kind == model.KindWorkflowRun {
			if existing, ok := ghaByRepo[r.Repo]; !ok || r.LastTransition.After(existing.LastTransition) {
				ghaByRepo[r.Repo] = r
			}
			continue
		}
		if r.Cluster == cluster {
			clusterRes = append(clusterRes, r)
		}
	}

	var sources []model.Resource
	var workloads []model.Resource
	var deployments []model.Resource
	for _, r := range clusterRes {
		switch {
		case r.Kind == model.KindGitRepository || r.Kind == model.KindHelmRepository:
			sources = append(sources, r)
		case r.Kind == model.KindDeployment:
			deployments = append(deployments, r)
		default:
			workloads = append(workloads, r)
		}
	}

	// Build parent→children map
	childrenOf := make(map[string][]model.Resource)
	hasParent := make(map[string]bool)

	managedByToKey := func(managedBy string) string {
		return string(model.KindKustomization) + "/" + managedBy
	}

	// Pass 1: Kustomizations → under their sourceRef GitRepo
	for _, r := range workloads {
		rk := resKey(r)
		if r.Kind == model.KindKustomization && r.SourceRef != "" {
			childrenOf["source:"+r.SourceRef] = append(childrenOf["source:"+r.SourceRef], r)
			hasParent[rk] = true
		}
	}

	// Pass 2: HelmRepos → under their managing Kustomization (ManagedBy label)
	for _, r := range sources {
		rk := resKey(r)
		if !hasParent[rk] && r.ManagedBy != "" {
			// Skip bootstrap GitRepo (managed by Kustomization with same ns/name)
			if r.Kind == model.KindGitRepository && r.ManagedBy == r.Namespace+"/"+r.Name {
				continue
			}
			pk := managedByToKey(r.ManagedBy)
			childrenOf[pk] = append(childrenOf[pk], r)
			hasParent[rk] = true
		}
	}

	// Pass 3: HelmReleases → under their HelmRepo (sourceRef)
	for _, r := range workloads {
		rk := resKey(r)
		if !hasParent[rk] && r.Kind == model.KindHelmRelease && r.SourceRef != "" {
			childrenOf[r.SourceRef] = append(childrenOf[r.SourceRef], r)
			hasParent[rk] = true
		}
	}

	// Pass 4: Remaining workloads with ManagedBy (shouldn't be many)
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

	// Pass 5: HelmReleases with dependsOn but still no parent → under their dep
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

	// Pass 6: Deployments under their Kustomization or HelmRelease (ManagedBy)
	for _, r := range deployments {
		rk := resKey(r)
		if r.ManagedBy == "" {
			continue
		}
		if strings.HasPrefix(r.ManagedBy, "hr:") {
			// HelmRelease-managed: "hr:namespace/name"
			hrRef := r.ManagedBy[3:]
			pk := string(model.KindHelmRelease) + "/" + hrRef
			childrenOf[pk] = append(childrenOf[pk], r)
		} else {
			// Kustomization-managed
			pk := managedByToKey(r.ManagedBy)
			childrenOf[pk] = append(childrenOf[pk], r)
		}
		hasParent[rk] = true
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

	// Only root GitRepos (no parent) as top-level
	var gitRepos []model.Resource
	for _, r := range sources {
		if r.Kind == model.KindGitRepository && !hasParent[resKey(r)] {
			gitRepos = append(gitRepos, r)
		}
	}
	sort.Slice(gitRepos, func(i, j int) bool {
		return gitRepos[i].Name < gitRepos[j].Name
	})

	for _, src := range gitRepos {
		rk := resKey(src)
		added[rk] = true

		srcNode := makeNode(src, "", ghaByRepo)
		root.AddChild(srcNode)

		sourceKey := "source:" + string(src.Kind) + "/" + src.Namespace + "/" + src.Name
		addSortedChildren(srcNode, childrenOf[sourceKey], childrenOf, added, depsLabel, ghaByRepo)
	}

	// Orphans
	allRes := append(sources, workloads...)
	for _, r := range allRes {
		rk := resKey(r)
		if !added[rk] {
			added[rk] = true
			node := makeNode(r, depsLabel[rk], ghaByRepo)
			addSortedChildren(node, childrenOf[rk], childrenOf, added, depsLabel, ghaByRepo)
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

func addSortedChildren(parentNode *tview.TreeNode, children []model.Resource, childrenOf map[string][]model.Resource, added map[string]bool, depsLabel map[string]string, ghaByRepo map[string]model.Resource) {
	sort.Slice(children, func(i, j int) bool {
		return children[i].Name < children[j].Name
	})

	for _, child := range children {
		rk := resKey(child)
		if added[rk] {
			continue
		}
		added[rk] = true
		childNode := makeNode(child, depsLabel[rk], ghaByRepo)
		// Collect children from both direct key and source: prefixed key
		var grandchildren []model.Resource
		grandchildren = append(grandchildren, childrenOf[rk]...)
		grandchildren = append(grandchildren, childrenOf["source:"+rk]...)
		addSortedChildren(childNode, grandchildren, childrenOf, added, depsLabel, ghaByRepo)
		parentNode.AddChild(childNode)
	}
}

func makeNode(r model.Resource, deps string, ghaByRepo map[string]model.Resource) *tview.TreeNode {
	color := healthColorHex(r.Health)

	var label string
	if r.Kind == model.KindDeployment {
		// Show just the image tag
		tag := ""
		if len(r.Images) > 0 {
			img := r.Images[0]
			if idx := strings.LastIndex(img, ":"); idx >= 0 {
				tag = img[idx+1:]
				// Strip sha256 digest
				if didx := strings.Index(tag, "@"); didx >= 0 {
					tag = tag[:didx]
				}
			}
		}
		label = fmt.Sprintf("[%s]%s[-] %s [#9696B4]%s[-]",
			color, r.Health.Symbol(), r.Name, tag)

		// Show pipeline status if source repo is known
		if r.Repo != "" {
			if gha, ok := ghaByRepo[r.Repo]; ok {
				pColor := healthColorHex(gha.Health)
				repoShort := r.Repo
				if idx := strings.LastIndex(repoShort, "/"); idx >= 0 {
					repoShort = repoShort[idx+1:]
				}
				label += fmt.Sprintf(" [%s]%s %s[-]", pColor, gha.Health.Symbol(), repoShort)
			} else {
				repoShort := r.Repo
				if idx := strings.LastIndex(repoShort, "/"); idx >= 0 {
					repoShort = repoShort[idx+1:]
				}
				label += fmt.Sprintf(" [#9696B4]%s[-]", repoShort)
			}
		}
	} else {
		label = fmt.Sprintf("[%s]%s[-] %s [#9696B4](%s)[-]",
			color, r.Health.Symbol(), r.Name, kindLabel(r.Kind))
	}

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

// shortImage returns a compact image reference: "repo:tag" or "org/repo:tag"
func shortImage(img string) string {
	// Remove common registry prefixes
	for _, prefix := range []string{"ghcr.io/", "docker.io/", "registry.k8s.io/", "quay.io/", "public.ecr.aws/"} {
		if strings.HasPrefix(img, prefix) {
			img = img[len(prefix):]
			break
		}
	}
	// Remove sha256 digest if tag is also present
	if idx := strings.Index(img, "@sha256:"); idx > 0 {
		img = img[:idx]
	}
	return img
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
	case model.KindDeployment:
		return "deploy"
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
