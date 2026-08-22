# 🚀 Antigravity-Unleashed (Go High-Performance Core)

> **Autonomous 24/7 Agent Daemon & Multi-Channel Gateway for Antigravity (Written in Pure Go)**

`antigravity-unleashed` is a compiled, single-binary autonomous AI agent framework designed for Google Antigravity. It delivers Hermes & OpenClaw-grade autonomy, 24/7 messaging gateways, persistent SQLite hybrid vector memory, and self-learning capabilities with an ultra-low memory footprint (<15MB RAM).

---

## 🌟 Key Architecture & Features

```mermaid
graph TD
    User([Telegram / Discord / CLI / Webhook]) -->|24/7 Remote Requests| Daemon[Go Master Daemon]
    
    subgraph Antigravity-Unleashed (Single Binary .exe)
        Daemon --> Engine[Engine Controller]
        Engine --> Tools[Go Tool Runner: Shell / File I/O / Search]
        
        subgraph Superpowers
            Engine <-->|Hybrid FTS5 + Cosine Vector RAG| Memory[(SQLite Memory)]
            Engine -->|Self-Reflection Loop| Reflection[Skill Synthesizer]
            Reflection -->|Autonomously Authors| Skills[Antigravity .agents/skills/ Directory]
        end
        
        Engine -->|Zero-API-Key Direct Bridge| AgyCLI[Local agy.exe CLI Session]
    end
```

### 1. ⚡ Pure Go Single-Binary Architecture
- **Ultra-Fast & Lightweight:** Compiled native binary with <10ms cold start and ~15MB RAM footprint.
- **Zero Dependencies:** Pure Go embedded SQLite (`modernc.org/sqlite`) requiring no CGO or Python runtime.

### 2. 🌐 24/7 Multi-Channel Gateways
- **Interactive CLI REPL:** Fast terminal interface with `:memory` and `:skills` inspectors.
- **Telegram Bot:** Background polling listener supporting live updates and auto-fallback markdown.
- **Discord Bot / Webhooks:** Multi-user team channel and DM pair programming.
- **REST API:** High-throughput HTTP server on port `8080` for CI/CD and cron triggers.

### 3. 🧠 Hybrid RAG Persistent Memory
- Combines **SQLite FTS5 full-text keyword indexing** with **384-dimensional cosine vector embeddings** for precision recall across all sessions.

### 4. 🪝 Native Antigravity Integration (Zero-API-Key)
- Directly executes through your local `agy.exe` binary with `--effort=high` and `--dangerously-skip-permissions`.
- Auto-authors standard `.agents/skills/<name>/SKILL.md` runbooks on task completion.

---

## 📦 Building & Running

### 1. Build from Source
```bash
cd antigravity-unleashed
go build -o agy-ul.exe .
```

### 2. Global Installation
Copy `agy-ul.exe` to your `agy/bin` directory (or any PATH directory):
```powershell
Copy-Item .\agy-ul.exe "$env:LOCALAPPDATA\agy\bin\agy-ul.exe"
```

### 3. Launch from Anywhere
From any terminal window:
```bash
agy-ul
```

---

## 📁 Repository Structure

```text
antigravity-unleashed/
├── main.go                         # Master daemon entry point
├── config.yaml                     # Active configuration
├── config.yaml.example             # Configuration template
├── go.mod / go.sum                 # Go module definitions
├── .agents/
│   ├── hooks.json                  # Antigravity lifecycle hooks
│   ├── rules/                      # Operational rules
│   └── skills/                     # Self-authored skills
├── data/
│   └── memory.sqlite               # Persistent SQLite + vector store
└── pkg/
    ├── config/                     # YAML loader with env expansion
    ├── memory/                     # Hybrid FTS5 + Vector Cosine SQLite store
    ├── engine/                     # agy.exe subprocess runner, tools, reflection
    └── gateways/                   # CLI REPL, Telegram Bot, REST API
```

---

## 📄 License
MIT License
