package view

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// CmdBar handles : command and / filter input.
type CmdBar struct {
	*tview.InputField
	onSubmit func(text string)
	onCancel func()
}

func NewCmdBar(onSubmit func(string), onCancel func()) *CmdBar {
	input := tview.NewInputField().
		SetFieldBackgroundColor(tcell.ColorDefault).
		SetFieldTextColor(tcell.ColorWhite)
	input.SetBorderPadding(0, 0, 1, 1)

	cb := &CmdBar{
		InputField: input,
		onSubmit:   onSubmit,
		onCancel:   onCancel,
	}

	input.SetDoneFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyEnter:
			text := input.GetText()
			input.SetText("")
			cb.onSubmit(text)
		case tcell.KeyEscape:
			input.SetText("")
			cb.onCancel()
		}
	})

	return cb
}

// Activate sets the label and focuses the input.
func (cb *CmdBar) Activate(prefix string) {
	cb.SetLabel(prefix)
	cb.SetText("")
}
