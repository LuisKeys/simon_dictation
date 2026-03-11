#!/usr/bin/env bash
# Helper script to toggle dictation by calling local control endpoint
curl -s -X POST http://127.0.0.1:8765/toggle-mute >/dev/null
exit 0
