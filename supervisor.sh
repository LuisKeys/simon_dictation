#!/bin/bash

if [[ "$(uname)" == "Darwin" ]]; then
    echo "supervisor.sh is Linux-only — on macOS just run ./main directly." >&2
    exit 1
fi

XBIND_PID=""
MAIN_PID=""
if [[ "$(uname)" == "Linux" ]]; then
    xbindkeys &
    XBIND_PID=$!
fi
# On macOS, the global hotkey is handled by skhd running as its own
# `brew services` daemon (see tools/skhdrc.example) — nothing to launch here.

cleanup() {
    [[ -n "$XBIND_PID" ]] && kill "$XBIND_PID" 2>/dev/null
    [[ -n "$MAIN_PID" ]] && kill "$MAIN_PID" 2>/dev/null
    exit
}

trap cleanup INT TERM

while true; do
    ./main &
    MAIN_PID=$!
    wait "$MAIN_PID"
    EXIT_CODE=$?
    if [[ "$EXIT_CODE" -eq 0 ]]; then
        echo "Process exited cleanly (superseded by a new instance or manual stop). Not restarting." >&2
        break
    fi
    echo "Process crashed with exit code $EXIT_CODE at $(date). Restarting..." >&2
    sleep 1
done