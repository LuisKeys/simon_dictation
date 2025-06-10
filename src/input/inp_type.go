package input

import (
	"os/exec"
	"strings"
)

// sendTextToFocusedWindow types the given text into the currently focused input.
func Send(text string) error {
	// Escape any single quotes in text for shell safety
	safeText := strings.ReplaceAll(text, `'`, `'\''`)
	// xdotool command: type text with no delay
	cmd := exec.Command("xdotool", "type", "--delay", "10", safeText)

	return cmd.Run()
}
