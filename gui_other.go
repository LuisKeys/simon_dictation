//go:build !darwin

package main

import vtt "github.com/luiskeys/simon_dictate/src/vtt"

// runControlUI is a no-op on non-darwin platforms. The floating control
// window is macOS-only; Linux uses the HTTP control server in main().
func runControlUI(_ *vtt.VTTService) {}
