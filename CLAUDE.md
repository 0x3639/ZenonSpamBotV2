# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

ZenonSpamBotv2 is a Zenon Network transaction testing tool consisting of two independent Go applications:
- **send-bot**: Sends configurable batches of transactions to a specified address
- **receive-bot**: Monitors and automatically receives incoming transactions

Both bots use the Zenon SDK (`github.com/MoonBaZZe/znn-sdk-go`) to interact with the Zenon blockchain via WebSocket RPC.

## Architecture

### Two-Bot System

Each bot is a standalone application in its own directory with separate configuration:

1. **send-bot/** - Transaction sender
   - Loads wallet, auto-receives pending transactions, then sends transactions in batches
   - Configured via `.env` file with sending parameters (recipient, token, amount, rate, duration)
   - Logs to `send-bot.log`

2. **receive-bot/** - Transaction receiver
   - Auto-receives all pending transactions on startup
   - Subscribes to unreceived transaction notifications via WebSocket for immediate receives
   - Subscribes to account blocks to track block publication and confirmation
   - Subscribes to momentums to track which pillar producers confirm transactions
   - Stops when all received transactions are confirmed or after timeout
   - Configured via `.env` file with momentum confirmation timeout
   - Logs to `receive-bot.log` and `momentum-tracking.log`

### Shared Code Patterns

Both applications share:
- `.env` file parsing (simple KEY=VALUE parser in `loadEnv()`)
- Wallet management (creates new wallet if missing, loads existing wallet)
- Zenon SDK client initialization and connection lifecycle
- Structured logging to both console and log files
- `--seed` flag to display wallet address and mnemonic

### Wallet Management

- Wallets stored in `~/.znn/wallet/` (from `node.DefaultDataDir()`)
- On first run, generates new wallet and displays mnemonic (MUST BE SAVED)
- Subsequent runs load existing wallet with password
- Each bot uses its own wallet (different `WALLET_NAME` in `.env`)

### Transaction Flow

**Send Bot:**
1. Auto-receives any pending transactions (calls `autoReceive()`)
2. Validates balance for configured duration (note: SDK v0.1.0 has bug in `GetAccountInfoByAddress`)
3. Sends transactions in batches: `TXS_PER_INTERVAL` transactions every `INTERVAL_SECONDS`
4. Runs for `DURATION_SECONDS` (or indefinitely if 0)

**Receive Bot:**
1. Auto-receives all pending unreceived transactions on startup
2. Subscribes to three event streams:
   - `ToUnreceivedAccountBlocksByAddress`: Immediately notified when new transactions arrive
   - `ToAccountBlocksByAddress`: Tracks when blocks are published and confirmed
   - `ToMomentums`: Tracks which pillar producers include transactions in momentums
3. For each unreceived block notification, immediately creates and sends receive transaction
4. Monitors transaction confirmations in momentums and tracks producer statistics
5. Stops when either:
   - All received transactions are confirmed in momentums, OR
   - `MOMENTUM_CONFIRMATION_TIMEOUT_SECONDS` passes since last received transaction
6. Logs momentum tracking data including producer addresses to `momentum-tracking.log`

### Amount Handling

Zenon uses 8 decimal places. The send-bot's `parseAmount()` converts decimal strings (e.g., "1.5") to base units (150000000).

### Known SDK Limitations

- SDK v0.1.0 has a bug in `GetAccountInfoByAddress` (calls wrong RPC method)
- Balance validation may fail, but bot proceeds if account exists (frontier block check)
- Comment in send-bot/main.go:484-486 documents this workaround

## Commands

### Build Both Bots
```bash
# From project root
cd send-bot && go build && cd ..
cd receive-bot && go build && cd ..
```

### Run Send Bot
```bash
cd send-bot
cp .env.example .env
# Edit .env with your configuration
go run main.go
# Or run compiled binary:
./send-bot
```

### Run Receive Bot
```bash
cd receive-bot
cp .env.example .env
# Edit .env with your configuration
go run main.go
# Or run compiled binary:
./receive-bot
```

### Display Wallet Info
```bash
# In either bot directory
go run main.go --seed
# Or with compiled binary:
./send-bot --seed
./receive-bot --seed
```

### Typical Development Workflow
```bash
# Build and test changes
cd send-bot
go build
./send-bot

# View logs in real-time
tail -f send-bot.log
```

## Configuration

Both bots require `.env` files (copy from `.env.example`):

**send-bot/.env:**
- `WALLET_NAME`, `PASSWORD`: Wallet credentials
- `RPC_URL`: WebSocket endpoint (e.g., `ws://127.0.0.1:35998`)
- `SEND_TO_ADDRESS`: Recipient address
- `ZTS_TOKEN_ADDRESS`: Token to send (ZNN is `zts1znnxxxxxxxxxxxxx9z4ulx`)
- `AMOUNT_PER_TX`: Decimal amount per transaction (e.g., "1.0")
- `TXS_PER_INTERVAL`: Number of transactions per interval
- `INTERVAL_SECONDS`: Seconds between batches
- `DURATION_SECONDS`: Total runtime (0 = indefinite)

**receive-bot/.env:**
- `WALLET_NAME`, `PASSWORD`: Wallet credentials
- `RPC_URL`: WebSocket endpoint
- `MOMENTUM_CONFIRMATION_TIMEOUT_SECONDS`: Maximum seconds to wait since last received transaction before stopping (default: 120)

## Dependencies

Main dependency: `github.com/MoonBaZZe/znn-sdk-go v0.1.0`
- Provides Zenon client (`zenon.NewZenon()`)
- Wallet management (`wallet` package)
- Account block types (`nom.AccountBlock`)

All dependencies are managed via Go modules (go.mod/go.sum).

## Testing Approach

For local testing, run both bots simultaneously:
1. Start receive-bot with wallet A
2. Start send-bot with wallet B, configured to send to wallet A's address
3. Monitor logs to verify transactions are sent and received

The bots are designed for stress-testing the Zenon network with configurable transaction rates.
