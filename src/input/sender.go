package input

import (
	"log"
	"os"
	"os/exec"
	"time"
)

type sendRequest struct {
	text string
	done chan error
}

var sendQueue chan sendRequest
var keyDelay = "12" // safer default delay in milliseconds to avoid missed shift/case in some apps
var clearModifiers = true

func init() {
	if v := os.Getenv("VTT_KEY_DELAY_MS"); v != "" {
		keyDelay = v
	}
	if os.Getenv("VTT_XDOTOOL_CLEAR_MODIFIERS") == "0" {
		clearModifiers = false
	}

	sendQueue = make(chan sendRequest, 128)
	go senderLoop()
}

// Enqueue encola un envío asíncrono.
func Enqueue(text string) {
	sendQueue <- sendRequest{text: text, done: nil}
}

// SendSync encola y espera hasta que el envío termine. Devuelve error de xdotool si ocurre.
func SendSync(text string) error {
	done := make(chan error, 1)
	sendQueue <- sendRequest{text: text, done: done}
	return <-done
}

func senderLoop() {
	for req := range sendQueue {
		err := runXDoTool(req.text)
		if req.done != nil {
			req.done <- err
		}
		// small pause between requests to reduce overlap risk
		time.Sleep(5 * time.Millisecond)
	}
}

func runXDoTool(text string) error {
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
