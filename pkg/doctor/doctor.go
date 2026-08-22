package doctor

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"agent-unleashed/pkg/adapters"
	"agent-unleashed/pkg/config"
	"agent-unleashed/pkg/skills"
)

type DiagnosticStatus string

const (
	StatusPass DiagnosticStatus = "PASS"
	StatusWarn DiagnosticStatus = "WARN"
	StatusFail DiagnosticStatus = "FAIL"
)

type CheckResult struct {
	Category    string           `json:"category"`
	Name        string           `json:"name"`
	Status      DiagnosticStatus `json:"status"`
	Message     string           `json:"message"`
	Remediation string           `json:"remediation,omitempty"`
}

type DoctorReport struct {
	Timestamp  time.Time     `json:"timestamp"`
	Results    []CheckResult `json:"results"`
	Passed     int           `json:"passed"`
	Warnings   int           `json:"warnings"`
	Failures   int           `json:"failures"`
	FixedCount int           `json:"fixed_count"`
}

func RunDiagnostics(configPath string, autoFix bool) *DoctorReport {
	report := &DoctorReport{
		Timestamp: time.Now(),
	}

	addResult := func(cat, name string, status DiagnosticStatus, msg, remediation string) {
		res := CheckResult{
			Category:    cat,
			Name:        name,
			Status:      status,
			Message:     msg,
			Remediation: remediation,
		}
		report.Results = append(report.Results, res)
		switch status {
		case StatusPass:
			report.Passed++
		case StatusWarn:
			report.Warnings++
		case StatusFail:
			report.Failures++
		}
	}

	// 1. Environment & Runtime
	addResult("System", "Go Runtime", StatusPass,
		fmt.Sprintf("OS: %s/%s | Go: %s | CPUs: %d", runtime.GOOS, runtime.GOARCH, runtime.Version(), runtime.NumCPU()), "")

	// 2. Directories & Permissions
	dirsToCheck := []string{"./data", "./.agents/skills"}
	for _, dir := range dirsToCheck {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			if autoFix {
				_ = os.MkdirAll(dir, 0755)
				report.FixedCount++
				addResult("Filesystem", fmt.Sprintf("Directory %s", dir), StatusPass,
					fmt.Sprintf("Created missing directory %s (Auto-Fixed)", dir), "")
			} else {
				addResult("Filesystem", fmt.Sprintf("Directory %s", dir), StatusWarn,
					fmt.Sprintf("Directory %s does not exist", dir), "Run 'agt-ul doctor --fix' to create directory")
			}
		} else {
			testFile := filepath.Join(dir, ".write_test")
			if err := os.WriteFile(testFile, []byte("ok"), 0644); err != nil {
				addResult("Filesystem", fmt.Sprintf("Write Permission %s", dir), StatusFail,
					fmt.Sprintf("Cannot write to %s: %v", dir, err), "Check user permissions")
			} else {
				_ = os.Remove(testFile)
				addResult("Filesystem", fmt.Sprintf("Directory %s", dir), StatusPass,
					"Directory exists and is writable", "")
			}
		}
	}

	// 3. Configuration Check
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		if autoFix {
			_ = os.WriteFile(configPath, []byte("# Agent-Unleashed default\n"), 0644)
			report.FixedCount++
			addResult("Config", "Configuration File", StatusWarn,
				"Regenerated default config.yaml (Auto-Fixed)", "")
		} else {
			addResult("Config", "Configuration File", StatusFail,
				fmt.Sprintf("Failed to load %s: %v", configPath, err), "Run 'agt-ul setup' or 'agt-ul doctor --fix'")
		}
	} else {
		addResult("Config", "Configuration File", StatusPass,
			fmt.Sprintf("Loaded %s successfully (Agent: %s, Driver: %s)", configPath, cfg.System.AgentName, cfg.Model.Driver), "")
	}

	// 4. AI CLI Adapters Health & Connectivity
	reg := adapters.NewAdapterRegistry()
	if cfg != nil {
		reg.Register(adapters.NewAgyAdapter(cfg.Model.AgyBinaryPath))
		reg.Register(adapters.NewClaudeAdapter(cfg.Model.ClaudeBinaryPath))
		reg.Register(adapters.NewAiderAdapter(cfg.Model.AiderBinaryPath))
		reg.Register(adapters.NewOllamaAdapter(cfg.Model.OllamaEndpoint))
		reg.Register(adapters.NewAPIAdapter(cfg.Model))
	} else {
		reg.Register(adapters.NewAgyAdapter(""))
		reg.Register(adapters.NewClaudeAdapter(""))
		reg.Register(adapters.NewAiderAdapter(""))
		reg.Register(adapters.NewOllamaAdapter(""))
	}

	available := reg.ListAvailable()
	if len(available) == 0 {
		addResult("AI Drivers", "Active AI CLI Driver", StatusWarn,
			"No local AI CLI agents detected in PATH", "Install Google Antigravity 'agy', Claude Code 'claude', or configure API keys with 'agt-ul setup'")
	} else {
		var names []string
		for _, a := range available {
			names = append(names, a.Name())
		}
		addResult("AI Drivers", "Active AI CLI Driver", StatusPass,
			fmt.Sprintf("Found %d active driver(s): [%s]", len(available), strings.Join(names, ", ")), "")
	}

	// 5. Database & Memory Palace Integrity
	dbPath := "./data/memory.sqlite"
	if cfg != nil && cfg.Memory.DBPath != "" {
		dbPath = cfg.Memory.DBPath
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		addResult("Database", "SQLite Connection", StatusFail,
			fmt.Sprintf("Cannot open database %s: %v", dbPath, err), "Ensure database path is writable")
	} else {
		defer db.Close()

		var integrity string
		err := db.QueryRow("PRAGMA integrity_check;").Scan(&integrity)
		if err != nil || integrity != "ok" {
			addResult("Database", "SQLite Integrity", StatusFail,
				fmt.Sprintf("Integrity check failed: %v", err), "Database file may be corrupted")
		} else {
			addResult("Database", "SQLite Integrity", StatusPass, "PRAGMA integrity_check: OK", "")
		}

		var count int
		_ = db.QueryRow("SELECT COUNT(*) FROM memories").Scan(&count)
		addResult("Database", "Memory Palace Drawers", StatusPass,
			fmt.Sprintf("%d persistent factual memories stored", count), "")

		var cronCount int
		_ = db.QueryRow("SELECT COUNT(*) FROM cron_jobs").Scan(&cronCount)
		addResult("Database", "Cron Automations", StatusPass,
			fmt.Sprintf("%d scheduled background jobs registered", cronCount), "")
	}

	// 6. Network Gateway Ports & Connectivity
	host := "127.0.0.1"
	port := 8080
	if cfg != nil && cfg.Gateways.RESTAPI.Port > 0 {
		port = cfg.Gateways.RESTAPI.Port
	}
	addr := fmt.Sprintf("%s:%d", host, port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		addResult("Network", fmt.Sprintf("REST/WebSocket Port %d", port), StatusWarn,
			fmt.Sprintf("Port %d is already in use (daemon may be running)", port), "")
	} else {
		_ = ln.Close()
		addResult("Network", fmt.Sprintf("REST/WebSocket Port %d", port), StatusPass,
			fmt.Sprintf("Port %d is free and ready", port), "")
	}

	// 7. Telegram Bot Token Verification
	if cfg != nil && cfg.Gateways.Telegram.Enabled && cfg.Gateways.Telegram.BotToken != "" {
		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Get(fmt.Sprintf("https://api.telegram.org/bot%s/getMe", cfg.Gateways.Telegram.BotToken))
		if err != nil || resp.StatusCode != 200 {
			addResult("Gateways", "Telegram Bot Auth", StatusFail,
				"Telegram bot token invalid or connection error", "Check token with BotFather or re-run 'agt-ul setup'")
		} else {
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)
			var tgRes struct {
				OK     bool `json:"ok"`
				Result struct {
					Username string `json:"username"`
				} `json:"result"`
			}
			if err := json.Unmarshal(body, &tgRes); err == nil && tgRes.OK {
				addResult("Gateways", "Telegram Bot Auth", StatusPass,
					fmt.Sprintf("Connected as @%s", tgRes.Result.Username), "")
			}
		}
	}

	// 8. Skills Indexing & Frontmatter Validation
	skillsDir := "./.agents/skills"
	if cfg != nil && cfg.System.SkillsDir != "" {
		skillsDir = cfg.System.SkillsDir
	}
	indexer := skills.NewSkillIndexer(skillsDir)
	skillsList := indexer.ListSkills()
	addResult("Skills", "Progressive Skill Indexer", StatusPass,
		fmt.Sprintf("Discovered %d specialized skill runbooks in %s", len(skillsList), skillsDir), "")

	return report
}

func PrintDoctorReport(report *DoctorReport) {
	fmt.Println("\n============================================================")
	fmt.Println("  🩺 AGENT-UNLEASHED (agt-ul) SYSTEM DOCTOR & DIAGNOSTICS")
	fmt.Println("============================================================")

	currentCat := ""
	for _, res := range report.Results {
		if res.Category != currentCat {
			currentCat = res.Category
			fmt.Printf("\n📋 [%s]\n", currentCat)
		}

		icon := "✅"
		if res.Status == StatusWarn {
			icon = "⚠️"
		} else if res.Status == StatusFail {
			icon = "❌"
		}

		fmt.Printf("  %s %-30s : %s\n", icon, res.Name, res.Message)
		if res.Remediation != "" {
			fmt.Printf("     └─ 💡 Fix: %s\n", res.Remediation)
		}
	}

	fmt.Println("\n============================================================")
	fmt.Printf("  Diagnostics Summary: %d Passed | %d Warnings | %d Failures\n",
		report.Passed, report.Warnings, report.Failures)
	if report.FixedCount > 0 {
		fmt.Printf("  ✨ Auto-Repaired %d issue(s) successfully!\n", report.FixedCount)
	}
	if report.Failures == 0 && report.Warnings == 0 {
		fmt.Println("  🚀 System Status: All checks passed. Agent-Unleashed is healthy!")
	} else if report.Failures > 0 {
		fmt.Println("  ⚠️ System Status: Issues detected. Try running 'agt-ul doctor --fix'")
	}
	fmt.Println("============================================================")
	fmt.Println()
}
