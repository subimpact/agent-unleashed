package gateways

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"antigravity-unleashed/pkg/engine"
)

type CLIGateway struct {
	engine *engine.UnleashedEngine
}

func NewCLIGateway(eng *engine.UnleashedEngine) *CLIGateway {
	return &CLIGateway{engine: eng}
}

func (c *CLIGateway) Start(ctx context.Context) error {
	fmt.Println("\n============================================================")
	fmt.Println("  🚀 ANTIGRAVITY-UNLEASHED (Go High-Performance Core)")
	fmt.Println("  Autonomous 24/7 Agent with Persistent Memory & Reflection")
	fmt.Println("============================================================")
	fmt.Println("Commands: ':memory' to view memories, ':skills' to view skills, ':exit' to quit.\n")

	reader := bufio.NewReader(os.Stdin)
	sessionID := "cli_main"

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		fmt.Print("👤 You > ")
		input, err := reader.ReadString('\n')
		if err != nil {
			break
		}

		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

		switch strings.ToLower(input) {
		case ":exit", "exit", "quit":
			fmt.Println("\n👋 Shutting down CLI...")
			return nil

		case ":memory":
			if c.engine.MemoryStore != nil {
				mems, err := c.engine.MemoryStore.GetRecentMemories(10)
				if err != nil {
					fmt.Printf("❌ Memory query error: %v\n\n", err)
				} else {
					fmt.Printf("\n🧠 Persistent Memories (%d):\n", len(mems))
					for _, m := range mems {
						fmt.Printf("  • [%s] %s (%s)\n", m.Category, m.Content, m.Source)
					}
					fmt.Println()
				}
			}
			continue

		case ":skills":
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

		fmt.Print("\n🤖 Antigravity > ")
		events := make(chan engine.Event)
		go c.engine.Chat(ctx, sessionID, input, "cli", events)

		for ev := range events {
			switch ev.Type {
			case engine.EventText:
				fmt.Print(ev.Content)
			case engine.EventMemory:
				fmt.Printf("\n[🧠 Recalled %d memories into context]\n", ev.Count)
			case engine.EventThought:
				fmt.Printf("[%s]\n", ev.Content)
			case engine.EventInsight:
				for _, ins := range ev.Insights {
					fmt.Printf("\n[✨ Self-Learning: %s]\n", ins)
				}
			}
		}
		fmt.Println("\n")
	}

	return nil
}
