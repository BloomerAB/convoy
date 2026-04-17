package view

import (
	"fmt"
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
	// rowMap maps table row number → resource index in sorted slice (-1 = section header)
	rowMap []int
	sorted []model.Resource
}

func NewDashboard(onDescribe func(model.Resource)) *Dashboard {
	table := tview.NewTable().
		SetSelectable(true, false).
		SetSeparator(' ')

	return &Dashboard{
		Table:      table,
		onDescribe: onDescribe,
	}
}

// DescribeSelected opens the describe view for the currently selected row.
func (d *Dashboard) DescribeSelected() {
	row, _ := d.GetSelection()
	if row >= 0 && row < len(d.rowMap) {
		idx := d.rowMap[row]
		if idx >= 0 && idx < len(d.sorted) && d.onDescribe != nil {
			d.onDescribe(d.sorted[idx])
		}
	}
}

// Refresh rebuilds the table with the given resources. Must be called on the UI goroutine.
func (d *Dashboard) Refresh(resources []model.Resource) {
	// Split into categories
	var failures, flux, gha []model.Resource
	for _, r := range resources {
		if r.Health.IsFailed() {
			failures = append(failures, r)
		}
		if r.Kind == model.KindWorkflowRun {
			gha = append(gha, r)
		} else {
			flux = append(flux, r)
		}
	}

	sortByTransition := func(s []model.Resource) {
		sort.Slice(s, func(i, j int) bool {
			return s[i].LastTransition.After(s[j].LastTransition)
		})
	}
	sortByTransition(failures)
	sortByHealth(flux)
	sortByTransition(gha)

	// Build the combined sorted list for describe lookups
	d.sorted = make([]model.Resource, 0, len(resources))
	d.sorted = append(d.sorted, failures...)
	d.sorted = append(d.sorted, flux...)
	d.sorted = append(d.sorted, gha...)

	selectedRow, _ := d.GetSelection()
	d.Clear()
	d.rowMap = nil

	row := 0
	resIdx := 0

	// --- Failures section ---
	if len(failures) > 0 {
		row = d.addSectionHeader(row, fmt.Sprintf("⚠ FAILURES (%d)", len(failures)), ui.ColorFailed)
		row = d.addColumnHeaders(row, render.FailureHeader())
		for _, r := range failures {
			cells := render.FailureRow(r)
			d.addDataRow(row, cells, ui.ColorFailed, resIdx)
			row++
			resIdx++
		}
		// Blank separator
		d.addSeparator(row)
		row++
	} else {
		// Skip failures in resIdx
	}

	// --- Flux section ---
	if len(flux) > 0 {
		row = d.addSectionHeader(row, fmt.Sprintf("── Flux (%d)", len(flux)), ui.ColorHeader)
		row = d.addColumnHeaders(row, render.FluxHeader())
		for _, r := range flux {
			cells := render.FluxRow(r)
			color := ui.HealthColor(r.Health)
			d.addDataRow(row, cells, color, resIdx)
			row++
			resIdx++
		}
		// Blank separator
		d.addSeparator(row)
		row++
	}

	// --- GitHub Actions section ---
	if len(gha) > 0 {
		row = d.addSectionHeader(row, fmt.Sprintf("── GitHub Actions (%d)", len(gha)), ui.ColorHeader)
		row = d.addColumnHeaders(row, render.GHAHeader())
		for _, r := range gha {
			cells := render.GHARow(r)
			color := ui.HealthColor(r.Health)
			d.addDataRow(row, cells, color, resIdx)
			row++
			resIdx++
		}
	}

	// Restore selection
	if selectedRow >= row {
		selectedRow = row - 1
	}
	if selectedRow < 0 {
		selectedRow = 0
	}
	// Skip non-selectable rows
	for selectedRow < row && selectedRow < len(d.rowMap) && d.rowMap[selectedRow] < 0 {
		selectedRow++
	}
	if selectedRow < row {
		d.Select(selectedRow, 0)
	}
}

func (d *Dashboard) addSectionHeader(row int, title string, color tcell.Color) int {
	cell := tview.NewTableCell(title).
		SetSelectable(false).
		SetTextColor(color).
		SetAttributes(tcell.AttrBold).
		SetExpansion(1)
	d.SetCell(row, 0, cell)
	// Fill remaining columns with empty non-selectable cells
	for col := 1; col < 8; col++ {
		d.SetCell(row, col, tview.NewTableCell("").SetSelectable(false))
	}
	d.rowMap = append(d.rowMap, -1)
	return row + 1
}

func (d *Dashboard) addColumnHeaders(row int, headers []string) int {
	for col, h := range headers {
		d.SetCell(row, col, tview.NewTableCell(h).
			SetSelectable(false).
			SetTextColor(ui.ColorHeader).
			SetAttributes(tcell.AttrBold))
	}
	d.rowMap = append(d.rowMap, -1)
	return row + 1
}

func (d *Dashboard) addDataRow(row int, cells []string, color tcell.Color, resIdx int) {
	for col, text := range cells {
		cell := tview.NewTableCell(text).SetTextColor(color)
		if col == 0 {
			cell.SetExpansion(0)
		} else {
			cell.SetExpansion(1)
		}
		d.SetCell(row, col, cell)
	}
	d.rowMap = append(d.rowMap, resIdx)
}

func (d *Dashboard) addSeparator(row int) {
	d.SetCell(row, 0, tview.NewTableCell("").SetSelectable(false))
	d.rowMap = append(d.rowMap, -1)
}

func sortByHealth(s []model.Resource) {
	sort.Slice(s, func(i, j int) bool {
		hi, ti := s[i].SortKey()
		hj, tj := s[j].SortKey()
		if hi != hj {
			return hi < hj
		}
		return ti < tj
	})
}
