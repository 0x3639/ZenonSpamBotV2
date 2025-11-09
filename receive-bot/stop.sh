#!/bin/bash

# Receive Bot Stop Script
# This script stops the receive-bot

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
cd "$SCRIPT_DIR"

if [ ! -f receive-bot.pid ]; then
    echo "No PID file found. Bot may not be running."
    exit 1
fi

PID=$(cat receive-bot.pid)

if ps -p $PID > /dev/null 2>&1; then
    echo "Stopping receive-bot (PID: $PID)..."
    kill $PID

    # Wait for process to stop (max 10 seconds)
    for i in {1..10}; do
        if ! ps -p $PID > /dev/null 2>&1; then
            echo "Receive bot stopped successfully"
            rm receive-bot.pid
            exit 0
        fi
        sleep 1
    done

    # Force kill if still running
    echo "Process didn't stop gracefully, force killing..."
    kill -9 $PID
    rm receive-bot.pid
    echo "Receive bot force stopped"
else
    echo "Process $PID not found. Removing stale PID file."
    rm receive-bot.pid
fi
