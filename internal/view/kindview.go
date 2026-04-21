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

// KindView shows all resources of a single kind.
type KindView struct {
	*tview.Table
	kind       model.ResourceKind
	activeOnly bool // true = only failed/progressing (for Actions)
	onDescribe func(model.Resource)
	sorted     []model.Resource
}

func NewKindView(kind model.ResourceKind, activeOnly bool, onDescribe func(model.Resource)) *KindView {
	table := tview.NewTable().
		SetSelectable(true, false).
		SetFixed(1, 0).
		SetSeparator(' ')

	kv := &KindView{
		Table:      table,
		kind:       kind,
		activeOnly: activeOnly,
		onDescribe: onDescribe,
	}

	return kv
}

func (kv *KindView) Title() string {
	switch kv.kind {
	case model.KindKustomization:
		return "Kustomizations"
	case model.KindHelmRelease:
		return "HelmReleases"
	case model.KindHelmRepository:
		return "HelmRepositories"
	case model.KindGitRepository:
		return "GitRepositories"
	case model.KindWorkflowRun:
		return "GitHub Actions"
	default:
		return string(kv.kind)
	}
}

// SelectedResource returns the currently selected resource.
func (kv *KindView) SelectedResource() *model.Resource {
	row, _ := kv.GetSelection()
	idx := row - 1
	if idx >= 0 && idx < len(kv.sorted) {
		r := kv.sorted[idx]
		return &r
	}
	return nil
}

// Refresh updates the table with resources filtered to this kind.
func (kv *KindView) Refresh(allResources []model.Resource, filter ...string) {
	filterText := ""
	if len(filter) > 0 {
		filterText = filter[0]
	}
	var filtered []model.Resource
	for _, r := range allResources {
		if r.Kind != kv.kind {
			continue
		}
		if kv.activeOnly && r.Health != model.HealthFailed && r.Health != model.HealthProgressing && r.Health != model.HealthUnknown {
			continue
		}
		filtered = append(filtered, r)
	}

	sort.Slice(filtered, func(i, j int) bool {
		hi, ti := filtered[i].SortKey()
		hj, tj := filtered[j].SortKey()
		if hi != hj {
			return hi < hj
		}
		return ti < tj
	})
	kv.sorted = filtered

	selectedRow, _ := kv.GetSelection()
	kv.Clear()

	headers := render.ResourceHeader()
	lastCol := len(headers) - 1
	for col, h := range headers {
		cell := tview.NewTableCell(h).
			SetSelectable(false).
			SetTextColor(ui.ColorHeader).
			SetAttributes(tcell.AttrBold)
		if col == lastCol {
			cell.SetText(fmt.Sprintf("%s (%d)", kv.Title(), len(filtered)))
			cell.SetAlign(tview.AlignRight)
		}
		kv.SetCell(0, col, cell)
	}

	for i, r := range filtered {
		row := i + 1
		cells := render.ResourceRow(r)
		color := ui.HealthColor(r.Health)

		for col, text := range cells {
			if filterText != "" && col > 0 {
				text = ui.Highlight(text, filterText)
			}
			cell := tview.NewTableCell(text).SetTextColor(color)
			if col == 0 {
				cell.SetExpansion(0)
			} else {
				cell.SetExpansion(1)
			}
			kv.SetCell(row, col, cell)
		}
	}

	numRows := len(filtered) + 1
	if selectedRow >= numRows {
		selectedRow = numRows - 1
	}
	if selectedRow < 1 && numRows > 1 {
		selectedRow = 1
	}
	kv.Select(selectedRow, 0)
}
