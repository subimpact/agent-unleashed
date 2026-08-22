# 🔌 Universal Pluggable CLI Adapters

`agent-unleashed` operates as a **Zero-API-Key Meta-Harness** that multiplexes your existing authenticated AI CLI tools.

---

## 🎯 Supported Drivers

| Driver Key | Binary / Target | Execution Semantics | Auth Mechanism |
| :--- | :--- | :--- | :--- |
| **`agy`** | `agy` / `antigravity` | Google Antigravity Subagent Pipe | Google Account Session |
| **`claude`** | `claude` | Anthropic Claude Code CLI | Anthropic Local Login |
| **`aider`** | `aider` | Aider Multi-File Architect | Local CLI Session |
| **`ollama`** | `ollama` / `curl` | Local Offline LLMs (DeepSeek-R1, Qwen2.5) | Localhost (100% Offline) |
| **`api`** | Direct HTTPS | Direct OpenAI/Gemini/Anthropic API | API Key in `config.yaml` |

---

## 🔀 Dynamic Driver Switching

You can switch the active execution driver on the fly without restarting the daemon:

### In Terminal REPL:
```bash
# List all detected AI CLI drivers on your system
:drivers

# Switch to Google Antigravity
:driver agy

# Switch to Claude Code
:driver claude

# Switch to local Ollama (Offline mode)
:driver ollama
```

### In `config.yaml`:
```yaml
system:
  active_driver: "auto" # Or explicitly "agy", "claude", "aider", "ollama", "api"
```

---

## 📊 Telemetry & Token Tracking (`pkg/adapters/`)

When a task executes, the adapter intercepts stdout/stderr streams to extract fine-grained telemetry:

```go
type ExecutionResult struct {
	Output          string        `json:"output"`
	InputTokens     int           `json:"input_tokens"`
	OutputTokens    int           `json:"output_tokens"`
	ThinkingTokens  int           `json:"thinking_tokens"`
	CacheReadTokens int           `json:"cache_read_tokens"`
	TotalTokens     int           `json:"total_tokens"`
	DurationSeconds float64       `json:"duration_seconds"`
	NumTurns        int           `json:"num_turns"`
	ContextLimit    int           `json:"context_limit"`
}
```

These metrics are immediately reported in the **Live Bottom Telemetry Bar** and aggregated in the session stats dashboard (`:stats`).

---

## 🛠️ Building Custom Adapters

To implement a new CLI driver, satisfy the `CLIAdapter` Go interface:

```go
package adapters

import "context"

type CLIAdapter interface {
	Name() string
	IsAvailable() bool
	Execute(ctx context.Context, prompt string, workspaceDir string) (*ExecutionResult, error)
}
```

Register your adapter in `pkg/adapters/registry.go`:
```go
registry.Register(NewCustomAdapter())
```
