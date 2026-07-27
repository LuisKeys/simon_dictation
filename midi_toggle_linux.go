//go:build linux

package main

import (
	midi "github.com/luiskeys/simon_dictate/src/midi"
	vtt "github.com/luiskeys/simon_dictate/src/vtt"
)

// startMidiToggle launches the optional MIDI triggers: pressing the
// configured mute note toggles dictation (as the Mute button does), and
// pressing the configured language note toggles language (as the EN/ES
// button does, if VTT_MIDI_LANG_NOTE is set). No-op unless VTT_MIDI_TOGGLE is
// set.
//
// Linux-only for now, which is why the wiring lives here and in gui_linux.go
// rather than in the shared main.go — that keeps the macOS build untouched.
func startMidiToggle(vttsrv *vtt.VTTService) {
	cfg, ok := midi.ConfigFromEnv()
	if !ok {
		return
	}

	go midi.Run(cfg, func(note int) {
		// Keep the buttons in sync: these toggles did not come from the buttons.
		switch note {
		case cfg.Note:
			setMuteLabel(toggleDictation(vttsrv))
		case cfg.LangNote:
			setLangLabel(toggleLanguage(vttsrv))
		}
	})
}
