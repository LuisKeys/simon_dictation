package input

import (
	"strings"

	rbo "github.com/go-vgo/robotgo"
)

// sendTextToFocusedWindow types the given text into the currently focused input.
func Send(text string) {
	// Escape any single quotes in text for shell safety
	safeText := strings.ReplaceAll(text, `'`, `'\''`)
	// xdotool command: type text with no delay
	rbo.TypeStr(safeText)
}
