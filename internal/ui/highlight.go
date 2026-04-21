package ui

import (
	"regexp"
	"strings"
)

// Highlight wraps matching parts of text with tview color tags for highlighting.
// Returns the text with matches wrapped in reverse video.
func Highlight(text string, filter string) string {
	if filter == "" {
		return text
	}

	re, err := regexp.Compile("(?i)(" + filter + ")")
	if err != nil {
		// Fallback to case-insensitive substring
		lower := strings.ToLower(text)
		lowerFilter := strings.ToLower(filter)
		idx := strings.Index(lower, lowerFilter)
		if idx < 0 {
			return text
		}
		match := text[idx : idx+len(filter)]
		return text[:idx] + "[::r]" + match + "[::-]" + text[idx+len(filter):]
	}

	return re.ReplaceAllString(text, "[#000000:#FFFF64]${1}[-:-]")
}
