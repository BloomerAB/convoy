package render

import (
	"fmt"
	"time"

	"github.com/bloomerab/convoy/internal/model"
)

// Header returns the column headers for Kustomization rows.
func KustomizationHeader() []string {
	return []string{"", "CLUSTER", "NAME", "STATUS", "MESSAGE", "REVISION", "AGE"}
}

// KustomizationRow renders a Kustomization resource to table cells.
func KustomizationRow(r model.Resource) []string {
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
		r.Name,
		r.Health.String(),
		msg,
		rev,
		formatAge(r.LastTransition),
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
