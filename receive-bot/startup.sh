#!/bin/bash

# Receive Bot Startup Script
# This script starts the receive-bot in the background

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
cd "$SCRIPT_DIR"

# Check if bot is already running
if [ -f receive-bot.pid ]; then
    PID=$(cat receive-bot.pid)
    if ps -p $PID > /dev/null 2>&1; then
        echo "Receive bot is already running (PID: $PID)"
        exit 1
    else
        echo "Removing stale PID file"
        rm receive-bot.pid
    fi
fi

# Start the bot in background
echo "Starting receive-bot..."
nohup ./receive-bot > receive-bot.stdout.log 2>&1 &
echo $! > receive-bot.pid

echo "Receive bot started (PID: $(cat receive-bot.pid))"
echo "Logs: receive-bot.log, receive-bot.stdout.log"
echo ""
echo "To stop: ./stop.sh"
echo "To view logs: tail -f receive-bot.log"
