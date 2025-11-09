#!/bin/bash

# Send Bot Startup Script
# This script starts the send-bot in the background

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
cd "$SCRIPT_DIR"

# Check if bot is already running
if [ -f send-bot.pid ]; then
    PID=$(cat send-bot.pid)
    if ps -p $PID > /dev/null 2>&1; then
        echo "Send bot is already running (PID: $PID)"
        exit 1
    else
        echo "Removing stale PID file"
        rm send-bot.pid
    fi
fi

# Start the bot in background
echo "Starting send-bot..."
nohup ./send-bot > send-bot.stdout.log 2>&1 &
echo $! > send-bot.pid

echo "Send bot started (PID: $(cat send-bot.pid))"
echo "Logs: send-bot.log, send-bot.stdout.log"
echo ""
echo "To stop: ./stop.sh"
echo "To view logs: tail -f send-bot.log"
