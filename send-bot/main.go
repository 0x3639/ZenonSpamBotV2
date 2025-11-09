package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"math/big"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/MoonBaZZe/znn-sdk-go/wallet"
	"github.com/MoonBaZZe/znn-sdk-go/zenon"
	"github.com/zenon-network/go-zenon/chain/nom"
	"github.com/zenon-network/go-zenon/common/types"
	"github.com/zenon-network/go-zenon/node"
	"github.com/zenon-network/go-zenon/rpc/api"
)

type Config struct {
	WalletName      string
	Password        string
	RpcURL          string
	SendToAddress   types.Address
	TokenAddress    types.ZenonTokenStandard
	AmountPerTx     *big.Int
	TxsPerInterval  int
	IntervalSeconds int
	DurationSeconds int
}

func loadEnv(filename string) (map[string]string, error) {
	env := make(map[string]string)

	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse KEY=VALUE
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			env[key] = value
		}
	}

	return env, scanner.Err()
}

func parseConfig(env map[string]string) (*Config, error) {
	config := &Config{}

	// Wallet configuration
	config.WalletName = env["WALLET_NAME"]
	if config.WalletName == "" {
		return nil, fmt.Errorf("WALLET_NAME not set")
	}

	config.Password = env["PASSWORD"]
	if config.Password == "" {
		return nil, fmt.Errorf("PASSWORD not set")
	}

	// Network configuration
	config.RpcURL = env["RPC_URL"]
	if config.RpcURL == "" {
		return nil, fmt.Errorf("RPC_URL not set")
	}

	// Send configuration
	sendToAddr, err := types.ParseAddress(env["SEND_TO_ADDRESS"])
	if err != nil {
		return nil, fmt.Errorf("invalid SEND_TO_ADDRESS: %w", err)
	}
	config.SendToAddress = sendToAddr

	tokenAddr, err := types.ParseZTS(env["ZTS_TOKEN_ADDRESS"])
	if err != nil {
		return nil, fmt.Errorf("invalid ZTS_TOKEN_ADDRESS: %w", err)
	}
	config.TokenAddress = tokenAddr

	// Parse amount (convert from decimal to base units with 8 decimals)
	amountStr := env["AMOUNT_PER_TX"]
	if amountStr == "" {
		return nil, fmt.Errorf("AMOUNT_PER_TX not set")
	}
	amount, err := parseAmount(amountStr)
	if err != nil {
		return nil, fmt.Errorf("invalid AMOUNT_PER_TX: %w", err)
	}
	config.AmountPerTx = amount

	txsPerInterval, err := strconv.Atoi(env["TXS_PER_INTERVAL"])
	if err != nil {
		return nil, fmt.Errorf("invalid TXS_PER_INTERVAL: %w", err)
	}
	config.TxsPerInterval = txsPerInterval

	intervalSeconds, err := strconv.Atoi(env["INTERVAL_SECONDS"])
	if err != nil {
		return nil, fmt.Errorf("invalid INTERVAL_SECONDS: %w", err)
	}
	config.IntervalSeconds = intervalSeconds

	durationSeconds, err := strconv.Atoi(env["DURATION_SECONDS"])
	if err != nil {
		return nil, fmt.Errorf("invalid DURATION_SECONDS: %w", err)
	}
	config.DurationSeconds = durationSeconds

	return config, nil
}

// parseAmount converts a decimal amount (e.g., "1.5") to base units (150000000)
// Zenon uses 8 decimal places
func parseAmount(amountStr string) (*big.Int, error) {
	// Parse the decimal string
	parts := strings.Split(amountStr, ".")
	wholePart := parts[0]
	decimalPart := ""

	if len(parts) > 1 {
		decimalPart = parts[1]
	}

	// Pad decimal part to 8 digits or truncate if longer
	if len(decimalPart) > 8 {
		decimalPart = decimalPart[:8]
	} else {
		decimalPart = decimalPart + strings.Repeat("0", 8-len(decimalPart))
	}

	// Combine whole and decimal parts
	combinedStr := wholePart + decimalPart

	amount := new(big.Int)
	_, ok := amount.SetString(combinedStr, 10)
	if !ok {
		return nil, fmt.Errorf("failed to parse amount: %s", amountStr)
	}

	return amount, nil
}

func setupWallet(walletName, password string) error {
	// Get default wallet path
	walletFile := filepath.Join(node.DefaultDataDir(), "wallet", walletName)

	// Check if wallet exists
	if _, err := os.Stat(walletFile); err == nil {
		// Wallet exists, load it
		fmt.Printf("Wallet '%s' found. Loading existing wallet...\n", walletName)

		// Load the wallet with password
		ks, err := wallet.ReadKeyFile(walletName, password, "")
		if err != nil {
			return fmt.Errorf("failed to load wallet: %w", err)
		}

		fmt.Printf("Wallet loaded successfully! Address: %s\n", ks.BaseAddress.String())
		return nil
	} else if os.IsNotExist(err) {
		// Wallet doesn't exist, create new one
		fmt.Printf("Wallet '%s' not found. Creating new wallet...\n", walletName)

		// Create new keystore
		ks, err := wallet.NewKeyStore()
		if err != nil {
			return fmt.Errorf("failed to create keystore: %w", err)
		}

		// Save wallet with password
		if err := wallet.WriteKeyFile(ks, walletName, password); err != nil {
			return fmt.Errorf("failed to save wallet: %w", err)
		}

		// Display wallet information
		fmt.Println("\n===========================================")
		fmt.Println("NEW WALLET CREATED")
		fmt.Println("===========================================")
		fmt.Printf("Wallet Name: %s\n", walletName)
		fmt.Printf("Address: %s\n", ks.BaseAddress.String())
		fmt.Printf("Seed Phrase: %s\n", ks.Mnemonic)
		fmt.Println("===========================================")
		fmt.Println("IMPORTANT: Save your seed phrase securely!")
		fmt.Println("===========================================\n")

		return nil
	} else {
		return fmt.Errorf("error checking wallet file: %w", err)
	}
}

func showSeedInfo(walletName, password string) error {
	// Load the wallet
	ks, err := wallet.ReadKeyFile(walletName, password, "")
	if err != nil {
		return fmt.Errorf("failed to load wallet: %w", err)
	}

	// Display wallet information
	fmt.Println("\n===========================================")
	fmt.Println("WALLET INFORMATION")
	fmt.Println("===========================================")
	fmt.Printf("Wallet Name: %s\n", walletName)
	fmt.Printf("Address: %s\n", ks.BaseAddress.String())
	fmt.Printf("Seed Phrase: %s\n", ks.Mnemonic)
	fmt.Println("===========================================\n")

	return nil
}

func initLogger() (*os.File, *log.Logger, error) {
	logFile, err := os.OpenFile("send-bot.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open log file: %w", err)
	}

	logger := log.New(logFile, "", 0)
	return logFile, logger, nil
}

func initAccountBlockLogger() (*os.File, *log.Logger, error) {
	logFile, err := os.OpenFile("account-blocks.log", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open account block log file: %w", err)
	}

	logger := log.New(logFile, "", 0)
	return logFile, logger, nil
}

func autoReceive(z *zenon.Zenon, logger *log.Logger) error {
	fmt.Println("Checking for unreceived transactions...")

	// Get unreceived blocks (page size limited to 10)
	unreceivedBlocks, err := z.Client.LedgerApi.GetUnreceivedBlocksByAddress(z.Address(), 0, 10)
	if err != nil {
		return fmt.Errorf("failed to get unreceived blocks: %w", err)
	}

	if len(unreceivedBlocks.List) == 0 {
		fmt.Println("No unreceived transactions found.")
		return nil
	}

	// Get initial account height (may be 0 if account doesn't exist yet)
	var initialHeight uint64
	initialAccountInfo, err := z.Client.LedgerApi.GetAccountInfoByAddress(z.Address())
	if err != nil {
		// Account might not exist yet, start from height 0
		fmt.Println("Account not found yet, will be created with first receive.")
		initialHeight = 0
	} else {
		initialHeight = initialAccountInfo.AccountHeight
	}

	fmt.Printf("Found %d unreceived transaction(s). Receiving...\n", len(unreceivedBlocks.List))
	logger.Printf("Found %d unreceived transaction(s). Receiving...", len(unreceivedBlocks.List))

	receivedCount := 0
	for _, block := range unreceivedBlocks.List {
		// Create receive transaction
		receiveTx := &nom.AccountBlock{
			BlockType:     nom.BlockTypeUserReceive,
			FromBlockHash: block.Hash,
		}

		// Send receive transaction
		if err := z.Send(receiveTx); err != nil {
			errMsg := fmt.Sprintf("Failed to receive block %s: %v", block.Hash.String(), err)
			fmt.Println(errMsg)
			logger.Println(errMsg)
			continue
		}

		receivedCount++
		msg := fmt.Sprintf("Received block %s | Hash: %s", block.Hash.String(), receiveTx.Hash.String())
		fmt.Println(msg)
		logger.Println(msg)
	}

	if receivedCount > 0 {
		// Wait for transactions to be confirmed
		expectedHeight := initialHeight + uint64(receivedCount)
		fmt.Printf("Waiting for %d receive transaction(s) to confirm (expected height: %d)...\n", receivedCount, expectedHeight)

		timeout := time.After(120 * time.Second)
		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-timeout:
				return fmt.Errorf("timeout waiting for receive transactions to confirm")
			case <-ticker.C:
				// First check if unreceived blocks are gone (primary confirmation method)
				checkUnreceived, err := z.Client.LedgerApi.GetUnreceivedBlocksByAddress(z.Address(), 0, 10)
				if err == nil && len(checkUnreceived.List) == 0 {
					fmt.Println("All receive transactions confirmed! (no unreceived blocks remaining)")
					logger.Println("All receive transactions confirmed!")
					return nil
				}

				// Also try to check account height if account exists
				currentAccountInfo, err := z.Client.LedgerApi.GetAccountInfoByAddress(z.Address())
				if err != nil {
					// Account doesn't exist yet, continue waiting
					fmt.Println("Waiting for receive transactions to be confirmed...")
					continue
				}

				currentHeight := currentAccountInfo.AccountHeight
				if currentHeight >= expectedHeight {
					fmt.Printf("All receive transactions confirmed! (height: %d)\n", currentHeight)
					logger.Printf("All receive transactions confirmed! (height: %d)", currentHeight)
					return nil
				}

				fmt.Printf("Waiting... current height: %d, expected: %d\n", currentHeight, expectedHeight)
			}
		}
	}

	return nil
}

func checkBalance(z *zenon.Zenon, config *Config, logger *log.Logger) error {
	fmt.Println("Checking account balance...")

	// First try to get frontier block to verify account exists
	frontierBlock, err := z.Client.LedgerApi.GetFrontierAccountBlock(z.Address())
	if err != nil || frontierBlock == nil {
		errMsg := fmt.Sprintf("Account has no confirmed blocks yet.\n"+
			"  Address: %s\n"+
			"  Error: %v\n"+
			"  The wallet needs to receive at least one transaction before the bot can run.\n"+
			"  Please send tokens to this address and wait for confirmation.", z.Address().String(), err)
		fmt.Println(errMsg)
		logger.Println(errMsg)
		return fmt.Errorf("no frontier block found for account")
	}

	fmt.Printf("Account found! Frontier height: %d\n", frontierBlock.Height)

	// Now get full account info with retries
	// Note: SDK v0.1.0 has a bug - calls wrong RPC method
	var accountInfo *api.AccountInfo

	for attempt := 1; attempt <= 5; attempt++ {
		accountInfo, err = z.Client.LedgerApi.GetAccountInfoByAddress(z.Address())
		if err == nil {
			break
		}

		if attempt < 5 {
			fmt.Printf("Attempt %d/5: Error getting account info, retrying in 3 seconds... (error: %v)\n", attempt, err)
			time.Sleep(3 * time.Second)
		}
	}

	if err != nil {
		warnMsg := fmt.Sprintf("Warning: Unable to verify balance (API error).\n"+
			"  Address: %s\n"+
			"  Error: %v\n"+
			"  The account exists (height %d) so proceeding anyway.\n"+
			"  NOTE: Bot will fail if insufficient balance. Monitor the first few transactions.", z.Address().String(), err, frontierBlock.Height)
		fmt.Println(warnMsg)
		logger.Println(warnMsg)
		fmt.Println("Skipping balance check and proceeding with sending...")
		return nil
	}

	// Get balance for the specific token
	balanceInfo, ok := accountInfo.BalanceInfoMap[config.TokenAddress]
	if !ok || balanceInfo == nil {
		warnMsg := fmt.Sprintf("Warning: No balance found for token %s.\n"+
			"  The account exists but may not have this token.\n"+
			"  Proceeding anyway - bot will fail if insufficient balance.", config.TokenAddress.String())
		fmt.Println(warnMsg)
		logger.Println(warnMsg)
		return nil
	}

	currentBalance := balanceInfo.Balance

	// Calculate total tokens needed
	var totalNeeded *big.Int
	var totalTxs int

	if config.DurationSeconds > 0 {
		// Calculate total transactions for the duration
		intervals := config.DurationSeconds / config.IntervalSeconds
		if config.DurationSeconds%config.IntervalSeconds != 0 {
			intervals++ // Round up for partial interval
		}
		totalTxs = intervals * config.TxsPerInterval
		totalNeeded = new(big.Int).Mul(config.AmountPerTx, big.NewInt(int64(totalTxs)))
	} else {
		// For indefinite duration, check we have at least 1 hour worth
		oneHourIntervals := 3600 / config.IntervalSeconds
		totalTxs = oneHourIntervals * config.TxsPerInterval
		totalNeeded = new(big.Int).Mul(config.AmountPerTx, big.NewInt(int64(totalTxs)))
	}

	// Compare balance with total needed
	if currentBalance.Cmp(totalNeeded) < 0 {
		errMsg := fmt.Sprintf("Insufficient balance!\n"+
			"  Current balance: %s (base units)\n"+
			"  Required: %s (base units)\n"+
			"  Token: %s\n"+
			"  Transactions planned: %d",
			currentBalance.String(),
			totalNeeded.String(),
			config.TokenAddress.String(),
			totalTxs)

		fmt.Println(errMsg)
		logger.Println(errMsg)
		return fmt.Errorf("insufficient balance")
	}

	// Log success
	var durationMsg string
	if config.DurationSeconds > 0 {
		durationMsg = fmt.Sprintf("%d transactions (%d seconds)", totalTxs, config.DurationSeconds)
	} else {
		durationMsg = fmt.Sprintf("at least %d transactions (1 hour)", totalTxs)
	}

	successMsg := fmt.Sprintf("Balance check passed!\n"+
		"  Current balance: %s (base units)\n"+
		"  Required for %s: %s (base units)\n"+
		"  Token: %s",
		currentBalance.String(),
		durationMsg,
		totalNeeded.String(),
		config.TokenAddress.String())

	fmt.Println(successMsg)
	logger.Println(successMsg)

	return nil
}

func runSendBot(config *Config, logger *log.Logger) error {
	// Initialize account block logger
	accountBlockLogFile, accountBlockLogger, err := initAccountBlockLogger()
	if err != nil {
		return fmt.Errorf("failed to initialize account block logger: %w", err)
	}
	defer accountBlockLogFile.Close()

	// Initialize Zenon client
	z, err := zenon.NewZenon(config.WalletName)
	if err != nil {
		return fmt.Errorf("failed to create Zenon client: %w", err)
	}

	// Start connection
	if err := z.Start(config.Password, config.RpcURL, 0); err != nil {
		return fmt.Errorf("failed to connect to node: %w", err)
	}
	defer z.Stop()

	fmt.Printf("Connected to %s\n", config.RpcURL)
	fmt.Printf("Sending from: %s\n", z.Address().String())
	fmt.Printf("Sending to: %s\n", config.SendToAddress.String())
	fmt.Printf("Token: %s\n", config.TokenAddress.String())
	fmt.Printf("Amount per TX: %s (base units)\n", config.AmountPerTx.String())
	fmt.Printf("Rate: %d TXs every %d seconds\n", config.TxsPerInterval, config.IntervalSeconds)
	if config.DurationSeconds > 0 {
		fmt.Printf("Duration: %d seconds\n", config.DurationSeconds)
	} else {
		fmt.Println("Duration: indefinite (press Ctrl+C to stop)")
	}
	fmt.Println()

	// Auto-receive any pending transactions
	if err := autoReceive(z, logger); err != nil {
		return err
	}
	fmt.Println()

	// Subscribe to account blocks for this address
	accountBlockSubscription, accountBlockCh, err := z.Client.SubscriberApi.ToAccountBlocksByAddress(z.Address())
	if err != nil {
		return fmt.Errorf("failed to subscribe to account blocks: %w", err)
	}
	defer accountBlockSubscription.Unsubscribe()

	// Write header to account block log
	accountBlockLogger.Println("=== Account Block Subscription Log ===")
	accountBlockLogger.Printf("Monitoring address: %s\n", z.Address().String())
	accountBlockLogger.Println()

	// Start account block processing goroutine
	done := make(chan bool)
	go func() {
		for {
			select {
			case accountBlocks := <-accountBlockCh:
				for _, block := range accountBlocks {
					// Log account block details to separate log
					logMsg := fmt.Sprintf("%s | Hash: %s | Height: %d | BlockType: %d | Address: %s | ToAddress: %s",
						time.Now().Format("2006-01-02 15:04:05"),
						block.Hash.String(),
						block.Height,
						block.BlockType,
						block.Address.String(),
						block.ToAddress.String())

					// Log to account block file
					accountBlockLogger.Println(logMsg)

					// Also log brief version to console
					consolMsg := fmt.Sprintf("%s | Account Block | Hash: %s | Height: %d | Type: %d",
						time.Now().Format("2006-01-02 15:04:05"),
						block.Hash.String(),
						block.Height,
						block.BlockType)
					fmt.Println(consolMsg)
				}
			case <-done:
				return
			}
		}
	}()

	// Note: Balance check removed due to SDK bug in GetAccountInfoByAddress
	// The SDK calls wrong RPC method. Monitor first few TXs for balance errors.
	fmt.Println("Starting to send transactions (balance check skipped - SDK limitation)...")
	fmt.Println()

	startTime := time.Now()
	totalTxsSent := 0

	for {
		// Check if duration has been reached
		if config.DurationSeconds > 0 {
			elapsed := time.Since(startTime).Seconds()
			if elapsed >= float64(config.DurationSeconds) {
				fmt.Printf("\nDuration of %d seconds reached. Stopping...\n", config.DurationSeconds)
				break
			}
		}

		// Send batch of transactions
		intervalStart := time.Now()
		for i := 0; i < config.TxsPerInterval; i++ {
			// Create send transaction
			tx := &nom.AccountBlock{
				BlockType:     nom.BlockTypeUserSend,
				ToAddress:     config.SendToAddress,
				Amount:        config.AmountPerTx,
				TokenStandard: config.TokenAddress,
				Data:          []byte{},
			}

			// Send transaction
			if err := z.Send(tx); err != nil {
				errMsg := fmt.Sprintf("[ERROR] %s | Failed to send TX: %v", time.Now().Format("2006-01-02 15:04:05"), err)
				fmt.Println(errMsg)
				logger.Println(errMsg)
				continue
			}

			totalTxsSent++

			// Log transaction
			logMsg := fmt.Sprintf("%s | TX #%d | Hash: %s | To: %s | Amount: %s | Token: %s",
				time.Now().Format("2006-01-02 15:04:05"),
				totalTxsSent,
				tx.Hash.String(),
				config.SendToAddress.String(),
				config.AmountPerTx.String(),
				config.TokenAddress.String(),
			)
			fmt.Println(logMsg)
			logger.Println(logMsg)
		}

		// Wait for the rest of the interval
		elapsed := time.Since(intervalStart)
		waitTime := time.Duration(config.IntervalSeconds)*time.Second - elapsed
		if waitTime > 0 {
			time.Sleep(waitTime)
		}
	}

	// Close the done channel to stop the account block goroutine
	close(done)

	fmt.Printf("\nTotal transactions sent: %d\n", totalTxsSent)
	logger.Printf("Total transactions sent: %d\n", totalTxsSent)

	return nil
}

func main() {
	// Parse command line flags
	showSeed := flag.Bool("seed", false, "Display wallet address and seed phrase")
	flag.Parse()

	fmt.Println("Starting Send Bot...")

	// Load environment variables
	env, err := loadEnv(".env")
	if err != nil {
		log.Fatalf("Error loading .env file: %v\nPlease copy .env.example to .env and configure it.", err)
	}

	// Parse configuration
	config, err := parseConfig(env)
	if err != nil {
		log.Fatalf("Error parsing configuration: %v", err)
	}

	// If --seed flag is set, display wallet info and exit
	if *showSeed {
		if err := showSeedInfo(config.WalletName, config.Password); err != nil {
			log.Fatalf("Error displaying wallet info: %v", err)
		}
		return
	}

	// Setup wallet
	if err := setupWallet(config.WalletName, config.Password); err != nil {
		log.Fatalf("Error setting up wallet: %v", err)
	}

	// Initialize logger
	logFile, logger, err := initLogger()
	if err != nil {
		log.Fatalf("Error initializing logger: %v", err)
	}
	defer logFile.Close()

	fmt.Println("Send Bot initialized successfully!")
	fmt.Println()

	// Run the send bot
	if err := runSendBot(config, logger); err != nil {
		log.Fatalf("Error running send bot: %v", err)
	}
}
