package view

import (
	"fmt"
	"strings"

	"github.com/bloomerab/convoy/internal/model"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// DescribeView shows full details of a single resource.
type DescribeView struct {
	*tview.TextView
}

func NewDescribeView(r model.Resource) *DescribeView {
	tv := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true)
	tv.SetBorder(true).
		SetTitle(fmt.Sprintf(" %s/%s (Esc: back) ", r.Kind, r.Name)).
		SetBorderColor(tcell.ColorCornflowerBlue)

	dv := &DescribeView{TextView: tv}
	dv.render(r)
	return dv
}

func (dv *DescribeView) render(r model.Resource) {
	var b strings.Builder

	field := func(label, value string) {
		if value != "" {
			fmt.Fprintf(&b, "[darkcyan]%-18s[-] %s\n", label+":", value)
		}
	}

	field("Name", r.Name)
	field("Kind", string(r.Kind))
	field("Namespace", r.Namespace)
	field("Cluster", r.Cluster)
	field("Environment", r.Environment)

	if r.Kind == model.KindWorkflowRun {
		field("Repo", r.Repo)
		field("Branch", r.Branch)
		field("Actor", r.Actor)
	}

	b.WriteString("\n")

	statusColor := "green"
	if r.Health.IsFailed() {
		statusColor = "red"
	} else if r.Health == model.HealthProgressing {
		statusColor = "yellow"
	}
	fmt.Fprintf(&b, "[darkcyan]%-18s[-] [%s]%s %s[-]\n", "Status:", statusColor, r.Health.Symbol(), r.Health.String())

	field("Revision", r.Revision)
	if !r.LastTransition.IsZero() {
		field("Last Transition", r.LastTransition.Format("2006-01-02 15:04:05 MST"))
	}

	if r.Message != "" {
		b.WriteString("\n[darkcyan]Message:[-]\n")
		b.WriteString(r.Message)
		b.WriteString("\n")
	}

	dv.SetText(b.String())
}
