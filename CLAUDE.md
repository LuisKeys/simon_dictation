# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

Simon Dictate is a Linux voice-dictation daemon written in Go. It captures microphone audio via PortAudio, runs local speech-to-text with whisper.cpp (via a C/C++ cgo wrapper), post-processes the transcript, and types the result into the focused window using `xdotool`. It is X11-specific (relies on `xdotool` and X11 dev libraries).

## Build & run

```bash
go build main.go        # produces ./main
./main                  # runs the daemon
./supervisor.sh         # runs xbindkeys + ./main in a crash-restart loop (production entrypoint)
```

There are no tests in this repo.

### Build prerequisites (cgo + whisper)

The build links against whisper.cpp, resolved through `pkg-config` (`whisper` package). whisper.cpp must be built/installed first, or `go build` fails with `whisper.h: No such file or directory`. If pkg-config can't find it, set `PKG_CONFIG_PATH` to whisper.cpp's `pkgconfig` dir (see README). System deps: `libx11-dev libxtst-dev libxi-dev libxkbcommon-dev libxinerama-dev xdotool cmake build-essential pkg-config`.

`src/vtt/libwhisper_wrapper.a` and `whisper_wrapper.o` are checked-in prebuilt artifacts of `whisper_wrapper.cpp`. If you change `whisper_wrapper.cpp` or `.h`, you must recompile the wrapper and rebuild the static archive — plain `go build` will not do it (the cgo directives in `vtt_whisper.go` link `-lwhisper_wrapper` from `src/vtt/`).

A model file must exist at the path in `MODEL` (`.env`), default `./vtt_models/ggml-large-v3.bin`. Pull ggml models from https://huggingface.co/ggerganov/whisper.cpp.

## Architecture

The whole app is one long-running process. `main.go` starts two goroutines:
1. **Control HTTP server** on `127.0.0.1:8765` — `POST /toggle-mute` (toggle dictation on/off) and `GET /status` (JSON state). `tools/toggle-mute.sh` + xbindkeys bind this to a hotkey.
2. **VTT service** (`vtt.Init().Run()`) — the audio→text pipeline.

### The audio pipeline (`src/vtt/`)

`VTTService` (defined in `vtt_srv_ent.go`) holds all state and config. `NewVTTSrv()` reads tuning knobs from env vars (see below).

Flow, all in `vtt_service.go`:
- `Listen()` — reads PortAudio frames, runs a **voice-activity detection (VAD) chain**: noise gate → 300–3400 Hz bandpass biquad filter → RMS/adaptive-noise silence threshold → crest-factor (percussive-transient rejection) → periodicity/autocorrelation (voice vs. breath). Buffers speech until `silenceDuration` of silence, then calls `dispatch()`.
- `dispatch()` — runs Whisper transcription in a goroutine, then the **text post-processing chain**: `normalizeText` → `knownTextFilter.Apply` (strip known Whisper hallucination phrases) → `nameCapitalizer.Apply` (proper-name capitalization) → `Commands()`. If the text is a recognized voice command it is consumed; otherwise it is typed via `input.Send`.

### Supporting units

- `vtt_whisper.go` — cgo bridge to whisper.cpp. **This is the only cgo file**; the `import "C"` block carries the compiler/linker directives.
- `vtt_commands.go` — voice command parser (language switch, dictation toggle, live add/remove/reload of names). Commands are matched against the transcript text, not keystrokes.
- `name_capitalizer.go` — deterministic proper-name capitalizer backed by dictionary files in `./vtt_models` (`names_full.txt`, `names_first.txt`, `names_last.txt`, `names_exceptions.txt`; override dir with `VTT_NAMES_DIR`). Full-name matches beat exceptions. Thread-safe (RWMutex) because voice commands mutate it live.
- `known_text_filter.go` — drops recurring Whisper artifact phrases from the output.
- `src/input/sender.go` — serialized `xdotool type` sender. All output goes through a single-goroutine queue (`Enqueue`/`SendSync`) to preserve ordering; `keyDelay` guards against dropped shift/case in some apps.

### Concurrency notes

`VTTService` state is guarded by `mutex` (RWMutex). Transcription runs in a detached goroutine per utterance (audio slice is copied to avoid races). Text output ordering is preserved by the single sender goroutine, and commands use blocking `SendSync` to stay ordered with state changes.

## Configuration (env vars, via `.env` or environment)

- `MODEL` — whisper model path.
- `VTT_NOISE_GATE`, `VTT_CREST_FACTOR_MAX`, `VTT_MIN_SPEECH_MS`, `VTT_PERIODICITY_MIN` — VAD tuning (0 disables the respective gate where noted in code).
- `VTT_INPUT_DEVICE` — PortAudio input device name.
- `VTT_NAMES_DIR` — override dictionary directory.
- `VTT_CAPITALIZE_SINGLE_NAMES=1` — allow capitalizing single-token names (off by default for precision).
- `VTT_KEY_DELAY_MS`, `VTT_XDOTOOL_CLEAR_MODIFIERS` — xdotool sender tuning.

## Voice commands

Spoken (not typed): "English"/"Spanish"/"Auto" (language & mute), "agregar nombre <N>" / "add name <N>", "quitar nombre <N>" / "remove name <N>", "recargar nombres" / "reload names". See README for the full list.
