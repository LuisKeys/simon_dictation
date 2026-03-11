# Simon Dictate

A voice dictation tool for converting speech to text for linux.

## Features

- Real-time speech recognition
- English and Spanish language support

## Voice commands:
"English" : Turn to English mode
"Spanish" : Turn to Spanish mode
"Auto" : Switch between mute and unmute

## Requirements

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

## Installation

```bash
# Clone the repository
git clone https://github.com/username/simon_dictate.git
cd simon_dictate
mkdir vtt_models

# Pull ggml-base.bin file from https://huggingface.co/ggerganov/whisper.cpp/tree/main
# Copy that file inside vtt_models

# Verify whisper package metadata is visible
pkg-config --cflags --libs whisper

# Build the app
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

This project includes a small local control HTTP endpoint to toggle the app-level dictation (not system microphone). The endpoint listens on `127.0.0.1:8765` and exposes:

- `POST /toggle-mute` — toggles dictation on/off
- `GET /status` — returns current dictation state as JSON

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