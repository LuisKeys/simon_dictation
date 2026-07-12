#!/bin/bash

if [[ "$(uname)" == "Darwin" ]]; then
    echo "supervisor.sh is Linux-only — on macOS just run ./main directly." >&2
    exit 1
fi

MAIN_PID=""
# Mute/language/exit are driven by the app's own floating control window on
# both Linux (GTK) and macOS (AppKit) — there is no external hotkey daemon to
# launch here.

cleanup() {
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