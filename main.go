package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"antigravity-unleashed/pkg/config"
	"antigravity-unleashed/pkg/engine"
	"antigravity-unleashed/pkg/gateways"
)

func main() {
	configPath := flag.String("config", "config.yaml", "Path to configuration YAML file")
	flag.Parse()

	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("❌ Failed to load config: %v", err)
	}

	eng, err := engine.NewUnleashedEngine(cfg)
	if err != nil {
		log.Fatalf("❌ Failed to initialize engine: %v", err)
	}
	defer eng.MemoryStore.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\n🛑 Shutdown signal received. Stopping gateways...")
		cancel()
	}()

	var wg sync.WaitGroup

	// 1. REST API Gateway
	if cfg.Gateways.RESTAPI.Enabled {
		wg.Add(1)
		restGW := gateways.NewRESTAPIGateway(eng, cfg.Gateways.RESTAPI)
		go func() {
			defer wg.Done()
			if err := restGW.Start(ctx); err != nil {
				log.Printf("[REST Gateway] Error: %v\n", err)
			}
		}()
	}

	// 2. Telegram Gateway
	if cfg.Gateways.Telegram.Enabled && cfg.Gateways.Telegram.BotToken != "" {
		wg.Add(1)
		tgGW := gateways.NewTelegramGateway(eng, cfg.Gateways.Telegram)
		go func() {
			defer wg.Done()
			if err := tgGW.Start(ctx); err != nil {
				log.Printf("[Telegram Gateway] Error: %v\n", err)
			}
		}()
	}

	// 3. CLI Gateway (Runs in foreground if enabled)
	if cfg.Gateways.CLI.Enabled {
		cliGW := gateways.NewCLIGateway(eng)
		if err := cliGW.Start(ctx); err != nil {
			log.Printf("[CLI Gateway] Error: %v\n", err)
		}
		cancel() // If CLI exits, cancel daemon context
	} else {
		log.Println("[Daemon] Running in headless 24/7 background mode.")
		<-ctx.Done()
	}

	wg.Wait()
	fmt.Println("✅ Antigravity-Unleashed shutdown complete.")
}
