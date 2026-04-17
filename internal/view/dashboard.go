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
	tableModel *model.TableModel
	onDescribe func(model.Resource)
	sorted     []model.Resource
}

func NewDashboard(tm *model.TableModel, onDescribe func(model.Resource)) *Dashboard {
	table := tview.NewTable().
		SetSelectable(true, false).
		SetFixed(1, 0).
		SetSeparator(' ')

	d := &Dashboard{
		Table:      table,
		tableModel: tm,
		onDescribe: onDescribe,
	}

	table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyRune && event.Rune() == 'd' {
			d.describeSelected()
			return nil
		}
		return event
	})

	tm.AddListener(d)
	return d
}

// OnDataChanged implements model.TableListener.
func (d *Dashboard) OnDataChanged() {
	d.refresh()
}

func (d *Dashboard) describeSelected() {
	row, _ := d.GetSelection()
	idx := row - 1
	if idx >= 0 && idx < len(d.sorted) && d.onDescribe != nil {
		d.onDescribe(d.sorted[idx])
	}
}

func (d *Dashboard) refresh() {
	resources := d.tableModel.Resources()

	sort.Slice(resources, func(i, j int) bool {
		hi, ti := resources[i].SortKey()
		hj, tj := resources[j].SortKey()
		if hi != hj {
			return hi < hj
		}
		return ti < tj
	})
	d.sorted = resources

	// Preserve current selection
	selectedRow, _ := d.GetSelection()

	headers := render.ResourceHeader()
	numCols := len(headers)
	numRows := len(resources) + 1 // +1 for header

	// Set header row
	for col, h := range headers {
		cell := d.GetCell(0, col)
		if cell == nil {
			cell = tview.NewTableCell(h).
				SetSelectable(false).
				SetTextColor(ui.ColorHeader).
				SetAttributes(tcell.AttrBold)
			d.SetCell(0, col, cell)
		} else {
			cell.SetText(h)
		}
	}

	// Update data rows in-place
	for i, r := range resources {
		row := i + 1
		cells := render.ResourceRow(r)
		color := ui.HealthColor(r.Health)

		for col, text := range cells {
			cell := d.GetCell(row, col)
			if cell == nil {
				cell = tview.NewTableCell(text).SetTextColor(color)
				if col == 0 {
					cell.SetExpansion(0)
				} else {
					cell.SetExpansion(1)
				}
				d.SetCell(row, col, cell)
			} else {
				cell.SetText(text)
				cell.SetTextColor(color)
			}
		}
	}

	// Remove excess rows
	currentRowCount := d.GetRowCount()
	for row := numRows; row < currentRowCount; row++ {
		for col := 0; col < numCols; col++ {
			d.SetCell(row, col, tview.NewTableCell(""))
		}
	}

	// Restore selection (clamped)
	if selectedRow >= numRows {
		selectedRow = numRows - 1
	}
	if selectedRow < 1 && numRows > 1 {
		selectedRow = 1
	}
	d.Select(selectedRow, 0)
}
