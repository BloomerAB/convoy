package model

import "time"

// ResourceKind identifies the type of Flux resource.
type ResourceKind string

const (
	KindKustomization ResourceKind = "Kustomization"
	KindHelmRelease   ResourceKind = "HelmRelease"
	KindGitRepository ResourceKind = "GitRepository"
	KindWorkflowRun   ResourceKind = "WorkflowRun"
)

// Resource is a normalized representation of any monitored resource.
type Resource struct {
	Cluster        string
	Environment    string
	Kind           ResourceKind
	Namespace      string
	Name           string
	Health         HealthStatus
	Message        string
	Revision       string
	LastTransition time.Time
}

// SortKey returns a comparable key for sorting: failures first, then by transition time (newest first).
func (r Resource) SortKey() (int, int64) {
	return int(r.Health), -r.LastTransition.Unix()
}
