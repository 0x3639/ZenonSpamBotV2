#!/bin/bash

# Send Bot Stop Script
# This script stops the send-bot

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
cd "$SCRIPT_DIR"

if [ ! -f send-bot.pid ]; then
    echo "No PID file found. Bot may not be running."
    exit 1
fi

PID=$(cat send-bot.pid)

if ps -p $PID > /dev/null 2>&1; then
    echo "Stopping send-bot (PID: $PID)..."
    kill $PID

    # Wait for process to stop (max 10 seconds)
    for i in {1..10}; do
        if ! ps -p $PID > /dev/null 2>&1; then
            echo "Send bot stopped successfully"
            rm send-bot.pid
            exit 0
        fi
        sleep 1
    done

    # Force kill if still running
    echo "Process didn't stop gracefully, force killing..."
    kill -9 $PID
    rm send-bot.pid
    echo "Send bot force stopped"
else
    echo "Process $PID not found. Removing stale PID file."
    rm send-bot.pid
fi
