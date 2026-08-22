package wizard

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"agent-unleashed/pkg/adapters"
	"agent-unleashed/pkg/config"
)

func RunSetupWizard(configPath string) error {
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		cfg = &config.AppConfig{}
	}

	reader := bufio.NewReader(os.Stdin)

	fmt.Println("\n============================================================")
	fmt.Println("  🧙 AGENT-UNLEASHED (agt-ul) INTERACTIVE SETUP WIZARD")
	fmt.Println("  Universal 24/7 Agent Operating System & Gateway")
	fmt.Println("============================================================")

	// Step 1: Detect Installed AI CLI Tools
	fmt.Println("\n🔍 Scanning system for installed AI CLI agents...")
	reg := adapters.NewAdapterRegistry()
	reg.Register(adapters.NewAgyAdapter(cfg.Model.AgyBinaryPath))
	reg.Register(adapters.NewClaudeAdapter(cfg.Model.ClaudeBinaryPath))
	reg.Register(adapters.NewCodexAdapter(cfg.Model.CodexBinaryPath))
	reg.Register(adapters.NewAiderAdapter(cfg.Model.AiderBinaryPath))
	reg.Register(adapters.NewOllamaAdapter(cfg.Model.OllamaEndpoint))

	allAdapters := reg.ListAll()
	var detected []string

	for _, a := range allAdapters {
		if a.Detect() {
			fmt.Printf("  ✅ Found: %s (%s)\n", a.DisplayName(), a.BinaryPath())
			detected = append(detected, a.Name())
		} else {
			fmt.Printf("  ⚪ Not found: %s\n", a.DisplayName())
		}
	}

	// Step 2: Driver Selection
	fmt.Println("\n--- 1. Primary Agent Driver ---")
	fmt.Println("Options: 'auto' (Cascade through detected tools), 'agy', 'claude', 'codex', 'aider', 'ollama', 'api'")
	currentDriver := cfg.Model.Driver
	if currentDriver == "" {
		currentDriver = "auto"
	}
	fmt.Printf("Select primary driver [%s]: ", currentDriver)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)
	if input != "" {
		cfg.Model.Driver = input
	}

	// Step 3: Telegram Bot Configuration
	fmt.Println("\n--- 2. Telegram 24/7 Gateway ---")
	fmt.Printf("Enable Telegram Bot? (y/n) [%v]: ", cfg.Gateways.Telegram.Enabled)
	input, _ = reader.ReadString('\n')
	input = strings.TrimSpace(strings.ToLower(input))
	if input == "y" || input == "yes" {
		cfg.Gateways.Telegram.Enabled = true
		fmt.Printf("Enter Telegram Bot Token [%s]: ", maskSecret(cfg.Gateways.Telegram.BotToken))
		tokenInput, _ := reader.ReadString('\n')
		tokenInput = strings.TrimSpace(tokenInput)
		if tokenInput != "" {
			cfg.Gateways.Telegram.BotToken = tokenInput
		}
	} else if input == "n" || input == "no" {
		cfg.Gateways.Telegram.Enabled = false
	}

	// Step 4: Discord Bot Configuration
	fmt.Println("\n--- 3. Discord 24/7 Gateway ---")
	fmt.Printf("Enable Discord Bot? (y/n) [%v]: ", cfg.Gateways.Discord.Enabled)
	input, _ = reader.ReadString('\n')
	input = strings.TrimSpace(strings.ToLower(input))
	if input == "y" || input == "yes" {
		cfg.Gateways.Discord.Enabled = true
		fmt.Printf("Enter Discord Bot Token [%s]: ", maskSecret(cfg.Gateways.Discord.BotToken))
		tokenInput, _ := reader.ReadString('\n')
		tokenInput = strings.TrimSpace(tokenInput)
		if tokenInput != "" {
			cfg.Gateways.Discord.BotToken = tokenInput
		}
	} else if input == "n" || input == "no" {
		cfg.Gateways.Discord.Enabled = false
	}

	// Step 5: Optional Cloud API Enrichment
	fmt.Println("\n--- 4. Optional Cloud API Keys (Press Enter to skip) ---")
	fmt.Printf("OpenRouter API Key [%s]: ", maskSecret(cfg.Model.OpenRouterAPIKey))
	orKey, _ := reader.ReadString('\n')
	orKey = strings.TrimSpace(orKey)
	if orKey != "" {
		cfg.Model.OpenRouterAPIKey = orKey
	}

	fmt.Printf("Google Gemini API Key [%s]: ", maskSecret(cfg.Model.GeminiAPIKey))
	gemKey, _ := reader.ReadString('\n')
	gemKey = strings.TrimSpace(gemKey)
	if gemKey != "" {
		cfg.Model.GeminiAPIKey = gemKey
	}

	// Save
	if err := config.SaveConfig(configPath, cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("\n✨ Configuration successfully saved to %s!\n", configPath)
	fmt.Println("Run 'agt-ul' to start the daemon.")
	return nil
}

func maskSecret(s string) string {
	if len(s) <= 8 {
		if s == "" {
			return "none"
		}
		return "***"
	}
	return s[:4] + "..." + s[len(s)-4:]
}
