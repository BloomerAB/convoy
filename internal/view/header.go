package view

import (
	"fmt"

	"github.com/bloomerab/convoy/internal/model"
	"github.com/rivo/tview"
)

// Header shows cluster count and failure summary.
type Header struct {
	*tview.TextView
}

func NewHeader() *Header {
	tv := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	tv.SetBorderPadding(0, 0, 1, 1)
	return &Header{TextView: tv}
}

func (h *Header) Update(resources []model.Resource, clusterCount int, mineOnly bool) {
	failures := 0
	ghaCount := 0
	for _, r := range resources {
		if r.Health.IsFailed() {
			failures++
		}
		if r.Kind == model.KindWorkflowRun {
			ghaCount++
		}
	}

	var failText string
	if failures > 0 {
		failText = fmt.Sprintf("  [red]%d failing[-]", failures)
	}

	mineText := ""
	if mineOnly {
		mineText = "  [yellow][mine][-]"
	}

	ghaText := ""
	if ghaCount > 0 {
		ghaText = fmt.Sprintf("  %d GHA runs", ghaCount)
	}

	h.SetText(fmt.Sprintf("[white::b]convoy[-]  %d clusters%s%s%s",
		clusterCount, ghaText, failText, mineText))
}
