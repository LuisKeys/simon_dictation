//go:build linux

package main

/*
#cgo pkg-config: gtk+-3.0
#include "gui_linux.h"
*/
import "C"

import (
	vtt "github.com/luiskeys/simon_dictate/src/vtt"
)

// guiService is set once, on the main goroutine, before gui_run() is entered.
// The C button callbacks run on that same (main) thread, so reading it is
// race-free; VTTService methods are themselves mutex-guarded.
var guiService *vtt.VTTService

//export goOnMuteClicked
func goOnMuteClicked() C.int {
	if toggleDictation(guiService) {
		return 1
	}
	return 0
}

//export goOnLangClicked
func goOnLangClicked() C.int {
	if guiService.GetLanguage() == "en" {
		guiService.SetLanguage("es")
		_ = vtt.Notification("Dictation", "Language: Spanish")
		return 0
	}
	guiService.SetLanguage("en")
	_ = vtt.Notification("Dictation", "Language: English")
	return 1
}

//export goOnExitClicked
func goOnExitClicked() {
	gracefulShutdownFor(guiService) // never returns (os.Exit)
}

// runControlUI wires up the service pointer and enters the GTK main loop.
// MUST be called from the main goroutine (with the OS thread locked); it
// blocks forever.
func runControlUI(vttsrv *vtt.VTTService) {
	guiService = vttsrv
	langIsEnglish := C.int(0)
	if vttsrv.GetLanguage() == "en" {
		langIsEnglish = 1
	}
	C.gui_run(langIsEnglish)
}
