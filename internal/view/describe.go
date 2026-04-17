package view

import (
	"fmt"
	"strings"

	"github.com/bloomerab/convoy/internal/model"
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
	tv.SetBorderPadding(0, 0, 1, 1)

	dv := &DescribeView{TextView: tv}
	dv.render(r)
	return dv
}

func (dv *DescribeView) render(r model.Resource) {
	var b strings.Builder

	field := func(label, value string) {
		if value != "" {
			fmt.Fprintf(&b, "[#6EB5FF]%-18s[-] %s\n", label+":", value)
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

	statusColor := "#64FF64"
	if r.Health.IsFailed() {
		statusColor = "#FF5050"
	} else if r.Health == model.HealthProgressing {
		statusColor = "#FFFF64"
	}
	fmt.Fprintf(&b, "[#6EB5FF]%-18s[-] [%s]%s %s[-]\n", "Status:", statusColor, r.Health.Symbol(), r.Health.String())

	field("Revision", r.Revision)
	if !r.LastTransition.IsZero() {
		field("Last Transition", r.LastTransition.Format("2006-01-02 15:04:05 MST"))
	}

	if r.Message != "" {
		b.WriteString("\n[#6EB5FF]Message:[-]\n")
		b.WriteString(r.Message)
		b.WriteString("\n")
	}

	dv.SetText(b.String())
}
