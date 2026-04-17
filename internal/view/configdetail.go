package view

import (
	"os"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// ConfigDetailView shows the raw YAML content of a config file.
type ConfigDetailView struct {
	*tview.TextView
	file   ConfigFile
	onEdit func(ConfigFile)
}

func NewConfigDetailView(file ConfigFile, onEdit func(ConfigFile)) *ConfigDetailView {
	tv := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true)
	tv.SetBorder(true).
		SetTitle(" " + file.Name + " (e: edit, Esc: back) ").
		SetBorderColor(tcell.ColorCornflowerBlue)

	cd := &ConfigDetailView{
		TextView: tv,
		file:     file,
		onEdit:   onEdit,
	}

	cd.loadContent()

	tv.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyRune && event.Rune() == 'e' {
			cd.onEdit(cd.file)
			return nil
		}
		return event
	})

	return cd
}

func (cd *ConfigDetailView) loadContent() {
	data, err := os.ReadFile(cd.file.Path)
	if err != nil {
		cd.SetText("[#6EB5FF]# " + cd.file.Path + "[-]\n\n[#FFFF64](file does not exist yet — press 'e' to create)[-]")
		return
	}
	cd.SetText("[#6EB5FF]# " + cd.file.Path + "[-]\n\n" + string(data))
}

// Reload re-reads the file from disk.
func (cd *ConfigDetailView) Reload() {
	cd.loadContent()
}
