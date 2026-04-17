package view

import (
	"os"
	"path/filepath"

	"github.com/bloomerab/convoy/internal/ui"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// ConfigFile represents a discoverable config file.
type ConfigFile struct {
	Name string
	Path string
}

// ConfigListView shows a table of config files. Enter to view YAML, 'e' to edit.
type ConfigListView struct {
	*tview.Table
	files    []ConfigFile
	onSelect func(ConfigFile)
	onEdit   func(ConfigFile)
}

func NewConfigListView(files []ConfigFile, onSelect func(ConfigFile), onEdit func(ConfigFile)) *ConfigListView {
	table := tview.NewTable().
		SetSelectable(true, false).
		SetFixed(1, 0).
		SetSeparator(' ')
	table.SetBorder(true).
		SetTitle(" Config (Enter: view, e: edit, Esc: back) ").
		SetBorderColor(tcell.ColorCornflowerBlue)

	cl := &ConfigListView{
		Table:    table,
		files:    files,
		onSelect: onSelect,
		onEdit:   onEdit,
	}

	cl.render()

	table.SetSelectedFunc(func(row, _ int) {
		idx := row - 1 // header offset
		if idx >= 0 && idx < len(cl.files) {
			cl.onSelect(cl.files[idx])
		}
	})

	table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyRune && event.Rune() == 'e' {
			row, _ := cl.GetSelection()
			idx := row - 1
			if idx >= 0 && idx < len(cl.files) {
				cl.onEdit(cl.files[idx])
			}
			return nil
		}
		return event
	})

	return cl
}

func (cl *ConfigListView) render() {
	headers := []string{"NAME", "PATH"}
	ui.SetHeaderRow(cl.Table, headers)

	for i, f := range cl.files {
		row := i + 1
		cl.SetCell(row, 0, tview.NewTableCell(f.Name).SetTextColor(tcell.ColorWhite).SetExpansion(1))
		cl.SetCell(row, 1, tview.NewTableCell(f.Path).SetTextColor(tcell.ColorGray).SetExpansion(2))
	}
}

// DiscoverConfigFiles returns all config files in the convoy config directory.
func DiscoverConfigFiles() []ConfigFile {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return nil
	}

	dir := filepath.Join(configDir, "convoy")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var files []ConfigFile
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		ext := filepath.Ext(name)
		if ext == ".yaml" || ext == ".yml" {
			files = append(files, ConfigFile{
				Name: name,
				Path: filepath.Join(dir, name),
			})
		}
	}

	// If no files found, include the default path so user can create it
	if len(files) == 0 {
		files = append(files, ConfigFile{
			Name: "config.yaml",
			Path: filepath.Join(dir, "config.yaml"),
		})
	}

	return files
}
