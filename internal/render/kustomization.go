package render

import (
	"fmt"
	"strings"
	"time"

	"github.com/bloomerab/convoy/internal/model"
)

// ResourceHeader returns the column headers for the dashboard table.
func ResourceHeader() []string {
	return []string{"", "NAME", "KIND", "SOURCE", "STATUS", "MESSAGE", "AGE"}
}

// ResourceRow renders any resource to table cells.
func ResourceRow(r model.Resource) []string {
	name := r.Name
	if r.Kind == model.KindWorkflowRun {
		repo := r.Repo
		if idx := strings.LastIndex(repo, "/"); idx >= 0 {
			repo = repo[idx+1:]
		}
		name = repo + "/" + r.Name
	}

	source := r.Cluster
	if r.Kind == model.KindWorkflowRun {
		source = r.Branch
		if r.Actor != "" {
			source = r.Branch + " (" + r.Actor + ")"
		}
	}

	return []string{
		r.Health.Symbol(),
		name,
		kindShort(r.Kind),
		source,
		r.Health.String(),
		truncate(r.Message, 50),
		formatAge(r.LastTransition),
	}
}

func kindShort(k model.ResourceKind) string {
	switch k {
	case model.KindKustomization:
		return "Kustomize"
	case model.KindHelmRelease:
		return "Helm"
	case model.KindGitRepository:
		return "GitRepo"
	case model.KindWorkflowRun:
		return "Actions"
	default:
		return string(k)
	}
}

func truncate(s string, max int) string {
	if len(s) > max {
		return s[:max-3] + "..."
	}
	return s
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
