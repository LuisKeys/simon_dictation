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

There are no tests in this repo.

### Build prerequisites (cgo + whisper)

The build links against whisper.cpp, resolved through `pkg-config` (`whisper` package). whisper.cpp must be built/installed first, or `go build` fails with `whisper.h: No such file or directory`. If pkg-config can't find it, set `PKG_CONFIG_PATH` to whisper.cpp's `pkgconfig` dir (see README). Linux system deps: `libx11-dev libxtst-dev libxi-dev libxkbcommon-dev libxinerama-dev libgtk-3-dev xdotool cmake build-essential pkg-config` (`libgtk-3-dev` is for the control window, resolved via `pkg-config gtk+-3.0`). macOS: `brew install portaudio pkg-config cmake` plus whisper.cpp built from source (Metal acceleration via `-DGGML_METAL=ON`). The macOS control window links `-framework Cocoa` (no extra install).

`src/vtt/libwhisper_wrapper_linux.a` (Linux) / `src/vtt/libwhisper_wrapper_darwin.a` (macOS) are prebuilt archives of `whisper_wrapper.cpp`, selected by OS-conditional `#cgo linux`/`#cgo darwin` LDFLAGS directives in `vtt_whisper.go` (`-lstdc++` on Linux, `-lc++` on macOS). Neither is checked into git (`*.a` is gitignored) — run `src/vtt/build_wrapper.sh` to build the one for your OS before `go build`. If you change `whisper_wrapper.cpp` or `.h`, rerun that script — plain `go build` will not rebuild the archive.

A model file must exist at the path in `MODEL` (`.env`), default `./vtt_models/ggml-large-v3.bin`. Pull ggml models from https://huggingface.co/ggerganov/whisper.cpp.

## Architecture

The whole app is one long-running process. `main.go` starts a mute-toggle mechanism and the VTT service:
1. **Control window** — a small native floating window with three buttons (Mute, language EN/ES, Exit), identical on both OSes. No HTTP server. macOS uses an AppKit window (`gui_darwin.go`/`.m`/`.h`, native cgo, `//go:build darwin`); Linux uses a GTK3 window (`gui_linux.go`/`.c`/`.h`, native cgo via `pkg-config gtk+-3.0`, `//go:build linux`). Both expose the same `runControlUI(*vtt.VTTService)` entry point and the same `//export` callbacks (`goOnMuteClicked` → `toggleDictation`, `goOnLangClicked` → `Get/SetLanguage`, `goOnExitClicked` → `gracefulShutdownFor`, also shared with SIGINT/SIGTERM). Because each platform's run loop (`[NSApp run]` / `gtk_main()`) must own the main OS thread, `main()` calls `runtime.LockOSThread()`, runs `vttsrv.Run()` on a goroutine, and hands the main goroutine to `runControlUI` (which blocks in the run loop). `gui_other.go` (`//go:build !darwin && !linux`) is a no-op stub for any other OS.
2. **VTT service** (`vtt.Init().Run()`) — the audio→text pipeline.

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

## Voice commands

Spoken (not typed): "English"/"Spanish"/"Auto" (language & mute), "agregar nombre <N>" / "add name <N>", "quitar nombre <N>" / "remove name <N>", "recargar nombres" / "reload names". See README for the full list.
