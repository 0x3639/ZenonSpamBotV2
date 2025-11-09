# Running Bots as Services

There are two ways to run the bots in the background:

## Option 1: Simple Shell Scripts (Quick & Easy)

### Start the bots:
```bash
# Start receive-bot
cd receive-bot
./startup.sh

# Start send-bot
cd send-bot
./startup.sh
```

### Stop the bots:
```bash
# Stop receive-bot
cd receive-bot
./stop.sh

# Stop send-bot
cd send-bot
./stop.sh
```

### Monitor logs:
```bash
# Receive bot logs
tail -f receive-bot/receive-bot.log
tail -f receive-bot/receive-bot.stdout.log

# Send bot logs
tail -f send-bot/send-bot.log
tail -f send-bot/send-bot.stdout.log
```

### Check if running:
```bash
# Check if receive-bot is running
ps aux | grep receive-bot

# Check if send-bot is running
ps aux | grep send-bot
```

---

## Option 2: Systemd Services (Production-Ready)

Systemd services provide better process management, automatic restarts, and logging integration.

### Install the services:

**Note:** You'll need to edit the service files to match your actual paths. The default paths assume `/root/ZenonSpamBotV2`.

1. **Edit service files** (if needed):
   ```bash
   # Update paths in these files to match your installation
   nano receive-bot/receive-bot.service
   nano send-bot/send-bot.service
   ```

2. **Copy service files to systemd**:
   ```bash
   sudo cp receive-bot/receive-bot.service /etc/systemd/system/
   sudo cp send-bot/send-bot.service /etc/systemd/system/
   ```

3. **Reload systemd**:
   ```bash
   sudo systemctl daemon-reload
   ```

### Using systemd services:

**Start the services:**
```bash
sudo systemctl start receive-bot
sudo systemctl start send-bot
```

**Stop the services:**
```bash
sudo systemctl stop receive-bot
sudo systemctl stop send-bot
```

**Check status:**
```bash
sudo systemctl status receive-bot
sudo systemctl status send-bot
```

**View logs:**
```bash
# View systemd logs
sudo journalctl -u receive-bot -f
sudo journalctl -u send-bot -f

# Or view bot logs directly
tail -f receive-bot/receive-bot.log
tail -f send-bot/send-bot.log
```

**Enable auto-start on boot (optional):**
```bash
sudo systemctl enable receive-bot
sudo systemctl enable send-bot
```

**Disable auto-start:**
```bash
sudo systemctl disable receive-bot
sudo systemctl disable send-bot
```

---

## Log Files

Each bot generates multiple log files:

### Receive Bot:
- `receive-bot.log` - Receive transactions and errors
- `momentum-tracking.log` - Momentum confirmations and producer statistics
- `account-blocks.log` - All account block events
- `receive-bot.stdout.log` - Stdout/stderr from background process (shell scripts only)

### Send Bot:
- `send-bot.log` - Send transactions and errors
- `account-blocks.log` - All account block events
- `send-bot.stdout.log` - Stdout/stderr from background process (shell scripts only)

---

## Troubleshooting

### Shell scripts say bot is already running but it's not:
```bash
# Remove stale PID file
rm receive-bot/receive-bot.pid
rm send-bot/send-bot.pid
```

### Find and kill orphaned processes:
```bash
# Find processes
ps aux | grep -E "(send|receive)-bot" | grep -v grep

# Kill by PID
kill <PID>

# Force kill if needed
kill -9 <PID>
```

### Systemd service won't start:
```bash
# Check service status and logs
sudo systemctl status receive-bot
sudo journalctl -u receive-bot -n 50
```

### Permission issues:
```bash
# Make scripts executable
chmod +x receive-bot/startup.sh receive-bot/stop.sh
chmod +x send-bot/startup.sh send-bot/stop.sh

# Make binaries executable
chmod +x receive-bot/receive-bot send-bot/send-bot
```
