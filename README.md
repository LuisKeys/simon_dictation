# Simon Dictate

A voice dictation tool for converting speech to text. Supports Linux (X11) and macOS.

## Features

- Real-time speech recognition
- English and Spanish language support

## Voice commands:
"English" : Turn to English mode
"Spanish" : Turn to Spanish mode
"Auto" : Switch between mute and unmute
"Agregar nombre <Nombre>" / "Add name <Name>" : Add a name to the capitalization dictionary (single-word or full name)
"Quitar nombre <Nombre>" / "Remove name <Name>" / "Delete name <Name>" : Remove a name from the capitalization dictionary
"Recargar nombres" / "Recarga nombres" / "Reload names" : Reload dictionary files from disk

## Requirements

### Linux (X11)

```bash
sudo apt update
sudo apt install libx11-dev libxtst-dev libxi-dev libxkbcommon-dev libxinerama-dev libgtk-3-dev
apt-get install xdotool
```

Whisper dependencies are resolved via `pkg-config` (`whisper` package), so you must build/install `whisper.cpp` first.

```bash
sudo apt install -y cmake build-essential pkg-config
```

If you want GPU acceleration (recommended for large models):

```bash
cd /home/lucho/projects/ai/whisper.cpp
cmake -B build -DGGML_CUDA=ON -DBUILD_SHARED_LIBS=ON -DCMAKE_BUILD_TYPE=Release
cmake --build build -j
sudo cmake --install build
```

If `pkg-config --cflags --libs whisper` does not work, export `PKG_CONFIG_PATH`:

```bash
export PKG_CONFIG_PATH=/home/lucho/projects/ai/whisper.cpp/build/install/lib/pkgconfig:$PKG_CONFIG_PATH
```

Build the wrapper archive linked by `vtt_whisper.go`, then build the app:

```bash
src/vtt/build_wrapper.sh
go build -o main .
```

### macOS

```bash
brew install portaudio pkg-config cmake
```

Build `whisper.cpp` from source (Metal acceleration is enabled by default on Apple Silicon):

```bash
git clone https://github.com/ggerganov/whisper.cpp
cd whisper.cpp
cmake -B build -DGGML_METAL=ON -DBUILD_SHARED_LIBS=ON -DCMAKE_BUILD_TYPE=Release
cmake --build build -j
cmake --install build
```

If `pkg-config --cflags --libs whisper` does not find it, export `PKG_CONFIG_PATH` to wherever `cmake --install` placed the `.pc` file (e.g. `/usr/local/lib/pkgconfig` or `/opt/homebrew/lib/pkgconfig`).

Build the wrapper archive, then build the app:

```bash
src/vtt/build_wrapper.sh
go build -o main .
```

Grant two permissions before running (System Settings → Privacy & Security):

- **Accessibility** — required for typing dictated text via CGEvent (add the built `./main` binary or the terminal app you launch it from).
- **Microphone** — required for PortAudio/CoreAudio to capture audio.

Without Accessibility, the app runs and transcribes but every typed insert fails silently at the OS level (logged as `cg_type_unicode failed`); grant it and restart `./main`.

## Installation

```bash
# Clone the repository
git clone https://github.com/username/simon_dictate.git
cd simon_dictate
mkdir vtt_models
cd vtt_models

# Pull ggml-base.bin file from https://huggingface.co/ggerganov/whisper.cpp/tree/main
# Copy that file inside vtt_models
curl -L \
  -o ggml-large-v3.bin \
  https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-large-v3.bin

cd ..

# Verify whisper package metadata is visible
pkg-config --cflags --libs whisper

# Build the whisper wrapper archive, then the app
src/vtt/build_wrapper.sh
go build -o main .
```

## Usage

```bash
./main
```

## Troubleshooting

- Error `fatal error: whisper.h: No such file or directory`
	- `whisper.cpp` is not installed (or not installed where `pkg-config` can find it)
	- check: `pkg-config --cflags --libs whisper`
	- if needed, set `PKG_CONFIG_PATH` as shown above and rebuild

- Startup is stuck repeating `Silence threshold too high, retrying noise collection...` and never transcribes
	- the ambient noise floor during startup calibration is above the accepted cap; calibration now retries a bounded number of times, then proceeds anyway, but you can tune it via `.env`:
		```
		VTT_SILENCE_CAP=0.1
		VTT_NOISE_CAL_RETRIES=3
		```
	- `VTT_SILENCE_CAP` (default `0.05`) is the highest calibrated silence threshold accepted without retrying — raise it for a noisier room, lower it for a very quiet one
	- `VTT_NOISE_CAL_RETRIES` (default `3`) is how many times to re-measure before accepting whatever threshold was measured; keep quiet during startup so calibration reflects the room, not your voice

- Keyboard clicks / ambient noise picked up by the mic get transcribed (often as a hallucinated filler word like "gracias")
	- the VAD is letting percussive transients through; tighten it via `.env`:
		```
		VTT_CREST_FACTOR_MAX=4
		VTT_NOISE_GATE=0.02
		VTT_MIN_SPEECH_MS=500
		```
	- `VTT_CREST_FACTOR_MAX` (default `8.0`) makes the percussive-transient rejection gate fire more readily on clicky audio; lower it further if clicks still get through
	- `VTT_NOISE_GATE` (default `0`, disabled) sets a hard RMS floor below which audio is ignored — start low and raise until clicks are gated but your speaking volume is not
	- `VTT_MIN_SPEECH_MS` (default `300`) requires a longer sustained voiced buffer before dispatching to Whisper, filtering out isolated single clicks
	- these are starting points; actual values depend on mic distance/gain, so test and adjust
	- as a backstop, a transcript that is *only* a known hallucinated filler word (currently just "gracias") is dropped entirely rather than typed — see `standaloneHallucinations` in `src/vtt/known_text_filter.go`

## License

MIT License

## Control window

On both Linux and macOS, when you run `./main` a small floating control window titled **Simon** appears near the top-right corner of the screen. It stays on top of other windows and has three buttons:

- **Mute** — toggles dictation on/off (a desktop notification shows the new state). The label reflects button clicks (Mute ↔ Muted); toggling via the "auto" voice command does not update it.
- **EN / ES** — toggles the recognition language between English and Spanish (a desktop notification shows the new language).
- **Exit** — shuts the app down cleanly (closes the audio stream and Whisper model, releases the pidfile). Closing the window has the same effect.

There is no HTTP endpoint and no external hotkey daemon — control is entirely through this window. On Linux it is a GTK3 window (`gui_linux.*`); on macOS an AppKit window (`gui_darwin.*`) that runs as an accessory (no Dock icon, no menu bar).

`supervisor.sh` is Linux-only and will refuse to run on macOS — on macOS, just run `./main` directly. There's no crash-restart supervision on macOS; if it crashes, restart it manually.

## Proper Name Capitalization

The dictation pipeline now applies a deterministic post-processing step to capitalize person names.

Dictionary files (default path `./vtt_models`, override with `VTT_NAMES_DIR`):

- `names_full.txt`: full names, one per line (highest-priority matching)
- `names_first.txt`: optional first names, one per line
- `names_last.txt`: optional last names, one per line
- `names_exceptions.txt`: ambiguous words to keep lowercase by default

Notes:

- Full-name matches are prioritized over exceptions.
- Names can also be added/removed live with voice commands.