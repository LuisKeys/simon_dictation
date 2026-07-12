#!/bin/bash
# Builds Simon Dictate on Linux: (re)builds the whisper wrapper archive and
# then compiles the whole Go package into ./main.
#
# Use this instead of `go build main.go` — that only compiles main.go and
# fails with "undefined: releasePidFile / acquireSingleInstance / runControlUI"
# because those symbols live in the other files of the package.
set -euo pipefail

if [[ "$(uname)" != "Linux" ]]; then
    echo "build_linux.sh is Linux-only. On macOS run src/vtt/build_wrapper.sh && go build -o main ." >&2
    exit 1
fi

cd "$(dirname "$0")"

echo "==> Building whisper wrapper archive..."
bash src/vtt/build_wrapper.sh

echo "==> Building ./main..."
go build -o main .

echo "==> Done. Run ./main or ./supervisor.sh"
