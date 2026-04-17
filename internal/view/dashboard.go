package view

import (
	"sort"

	"github.com/bloomerab/convoy/internal/model"
	"github.com/bloomerab/convoy/internal/render"
	"github.com/bloomerab/convoy/internal/ui"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// Dashboard is the main view showing all clusters.
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

// DescribeSelected opens the describe view for the currently selected row.
func (d *Dashboard) DescribeSelected() {
	row, _ := d.GetSelection()
	idx := row - 1
	if idx >= 0 && idx < len(d.sorted) && d.onDescribe != nil {
		d.onDescribe(d.sorted[idx])
	}
}

// Refresh rebuilds the table with the given resources. Must be called on the UI goroutine.
func (d *Dashboard) Refresh(resources []model.Resource) {
	sort.Slice(resources, func(i, j int) bool {
		hi, ti := resources[i].SortKey()
		hj, tj := resources[j].SortKey()
		if hi != hj {
			return hi < hj
		}
		return ti < tj
	})
	d.sorted = resources

	selectedRow, _ := d.GetSelection()

	d.Clear()

	headers := render.ResourceHeader()
	for col, h := range headers {
		d.SetCell(0, col, tview.NewTableCell(h).
			SetSelectable(false).
			SetTextColor(ui.ColorHeader).
			SetAttributes(tcell.AttrBold))
	}

	for i, r := range resources {
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

	numRows := len(resources) + 1
	if selectedRow >= numRows {
		selectedRow = numRows - 1
	}
	if selectedRow < 1 && numRows > 1 {
		selectedRow = 1
	}
	d.Select(selectedRow, 0)
}
