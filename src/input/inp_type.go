package input

import (
	"os/exec"
)

// Send types the given text into the currently focused input using xdotool,
// which correctly handles UTF-8 characters including accented letters.
func Send(text string) {
	exec.Command("xdotool", "type", "--clearmodifiers", "--delay", "10", "--", text).Run()
}
