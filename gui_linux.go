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
	if toggleLanguage(guiService) {
		return 1
	}
	return 0
}

//export goOnExitClicked
func goOnExitClicked() {
	gracefulShutdownFor(guiService) // never returns (os.Exit)
}

// setMuteLabel syncs the Mute button with the dictation state after a toggle
// that did not originate from the button itself (e.g. the MIDI trigger).
// Safe to call from any goroutine: gui_set_mute_label marshals the update onto
// the GTK main loop via g_idle_add, and no-ops if the window does not exist yet.
func setMuteLabel(enabled bool) {
	state := C.int(0)
	if enabled {
		state = 1
	}
	C.gui_set_mute_label(state)
}

// setLangLabel syncs the EN/ES button after a language change that did not
// originate from the button itself (e.g. the MIDI trigger). Safe to call from
// any goroutine; see setMuteLabel.
func setLangLabel(english bool) {
	state := C.int(0)
	if english {
		state = 1
	}
	C.gui_set_lang_label(state)
}

// runControlUI wires up the service pointer and enters the GTK main loop.
// MUST be called from the main goroutine (with the OS thread locked); it
// blocks forever.
func runControlUI(vttsrv *vtt.VTTService) {
	guiService = vttsrv

	// Last chance to start background workers: gtk_main() below never returns.
	startMidiToggle(vttsrv)

	langIsEnglish := C.int(0)
	if vttsrv.GetLanguage() == "en" {
		langIsEnglish = 1
	}
	C.gui_run(langIsEnglish)
}
