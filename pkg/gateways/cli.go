package gateways

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"agent-unleashed/pkg/engine"
)

type CLIGateway struct {
	engine *engine.UnleashedEngine
}

func NewCLIGateway(eng *engine.UnleashedEngine) *CLIGateway {
	return &CLIGateway{engine: eng}
}

func (c *CLIGateway) Start(ctx context.Context) error {
	fmt.Println("\n============================================================")
	fmt.Println("  🚀 AGENT-UNLEASHED (agt-ul) Universal Go Agent Harness")
	fmt.Println("  Multi-CLI Orchestration | Palace-Mnemosyne Memory | 24/7 Gateways")
	fmt.Println("============================================================")
	fmt.Printf("Active Driver: [%s] | Type ':help' for commands, ':exit' to quit.\n\n", c.engine.GetActiveDriverName())

	reader := bufio.NewReader(os.Stdin)
	sessionID := "cli_main"

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		fmt.Printf("👤 You [%s] > ", c.engine.GetActiveDriverName())
		input, err := reader.ReadString('\n')
		if err != nil {
			break
		}

		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

		lower := strings.ToLower(input)

		if lower == ":exit" || lower == "exit" || lower == "quit" {
			fmt.Println("\n👋 Shutting down CLI...")
			return nil
		}

		if lower == ":help" {
			fmt.Println("\n📖 Available Commands:")
			fmt.Println("  :drivers        - List all detected AI CLI tools on this machine")
			fmt.Println("  :driver <name>  - Switch active driver (e.g. ':driver agy', ':driver claude')")
			fmt.Println("  :memory         - View Palace-Mnemosyne memory stats and rooms")
			fmt.Println("  :skills         - List learned .agents/skills/ runbooks")
			fmt.Println("  :exit           - Exit CLI")
			fmt.Println()
			continue
		}

		if lower == ":drivers" {
			fmt.Println("\n🔍 Detected AI CLI Tools on System:")
			available := c.engine.Registry.ListAvailable()
			if len(available) == 0 {
				fmt.Println("  ⚪ No external CLI tools detected. (Cloud API fallback will be used).")
			} else {
				for _, a := range available {
					activeMark := ""
					if a.Name() == c.engine.GetActiveDriverName() {
						activeMark = " ⭐ [ACTIVE]"
					}
					fmt.Printf("  ✅ %s (%s)%s\n", a.DisplayName(), a.BinaryPath(), activeMark)
				}
			}
			fmt.Println()
			continue
		}

		if strings.HasPrefix(lower, ":driver ") {
			targetDriver := strings.TrimSpace(input[8:])
			c.engine.SetDriver(targetDriver)
			fmt.Printf("🔄 Switched active driver to: [%s]\n\n", targetDriver)
			continue
		}

		if lower == ":memory" {
			if c.engine.MemoryStore != nil {
				stats, err := c.engine.MemoryStore.GetStats()
				if err != nil {
					fmt.Printf("❌ Memory query error: %v\n\n", err)
				} else {
					fmt.Printf("\n🏛️ Palace-Mnemosyne Memory Palace (%d Total Drawers):\n", stats.TotalMemories)
					for room, count := range stats.Rooms {
						fmt.Printf("  🚪 Room [%s]: %d drawers\n", room, count)
					}
					fmt.Println("\nRecent Entries:")
					for _, m := range stats.TopAccessed {
						fmt.Printf("  • [%s:%s] %s (decay: %.2f, score: %.2f)\n", m.Room, m.Hall, m.Content, m.DecayFactor, m.Similarity)
					}
					fmt.Println()
				}
			}
			continue
		}

		if lower == ":skills" {
			skillsDir := "./.agents/skills"
			entries, err := os.ReadDir(skillsDir)
			if err != nil {
				fmt.Printf("⚡ No skills found in %s\n\n", skillsDir)
			} else {
				fmt.Printf("\n⚡ Learned Skills (%d):\n", len(entries))
				for _, e := range entries {
					if e.IsDir() {
						fmt.Printf("  • %s\n", e.Name())
					}
				}
				fmt.Println()
			}
			continue
		}

		fmt.Print("\n🤖 Agent > ")
		events := make(chan engine.Event)
		go c.engine.Chat(ctx, sessionID, input, "cli", events)

		for ev := range events {
			switch ev.Type {
			case engine.EventText:
				fmt.Print(ev.Content)
			case engine.EventMemory:
				fmt.Printf("\n[🏛️ Recalled %d memories from Palace into context]\n", ev.Count)
			case engine.EventThought:
				fmt.Printf("[%s]\n", ev.Content)
			case engine.EventInsight:
				for _, ins := range ev.Insights {
					fmt.Printf("\n[✨ Self-Learning: %s]\n", ins)
				}
			}
		}
		fmt.Println()
	}

	return nil
}
