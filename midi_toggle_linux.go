//go:build linux

package main

import (
	midi "github.com/luiskeys/simon_dictate/src/midi"
	vtt "github.com/luiskeys/simon_dictate/src/vtt"
)

// startMidiToggle launches the optional MIDI mute trigger: pressing the
// configured note on a MIDI controller toggles dictation, exactly as the Mute
// button does. No-op unless VTT_MIDI_TOGGLE is set.
//
// Linux-only for now, which is why the wiring lives here and in gui_linux.go
// rather than in the shared main.go — that keeps the macOS build untouched.
func startMidiToggle(vttsrv *vtt.VTTService) {
	cfg, ok := midi.ConfigFromEnv()
	if !ok {
		return
	}

	go midi.Run(cfg, func() {
		// Keep the button in sync: this toggle did not come from the button.
		setMuteLabel(toggleDictation(vttsrv))
	})
}
