package app

import (
	"github.com/rivo/tview"
)

// PageStack manages navigation with history.
// All views push onto the stack. Esc always pops back one step.
// Dashboard is the base that can never be popped.
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

// Push adds a page on top. Esc will pop back to the previous view.
func (ps *PageStack) Push(name string, page tview.Primitive) {
	ps.stack = append(ps.stack, name)
	ps.AddAndSwitchToPage(name, page, true)
}

// Pop removes the top page and returns to the previous one.
func (ps *PageStack) Pop() {
	if len(ps.stack) <= 1 {
		return
	}
	current := ps.stack[len(ps.stack)-1]
	ps.stack = ps.stack[:len(ps.stack)-1]
	ps.RemovePage(current)
	ps.SwitchToPage(ps.stack[len(ps.stack)-1])
}

// PopTo pops everything until reaching the named page (or dashboard).
func (ps *PageStack) PopTo(name string) {
	for len(ps.stack) > 1 && ps.stack[len(ps.stack)-1] != name {
		old := ps.stack[len(ps.stack)-1]
		ps.stack = ps.stack[:len(ps.stack)-1]
		ps.RemovePage(old)
	}
	if len(ps.stack) > 0 {
		ps.SwitchToPage(ps.stack[len(ps.stack)-1])
	}
}

// Current returns the name of the current page.
func (ps *PageStack) Current() string {
	if len(ps.stack) == 0 {
		return ""
	}
	return ps.stack[len(ps.stack)-1]
}
