package client

import (
	"fmt"
	"sync"

	"github.com/bloomerab/convoy/config"
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
