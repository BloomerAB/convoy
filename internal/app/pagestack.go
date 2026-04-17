package app

import (
	"github.com/rivo/tview"
)

// PageStack manages a stack of views with push/pop navigation.
// Switch replaces the current view (k9s-style : commands).
// Push/Pop are for overlays (d, l, ? etc.) that go back on Esc.
type PageStack struct {
	*tview.Pages
	stack []string
}

func NewPageStack() *PageStack {
	return &PageStack{
		Pages: tview.NewPages(),
		stack: make([]string, 0),
	}
}

// Switch replaces the current primary view (like k9s : commands).
// Keeps the "dashboard" base, removes everything above it, then adds the new page.
func (ps *PageStack) Switch(name string, page tview.Primitive) {
	// Pop everything down to the base (dashboard)
	for len(ps.stack) > 1 {
		old := ps.stack[len(ps.stack)-1]
		ps.stack = ps.stack[:len(ps.stack)-1]
		ps.RemovePage(old)
	}
	// Now push the new page on top of dashboard
	ps.stack = append(ps.stack, name)
	ps.AddAndSwitchToPage(name, page, true)
}

// Push adds an overlay page on top (for describe, logs, help).
func (ps *PageStack) Push(name string, page tview.Primitive) {
	ps.stack = append(ps.stack, name)
	ps.AddAndSwitchToPage(name, page, true)
}

// Pop removes the top overlay and returns to the previous view.
func (ps *PageStack) Pop() {
	if len(ps.stack) <= 1 {
		return
	}
	current := ps.stack[len(ps.stack)-1]
	ps.stack = ps.stack[:len(ps.stack)-1]
	ps.RemovePage(current)
	ps.SwitchToPage(ps.stack[len(ps.stack)-1])
}

// Current returns the name of the current page.
func (ps *PageStack) Current() string {
	if len(ps.stack) == 0 {
		return ""
	}
	return ps.stack[len(ps.stack)-1]
}
