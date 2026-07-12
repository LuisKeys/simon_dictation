package input

import (
	"os"
	"time"
)

type sendRequest struct {
	text string
	done chan error
}

var sendQueue chan sendRequest
var keyDelay = "12"       // safer default delay in milliseconds to avoid missed shift/case in some apps; Linux (xdotool) only
var clearModifiers = true // Linux (xdotool) only; no-op on macOS

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
		err := typeText(req.text)
		if req.done != nil {
			req.done <- err
		}
		// small pause between requests to reduce overlap risk
		time.Sleep(5 * time.Millisecond)
	}
}
