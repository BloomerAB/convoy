package ui

import (
	"github.com/bloomerab/convoy/internal/model"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// SetHeaderRow configures the first row of a table as a header.
func SetHeaderRow(table *tview.Table, headers []string) {
	for col, h := range headers {
		cell := tview.NewTableCell(h).
			SetSelectable(false).
			SetTextColor(ColorHeader).
			SetAttributes(tcell.AttrBold)
		table.SetCell(0, col, cell)
	}
}

// HealthColor returns the tcell color for a health status.
func HealthColor(h model.HealthStatus) tcell.Color {
	switch h {
	case model.HealthReady:
		return ColorReady
	case model.HealthFailed:
		return ColorFailed
	case model.HealthProgressing:
		return ColorProgressing
	case model.HealthSuspended:
		return ColorSuspended
	default:
		return ColorUnknown
	}
}
