package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/joho/godotenv"
	vtt "github.com/luiskeys/simon_dictate/src/vtt"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	var wg sync.WaitGroup
	vttsrv := vtt.Init()

	// Start local control HTTP server for toggle/status endpoints
	http.HandleFunc("/toggle-mute", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "use POST", http.StatusMethodNotAllowed)
			return
		}
		newState := vttsrv.ToggleDictation()
		// show a desktop notification for feedback
		_ = vtt.Notification("Dictation", fmt.Sprintf("Dictation enabled: %v", newState))
		resp := map[string]bool{"dictation_enabled": newState}
		_ = json.NewEncoder(w).Encode(resp)
	})

	http.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "use GET", http.StatusMethodNotAllowed)
			return
		}
		resp := map[string]bool{"dictation_enabled": vttsrv.GetDictation()}
		_ = json.NewEncoder(w).Encode(resp)
	})

	go func() {
		log.Println("Control endpoint listening on 127.0.0.1:8765")
		if err := http.ListenAndServe("127.0.0.1:8765", nil); err != nil {
			log.Fatalf("Control server error: %v", err)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		vttsrv.Run()
	}()

	wg.Wait()
}
