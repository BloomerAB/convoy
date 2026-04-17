package render

import (
	"fmt"
	"time"

	"github.com/bloomerab/convoy/internal/model"
)

// ResourceHeader returns the column headers for the dashboard table.
func ResourceHeader() []string {
	return []string{"", "CLUSTER", "KIND", "NAME", "STATUS", "MESSAGE", "REVISION", "AGE"}
}

// ResourceRow renders any resource to table cells.
func ResourceRow(r model.Resource) []string {
	rev := r.Revision
	if len(rev) > 12 {
		rev = rev[:12]
	}

	msg := r.Message
	if len(msg) > 60 {
		msg = msg[:57] + "..."
	}

	return []string{
		r.Health.Symbol(),
		r.Cluster,
		kindShort(r.Kind),
		r.Name,
		r.Health.String(),
		msg,
		rev,
		formatAge(r.LastTransition),
	}
}

func kindShort(k model.ResourceKind) string {
	switch k {
	case model.KindKustomization:
		return "Kustomization"
	case model.KindHelmRelease:
		return "HelmRelease"
	case model.KindGitRepository:
		return "GitRepo"
	case model.KindWorkflowRun:
		return "Workflow"
	default:
		return string(k)
	}
}

func formatAge(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}
