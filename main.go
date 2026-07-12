package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"

	"github.com/joho/godotenv"
	vtt "github.com/luiskeys/simon_dictate/src/vtt"
)

const pidFilePath = "/tmp/simon-dictate.pid"

func toggleDictation(vttsrv *vtt.VTTService) bool {
	newState := vttsrv.ToggleDictation()
	_ = vtt.Notification("Dictation", fmt.Sprintf("Dictation enabled: %v", newState))
	return newState
}

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	// Kill any previously running instance and claim the pidfile for this one.
	if err := acquireSingleInstance(pidFilePath); err != nil {
		log.Fatalf("Failed to acquire single-instance lock: %v", err)
	}

	var wg sync.WaitGroup
	vttsrv := vtt.Init()

	shutdownCh := make(chan os.Signal, 1)
	signal.Notify(shutdownCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-shutdownCh
		log.Println("Shutting down...")
		vttsrv.Shutdown()
		releasePidFile(pidFilePath, os.Getpid())
		os.Exit(0)
	}()

	if runtime.GOOS == "darwin" {
		// No local HTTP server on macOS; toggling is done via SIGUSR1,
		// triggered by a skhd shortcut sending a signal to this PID.
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGUSR1)
		go func() {
			for range sigCh {
				toggleDictation(vttsrv)
			}
		}()
	} else {
		// Start local control HTTP server for toggle/status endpoints
		http.HandleFunc("/toggle-mute", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				http.Error(w, "use POST", http.StatusMethodNotAllowed)
				return
			}
			newState := toggleDictation(vttsrv)
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
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		vttsrv.Run()
	}()

	wg.Wait()
}
