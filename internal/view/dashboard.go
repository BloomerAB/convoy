package view

import (
	"sort"

	"github.com/bloomerab/convoy/internal/model"
	"github.com/bloomerab/convoy/internal/render"
	"github.com/bloomerab/convoy/internal/ui"
	"github.com/rivo/tview"
)

// Dashboard is the main view showing all clusters.
type Dashboard struct {
	*tview.Table
	tableModel *model.TableModel
}

func NewDashboard(tm *model.TableModel) *Dashboard {
	table := tview.NewTable().
		SetSelectable(true, false).
		SetFixed(1, 0).
		SetSeparator(' ')

	d := &Dashboard{
		Table:      table,
		tableModel: tm,
	}

	tm.AddListener(d)
	return d
}

// OnDataChanged implements model.TableListener.
func (d *Dashboard) OnDataChanged() {
	d.refresh()
}

func (d *Dashboard) refresh() {
	resources := d.tableModel.Resources()

	// Sort: failures first, then by health priority, then by transition time
	sort.Slice(resources, func(i, j int) bool {
		hi, ti := resources[i].SortKey()
		hj, tj := resources[j].SortKey()
		if hi != hj {
			return hi < hj
		}
		return ti < tj
	})

	d.Clear()

	headers := render.KustomizationHeader()
	ui.SetHeaderRow(d.Table, headers)

	for i, r := range resources {
		row := i + 1 // offset for header
		cells := render.KustomizationRow(r)
		color := ui.HealthColor(r.Health)

		for col, text := range cells {
			cell := tview.NewTableCell(text).SetTextColor(color)
			if col == 0 { // symbol column
				cell.SetExpansion(0)
			} else {
				cell.SetExpansion(1)
			}
			d.SetCell(row, col, cell)
		}
	}

	d.ScrollToBeginning()
}
