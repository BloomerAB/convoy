package view

import (
	"fmt"

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
	tv.SetText(footerText("", false))
	return &Footer{TextView: tv}
}

func (f *Footer) UpdateFilter(filter string, mineOnly bool) {
	f.SetText(footerText(filter, mineOnly))
}

func footerText(filter string, mineOnly bool) string {
	mineHint := "m:mine"
	if mineOnly {
		mineHint = "m:all"
	}

	filterHint := ""
	if filter != "" {
		filterHint = fmt.Sprintf("  [yellow]/%s[-] (Esc clear)", filter)
	}

	return fmt.Sprintf("[darkcyan]:cmd  /filter  d:describe  %s  r:refresh  q:quit[-]%s", mineHint, filterHint)
}
