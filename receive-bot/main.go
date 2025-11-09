package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/MoonBaZZe/znn-sdk-go/wallet"
	"github.com/MoonBaZZe/znn-sdk-go/zenon"
	"github.com/zenon-network/go-zenon/chain/nom"
	"github.com/zenon-network/go-zenon/node"
)

type Config struct {
	WalletName                         string
	Password                           string
	RpcURL                             string
	MomentumConfirmationTimeoutSeconds int
}

type MomentumTracker struct {
	receivedTxHashes  map[string]bool // TX hashes we've received
	confirmedTxHashes map[string]bool // TX hashes confirmed in momentums
	producerStats     map[string]int  // Producer address -> count of momentums
	mutex             sync.RWMutex
}

func newMomentumTracker() *MomentumTracker {
	return &MomentumTracker{
		receivedTxHashes:  make(map[string]bool),
		confirmedTxHashes: make(map[string]bool),
		producerStats:     make(map[string]int),
	}
}

func (mt *MomentumTracker) addReceivedTx(hash string) {
	mt.mutex.Lock()
	defer mt.mutex.Unlock()
	mt.receivedTxHashes[hash] = true
}

func (mt *MomentumTracker) addConfirmedTx(hash string, producer string) {
	mt.mutex.Lock()
	defer mt.mutex.Unlock()
	if !mt.confirmedTxHashes[hash] {
		mt.confirmedTxHashes[hash] = true
		mt.producerStats[producer]++
	}
}

func (mt *MomentumTracker) allConfirmed() bool {
	mt.mutex.RLock()
	defer mt.mutex.RUnlock()
	return len(mt.receivedTxHashes) > 0 && len(mt.confirmedTxHashes) == len(mt.receivedTxHashes)
}

func (mt *MomentumTracker) getStats() (int, int, map[string]int) {
	mt.mutex.RLock()
	defer mt.mutex.RUnlock()
	stats := make(map[string]int)
	for k, v := range mt.producerStats {
		stats[k] = v
	}
	return len(mt.receivedTxHashes), len(mt.confirmedTxHashes), stats
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

	// Receive configuration
	momentumTimeout, err := strconv.Atoi(env["MOMENTUM_CONFIRMATION_TIMEOUT_SECONDS"])
	if err != nil {
		return nil, fmt.Errorf("invalid MOMENTUM_CONFIRMATION_TIMEOUT_SECONDS: %w", err)
	}
	config.MomentumConfirmationTimeoutSeconds = momentumTimeout

	return config, nil
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
		fmt.Println("===========================================")
		fmt.Println()

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
	fmt.Println("===========================================")
	fmt.Println()

	return nil
}

func initLogger() (*os.File, *log.Logger, error) {
	logFile, err := os.OpenFile("receive-bot.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open log file: %w", err)
	}

	logger := log.New(logFile, "", 0)
	return logFile, logger, nil
}

func initMomentumLogger() (*os.File, *log.Logger, error) {
	logFile, err := os.OpenFile("momentum-tracking.log", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open momentum tracking log file: %w", err)
	}

	logger := log.New(logFile, "", 0)
	return logFile, logger, nil
}

// autoReceivePending receives all pending unreceived transactions on startup
func autoReceivePending(z *zenon.Zenon, logger *log.Logger) (int, error) {
	totalReceived := 0

	fmt.Println("Checking for unreceived transactions on startup...")

	for {
		unreceivedBlocks, err := z.Client.LedgerApi.GetUnreceivedBlocksByAddress(z.Address(), 0, 10)
		if err != nil {
			return totalReceived, fmt.Errorf("failed to get unreceived blocks: %w", err)
		}

		if len(unreceivedBlocks.List) == 0 {
			break
		}

		for _, block := range unreceivedBlocks.List {
			receiveTx := &nom.AccountBlock{
				BlockType:     nom.BlockTypeUserReceive,
				FromBlockHash: block.Hash,
			}

			if err := z.Send(receiveTx); err != nil {
				logMsg := fmt.Sprintf("[ERROR] %s | Failed to receive pending block %s: %v",
					time.Now().Format("2006-01-02 15:04:05"),
					block.Hash.String(),
					err)
				fmt.Println(logMsg)
				logger.Println(logMsg)
				continue
			}

			totalReceived++
			logMsg := fmt.Sprintf("%s | Startup RX #%d | Send Hash: %s | Received Hash: %s",
				time.Now().Format("2006-01-02 15:04:05"),
				totalReceived,
				block.Hash.String(),
				receiveTx.Hash.String())
			fmt.Println(logMsg)
			logger.Println(logMsg)
		}
	}

	if totalReceived > 0 {
		fmt.Printf("Auto-received %d pending transactions on startup\n", totalReceived)
	} else {
		fmt.Println("No pending transactions found on startup")
	}
	fmt.Println()

	return totalReceived, nil
}

func runReceiveBot(config *Config, logger *log.Logger) error {
	totalReceived := 0
	var activityMutex sync.Mutex
	lastActivityTime := time.Now()

	// Helper function to update last activity time (thread-safe)
	updateActivity := func() {
		activityMutex.Lock()
		lastActivityTime = time.Now()
		activityMutex.Unlock()
	}

	// Helper function to get time since last activity (thread-safe)
	getTimeSinceActivity := func() time.Duration {
		activityMutex.Lock()
		defer activityMutex.Unlock()
		return time.Since(lastActivityTime)
	}

	// Initialize momentum tracker
	tracker := newMomentumTracker()

	// Initialize momentum logger
	momentumLogFile, momentumLogger, err := initMomentumLogger()
	if err != nil {
		return fmt.Errorf("failed to initialize momentum logger: %w", err)
	}
	defer momentumLogFile.Close()

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
	fmt.Printf("Monitoring address: %s\n", z.Address().String())
	fmt.Printf("Momentum confirmation timeout: %d seconds\n", config.MomentumConfirmationTimeoutSeconds)
	fmt.Println()

	// Auto-receive any pending transactions on startup
	startupReceived, err := autoReceivePending(z, logger)
	if err != nil {
		return fmt.Errorf("failed to auto-receive pending transactions: %w", err)
	}
	totalReceived += startupReceived

	fmt.Println("Starting subscription-based monitoring for unreceived transactions and momentums...")
	fmt.Println()

	// Subscribe to momentums
	momentumSubscription, momentumCh, err := z.Client.SubscriberApi.ToMomentums()
	if err != nil {
		return fmt.Errorf("failed to subscribe to momentums: %w", err)
	}
	defer momentumSubscription.Unsubscribe()

	// Subscribe to unreceived account blocks
	unreceivedSubscription, unreceivedCh, err := z.Client.SubscriberApi.ToUnreceivedAccountBlocksByAddress(z.Address())
	if err != nil {
		return fmt.Errorf("failed to subscribe to unreceived account blocks: %w", err)
	}
	defer unreceivedSubscription.Unsubscribe()

	// Subscribe to account blocks for this address
	accountBlockSubscription, accountBlockCh, err := z.Client.SubscriberApi.ToAccountBlocksByAddress(z.Address())
	if err != nil {
		return fmt.Errorf("failed to subscribe to account blocks: %w", err)
	}
	defer accountBlockSubscription.Unsubscribe()

	// Write header to momentum log
	momentumLogger.Println("=== Momentum Tracking Log ===")
	momentumLogger.Println()

	// Start momentum processing goroutine
	momentumDone := make(chan bool)
	go func() {
		for {
			select {
			case momentums := <-momentumCh:
				for _, momentum := range momentums {
					// Query full momentum details
					detailedMom, err := z.Client.LedgerApi.GetMomentumByHash(momentum.Hash)
					if err != nil {
						fmt.Printf("[ERROR] Failed to get momentum details: %v\n", err)
						continue
					}

					// Check if this momentum contains any of our transactions
					foundOurTx := false
					for _, accountHeader := range detailedMom.Content {
						txHash := accountHeader.Hash.String()
						tracker.mutex.RLock()
						_, isOurTx := tracker.receivedTxHashes[txHash]
						tracker.mutex.RUnlock()

						if isOurTx {
							foundOurTx = true
							producer := detailedMom.Producer.String()
							tracker.addConfirmedTx(txHash, producer)

							// Log to momentum tracking file
							logMsg := fmt.Sprintf("%s | Momentum #%d | Hash: %s | Producer: %s | Confirmed TX: %s",
								time.Now().Format("2006-01-02 15:04:05"),
								detailedMom.Height,
								detailedMom.Hash.String(),
								producer,
								txHash)
							fmt.Println(logMsg)
							momentumLogger.Println(logMsg)
						}
					}

					// If this momentum contained our transactions, log summary
					if foundOurTx {
						total, confirmed, _ := tracker.getStats()
						fmt.Printf("Progress: %d/%d transactions confirmed in momentums\n", confirmed, total)
					}
				}
			case <-momentumDone:
				return
			}
		}
	}()

	// Start unreceived blocks processing goroutine
	go func() {
		for {
			select {
			case unreceivedBlocks := <-unreceivedCh:
				for _, block := range unreceivedBlocks {
					fmt.Printf("Unreceived block detected: %s\n", block.Hash.String())

					// Create receive transaction
					receiveTx := &nom.AccountBlock{
						BlockType:     nom.BlockTypeUserReceive,
						FromBlockHash: block.Hash,
					}

					// Send receive transaction
					if err := z.Send(receiveTx); err != nil {
						errMsg := fmt.Sprintf("[ERROR] %s | Failed to receive block %s: %v",
							time.Now().Format("2006-01-02 15:04:05"),
							block.Hash.String(),
							err)
						fmt.Println(errMsg)
						logger.Println(errMsg)
						continue
					}

					totalReceived++

					// Track this received TX hash
					txHash := receiveTx.Hash.String()
					tracker.addReceivedTx(txHash)

					// Log successful receive
					logMsg := fmt.Sprintf("%s | RX #%d | Send Hash: %s | Received Hash: %s",
						time.Now().Format("2006-01-02 15:04:05"),
						totalReceived,
						block.Hash.String(),
						txHash)
					fmt.Println(logMsg)
					logger.Println(logMsg)

					// Update last activity time since we received new transaction
					updateActivity()
				}
			case <-momentumDone:
				return
			}
		}
	}()

	// Start account block processing goroutine (to track publish/confirmation)
	go func() {
		for {
			select {
			case accountBlocks := <-accountBlockCh:
				for _, block := range accountBlocks {
					// Log when our account blocks are published or confirmed
					logMsg := fmt.Sprintf("%s | Account Block Event | Hash: %s | Height: %d | Type: %d",
						time.Now().Format("2006-01-02 15:04:05"),
						block.Hash.String(),
						block.Height,
						block.BlockType)
					fmt.Println(logMsg)
					logger.Println(logMsg)
				}
			case <-momentumDone:
				return
			}
		}
	}()

	// Start monitoring loop for completion and timeout
	for {

		// Check if all transactions are confirmed in momentums
		if tracker.allConfirmed() {
			close(momentumDone)

			// Generate final summary
			total, confirmed, producerStats := tracker.getStats()
			fmt.Printf("\n=== All %d transactions confirmed in momentums ===\n", confirmed)
			fmt.Println("\nProducer Statistics:")
			momentumLogger.Println("\n=== Final Summary ===")
			momentumLogger.Printf("Total transactions received: %d\n", total)
			momentumLogger.Printf("Total transactions confirmed: %d\n", confirmed)
			momentumLogger.Println("\nProducer Statistics:")

			for producer, count := range producerStats {
				summary := fmt.Sprintf("  %s: %d momentums", producer, count)
				fmt.Println(summary)
				momentumLogger.Println(summary)
			}

			logger.Printf("All %d transactions confirmed. Bot shutting down.", confirmed)
			return nil
		}

		// Check if confirmation timeout has been exceeded since last activity
		elapsed := getTimeSinceActivity().Seconds()
		if elapsed >= float64(config.MomentumConfirmationTimeoutSeconds) {
			close(momentumDone)

			total, confirmed, producerStats := tracker.getStats()
			fmt.Printf("\nConfirmation timeout reached after %.0f seconds\n", elapsed)
			fmt.Printf("Total transactions received: %d\n", total)
			fmt.Printf("Transactions confirmed in momentums: %d\n", confirmed)
			fmt.Printf("Transactions still unconfirmed: %d\n", total-confirmed)

			if len(producerStats) > 0 {
				fmt.Println("\nProducer Statistics:")
				momentumLogger.Println("\n=== Timeout Summary ===")
				momentumLogger.Printf("Timeout reached after %.0f seconds\n", elapsed)
				momentumLogger.Printf("Total transactions: %d, Confirmed: %d, Unconfirmed: %d\n", total, confirmed, total-confirmed)
				momentumLogger.Println("\nProducer Statistics:")

				for producer, count := range producerStats {
					summary := fmt.Sprintf("  %s: %d momentums", producer, count)
					fmt.Println(summary)
					momentumLogger.Println(summary)
				}
			}

			logger.Printf("Confirmation timeout. %d/%d transactions confirmed.", confirmed, total)
			return nil
		}

		// Short sleep to avoid busy-waiting
		time.Sleep(1 * time.Second)
	}
}

func main() {
	// Parse command line flags
	showSeed := flag.Bool("seed", false, "Display wallet address and seed phrase")
	flag.Parse()

	fmt.Println("Starting Receive Bot...")

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

	fmt.Println("Receive Bot initialized successfully!")
	fmt.Println()

	// Run the receive bot
	if err := runReceiveBot(config, logger); err != nil {
		log.Fatalf("Error running receive bot: %v", err)
	}
}
