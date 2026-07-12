//go:build linux

package input

import (
	"log"
	"os/exec"
)

func typeText(text string) error {
	log.Printf("typing: %q", text)
	args := []string{"type", "--delay", keyDelay}
	if clearModifiers {
		args = append(args, "--clearmodifiers")
	}
	args = append(args, "--", text)
	cmd := exec.Command("xdotool", args...)
	err := cmd.Run()
	if err != nil {
		log.Printf("xdotool error: %v", err)
	}
	return err
}
