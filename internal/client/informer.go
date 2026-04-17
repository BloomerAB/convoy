package client

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/bloomerab/convoy/internal/model"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
)

var (
	KustomizationGVR = schema.GroupVersionResource{
		Group:    "kustomize.toolkit.fluxcd.io",
		Version:  "v1",
		Resource: "kustomizations",
	}
	HelmReleaseGVR = schema.GroupVersionResource{
		Group:    "helm.toolkit.fluxcd.io",
		Version:  "v2",
		Resource: "helmreleases",
	}
	GitRepositoryGVR = schema.GroupVersionResource{
		Group:    "source.toolkit.fluxcd.io",
		Version:  "v1",
		Resource: "gitrepositories",
	}
	HelmRepositoryGVR = schema.GroupVersionResource{
		Group:    "source.toolkit.fluxcd.io",
		Version:  "v1",
		Resource: "helmrepositories",
	}
)

// FluxWatcher watches Flux CRDs on a single cluster via list+watch.
type FluxWatcher struct {
	client      *ClusterClient
	gvr         schema.GroupVersionResource
	kind        model.ResourceKind
	mu          sync.RWMutex
	resources   map[string]model.Resource
	onChange    func()
}

// NewFluxWatcher creates a watcher for a specific Flux CRD on a cluster.
func NewFluxWatcher(client *ClusterClient, gvr schema.GroupVersionResource, kind model.ResourceKind, onChange func()) *FluxWatcher {
	return &FluxWatcher{
		client:    client,
		gvr:       gvr,
		kind:      kind,
		resources: make(map[string]model.Resource),
		onChange:  onChange,
	}
}

// Start begins list+watch. Blocks until ctx is cancelled. Reconnects on error.
func (w *FluxWatcher) Start(ctx context.Context) error {
	for {
		if err := w.listAndWatch(ctx); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			// Reconnect after brief pause
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(5 * time.Second):
			}
		}
	}
}

func (w *FluxWatcher) listAndWatch(ctx context.Context) error {
	client := w.client.Dynamic.Resource(w.gvr)

	list, err := client.List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("list %s on %s: %w", w.gvr.Resource, w.client.Name, err)
	}

	w.mu.Lock()
	w.resources = make(map[string]model.Resource, len(list.Items))
	for _, item := range list.Items {
		r := w.unstructuredToResource(item)
		w.resources[r.Namespace+"/"+r.Name] = r
	}
	w.mu.Unlock()
	w.onChange()

	watcher, err := client.Watch(ctx, metav1.ListOptions{
		ResourceVersion: list.GetResourceVersion(),
	})
	if err != nil {
		return fmt.Errorf("watch %s on %s: %w", w.gvr.Resource, w.client.Name, err)
	}
	defer watcher.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event, ok := <-watcher.ResultChan():
			if !ok {
				return fmt.Errorf("watch channel closed")
			}
			w.handleEvent(event)
		}
	}
}

func (w *FluxWatcher) handleEvent(event watch.Event) {
	obj, ok := event.Object.(*unstructured.Unstructured)
	if !ok {
		return
	}

	key := obj.GetNamespace() + "/" + obj.GetName()

	w.mu.Lock()
	switch event.Type {
	case watch.Added, watch.Modified:
		w.resources[key] = w.unstructuredToResource(*obj)
	case watch.Deleted:
		delete(w.resources, key)
	}
	w.mu.Unlock()
	w.onChange()
}

func (w *FluxWatcher) unstructuredToResource(obj unstructured.Unstructured) model.Resource {
	r := model.Resource{
		Cluster:     w.client.Name,
		Environment: w.client.Environment,
		Kind:        w.kind,
		Namespace:   obj.GetNamespace(),
		Name:        obj.GetName(),
		Health:      model.HealthUnknown,
	}

	conditions, found, _ := unstructured.NestedSlice(obj.Object, "status", "conditions")
	if found {
		r.Health, r.Message = extractHealth(conditions)
	}

	// Check if suspended
	suspended, _, _ := unstructured.NestedBool(obj.Object, "spec", "suspend")
	if suspended {
		r.Health = model.HealthSuspended
		if r.Message == "" {
			r.Message = "Suspended"
		}
	}

	// If still unknown, infer from context
	if r.Health == model.HealthUnknown {
		// OCI HelmRepositories have no conditions/artifacts — they're just pointers
		specType, _, _ := unstructured.NestedString(obj.Object, "spec", "type")
		if specType == "oci" {
			r.Health = model.HealthReady
		}

		// Has artifact = successfully fetched
		_, hasArtifact, _ := unstructured.NestedMap(obj.Object, "status", "artifact")
		if hasArtifact {
			r.Health = model.HealthReady
		}
	}

	r.Revision = extractRevision(obj, w.kind)
	r.LastTransition = extractLastTransition(conditions)
	r.Interval = extractInterval(obj)
	r.NextRun = calculateNextRun(obj, r.Interval)
	r.SourceRef = extractSourceRef(obj, w.kind)
	r.DependsOn = extractDependsOn(obj)
	r.ManagedBy = extractManagedBy(obj)

	return r
}

func extractHealth(conditions []interface{}) (model.HealthStatus, string) {
	// Parse all conditions into a map for lookup
	condMap := make(map[string]map[string]string)
	for _, c := range conditions {
		cond, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		condType, _ := cond["type"].(string)
		if condType == "" {
			continue
		}
		entry := make(map[string]string)
		for k, v := range cond {
			if s, ok := v.(string); ok {
				entry[k] = s
			}
		}
		condMap[condType] = entry
	}

	// Check Ready first (primary condition)
	if ready, ok := condMap["Ready"]; ok {
		switch ready["status"] {
		case "True":
			return model.HealthReady, ready["message"]
		case "False":
			reason := ready["reason"]
			if reason == "Progressing" || reason == "ArtifactOutdated" {
				return model.HealthProgressing, ready["message"]
			}
			return model.HealthFailed, ready["message"]
		default:
			return model.HealthProgressing, ready["message"]
		}
	}

	// No Ready condition — check Stalled
	if stalled, ok := condMap["Stalled"]; ok && stalled["status"] == "True" {
		return model.HealthFailed, stalled["message"]
	}

	// Check Reconciling
	if reconciling, ok := condMap["Reconciling"]; ok && reconciling["status"] == "True" {
		return model.HealthProgressing, reconciling["message"]
	}

	// Check Healthy/HealthDegraded (HelmRelease-specific)
	if degraded, ok := condMap["HealthDegraded"]; ok && degraded["status"] == "True" {
		return model.HealthFailed, degraded["message"]
	}
	if healthy, ok := condMap["Healthy"]; ok && healthy["status"] == "True" {
		return model.HealthReady, healthy["message"]
	}

	// If suspended
	if len(condMap) == 0 {
		return model.HealthUnknown, ""
	}

	return model.HealthUnknown, ""
}

func extractRevision(obj unstructured.Unstructured, kind model.ResourceKind) string {
	switch kind {
	case model.KindKustomization:
		rev, _, _ := unstructured.NestedString(obj.Object, "status", "lastAppliedRevision")
		return rev
	case model.KindHelmRelease:
		rev, _, _ := unstructured.NestedString(obj.Object, "status", "lastAppliedRevision")
		if rev == "" {
			rev, _, _ = unstructured.NestedString(obj.Object, "status", "lastAttemptedRevision")
		}
		return rev
	case model.KindGitRepository, model.KindHelmRepository:
		artifact, found, _ := unstructured.NestedMap(obj.Object, "status", "artifact")
		if found {
			rev, _ := artifact["revision"].(string)
			return rev
		}
		return ""
	default:
		return ""
	}
}

func extractLastTransition(conditions []interface{}) time.Time {
	for _, c := range conditions {
		cond, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		condType, _ := cond["type"].(string)
		if condType != "Ready" {
			continue
		}
		ts, _ := cond["lastTransitionTime"].(string)
		if t, err := time.Parse(time.RFC3339, ts); err == nil {
			return t
		}
	}
	return time.Time{}
}

func extractSourceRef(obj unstructured.Unstructured, kind model.ResourceKind) string {
	ns := obj.GetNamespace()

	switch kind {
	case model.KindKustomization:
		// spec.sourceRef.kind, spec.sourceRef.name, spec.sourceRef.namespace
		refKind, _, _ := unstructured.NestedString(obj.Object, "spec", "sourceRef", "kind")
		refName, _, _ := unstructured.NestedString(obj.Object, "spec", "sourceRef", "name")
		refNs, _, _ := unstructured.NestedString(obj.Object, "spec", "sourceRef", "namespace")
		if refName == "" {
			return ""
		}
		if refNs == "" {
			refNs = ns
		}
		if refKind == "" {
			refKind = "GitRepository"
		}
		return refKind + "/" + refNs + "/" + refName

	case model.KindHelmRelease:
		// spec.chart.spec.sourceRef or spec.chartRef
		refKind, _, _ := unstructured.NestedString(obj.Object, "spec", "chart", "spec", "sourceRef", "kind")
		refName, _, _ := unstructured.NestedString(obj.Object, "spec", "chart", "spec", "sourceRef", "name")
		refNs, _, _ := unstructured.NestedString(obj.Object, "spec", "chart", "spec", "sourceRef", "namespace")
		if refName == "" {
			// Try chartRef (Flux v2 alternative)
			refKind, _, _ = unstructured.NestedString(obj.Object, "spec", "chartRef", "kind")
			refName, _, _ = unstructured.NestedString(obj.Object, "spec", "chartRef", "name")
			refNs, _, _ = unstructured.NestedString(obj.Object, "spec", "chartRef", "namespace")
		}
		if refName == "" {
			return ""
		}
		if refNs == "" {
			refNs = ns
		}
		if refKind == "" {
			refKind = "HelmRepository"
		}
		return refKind + "/" + refNs + "/" + refName
	}

	return ""
}

func extractDependsOn(obj unstructured.Unstructured) []string {
	deps, found, _ := unstructured.NestedSlice(obj.Object, "spec", "dependsOn")
	if !found {
		return nil
	}

	ns := obj.GetNamespace()
	var result []string
	for _, d := range deps {
		dep, ok := d.(map[string]interface{})
		if !ok {
			continue
		}
		name, _ := dep["name"].(string)
		depNs, _ := dep["namespace"].(string)
		if name == "" {
			continue
		}
		if depNs == "" {
			depNs = ns
		}
		result = append(result, depNs+"/"+name)
	}
	return result
}

func extractManagedBy(obj unstructured.Unstructured) string {
	labels := obj.GetLabels()
	name := labels["kustomize.toolkit.fluxcd.io/name"]
	ns := labels["kustomize.toolkit.fluxcd.io/namespace"]
	if name != "" && ns != "" {
		return ns + "/" + name
	}
	return ""
}

func extractInterval(obj unstructured.Unstructured) time.Duration {
	raw, found, _ := unstructured.NestedString(obj.Object, "spec", "interval")
	if !found || raw == "" {
		return 0
	}
	// Flux uses Go duration format (e.g. "5m", "1h", "30s")
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0
	}
	return d
}

func calculateNextRun(obj unstructured.Unstructured, interval time.Duration) time.Time {
	if interval == 0 {
		return time.Time{}
	}

	// Use lastHandledReconcileAt if available, otherwise lastTransitionTime
	lastReconcile, found, _ := unstructured.NestedString(obj.Object, "status", "lastHandledReconcileAt")
	if found && lastReconcile != "" {
		if t, err := time.Parse(time.RFC3339, lastReconcile); err == nil {
			return t.Add(interval)
		}
	}

	return time.Time{}
}

// Resources returns the current cached resources.
func (w *FluxWatcher) Resources() []model.Resource {
	w.mu.RLock()
	defer w.mu.RUnlock()
	result := make([]model.Resource, 0, len(w.resources))
	for _, r := range w.resources {
		result = append(result, r)
	}
	return result
}

// resourceClient returns the appropriate dynamic resource client for all namespaces.
func resourceClient(dynClient dynamic.Interface, gvr schema.GroupVersionResource) dynamic.ResourceInterface {
	return dynClient.Resource(gvr).Namespace("")
}
