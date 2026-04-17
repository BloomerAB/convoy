package ui

import "github.com/gdamore/tcell/v2"

var (
	ColorReady       = tcell.NewRGBColor(100, 255, 100)
	ColorFailed      = tcell.NewRGBColor(255, 80, 80)
	ColorProgressing = tcell.NewRGBColor(255, 255, 100)
	ColorSuspended   = tcell.NewRGBColor(150, 150, 180)
	ColorUnknown     = tcell.ColorWhite
	ColorHeader      = tcell.ColorCornflowerBlue
	ColorTitle       = tcell.ColorWhite
	ColorHint        = tcell.ColorDarkCyan
	ColorStale       = tcell.ColorOrange
)
