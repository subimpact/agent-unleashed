package gateways

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"agent-unleashed/pkg/adapters"
	"agent-unleashed/pkg/cron"
	"agent-unleashed/pkg/doctor"
	"agent-unleashed/pkg/engine"
	"agent-unleashed/pkg/updater"
)

type CLIGateway struct {
	engine *engine.UnleashedEngine
	cronEng *cron.CronEngine
}

func NewCLIGateway(eng *engine.UnleashedEngine, ce *cron.CronEngine) *CLIGateway {
	return &CLIGateway{engine: eng, cronEng: ce}
}

func (c *CLIGateway) Start(ctx context.Context) error {
	fmt.Println("\n================================================================================")
	fmt.Println("  🚀 AGENT-UNLEASHED (agt-ul) Universal Agent Harness & Gateway")
	fmt.Println("  Palace-Mnemosyne Memory | Multi-CLI Orchestration | Live Context Dashboard")
	fmt.Println("================================================================================")
	fmt.Printf("Active Driver: [%s] | Verbose: [%v] | Type ':help' for commands, ':exit' to quit.\n\n",
		c.engine.GetActiveDriverName(), c.engine.IsVerbose())

	reader := bufio.NewReader(os.Stdin)
	sessionID := "cli_main"

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		driverBadge := c.engine.GetActiveDriverName()
		if c.engine.IsVerbose() {
			driverBadge += "-verbose"
		}

		fmt.Printf("👤 You [%s] > ", driverBadge)
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

		if lower == ":clear" {
			fmt.Print("\033[H\033[2J")
			continue
		}

		if lower == ":help" {
			c.printHelp()
			continue
		}

		if lower == ":verbose" {
			newV := !c.engine.IsVerbose()
			c.engine.SetVerbose(newV)
			if newV {
				fmt.Println("🔍 Verbose Mode: [ENABLED] (Showing commands, token breakdowns, and raw outputs)")
			} else {
				fmt.Println("🔇 Verbose Mode: [DISABLED]")
			}
			fmt.Println()
			continue
		}

		if lower == ":update" || lower == ":upgrade" {
			_, err := updater.RunUpdate(".", false)
			if err != nil {
				fmt.Printf("❌ Update failed: %v\n\n", err)
			}
			continue
		}

		if lower == ":doctor" {
			rep := doctor.RunDiagnostics("config.yaml", false)
			doctor.PrintDoctorReport(rep)
			continue
		}

		if lower == ":context" {
			c.printContextDashboard()
			continue
		}

		if lower == ":stats" {
			c.printSessionStats()
			continue
		}

		if lower == ":profile" {
			if c.engine.ProfileMgr != nil {
				fmt.Println("\n============================================================")
				fmt.Println("  👤 DIALECTIC USER PERSONA & CODING PROFILE")
				fmt.Println("============================================================")
				fmt.Println(c.engine.ProfileMgr.GetSummary())
				fmt.Println("============================================================")
				fmt.Println()
			}
			continue
		}

		if strings.HasPrefix(lower, ":cron") {
			c.handleCronCommand(input)
			continue
		}

		if lower == ":drivers" {
			c.printDrivers()
			continue
		}

		if strings.HasPrefix(lower, ":driver ") {
			targetDriver := strings.TrimSpace(input[8:])
			c.engine.SetDriver(targetDriver)
			fmt.Printf("🔄 Switched active driver to: [%s]\n\n", targetDriver)
			continue
		}

		if strings.HasPrefix(lower, ":wiki") {
			c.handleWikiCommand(input)
			continue
		}

		if lower == ":memory" {
			c.printMemoryPalace()
			continue
		}

		if lower == ":skills" {
			c.printSkills()
			continue
		}

		fmt.Print("\n🤖 Agent > ")
		events := make(chan engine.Event)
		go c.engine.Chat(ctx, sessionID, input, "cli", events)

		var lastStats *adapters.ExecutionResult

		for ev := range events {
			switch ev.Type {
			case engine.EventText:
				fmt.Print(ev.Content)

			case engine.EventMemory:
				fmt.Printf("\n[🏛️ Recalled %d memories from Palace into context]\n", ev.Count)
				if c.engine.IsVerbose() {
					for _, m := range ev.Memories {
						fmt.Printf("   ├─ %s\n", m)
					}
				}

			case engine.EventThought:
				if c.engine.IsVerbose() {
					fmt.Printf("[%s]\n", ev.Content)
				}

			case engine.EventInsight:
				for _, ins := range ev.Insights {
					fmt.Printf("\n[✨ Self-Learning: %s]\n", ins)
				}

			case engine.EventUsage:
				lastStats = ev.Stats
			}
		}
		fmt.Println()

		// Render Bottom Live Context / Console Bar
		if lastStats != nil {
			c.renderBottomConsoleBar(lastStats)
		} else {
			lastRes := c.engine.GetLastResult()
			if lastRes != nil {
				c.renderBottomConsoleBar(lastRes)
			}
		}
		fmt.Println()
	}

	return nil
}

func (c *CLIGateway) renderBottomConsoleBar(stats *adapters.ExecutionResult) {
	used := stats.TotalTokens
	limit := stats.ContextLimit
	if limit <= 0 {
		limit = 1048576 // Default 1M
	}
	left := limit - used
	if left < 0 {
		left = 0
	}
	pctUsed := float64(used) / float64(limit) * 100.0
	pctLeft := 100.0 - pctUsed

	fmt.Println("────────────────────────────────────────────────────────────────────────────────")
	fmt.Printf("📊 [Context: %s / %s tok (%.1f%% left) | In: %d | Out: %d",
		formatNumber(used), formatNumber(limit), pctLeft, stats.InputTokens, stats.OutputTokens)

	if stats.ThinkingTokens > 0 {
		fmt.Printf(" | Think: %d", stats.ThinkingTokens)
	}
	if stats.CacheReadTokens > 0 {
		fmt.Printf(" | Cache: %d", stats.CacheReadTokens)
	}

	fmt.Printf(" | Time: %.2fs | Driver: %s]\n", stats.DurationSeconds, c.engine.GetActiveDriverName())

	if c.engine.IsVerbose() && stats.RawCommand != "" {
		fmt.Printf("🔍 [Command Executed: %s]\n", stats.RawCommand)
	}
	fmt.Println("────────────────────────────────────────────────────────────────────────────────")
}

func (c *CLIGateway) printContextDashboard() {
	stats := c.engine.GetLastResult()
	limit := 1048576
	used := 0
	inTok := 0
	outTok := 0
	thinkTok := 0

	if stats != nil {
		if stats.ContextLimit > 0 {
			limit = stats.ContextLimit
		}
		used = stats.TotalTokens
		inTok = stats.InputTokens
		outTok = stats.OutputTokens
		thinkTok = stats.ThinkingTokens
	}

	left := limit - used
	pctUsed := float64(used) / float64(limit) * 100.0
	if pctUsed > 100.0 {
		pctUsed = 100.0
	}

	// Visual ASCII Progress Bar (30 chars wide)
	barWidth := 30
	filled := int((pctUsed / 100.0) * float64(barWidth))
	if filled < 0 {
		filled = 0
	}
	empty := barWidth - filled
	bar := strings.Repeat("█", filled) + strings.Repeat("░", empty)

	fmt.Println("\n============================================================")
	fmt.Println("  📊 LIVE CONTEXT WINDOW DASHBOARD")
	fmt.Println("============================================================")
	fmt.Printf("Context Limit       : %s tokens\n", formatNumber(limit))
	fmt.Printf("Tokens Used         : %s tokens (%.2f%%)\n", formatNumber(used), pctUsed)
	fmt.Printf("Tokens Available    : %s tokens (%.2f%%)\n\n", formatNumber(left), 100.0-pctUsed)
	fmt.Printf("Usage Meter         : [%s] %.2f%%\n\n", bar, pctUsed)
	fmt.Printf("Input Prompt Tokens : %s\n", formatNumber(inTok))
	fmt.Printf("Output Tokens       : %s\n", formatNumber(outTok))
	if thinkTok > 0 {
		fmt.Printf("Thinking / CoT Tok  : %s\n", formatNumber(thinkTok))
	}
	fmt.Printf("Session Total Tok   : %s\n", formatNumber(c.engine.GetTotalTokens()))
	fmt.Println("============================================================")
	fmt.Println()
}

func (c *CLIGateway) printSessionStats() {
	fmt.Println("\n============================================================")
	fmt.Println("  📈 SESSION DIAGNOSTICS & METRICS")
	fmt.Println("============================================================")
	fmt.Printf("Active Driver       : %s\n", c.engine.GetActiveDriverName())
	fmt.Printf("Verbose Mode        : %v\n", c.engine.IsVerbose())
	fmt.Printf("Cumulative Tokens   : %s\n", formatNumber(c.engine.GetTotalTokens()))

	stats := c.engine.GetLastResult()
	if stats != nil {
		fmt.Printf("Last Turn Latency   : %.2f seconds\n", stats.DurationSeconds)
		fmt.Printf("Last Turn Tokens    : %s\n", formatNumber(stats.TotalTokens))
		if stats.RawCommand != "" {
			fmt.Printf("Last CLI Invocation : %s\n", stats.RawCommand)
		}
	}
	fmt.Println("============================================================")
	fmt.Println()
}

func (c *CLIGateway) handleCronCommand(input string) {
	if c.cronEng == nil {
		fmt.Println("❌ Cron Engine not active.")
		return
	}

	parts := strings.Fields(input)
	if len(parts) == 1 || parts[1] == "list" {
		jobs := c.cronEng.ListJobs()
		fmt.Printf("\n⏰ Active Cron Automations (%d):\n", len(jobs))
		if len(jobs) == 0 {
			fmt.Println("  ⚪ No active scheduled tasks. Add one with ':cron add \"0 9 * * *\" \"Prompt\"'")
		} else {
			for _, j := range jobs {
				fmt.Printf("  • [%s] Schedule: `%s` | Channel: %s | Prompt: %s\n", j.ID, j.Schedule, j.Channel, j.Prompt)
			}
		}
		fmt.Println()
		return
	}

	if parts[1] == "remove" || parts[1] == "rm" || parts[1] == "del" {
		if len(parts) < 3 {
			fmt.Println("Usage: :cron remove <job_id>")
			return
		}
		jobID := parts[2]
		if err := c.cronEng.RemoveJob(jobID); err != nil {
			fmt.Printf("❌ Failed to remove job: %v\n", err)
		} else {
			fmt.Printf("✅ Removed cron job [%s]\n", jobID)
		}
		return
	}

	if parts[1] == "add" {
		// Parse :cron add "<schedule>" "<prompt>"
		raw := strings.TrimPrefix(input, ":cron add")
		raw = strings.TrimSpace(raw)
		// Extract quoted arguments or standard spacing
		splitQuotes := strings.Split(raw, "\"")
		var sched, prompt string
		if len(splitQuotes) >= 4 {
			sched = strings.TrimSpace(splitQuotes[1])
			prompt = strings.TrimSpace(splitQuotes[3])
		} else {
			// Fallback: first 5 tokens are schedule, rest is prompt
			fields := strings.Fields(raw)
			if len(fields) >= 6 {
				sched = strings.Join(fields[:5], " ")
				prompt = strings.Join(fields[5:], " ")
			}
		}

		if sched == "" || prompt == "" {
			fmt.Println("Usage: :cron add \"<5-field-schedule>\" \"<prompt>\"")
			fmt.Println("Example: :cron add \"0 9 * * *\" \"Check git log and summarize progress\"")
			return
		}

		job, err := c.cronEng.AddJob(sched, prompt, "log", "")
		if err != nil {
			fmt.Printf("❌ Failed to add cron job: %v\n", err)
		} else {
			fmt.Printf("✅ Scheduled Cron Job [%s] with schedule `%s`\n", job.ID, job.Schedule)
		}
		return
	}

	fmt.Println("Unknown cron command. Try ':cron list', ':cron add \"<schedule>\" \"<prompt>\"', or ':cron remove <id>'")
}

func (c *CLIGateway) printHelp() {
	fmt.Println("\n📖 Available Commands:")
	fmt.Println("  :doctor         - Run system health check & diagnostics")
	fmt.Println("  :update         - Self-update and recompile binary from source")
	fmt.Println("  :context        - Visual context window gauge & token breakdown")
	fmt.Println("  :verbose        - Toggle verbose mode on/off (detailed commands & debug)")
	fmt.Println("  :profile        - View & manage dialectic user persona & coding profile")
	fmt.Println("  :wiki           - Browse or search project LLM-Wiki knowledge graph (:wiki list/read/search)")
	fmt.Println("  :cron           - List or manage 24/7 background scheduled tasks (:cron list/add/remove)")
	fmt.Println("  :stats          - View session execution diagnostics and token metrics")
	fmt.Println("  :drivers        - List all detected AI CLI tools on this system")
	fmt.Println("  :driver <name>  - Switch active driver (e.g. ':driver agy', ':driver claude')")
	fmt.Println("  :memory         - Browse Palace-Mnemosyne memory stats and rooms")
	fmt.Println("  :skills         - List learned .agents/skills/ runbooks")
	fmt.Println("  :clear          - Clear terminal screen")
	fmt.Println("  :exit           - Exit CLI")
	fmt.Println()
}

func (c *CLIGateway) handleWikiCommand(input string) {
	if c.engine.WikiEngine == nil {
		fmt.Println("❌ LLM-Wiki is not initialized.")
		return
	}

	parts := strings.Fields(input)
	if len(parts) == 1 || (len(parts) == 2 && parts[1] == "list") {
		pages := c.engine.WikiEngine.ListPages()
		fmt.Printf("\n📚 Project LLM-Wiki Knowledge Base (%d Pages):\n", len(pages))
		for _, p := range pages {
			fmt.Printf("  • [[%-20s]] : %s (%s)\n", p.Slug, p.Title, p.Summary)
		}
		fmt.Println()
		return
	}

	sub := parts[1]
	switch sub {
	case "read", "view":
		if len(parts) < 3 {
			fmt.Println("Usage: :wiki read <slug> (e.g., ':wiki read architecture')")
			return
		}
		slug := parts[2]
		page, err := c.engine.WikiEngine.GetPage(slug)
		if err != nil {
			fmt.Printf("❌ %v\n\n", err)
			return
		}
		fmt.Printf("\n============================================================\n")
		fmt.Println(page.Content)
		fmt.Printf("============================================================\n\n")

	case "search", "find":
		if len(parts) < 3 {
			fmt.Println("Usage: :wiki search <query>")
			return
		}
		query := strings.Join(parts[2:], " ")
		matches := c.engine.WikiEngine.Search(query)
		fmt.Printf("\n🔍 LLM-Wiki Search Results for '%s' (%d matches):\n", query, len(matches))
		for _, m := range matches {
			fmt.Printf("  • [[%-20s]] : %s\n", m.Slug, m.Title)
		}
		fmt.Println()

	default:
		fmt.Println("Usage:")
		fmt.Println("  :wiki                - List all wiki pages")
		fmt.Println("  :wiki read <slug>    - Read a wiki page")
		fmt.Println("  :wiki search <query> - Search across wiki articles")
	}
}

func (c *CLIGateway) printDrivers() {
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
}

func (c *CLIGateway) printMemoryPalace() {
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
}

func (c *CLIGateway) printSkills() {
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
}

func formatNumber(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	if n < 1000000 {
		return fmt.Sprintf("%d,%03d", n/1000, n%1000)
	}
	return fmt.Sprintf("%d,%03d,%03d", n/1000000, (n%1000000)/1000, n%1000)
}
