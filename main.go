package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"agent-unleashed/pkg/config"
	"agent-unleashed/pkg/cron"
	"agent-unleashed/pkg/doctor"
	"agent-unleashed/pkg/engine"
	"agent-unleashed/pkg/gateways"
	"agent-unleashed/pkg/skills"
	"agent-unleashed/pkg/updater"
	"agent-unleashed/pkg/wizard"
)

func main() {
	configPath := "config.yaml"

	// Subcommand routing
	if len(os.Args) > 1 {
		subcmd := os.Args[1]

		switch subcmd {
		case "setup", "config":
			if err := wizard.RunSetupWizard(configPath); err != nil {
				log.Fatalf("Setup failed: %v", err)
			}
			return

		case "update", "upgrade":
			if _, err := updater.RunUpdate(".", false); err != nil {
				log.Fatalf("Update failed: %v", err)
			}
			return

		case "version", "-v", "--version":
			if len(os.Args) == 2 {
				fmt.Printf("Agent-Unleashed (agt-ul) v%s\n", updater.GetVersion())
				return
			}

		case "skill", "skills":
			if len(os.Args) >= 4 && (os.Args[2] == "install" || os.Args[2] == "add") {
				source := os.Args[3]
				cfg, _ := config.LoadConfig(configPath)
				indexer := skills.NewSkillIndexer(cfg.System.SkillsDir)
				meta, err := indexer.InstallSkill(source)
				if err != nil {
					log.Fatalf("❌ Skill install failed: %v", err)
				}
				fmt.Printf("✅ Successfully installed skill '%s' to %s\n", meta.Name, meta.Path)
				return
			}
			cfg, _ := config.LoadConfig(configPath)
			indexer := skills.NewSkillIndexer(cfg.System.SkillsDir)
			fmt.Printf("\n⚡ Installed Skills (%d in %s):\n", len(indexer.ListSkills()), cfg.System.SkillsDir)
			for _, s := range indexer.ListSkills() {
				fmt.Printf("  • %-25s : %s\n", s.Name, s.Description)
			}
			fmt.Println()
			return

		case "doctor":
			autoFix := false
			for _, a := range os.Args[2:] {
				if a == "--fix" || a == "-f" {
					autoFix = true
				}
			}
			rep := doctor.RunDiagnostics(configPath, autoFix)
			doctor.PrintDoctorReport(rep)
			return

		case "status":
			runStatusReport(configPath)
			return

		case "memory":
			runMemoryReport(configPath)
			return

		case "wiki":
			runWikiReport(configPath)
			return

		case "lcm":
			runLCMReport(configPath)
			return

		case "cron":
			runCronReport(configPath)
			return

		case "hook-memory":
			runHookMemory(configPath)
			return

		case "hook-reflect":
			runHookReflect(configPath)
			return

		case "--help", "-h", "help":
			printHelp()
			return
		}
	}

	// Flag parsing for default run
	verboseFlag := false
	for _, arg := range os.Args[1:] {
		if arg == "-v" || arg == "--verbose" {
			verboseFlag = true
		}
	}

	// Default: Run Master Daemon & Interactive REPL
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("❌ Failed to load config: %v", err)
	}

	eng, err := engine.NewUnleashedEngine(cfg)
	if err != nil {
		log.Fatalf("❌ Failed to initialize engine: %v", err)
	}
	eng.SetVerbose(verboseFlag)
	if eng.MemoryStore != nil {
		defer closeEngine(eng)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize In-Process Cron Engine
	var cronEng *cron.CronEngine
	if eng.MemoryStore != nil {
		cronEng, _ = cron.NewCronEngine(eng.MemoryStore.GetDB(), eng, func(channel, targetID, response string) {
			log.Printf("[Cron Dispatched -> %s (%s)]: %s\n", channel, targetID, response)
		})
		if cronEng != nil {
			go cronEng.Start(ctx)
		}
	}

	// Signal Trapping for Graceful Shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\n🛑 Shutdown signal received. Stopping gateways...")
		cancel()
	}()

	var wg sync.WaitGroup

	// 1. REST API & WebSocket Gateway
	if cfg.Gateways.RESTAPI.Enabled {
		restGW, restErr := gateways.NewRESTAPIGateway(eng, cfg.Gateways.RESTAPI, cfg.System.DataDir)
		if restErr != nil {
			log.Printf("[REST Gateway] Disabled: %v\n", restErr)
		} else {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if err := restGW.Start(ctx); err != nil {
					log.Printf("[REST Gateway] Error: %v\n", err)
				}
			}()
		}
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

	// 3. Discord Gateway
	if cfg.Gateways.Discord.Enabled && cfg.Gateways.Discord.BotToken != "" {
		wg.Add(1)
		dcGW := gateways.NewDiscordGateway(eng, cfg.Gateways.Discord)
		go func() {
			defer wg.Done()
			if err := dcGW.Start(ctx); err != nil {
				log.Printf("[Discord Gateway] Error: %v\n", err)
			}
		}()
	}

	// 4. CLI Gateway (Foreground)
	if cfg.Gateways.CLI.Enabled {
		cliGW := gateways.NewCLIGateway(eng, cronEng)
		if err := cliGW.Start(ctx); err != nil {
			log.Printf("[CLI Gateway] Error: %v\n", err)
		}
		cancel()
	} else {
		log.Println("[Daemon] Running in headless 24/7 background mode. Press Ctrl+C to stop.")
		<-ctx.Done()
	}

	wg.Wait()
	fmt.Println("✅ Agent-Unleashed shutdown complete.")
}

// closeEngine releases the memory store if there is one. With memory.enabled
// set to false the engine leaves MemoryStore nil, and an unguarded deferred
// Close() panicked - including while unwinding an earlier panic, which hid the
// original failure entirely.
func closeEngine(eng *engine.UnleashedEngine) {
	if eng != nil {
		_ = eng.MemoryStore.Close()
	}
}

func printHelp() {
	fmt.Printf("Agent-Unleashed (agt-ul) v%s - Universal Go Agent Harness & Gateway\n", updater.GetVersion())
	fmt.Println("\nUsage:")
	fmt.Println("  agt-ul               Start interactive REPL & 24/7 daemon")
	fmt.Println("  agt-ul -v            Start in Verbose mode")
	fmt.Println("  agt-ul update        Self-update and rebuild binary from source")
	fmt.Println("  agt-ul doctor        Run full system health check & diagnostics")
	fmt.Println("  agt-ul doctor --fix  Run diagnostics and auto-repair issues")
	fmt.Println("  agt-ul skill install <repo> Install skill from GitHub (e.g. Leonxlnx/taste-skill)")
	fmt.Println("  agt-ul skills        List all discovered skills")
	fmt.Println("  agt-ul setup         Run interactive setup wizard")
	fmt.Println("  agt-ul status        Display detected CLI tools & system diagnostics")
	fmt.Println("  agt-ul memory        Inspect Palace-Mnemosyne memory stats")
	fmt.Println("  agt-ul wiki          Browse compiled project LLM-Wiki articles")
	fmt.Println("  agt-ul lcm           Lossless Context Management summary & message DAG")
	fmt.Println("  agt-ul cron          List active background scheduled tasks")
	fmt.Println("  agt-ul version       Display version and build info")
	fmt.Println("  agt-ul hook-memory   Antigravity PreInvocation lifecycle hook")
	fmt.Println("  agt-ul hook-reflect  Antigravity Stop lifecycle hook")
}

func runLCMReport(configPath string) {
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
	eng, err := engine.NewUnleashedEngine(cfg)
	if err != nil {
		log.Fatalf("Engine error: %v", err)
	}
	defer closeEngine(eng)

	if eng.LCMEngine == nil {
		fmt.Println("❌ Lossless Context Management (LCM) engine is not initialized.")
		return
	}

	desc, err := eng.LCMEngine.Describe("default")
	if err != nil {
		fmt.Printf("❌ LCM error: %v\n", err)
		return
	}
	fmt.Println(desc)
}

func runWikiReport(configPath string) {
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
	eng, err := engine.NewUnleashedEngine(cfg)
	if err != nil {
		log.Fatalf("Engine error: %v", err)
	}
	defer closeEngine(eng)

	if eng.WikiEngine == nil {
		fmt.Println("❌ LLM-Wiki is not initialized.")
		return
	}

	pages := eng.WikiEngine.ListPages()
	fmt.Printf("\n📚 Project LLM-Wiki Knowledge Base (%d Pages):\n", len(pages))
	for _, p := range pages {
		fmt.Printf("  • [[%-20s]] : %s (%s)\n", p.Slug, p.Title, p.Summary)
	}
	fmt.Println()
}

func runCronReport(configPath string) {
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
	eng, err := engine.NewUnleashedEngine(cfg)
	if err != nil {
		log.Fatalf("Engine error: %v", err)
	}
	defer closeEngine(eng)

	db := eng.MemoryStore.GetDB()
	if db == nil {
		fmt.Println()
		fmt.Println("Cron jobs live in the memory database, which is disabled (set memory.enabled: true in config.yaml).")
		fmt.Println()
		return
	}

	cronEng, err := cron.NewCronEngine(db, eng, nil)
	if err != nil {
		log.Fatalf("Cron error: %v", err)
	}

	jobs := cronEng.ListJobs()
	fmt.Printf("\n⏰ AGENT-UNLEASHED 24/7 CRON AUTOMATIONS (%d Active):\n", len(jobs))
	if len(jobs) == 0 {
		fmt.Println("  ⚪ No scheduled tasks currently registered.")
	} else {
		for _, j := range jobs {
			fmt.Printf("  • [%s] Schedule: `%s` | Channel: %s | Prompt: %s\n", j.ID, j.Schedule, j.Channel, j.Prompt)
		}
	}
	fmt.Println()
}

func runStatusReport(configPath string) {
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	eng, err := engine.NewUnleashedEngine(cfg)
	if err != nil {
		log.Fatalf("Engine error: %v", err)
	}
	defer closeEngine(eng)

	fmt.Println("\n============================================================")
	fmt.Println("  📊 AGENT-UNLEASHED (agt-ul) SYSTEM STATUS REPORT")
	fmt.Println("============================================================")
	fmt.Printf("Agent Name: %s\n", cfg.System.AgentName)
	fmt.Printf("Active Driver: %s\n\n", eng.GetActiveDriverName())

	fmt.Println("🔍 Installed AI CLI Tools:")
	for _, a := range eng.Registry.ListAll() {
		detected := a.Detect()
		status := "❌ Not Installed"
		if detected {
			status = fmt.Sprintf("✅ Active (%s)", a.BinaryPath())
		}
		fmt.Printf("  • %-35s : %s\n", a.DisplayName(), status)
	}

	fmt.Println("\n🌐 Active 24/7 Gateways:")
	fmt.Printf("  • Interactive CLI : %v\n", cfg.Gateways.CLI.Enabled)
	fmt.Printf("  • Telegram Bot    : %v\n", cfg.Gateways.Telegram.Enabled)
	fmt.Printf("  • Discord Bot     : %v\n", cfg.Gateways.Discord.Enabled)
	fmt.Printf("  • REST API        : %v (http://%s:%d)\n", cfg.Gateways.RESTAPI.Enabled, cfg.Gateways.RESTAPI.Host, cfg.Gateways.RESTAPI.Port)

	stats, statsErr := eng.MemoryStore.GetStats()
	if statsErr != nil || stats == nil {
		fmt.Println()
		fmt.Println("Palace-Mnemosyne memory is disabled (set memory.enabled: true in config.yaml).")
		fmt.Println()
		return
	}
	fmt.Println("\n🏛️ Palace-Mnemosyne Memory:")
	fmt.Printf("  • Total Drawers   : %d\n", stats.TotalMemories)
	for room, count := range stats.Rooms {
		fmt.Printf("    🚪 Room [%s]: %d drawers\n", room, count)
	}
	fmt.Println()
}

func runMemoryReport(configPath string) {
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
	eng, err := engine.NewUnleashedEngine(cfg)
	if err != nil {
		log.Fatalf("Engine error: %v", err)
	}
	defer closeEngine(eng)

	stats, statsErr := eng.MemoryStore.GetStats()
	if statsErr != nil || stats == nil {
		fmt.Println()
		fmt.Println("Palace-Mnemosyne memory is disabled (set memory.enabled: true in config.yaml).")
		fmt.Println()
		return
	}
	fmt.Println("\n🏛️ Palace-Mnemosyne Memory Palace Hierarchy:")
	fmt.Printf("Total Factual Drawers: %d\n\n", stats.TotalMemories)
	for room, count := range stats.Rooms {
		fmt.Printf("  🚪 Room [%s] (%d entries):\n", room, count)
		mems, _ := eng.MemoryStore.SearchMemories("", room, 3, 0.0)
		for _, m := range mems {
			fmt.Printf("     • [%s] %s (decay: %.2f)\n", m.Hall, m.Content, m.DecayFactor)
		}
	}
	fmt.Println()
}

func runHookMemory(configPath string) {
	raw, err := io.ReadAll(os.Stdin)
	if err != nil || len(raw) == 0 {
		fmt.Println(`{"injectSteps": []}`)
		return
	}

	cfg, _ := config.LoadConfig(configPath)
	eng, err := engine.NewUnleashedEngine(cfg)
	if err != nil || eng.MemoryStore == nil {
		fmt.Println(`{"injectSteps": []}`)
		return
	}
	defer closeEngine(eng)

	recent, err := eng.MemoryStore.GetRecentMemories(3)
	if err != nil || len(recent) == 0 {
		fmt.Println(`{"injectSteps": []}`)
		return
	}

	var lines []string
	for _, m := range recent {
		lines = append(lines, fmt.Sprintf("- [%s:%s] %s", m.Room, m.Hall, m.Content))
	}

	injectedMsg := fmt.Sprintf("🏛️ [Palace-Mnemosyne Memory]:\n%s", strings.Join(lines, "\n"))
	outJSON, _ := json.Marshal(map[string]interface{}{
		"injectSteps": []map[string]string{
			{"ephemeralMessage": injectedMsg},
		},
	})
	fmt.Println(string(outJSON))
}

func runHookReflect(configPath string) {
	raw, err := io.ReadAll(os.Stdin)
	if err != nil || len(raw) == 0 {
		fmt.Println(`{}`)
		return
	}

	var payload map[string]interface{}
	_ = json.Unmarshal(raw, &payload)

	cfg, _ := config.LoadConfig(configPath)
	eng, err := engine.NewUnleashedEngine(cfg)
	if err != nil || eng.MemoryStore == nil {
		fmt.Println(`{}`)
		return
	}
	defer closeEngine(eng)

	convID, _ := payload["conversationId"].(string)
	if convID == "" {
		convID = "unknown"
	}
	if len(convID) > 8 {
		convID = convID[:8]
	}

	termReason, _ := payload["terminationReason"].(string)
	if termReason == "" {
		termReason = "completed"
	}

	_, _ = eng.MemoryStore.AddPalaceMemory(
		"default",
		"lessons",
		"lesson",
		fmt.Sprintf("Completed task in conversation %s with status '%s'.", convID, termReason),
		"antigravity_stop_hook",
		payload,
	)

	fmt.Println(`{}`)
}
