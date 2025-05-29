#!/bin/bash
while true; do
    ./main
    echo "Process crashed with exit code $? at $(date). Restarting..." >&2
    sleep 1
done