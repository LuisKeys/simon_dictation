# Simon Dictate

A voice dictation tool for converting speech to text for linux.

## Features

- Real-time speech recognition
- English and Spanish language support

## Voice commands:
"English" : Turn to English mode
"Spanish" : Turn to Spanish mode
"Origami" : Switch between mute and unmute

## Requirements

```bash
sudo apt update
sudo apt install libx11-dev libxtst-dev libxi-dev libxkbcommon-dev libxinerama-dev
apt-get install xdotool
```

## Installation

```bash
# Clone the repository
git clone https://github.com/username/simon_dictate.git
cd simon_dictate
mkdir vtt_models

# Pull ggml-base.bin file from https://huggingface.co/ggerganov/whisper.cpp/tree/main
# Copy that file inside vtt_models

# Build the app
go build /main.go
```

## Usage

```bash
./main
```

## License

MIT License