package view

import (
	"github.com/rivo/tview"
)

// Footer shows key hint bar.
type Footer struct {
	*tview.TextView
}

func NewFooter() *Footer {
	tv := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	tv.SetBorderPadding(0, 0, 1, 1)
	tv.SetText("[darkcyan]:cmd  /filter  ↑↓ scroll  Enter detail  q quit  ? help[-]")
	return &Footer{TextView: tv}
}
