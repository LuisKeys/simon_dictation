package input

import (
	"log"
	"os/exec"
	"time"
)

type sendRequest struct {
	text string
	done chan error
}

var sendQueue chan sendRequest
var keyDelay = "1"

func init() {
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
	cmd := exec.Command("xdotool", "type", "--clearmodifiers", "--delay", keyDelay, "--", text)
	err := cmd.Run()
	if err != nil {
		log.Printf("xdotool error: %v", err)
	}
	return err
}
