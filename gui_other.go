//go:build !darwin && !linux

package main

import vtt "github.com/luiskeys/simon_dictate/src/vtt"

// runControlUI is a no-op on platforms without a native control window. macOS
// uses an AppKit window (gui_darwin.*) and Linux a GTK3 window (gui_linux.*);
// any other OS simply runs the audio pipeline with no UI.
func runControlUI(_ *vtt.VTTService) {}
