package app

import (
	"github.com/rivo/tview"
)

// PageStack manages a stack of views with push/pop navigation.
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

// Push adds a named page and switches to it.
func (ps *PageStack) Push(name string, page tview.Primitive) {
	ps.stack = append(ps.stack, name)
	ps.AddAndSwitchToPage(name, page, true)
}

// Pop removes the current page and returns to the previous one.
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
