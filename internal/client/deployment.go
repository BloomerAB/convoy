package client

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/bloomerab/convoy/internal/model"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
)

var DeploymentGVR = schema.GroupVersionResource{
	Group:    "apps",
	Version:  "v1",
	Resource: "deployments",
}

// DeploymentWatcher watches Deployments on a single cluster.
type DeploymentWatcher struct {
	client    *ClusterClient
	mu        sync.RWMutex
	resources map[string]model.Resource
}

func NewDeploymentWatcher(client *ClusterClient) *DeploymentWatcher {
	return &DeploymentWatcher{
		client:    client,
		resources: make(map[string]model.Resource),
	}
}

func (w *DeploymentWatcher) Start(ctx context.Context) error {
	for {
		if err := w.listAndWatch(ctx); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(5 * time.Second):
			}
		}
	}
}

func (w *DeploymentWatcher) listAndWatch(ctx context.Context) error {
	client := w.client.Dynamic.Resource(DeploymentGVR)

	list, err := client.List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("list deployments on %s: %w", w.client.Name, err)
	}

	w.mu.Lock()
	w.resources = make(map[string]model.Resource, len(list.Items))
	for _, item := range list.Items {
		r := w.toResource(item)
		w.resources[r.Namespace+"/"+r.Name] = r
	}
	w.mu.Unlock()

	watcher, err := client.Watch(ctx, metav1.ListOptions{
		ResourceVersion: list.GetResourceVersion(),
	})
	if err != nil {
		return fmt.Errorf("watch deployments on %s: %w", w.client.Name, err)
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

func (w *DeploymentWatcher) handleEvent(event watch.Event) {
	obj, ok := event.Object.(*unstructured.Unstructured)
	if !ok {
		return
	}
	key := obj.GetNamespace() + "/" + obj.GetName()

	w.mu.Lock()
	switch event.Type {
	case watch.Added, watch.Modified:
		w.resources[key] = w.toResource(*obj)
	case watch.Deleted:
		delete(w.resources, key)
	}
	w.mu.Unlock()
}

func (w *DeploymentWatcher) toResource(obj unstructured.Unstructured) model.Resource {
	r := model.Resource{
		Cluster:     w.client.Name,
		Environment: w.client.Environment,
		Kind:        model.KindDeployment,
		Namespace:   obj.GetNamespace(),
		Name:        obj.GetName(),
		Health:      model.HealthReady,
	}

	// Extract images
	containers, found, _ := unstructured.NestedSlice(obj.Object, "spec", "template", "spec", "containers")
	if found {
		for _, c := range containers {
			cm, ok := c.(map[string]interface{})
			if !ok {
				continue
			}
			if img, ok := cm["image"].(string); ok {
				r.Images = append(r.Images, img)
			}
		}
	}

	// Health from status
	desired, _, _ := unstructured.NestedInt64(obj.Object, "spec", "replicas")
	ready, _, _ := unstructured.NestedInt64(obj.Object, "status", "readyReplicas")
	updated, _, _ := unstructured.NestedInt64(obj.Object, "status", "updatedReplicas")

	if desired > 0 && ready == 0 {
		r.Health = model.HealthFailed
		r.Message = fmt.Sprintf("0/%d ready", desired)
	} else if ready < desired {
		r.Health = model.HealthProgressing
		r.Message = fmt.Sprintf("%d/%d ready", ready, desired)
	} else if updated < desired {
		r.Health = model.HealthProgressing
		r.Message = fmt.Sprintf("%d/%d updated", updated, desired)
	}

	// Source repo from annotation only
	annotations := obj.GetAnnotations()
	if repo := annotations["doktor.se/source-repo"]; repo != "" {
		r.Repo = normalizeGitHubRepo(repo)
	}

	// ManagedBy — check both kustomize and helm labels
	labels := obj.GetLabels()
	if ksName := labels["kustomize.toolkit.fluxcd.io/name"]; ksName != "" {
		ksNs := labels["kustomize.toolkit.fluxcd.io/namespace"]
		if ksNs == "" {
			ksNs = obj.GetNamespace()
		}
		r.ManagedBy = ksNs + "/" + ksName
	} else if hrName := labels["helm.toolkit.fluxcd.io/name"]; hrName != "" {
		hrNs := labels["helm.toolkit.fluxcd.io/namespace"]
		if hrNs == "" {
			hrNs = obj.GetNamespace()
		}
		// Store as HelmRelease ref
		r.ManagedBy = "hr:" + hrNs + "/" + hrName
	}

	return r
}

func normalizeGitHubRepo(url string) string {
	url = strings.TrimSuffix(url, ".git")
	url = strings.TrimSuffix(url, "/")
	// "https://github.com/org/repo" → "org/repo"
	if idx := strings.Index(url, "github.com/"); idx >= 0 {
		return url[idx+len("github.com/"):]
	}
	return url
}

func (w *DeploymentWatcher) Resources() []model.Resource {
	w.mu.RLock()
	defer w.mu.RUnlock()
	result := make([]model.Resource, 0, len(w.resources))
	for _, r := range w.resources {
		result = append(result, r)
	}
	return result
}
