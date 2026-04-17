package view

import (
	"fmt"
	"strings"

	"github.com/bloomerab/convoy/internal/model"
	"github.com/rivo/tview"
)

// RunLogView shows the jobs/steps of a GitHub Actions run.
type RunLogView struct {
	*tview.TextView
	Resource model.Resource
}

func NewRunLogView(r model.Resource) *RunLogView {
	repo := r.Repo
	if idx := strings.LastIndex(repo, "/"); idx >= 0 {
		repo = repo[idx+1:]
	}

	tv := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true)
	tv.SetBorderPadding(0, 0, 1, 1)
	tv.SetText("[#FFFF64]Loading jobs...[-]")

	return &RunLogView{TextView: tv, Resource: r}
}

func (v *RunLogView) SetContent(content string) {
	// Colorize output
	var b strings.Builder
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "✗"):
			fmt.Fprintf(&b, "[#FF5050]%s[-]\n", line)
		case strings.Contains(line, "[FAILED]"):
			fmt.Fprintf(&b, "[#FF5050]%s[-]\n", line)
		case strings.HasPrefix(trimmed, "●"):
			fmt.Fprintf(&b, "[#FFFF64]%s[-]\n", line)
		case strings.HasPrefix(trimmed, "◌"):
			fmt.Fprintf(&b, "[#9696B4]%s[-]\n", line)
		default:
			fmt.Fprintf(&b, "%s\n", line)
		}
	}
	v.SetText(b.String())
}

func (v *RunLogView) SetError(err error) {
	v.SetText(fmt.Sprintf("[#FF5050]Error fetching jobs: %v[-]", err))
}
