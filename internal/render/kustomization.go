package render

import (
	"fmt"
	"strings"
	"time"

	"github.com/bloomerab/convoy/internal/model"
)

// ResourceHeader returns the column headers for the dashboard table.
func ResourceHeader() []string {
	return []string{"", "NAME", "KIND", "CLUSTER", "STATUS", "MESSAGE", "REVISION", "AGE"}
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

	name := r.Name
	if r.Kind == model.KindWorkflowRun {
		// Show repo/workflow for GHA runs
		repo := r.Repo
		if idx := strings.LastIndex(repo, "/"); idx >= 0 {
			repo = repo[idx+1:]
		}
		name = repo + "/" + r.Name
	}

	cluster := r.Cluster
	if r.Kind == model.KindWorkflowRun {
		cluster = r.Branch
	}

	return []string{
		r.Health.Symbol(),
		name,
		kindShort(r.Kind),
		cluster,
		r.Health.String(),
		msg,
		rev,
		formatAge(r.LastTransition),
	}
}

func kindShort(k model.ResourceKind) string {
	switch k {
	case model.KindKustomization:
		return "Ks"
	case model.KindHelmRelease:
		return "Hr"
	case model.KindGitRepository:
		return "GitRepo"
	case model.KindWorkflowRun:
		return "GHA"
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
