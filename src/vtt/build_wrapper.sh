#!/bin/bash
# Builds src/vtt/whisper_wrapper.cpp into the static archive linked by the
# cgo directives in vtt_whisper.go. whisper.cpp (and its pkg-config file)
# must already be installed and discoverable via PKG_CONFIG_PATH.
set -euo pipefail

cd "$(dirname "$0")"

OS="$(uname)"
case "$OS" in
    Linux)
        CXX=g++
        OUT=libwhisper_wrapper_linux.a
        ;;
    Darwin)
        CXX=clang++
        OUT=libwhisper_wrapper_darwin.a
        ;;
    *)
        echo "Unsupported OS: $OS" >&2
        exit 1
        ;;
esac

WHISPER_CFLAGS="$(pkg-config --cflags whisper)"

"$CXX" -std=c++17 -c $WHISPER_CFLAGS -o whisper_wrapper.o whisper_wrapper.cpp
ar rcs "$OUT" whisper_wrapper.o
rm -f whisper_wrapper.o

echo "Built $OUT"
