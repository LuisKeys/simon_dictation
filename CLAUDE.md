# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

Simon Dictate is a voice-dictation daemon written in Go, supporting both Linux (X11) and macOS. It captures microphone audio via PortAudio, runs local speech-to-text with whisper.cpp (via a C/C++ cgo wrapper), post-processes the transcript, and types the result into the focused window — via `xdotool` on Linux, or a native CGEvent cgo wrapper on macOS.

## Build & run

```bash
go build -o main .      # produces ./main (builds the whole package; `go build main.go` compiles only that one file and fails)
./main                  # runs the daemon
./supervisor.sh         # Linux only: runs ./main in a crash-restart loop (production entrypoint). On macOS, run ./main directly instead — supervisor.sh refuses to run there.
```

The only tested package is `src/midi` (`go test ./src/midi/`); nothing else in the repo has tests.

### Build prerequisites (cgo + whisper)

The build links against whisper.cpp, resolved through `pkg-config` (`whisper` package). whisper.cpp must be built/installed first, or `go build` fails with `whisper.h: No such file or directory`. If pkg-config can't find it, set `PKG_CONFIG_PATH` to whisper.cpp's `pkgconfig` dir (see README). Linux system deps: `libx11-dev libxtst-dev libxi-dev libxkbcommon-dev libxinerama-dev libgtk-3-dev xdotool cmake build-essential pkg-config` (`libgtk-3-dev` is for the control window, resolved via `pkg-config gtk+-3.0`). macOS: `brew install portaudio pkg-config cmake` plus whisper.cpp built from source (Metal acceleration via `-DGGML_METAL=ON`). The macOS control window links `-framework Cocoa` (no extra install).

`src/vtt/libwhisper_wrapper_linux.a` (Linux) / `src/vtt/libwhisper_wrapper_darwin.a` (macOS) are prebuilt archives of `whisper_wrapper.cpp`, selected by OS-conditional `#cgo linux`/`#cgo darwin` LDFLAGS directives in `vtt_whisper.go` (`-lstdc++` on Linux, `-lc++` on macOS). Neither is checked into git (`*.a` is gitignored) — run `src/vtt/build_wrapper.sh` to build the one for your OS before `go build`. If you change `whisper_wrapper.cpp` or `.h`, rerun that script — plain `go build` will not rebuild the archive.

A model file must exist at the path in `MODEL` (`.env`), default `./vtt_models/ggml-large-v3.bin`. Pull ggml models from https://huggingface.co/ggerganov/whisper.cpp.

## Architecture

The whole app is one long-running process. `main.go` starts a mute-toggle mechanism and the VTT service:
1. **Control window** — a small native floating window with three buttons (Mute, language EN/ES, Exit), identical on both OSes. No HTTP server. macOS uses an AppKit window (`gui_darwin.go`/`.m`/`.h`, native cgo, `//go:build darwin`); Linux uses a GTK3 window (`gui_linux.go`/`.c`/`.h`, native cgo via `pkg-config gtk+-3.0`, `//go:build linux`). Both expose the same `runControlUI(*vtt.VTTService)` entry point and the same `//export` callbacks (`goOnMuteClicked` → `toggleDictation`, `goOnLangClicked` → `Get/SetLanguage`, `goOnExitClicked` → `gracefulShutdownFor`, also shared with SIGINT/SIGTERM). Because each platform's run loop (`[NSApp run]` / `gtk_main()`) must own the main OS thread, `main()` calls `runtime.LockOSThread()`, runs `vttsrv.Run()` on a goroutine, and hands the main goroutine to `runControlUI` (which blocks in the run loop). `gui_other.go` (`//go:build !darwin && !linux`) is a no-op stub for any other OS.
2. **VTT service** (`vtt.Init().Run()`) — the audio→text pipeline.
3. **MIDI mute toggle** (Linux only, opt-in) — on Linux, `runControlUI` calls `startMidiToggle` (`midi_toggle_linux.go`) just before entering `gtk_main()`, which is the last chance to start background workers. It is a no-op unless `VTT_MIDI_TOGGLE` is set. See `src/midi/` below. The wiring deliberately lives in `gui_linux.go`/`midi_toggle_linux.go` rather than the shared `main.go`, so the macOS build needs no stubs and is untouched.

### The audio pipeline (`src/vtt/`)

`VTTService` (defined in `vtt_srv_ent.go`) holds all state and config. `NewVTTSrv()` reads tuning knobs from env vars (see below).

Flow, all in `vtt_service.go`:
- `Listen()` — reads PortAudio frames, runs a **voice-activity detection (VAD) chain**: noise gate → 300–3400 Hz bandpass biquad filter → RMS/adaptive-noise silence threshold → crest-factor (percussive-transient rejection) → periodicity/autocorrelation (voice vs. breath). Buffers speech until `silenceDuration` of silence, then calls `dispatch()`.
- `dispatch()` — runs Whisper transcription in a goroutine, then the **text post-processing chain**: `normalizeText` → `knownTextFilter.Apply` (strip known Whisper hallucination phrases) → `nameCapitalizer.Apply` (proper-name capitalization) → `Commands()`. If the text is a recognized voice command it is consumed; otherwise it is typed via `input.Send`.

### Supporting units

- `vtt_whisper.go` — cgo bridge to whisper.cpp, with OS-conditional `#cgo linux`/`#cgo darwin` LDFLAGS. This and the `src/input` sender files are the only cgo in the repo.
- `vtt_commands.go` — voice command parser (language switch, dictation toggle, live add/remove/reload of names). Commands are matched against the transcript text, not keystrokes.
- `name_capitalizer.go` — deterministic proper-name capitalizer backed by dictionary files in `./vtt_models` (`names_full.txt`, `names_first.txt`, `names_last.txt`, `names_exceptions.txt`; override dir with `VTT_NAMES_DIR`). Full-name matches beat exceptions. Thread-safe (RWMutex) because voice commands mutate it live.
- `known_text_filter.go` — drops recurring Whisper artifact phrases from the output.
- `src/midi/listener.go` (`//go:build linux`) — optional MIDI-note trigger for mute/unmute. Reads the ALSA rawmidi character device (`/dev/snd/midiC<card>D<sub>`) directly with `os.Open`, so it needs **no cgo and no ALSA/rtmidi/portmidi dependency**. Three parts: (1) `resolveDevice` maps a card-name fragment to a node via `/proc/asound/cards`, re-resolved on every connection attempt because ALSA card numbers shift when a USB device is replugged; (2) `Run` is a reconnect loop with 1s→30s backoff that survives an unplugged device or an `EBUSY` from a DAW holding the exclusive-open node, logging each failure once instead of per retry; (3) `parser` is a byte-at-a-time state machine handling running status, note-on-with-velocity-0-as-note-off, interleaved System Real-Time bytes (notably Active Sensing `0xFE`), SysEx skipping, and per-type data lengths. Linux-only for now — the parser is portable if CoreMIDI support is added. **This is the only tested unit in the repo** (`src/midi/listener_test.go`: parser table tests plus an end-to-end run over a FIFO, which works because `resolveDevice` accepts any absolute path).
- `src/input/sender.go` — serialized text sender, OS-agnostic queue (`Enqueue`/`SendSync`) preserving output ordering. The actual typing call (`typeText`) is platform-specific: `sender_linux.go` shells out to `xdotool type` (`keyDelay` guards against dropped shift/case in some apps); `sender_darwin.go` posts a native CGEvent via the `cg_events_darwin.c`/`.h` cgo wrapper (whole-string Unicode post, no per-key delay needed).

### Concurrency notes

`VTTService` state is guarded by `mutex` (RWMutex). Transcription runs in a detached goroutine per utterance (audio slice is copied to avoid races). Text output ordering is preserved by the single sender goroutine, and commands use blocking `SendSync` to stay ordered with state changes.

## Configuration (env vars, via `.env` or environment)

- `MODEL` — whisper model path.
- `VTT_INPUT_GAIN` (default 1.0) — input gain multiplier applied to raw mic samples in `processAudio` (before the bandpass filter), clamped to `[-1, 1]`. Raise it (e.g. 3–5) if you have to speak loudly for detection to trigger — quiet/low-level microphones make the whole VAD chain see levels too low to clear the adaptive silence threshold. Lower it if speech clips or background noise leaks through. `1.0` is a no-op.
- `VTT_NOISE_GATE`, `VTT_CREST_FACTOR_MAX`, `VTT_MIN_SPEECH_MS`, `VTT_PERIODICITY_MIN` — VAD tuning (0 disables the respective gate where noted in code).
- `VTT_SILENCE_CAP` (default 0.05), `VTT_NOISE_CAL_RETRIES` (default 3) — startup noise-calibration tuning. Calibration retries if the measured silence threshold exceeds `VTT_SILENCE_CAP` (likely speech during calibration), up to `VTT_NOISE_CAL_RETRIES` times, then accepts the measured value so the daemon always starts.
- `VTT_SILENCE_MULT` (default 15.0) — multiplier applied to `(mean + 2*stddev)` when computing the adaptive silence threshold at calibration. Lower it (e.g. 8–10) if quiet/short words (like "Hugo") never trigger detection; raise it if background noise leaks through.
- `VTT_VAD_DEBUG` (unset/`0` = off) — diagnostics. When set, logs per-frame VAD metrics (rms, threshold, crest factor, ZCR, periodicity, per-gate results) and dumps each dispatched utterance to a 16 kHz mono WAV under `./vad_debug/` (override dir with `VTT_VAD_DEBUG_DIR`). Off by default; no production impact.
- `VTT_INPUT_DEVICE` — PortAudio input device name.
- `VTT_NAMES_DIR` — override dictionary directory.
- `VTT_CAPITALIZE_SINGLE_NAMES=1` — allow capitalizing single-token names (off by default for precision).
- `VTT_KEY_DELAY_MS`, `VTT_XDOTOOL_CLEAR_MODIFIERS` — xdotool sender tuning (Linux only; no effect on macOS).

### MIDI mute toggle (Linux only; no effect on macOS)

Pressing a configurable note on a MIDI controller toggles dictation, the same as clicking **Mute**. Opt-in: with `VTT_MIDI_TOGGLE` unset, no MIDI device is ever opened and behaviour is unchanged.

- `VTT_MIDI_TOGGLE` (unset = off) — `1` enables the listener.
- `VTT_MIDI_NOTE` (default 60) — MIDI note number that toggles mute. 60 is middle C (Do central). Out-of-range values (not 0–127) are ignored in favour of the default.
- `VTT_MIDI_DEVICE` (unset = autodetect) — a card-name fragment matched case-insensitively against the short id and long name in `/proc/asound/cards` (e.g. `mk3`, `MicroLab`, `Arturia` all resolve to the same card), or an absolute rawmidi path (`/dev/snd/midiC4D0`). Prefer the name: card numbers are reassigned on replug. Unset takes the first `/dev/snd/midiC*D*` in sorted order.
- `VTT_MIDI_CHANNEL` (unset = any) — restrict to one MIDI channel, `0`–`15`.
- `VTT_MIDI_DEBOUNCE_MS` (default 250) — minimum interval between triggers, so one key press is one toggle even with key bounce or a retriggering held key.
- `VTT_MIDI_DEBUG` (unset/`0` = off) — logs every parsed note-on (`MIDI: note-on ch=0 note=60 vel=97`). This is how you discover the note number of a given key: enable it, press the key, read the log, then set `VTT_MIDI_NOTE`.

A toggle from MIDI updates the GTK Mute button through `setMuteLabel` (`gui_linux.go`) → `gui_set_mute_label` (`gui_linux.c`), which marshals onto the GTK main loop with `g_idle_add` and is therefore safe to call from the listener goroutine.

## Voice commands

Spoken (not typed): "English"/"Spanish"/"Auto" (language & mute), "agregar nombre <N>" / "add name <N>", "quitar nombre <N>" / "remove name <N>", "recargar nombres" / "reload names". See README for the full list.
