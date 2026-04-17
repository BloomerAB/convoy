package client

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/bloomerab/convoy/config"
	"github.com/bloomerab/convoy/internal/model"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
)

// ClusterClient holds a dynamic client for a single cluster.
type ClusterClient struct {
	Name        string
	Environment string
	Dynamic     dynamic.Interface
}

// ClusterFactory manages k8s clients for multiple clusters.
type ClusterFactory struct {
	mu      sync.RWMutex
	clients map[string]*ClusterClient
}

func NewClusterFactory() *ClusterFactory {
	return &ClusterFactory{
		clients: make(map[string]*ClusterClient),
	}
}

// AddCluster creates a dynamic client for the given cluster config.
func (f *ClusterFactory) AddCluster(cluster config.Cluster) error {
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	overrides := &clientcmd.ConfigOverrides{CurrentContext: cluster.Context}
	cfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, overrides).ClientConfig()
	if err != nil {
		return fmt.Errorf("client config for %s: %w", cluster.Name, err)
	}

	cfg.Timeout = 5_000_000_000 // 5s

	dynClient, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return fmt.Errorf("dynamic client for %s: %w", cluster.Name, err)
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	f.clients[cluster.Name] = &ClusterClient{
		Name:        cluster.Name,
		Environment: cluster.Environment,
		Dynamic:     dynClient,
	}
	return nil
}

// Client returns the client for the named cluster.
func (f *ClusterFactory) Client(name string) (*ClusterClient, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	c, ok := f.clients[name]
	return c, ok
}

// Reconcile triggers a Flux reconciliation by annotating the resource.
func (f *ClusterFactory) Reconcile(ctx context.Context, r model.Resource) error {
	cc, ok := f.Client(r.Cluster)
	if !ok {
		return fmt.Errorf("cluster %s not found", r.Cluster)
	}

	var gvr = KustomizationGVR
	switch r.Kind {
	case model.KindKustomization:
		gvr = KustomizationGVR
	case model.KindHelmRelease:
		gvr = HelmReleaseGVR
	case model.KindGitRepository:
		gvr = GitRepositoryGVR
	case model.KindHelmRepository:
		gvr = HelmRepositoryGVR
	default:
		return fmt.Errorf("cannot reconcile %s", r.Kind)
	}

	patch := fmt.Sprintf(`{"metadata":{"annotations":{"reconcile.fluxcd.io/requestedAt":"%s"}}}`,
		time.Now().Format(time.RFC3339Nano))

	_, err := cc.Dynamic.Resource(gvr).Namespace(r.Namespace).Patch(
		ctx, r.Name, types.MergePatchType, []byte(patch), metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("reconcile %s/%s on %s: %w", r.Namespace, r.Name, r.Cluster, err)
	}
	return nil
}

// Clients returns all cluster clients.
func (f *ClusterFactory) Clients() []*ClusterClient {
	f.mu.RLock()
	defer f.mu.RUnlock()
	result := make([]*ClusterClient, 0, len(f.clients))
	for _, c := range f.clients {
		result = append(result, c)
	}
	return result
}
