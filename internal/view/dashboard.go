package view

import (
	"sort"

	"github.com/bloomerab/convoy/internal/model"
	"github.com/bloomerab/convoy/internal/render"
	"github.com/bloomerab/convoy/internal/ui"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// Dashboard shows resources that need attention: failed and progressing.
type Dashboard struct {
	*tview.Table
	onDescribe func(model.Resource)
	sorted     []model.Resource
}

func NewDashboard(onDescribe func(model.Resource)) *Dashboard {
	table := tview.NewTable().
		SetSelectable(true, false).
		SetFixed(1, 0).
		SetSeparator(' ')

	return &Dashboard{
		Table:      table,
		onDescribe: onDescribe,
	}
}

// SelectedResource returns the currently selected resource, or nil.
func (d *Dashboard) SelectedResource() *model.Resource {
	row, _ := d.GetSelection()
	idx := row - 1 // header offset
	if idx >= 0 && idx < len(d.sorted) {
		r := d.sorted[idx]
		return &r
	}
	return nil
}

// DescribeSelected opens the describe view for the currently selected row.
func (d *Dashboard) DescribeSelected() {
	if r := d.SelectedResource(); r != nil && d.onDescribe != nil {
		d.onDescribe(*r)
	}
}

// Refresh rebuilds the table. Must be called on the UI goroutine.
func (d *Dashboard) Refresh(resources []model.Resource, showAll bool) {
	var active []model.Resource
	if showAll {
		active = resources
	} else {
		// Only show things that need attention
		for _, r := range resources {
			if r.Health == model.HealthFailed || r.Health == model.HealthProgressing || r.Health == model.HealthUnknown {
				active = append(active, r)
			}
		}
	}

	// Sort: failed first, then progressing, then unknown. Within each: newest first.
	sort.Slice(active, func(i, j int) bool {
		hi, ti := active[i].SortKey()
		hj, tj := active[j].SortKey()
		if hi != hj {
			return hi < hj
		}
		return ti < tj
	})
	d.sorted = active

	selectedRow, _ := d.GetSelection()
	d.Clear()

	headers := render.ResourceHeader()
	for col, h := range headers {
		d.SetCell(0, col, tview.NewTableCell(h).
			SetSelectable(false).
			SetTextColor(ui.ColorHeader).
			SetAttributes(tcell.AttrBold))
	}

	for i, r := range active {
		row := i + 1
		cells := render.ResourceRow(r)
		color := ui.HealthColor(r.Health)

		for col, text := range cells {
			cell := tview.NewTableCell(text).SetTextColor(color)
			if col == 0 {
				cell.SetExpansion(0)
			} else {
				cell.SetExpansion(1)
			}
			d.SetCell(row, col, cell)
		}
	}

	numRows := len(active) + 1
	if selectedRow >= numRows {
		selectedRow = numRows - 1
	}
	if selectedRow < 1 && numRows > 1 {
		selectedRow = 1
	}
	d.Select(selectedRow, 0)
}
