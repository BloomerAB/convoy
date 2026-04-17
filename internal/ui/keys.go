package ui

import "github.com/gdamore/tcell/v2"

// Key bindings
var (
	KeyQuit      = tcell.KeyCtrlC
	KeyEscape    = tcell.KeyEscape
	KeyEnter     = tcell.KeyEnter
	KeyRefresh   = tcell.NewEventKey(tcell.KeyRune, 'r', tcell.ModNone)
	KeyHelp      = tcell.NewEventKey(tcell.KeyRune, '?', tcell.ModNone)
	KeyCommand   = tcell.NewEventKey(tcell.KeyRune, ':', tcell.ModNone)
	KeyFilter    = tcell.NewEventKey(tcell.KeyRune, '/', tcell.ModNone)
)
