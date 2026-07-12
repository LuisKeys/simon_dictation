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
sudo apt install libx11-dev libxtst-dev libxi-dev libxkbcommon-dev libxinerama-dev
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
go build main.go
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
go build main.go
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
go build main.go
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

## License

MIT License

## Hotkey / Mute shortcut

On Linux this project includes a small local control HTTP endpoint to toggle the app-level dictation (not system microphone). The endpoint listens on `127.0.0.1:8765` and exposes:

- `POST /toggle-mute` — toggles dictation on/off
- `GET /status` — returns current dictation state as JSON

On macOS there is no HTTP endpoint and no keyboard shortcut: a small floating control window handles muting and quitting (see the macOS section below).

### Linux

Quick setup using `xbindkeys` (recommended):

1. Make sure the app is running (`./main`).
2. Copy `tools/toggle-mute.sh` somewhere and make it executable:

```bash
chmod +x tools/toggle-mute.sh
```

3. Install and configure `xbindkeys` (`sudo apt install xbindkeys`). Add an entry to `~/.xbindkeysrc` (see `tools/xbindkeys.example`) mapping `Alt+Ctrl+Shift+M` to run the script.

4. Start `xbindkeys`:

```bash
xbindkeys
```

Press `Alt+Ctrl+Shift+M` to toggle dictation. A desktop notification will show the new state.

`supervisor.sh` launches and manages `xbindkeys` automatically alongside `./main` on Linux.

### macOS

No hotkey or `skhd` setup is needed. When you run `./main`, a small floating window titled **Simon** appears near the top-right corner of the screen. It stays on top of other windows and can be dragged by its title bar. It has two buttons:

- **Mute** — toggles dictation on/off (a desktop notification shows the new state). The label reflects button clicks; toggling via the "auto" voice command does not update it.
- **Exit** — shuts the app down cleanly (closes the audio stream and Whisper model, releases the pidfile).

The app runs as an accessory (no Dock icon, no menu bar). Since there is no HTTP endpoint on macOS, `tools/toggle-mute.sh` is Linux-only.

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