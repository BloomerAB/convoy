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

func (h *Header) Update(resources []model.Resource, clusterCount int, mineOnly bool, showAll bool) {
	failures := 0
	progressing := 0
	total := len(resources)
	for _, r := range resources {
		if r.Health.IsFailed() {
			failures++
		}
		if r.Health == model.HealthProgressing {
			progressing++
		}
	}

	var failText string
	if failures > 0 {
		failText = fmt.Sprintf("  [red]%d failing[-]", failures)
	}

	var progText string
	if progressing > 0 {
		progText = fmt.Sprintf("  [yellow]%d progressing[-]", progressing)
	}

	mineText := ""
	if mineOnly {
		mineText = "  [yellow][mine][-]"
	}

	viewMode := "[darkcyan]active[-]"
	if showAll {
		viewMode = fmt.Sprintf("[darkcyan]all %d[-]", total)
	}

	h.SetText(fmt.Sprintf("[white::b]convoy[-]  %d clusters  %s%s%s%s",
		clusterCount, viewMode, failText, progText, mineText))
}
