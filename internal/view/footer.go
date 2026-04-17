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
	tv.SetText(footerText("", false, ""))
	return &Footer{TextView: tv}
}

func (f *Footer) Update(filter string, mineOnly bool, kindFilter string) {
	f.SetText(footerText(filter, mineOnly, kindFilter))
}

func footerText(filter string, mineOnly bool, kindFilter string) string {
	mineHint := "m:mine"
	if mineOnly {
		mineHint = "m:all"
	}

	indicators := ""
	if kindFilter != "" {
		indicators += fmt.Sprintf("  [aqua]%s[-]", kindFilter)
	}
	if filter != "" {
		indicators += fmt.Sprintf("  [yellow]/%s[-]", filter)
	}
	if filter != "" || kindFilter != "" {
		indicators += " (Esc clear)"
	}

	return fmt.Sprintf("[darkcyan]:cmd  /filter  d:describe  %s  r:refresh  q:quit[-]%s", mineHint, indicators)
}
