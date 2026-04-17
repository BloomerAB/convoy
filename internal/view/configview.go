package view

import (
	"github.com/bloomerab/convoy/config"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"gopkg.in/yaml.v3"
)

// ConfigView shows the current config as read-only YAML. Press 'e' to edit.
type ConfigView struct {
	*tview.TextView
	onEdit func()
}

func NewConfigView(cfg config.Config, onEdit func()) *ConfigView {
	tv := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true)
	tv.SetBorder(true).
		SetTitle(" Config (e: edit, Esc: back) ").
		SetBorderColor(tcell.ColorCornflowerBlue)

	cv := &ConfigView{
		TextView: tv,
		onEdit:   onEdit,
	}

	cv.Render(cfg)

	tv.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyRune && event.Rune() == 'e' {
			cv.onEdit()
			return nil
		}
		return event
	})

	return cv
}

func (cv *ConfigView) Render(cfg config.Config) {
	data, err := yaml.Marshal(&cfg)
	if err != nil {
		cv.SetText("[red]Error rendering config: " + err.Error() + "[-]")
		return
	}

	path, _ := config.Path()
	header := "[darkcyan]# " + path + "[-]\n\n"
	cv.SetText(header + string(data))
}
