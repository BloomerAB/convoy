package view

import (
	"fmt"

	"github.com/bloomerab/convoy/internal/model"
	"github.com/bloomerab/convoy/internal/ui"
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

func (h *Header) Update(resources []model.Resource, clusterCount int) {
	failures := 0
	for _, r := range resources {
		if r.Health.IsFailed() {
			failures++
		}
	}

	var failText string
	if failures > 0 {
		failText = fmt.Sprintf("  [%s]%d failing[-]", colorTag(ui.ColorFailed), failures)
	}

	h.SetText(fmt.Sprintf("[%s]convoy[-]  %d clusters%s",
		colorTag(ui.ColorTitle), clusterCount, failText))
}

func colorTag(c interface{}) string {
	return fmt.Sprintf("%v", c)
}
