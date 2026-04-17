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
	Actor          string // GitHub username who triggered the run
	Repo           string // GitHub repo (org/name) for workflow runs
	Branch         string // Branch name for workflow runs
	RunID          int64         // GitHub Actions run ID
	URL            string        // Web URL (GitHub run URL, etc.)
	Interval       time.Duration // Flux reconciliation interval
	NextRun        time.Time     // Estimated next reconciliation
}

// SortKey returns a comparable key for sorting:
// failures first, then by next run time (soonest first), then by transition time (newest first).
func (r Resource) SortKey() (int, int64) {
	health := int(r.Health)
	if !r.NextRun.IsZero() && health > int(HealthFailed) {
		return health, r.NextRun.Unix()
	}
	return health, -r.LastTransition.Unix()
}
