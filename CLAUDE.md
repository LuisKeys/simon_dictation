# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

Simon Dictate is a voice-dictation daemon written in Go, supporting both Linux (X11) and macOS. It captures microphone audio via PortAudio, runs local speech-to-text with whisper.cpp (via a C/C++ cgo wrapper), post-processes the transcript, and types the result into the focused window — via `xdotool` on Linux, or a native CGEvent cgo wrapper on macOS.

## Build & run

```bash
go build main.go        # produces ./main
./main                  # runs the daemon
./supervisor.sh         # Linux only: runs xbindkeys + ./main in a crash-restart loop (production entrypoint). On macOS, run ./main directly instead — supervisor.sh refuses to run there.
```

There are no tests in this repo.

### Build prerequisites (cgo + whisper)

The build links against whisper.cpp, resolved through `pkg-config` (`whisper` package). whisper.cpp must be built/installed first, or `go build` fails with `whisper.h: No such file or directory`. If pkg-config can't find it, set `PKG_CONFIG_PATH` to whisper.cpp's `pkgconfig` dir (see README). Linux system deps: `libx11-dev libxtst-dev libxi-dev libxkbcommon-dev libxinerama-dev xdotool cmake build-essential pkg-config`. macOS: `brew install portaudio pkg-config cmake` plus whisper.cpp built from source (Metal acceleration via `-DGGML_METAL=ON`). The macOS control window links `-framework Cocoa` (no extra install).

`src/vtt/libwhisper_wrapper_linux.a` (Linux) / `src/vtt/libwhisper_wrapper_darwin.a` (macOS) are prebuilt archives of `whisper_wrapper.cpp`, selected by OS-conditional `#cgo linux`/`#cgo darwin` LDFLAGS directives in `vtt_whisper.go` (`-lstdc++` on Linux, `-lc++` on macOS). Neither is checked into git (`*.a` is gitignored) — run `src/vtt/build_wrapper.sh` to build the one for your OS before `go build`. If you change `whisper_wrapper.cpp` or `.h`, rerun that script — plain `go build` will not rebuild the archive.

A model file must exist at the path in `MODEL` (`.env`), default `./vtt_models/ggml-large-v3.bin`. Pull ggml models from https://huggingface.co/ggerganov/whisper.cpp.

## Architecture

The whole app is one long-running process. `main.go` starts a mute-toggle mechanism and the VTT service:
1. **Mute toggle** — OS-conditional in `main.go`. On Linux, a control HTTP server on `127.0.0.1:8765` (`POST /toggle-mute`, `GET /status`); `tools/toggle-mute.sh` binds this to a hotkey via xbindkeys. On macOS, no HTTP server is started — instead a small floating AppKit control window (`gui_darwin.go`/`.m`/`.h`, native cgo, `//go:build darwin`) shows "Mute" and "Exit" buttons. Because the Cocoa run loop (`[NSApp run]`) must own the main OS thread, `main()` calls `runtime.LockOSThread()`, runs `vttsrv.Run()` on a goroutine, and hands the main goroutine to `runControlUI` (which blocks in the run loop); the `!darwin` build gets a no-op `runControlUI` stub (`gui_other.go`). The "Exit" button and SIGINT/SIGTERM share `gracefulShutdownFor`. Button clicks reach Go via `//export goOnMuteClicked`/`goOnExitClicked`.
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
- `VTT_NOISE_GATE`, `VTT_CREST_FACTOR_MAX`, `VTT_MIN_SPEECH_MS`, `VTT_PERIODICITY_MIN` — VAD tuning (0 disables the respective gate where noted in code).
- `VTT_INPUT_DEVICE` — PortAudio input device name.
- `VTT_NAMES_DIR` — override dictionary directory.
- `VTT_CAPITALIZE_SINGLE_NAMES=1` — allow capitalizing single-token names (off by default for precision).
- `VTT_KEY_DELAY_MS`, `VTT_XDOTOOL_CLEAR_MODIFIERS` — xdotool sender tuning (Linux only; no effect on macOS).

## Voice commands

Spoken (not typed): "English"/"Spanish"/"Auto" (language & mute), "agregar nombre <N>" / "add name <N>", "quitar nombre <N>" / "remove name <N>", "recargar nombres" / "reload names". See README for the full list.
