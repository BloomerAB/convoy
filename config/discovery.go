package config

import (
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

// DiscoverClusters reads kubeconfig and returns a Cluster entry for each context.
func DiscoverClusters() ([]Cluster, error) {
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	kubeConfig, err := rules.Load()
	if err != nil {
		return nil, err
	}

	clusters := make([]Cluster, 0, len(kubeConfig.Contexts))
	for name := range kubeConfig.Contexts {
		clusters = append(clusters, clusterFromContext(name, kubeConfig.Contexts[name]))
	}
	return clusters, nil
}

func clusterFromContext(name string, ctx *api.Context) Cluster {
	return Cluster{
		Context:     name,
		Name:        name,
		Environment: guessEnvironment(name),
	}
}

func guessEnvironment(name string) string {
	for _, suffix := range []string{"prod", "production"} {
		if containsSuffix(name, suffix) {
			return "production"
		}
	}
	for _, suffix := range []string{"staging", "stage", "stg"} {
		if containsSuffix(name, suffix) {
			return "staging"
		}
	}
	return "unknown"
}

func containsSuffix(s, substr string) bool {
	for i := range s {
		if i+len(substr) <= len(s) && s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
