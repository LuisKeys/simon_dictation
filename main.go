package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"
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

// gracefulShutdownFor tears down the service, releases the pidfile and exits.
// Shared by the SIGINT/SIGTERM handler and the macOS "Exit" button.
func gracefulShutdownFor(vttsrv *vtt.VTTService) {
	log.Println("Shutting down...")
	vttsrv.Shutdown()
	releasePidFile(pidFilePath, os.Getpid())
	os.Exit(0)
}

func main() {
	// Pin the main goroutine to the main OS thread. Harmless on Linux; on
	// macOS it is required so [NSApp run] (the Cocoa run loop) owns the main
	// thread.
	runtime.LockOSThread()

	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	// Kill any previously running instance and claim the pidfile for this one.
	if err := acquireSingleInstance(pidFilePath); err != nil {
		log.Fatalf("Failed to acquire single-instance lock: %v", err)
	}

	vttsrv := vtt.Init()

	shutdownCh := make(chan os.Signal, 1)
	signal.Notify(shutdownCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-shutdownCh
		gracefulShutdownFor(vttsrv)
	}()

	// Both macOS and Linux drive mute/language/exit from a small native floating
	// control window (gui_darwin.* / gui_linux.*). The audio pipeline runs on a
	// goroutine while the main goroutine is reserved for the platform run loop
	// (Cocoa's [NSApp run] / GTK's gtk_main), which must own the main OS thread.
	go vttsrv.Run()
	runControlUI(vttsrv) // blocks forever; process exits via os.Exit
}
