#!/bin/bash

xbindkeys &
XBIND_PID=$!

cleanup() {
    kill $XBIND_PID 2>/dev/null
    exit
}

trap cleanup INT TERM

while true; do
    ./main
    EXIT_CODE=$?
    echo "Process crashed with exit code $EXIT_CODE at $(date). Restarting..." >&2
    sleep 1
done