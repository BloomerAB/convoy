package view

import (
	"fmt"
	"strings"

	"github.com/bloomerab/convoy/internal/model"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// RunLogView shows the jobs/steps of a GitHub Actions run.
type RunLogView struct {
	*tview.TextView
}

func NewRunLogView(r model.Resource) *RunLogView {
	repo := r.Repo
	if idx := strings.LastIndex(repo, "/"); idx >= 0 {
		repo = repo[idx+1:]
	}

	tv := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true)
	tv.SetBorder(true).
		SetTitle(fmt.Sprintf(" %s/%s #%s (Esc: back) ", repo, r.Name, r.Revision)).
		SetBorderColor(tcell.ColorCornflowerBlue)
	tv.SetText("[yellow]Loading jobs...[-]")

	return &RunLogView{TextView: tv}
}

func (v *RunLogView) SetContent(content string) {
	// Colorize output
	var b strings.Builder
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "✗"):
			fmt.Fprintf(&b, "[red]%s[-]\n", line)
		case strings.Contains(line, "[FAILED]"):
			fmt.Fprintf(&b, "[red]%s[-]\n", line)
		case strings.HasPrefix(trimmed, "●"):
			fmt.Fprintf(&b, "[yellow]%s[-]\n", line)
		case strings.HasPrefix(trimmed, "◌"):
			fmt.Fprintf(&b, "[gray]%s[-]\n", line)
		default:
			fmt.Fprintf(&b, "%s\n", line)
		}
	}
	v.SetText(b.String())
}

func (v *RunLogView) SetError(err error) {
	v.SetText(fmt.Sprintf("[red]Error fetching jobs: %v[-]", err))
}
