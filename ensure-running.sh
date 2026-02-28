#!/bin/bash
# Ensure spank is running. Called by Claude Code hook on session start.
# Exit silently if already running or if port is occupied.

if curl -s --connect-timeout 1 http://127.0.0.1:19222/hook -X POST -d '{}' > /dev/null 2>&1; then
    exit 0
fi

# Not running — start in background
sudo /usr/local/bin/spank --sound sexy > /tmp/spank-claude.log 2>&1 &
disown

# Wait briefly for server to come up
sleep 1
