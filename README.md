# ⚡ Agent-Unleashed (`agt-ul`)

<div align="center">

[![Go Version](https://img.shields.io/badge/Go-1.23+-00ADD8?style=flat&logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-emerald.svg)](LICENSE)
[![Platform](https://img.shields.io/badge/Platform-Windows%20%7C%20macOS%20%7C%20Linux-black)](https://agent.subimpact.net)
[![Website](https://img.shields.io/badge/Website-agent.subimpact.net-10b981)](https://agent.subimpact.net)
[![Buy Me A Coffee](https://img.shields.io/badge/Buy%20Me%20A%20Coffee-subimpact-FFDD00?style=flat&logo=buy-me-a-coffee&logoColor=black)](https://buymeacoffee.com/subimpact)
[![RAM](https://img.shields.io/badge/RAM-%3C15MB-cyan)](https://agent.subimpact.net)
[![Zero API Keys](https://img.shields.io/badge/API%20Keys-%240.00%20(Local%20CLI)-success)](https://agent.subimpact.net)

**The Universal 24/7 Agent Gateway & Cognitive Operating System (Written in Pure Go)**

[Website](https://agent.subimpact.net) &bull; [☕ Support](https://buymeacoffee.com/subimpact) &bull; [Quickstart](#-quickstart--installation) &bull; [Architecture](docs/ARCHITECTURE.md) &bull; [Memory Palace](docs/MEMORY_PALACE.md) &bull; [CLI Adapters](docs/CLI_ADAPTERS.md) &bull; [Gateways](docs/GATEWAYS.md) &bull; [Cron Engine](docs/CRON_AUTOMATIONS.md)

</div>

---

`agent-unleashed` (`agt-ul`) turns **any local AI CLI tool** (Google Antigravity `agy`, Claude Code `claude`, `aider`, `ollama`, or Cloud APIs) into a 24/7 autonomous, self-learning daemon with a persistent **Palace-Mnemosyne cognitive memory engine**, **in-process cron automations**, and **live context telemetry**.

---

## 🏛️ System Architecture

```mermaid
graph TD
    User([Telegram / Discord / CLI / WebSockets / Webhooks]) -->|Requests| Daemon[agt-ul Universal Go Master Daemon]
    
    Daemon --> Ingress[24/7 Multi-Channel Ingress: pkg/gateways/]
    Ingress --> Core[Engine Orchestrator: pkg/engine/]

    subgraph Zero-API-Key Local CLI Subprocesses
        Core --> Router[Universal CLI Adapter & Model Router: pkg/adapters/]
        Router --> Agy[Google Antigravity: agy]
        Router --> Claude[Claude Code: claude]
        Router --> Aider[Aider CLI: aider]
        Router --> Ollama[Local Offline Ollama: ollama]
    end

    subgraph Palace-Mnemosyne Cognitive Memory
        Core <-->|Spatial Loci + FTS5 + Vector Cosine| Palace[(SQLite Memory Palace: pkg/memory/)]
        Palace --> Decay[Ebbinghaus Decay: R = e^-λΔt]
        Palace --> Profile[Dialectic User Persona: room:user_profile]
    end

    subgraph Autonomous Background Scheduling
        Core <-->|5-Part Cron Evaluator| CronEngine[⏰ In-Process 24/7 Cron Engine: pkg/cron/]
    end

    subgraph Progressive Skill Intelligence
        Core <-->|1-Line YAML Indexing| SkillIndexer[Progressive Skill Indexer: pkg/skills/]
    end

    subgraph Diagnostics & Self-Healing
        Daemon <--> Doctor[🩺 10-Point System Doctor: pkg/doctor/]
        Daemon <--> Updater[🔄 Self-Updater: pkg/updater/]
    end
```

---

## 🌟 Core Superpowers

1. **🔌 Universal Pluggable CLI Adapters:** Multiplexes `agy`, `claude`, `aider`, `ollama`, and cloud APIs with zero API key billing. Switch drivers on the fly via `:driver <name>`. See [CLI Adapters Guide](docs/CLI_ADAPTERS.md).
2. **🏛️ Palace-Mnemosyne Cognitive Memory:** Spatial method-of-loci hierarchy (Wings $\rightarrow$ Rooms $\rightarrow$ Drawers) + Ebbinghaus temporal decay ($R = e^{-\lambda \Delta t}$) + FTS5 vector hybrid RAG. See [Memory Palace Guide](docs/MEMORY_PALACE.md).
3. **⏰ In-Process 24/7 Cron Engine:** Native Goroutine scheduler for recurring unattended tasks with SQLite persistence. See [Cron Automations Guide](docs/CRON_AUTOMATIONS.md).
4. **📊 Live Context Dashboard & Verbose Telemetry:** Real-time bottom console bar tracking input tokens, thinking tokens, cache read rates, and latency per turn (`:context`, `:verbose`).
5. **🌐 24/7 Multi-Channel Gateways:** Real-time WebSocket streaming (`ws://localhost:8080/ws`), Telegram Bot, Discord DM and mention pair programming, and REST APIs. Every gateway is authenticated and fails closed — see [Gateways Guide](docs/GATEWAYS.md).
6. **🩺 System Doctor & Diagnostics:** 10-point system health audit (`agt-ul doctor`) and auto-remediation (`agt-ul doctor --fix`). See [Doctor & Updater Guide](docs/DOCTOR_AND_UPDATER.md).
7. **⚡ Built-In Skill Package Manager:** Install specialized skills directly from GitHub (`agt-ul skill install Leonxlnx/taste-skill`). See [Skills & Plugins Guide](docs/SKILLS_AND_PLUGINS.md).
8. **🧠 Lossless Context Management (LCM):** Hierarchical summary nodes + a permanent SQLite message ledger, replayed into every prompt within a token budget, with verbatim retrieval of anything compressed (`:lcm grep`, `:lcm describe`, `:lcm expand`). Inspired by [hermes-lcm](https://github.com/stephenschoettler/hermes-lcm). See [Lossless Context Management Guide](docs/LOSSLESS_CONTEXT_MANAGEMENT.md).

---

## 🚀 Quickstart & Installation

### 1. Install `agent-unleashed` (`agt-ul`)

#### Windows (PowerShell)
```powershell
irm https://agent.subimpact.net/install.ps1 | iex
```

#### macOS / Linux / Termux
```bash
curl -fsSL https://agent.subimpact.net/install.sh | bash
```

#### Build from Source
```bash
git clone https://github.com/subimpact/agent-unleashed.git
cd agent-unleashed
go build -o agt-ul.exe .
```

---

## 🛠️ Prerequisites: Supported AI CLIs Setup

`agent-unleashed` orchestrates your local AI CLI tools. You only need **at least one** installed:

| CLI Tool | Install Command | Auth Command |
| :--- | :--- | :--- |
| **🚀 Google Antigravity (`agy`)** | `npm install -g @google/antigravity-cli` | `agy auth login` |
| **🧠 Claude Code (`claude`)** | `npm install -g @anthropic-ai/claude-code` | `claude` *(Browser login)* |
| **⚡ OpenAI Codex (`codex`)** | `npm install -g @openai/codex` | `codex auth login` |
| **🤖 Aider (`aider`)** | `pipx install aider-chat` *(or `pip install aider-chat`)* | Uses local env/keys |
| **🦙 Ollama (`ollama`)** | [ollama.com/download](https://ollama.com/download) / `brew install ollama` | `ollama pull qwen2.5-coder:7b` |

👉 **Full Step-by-Step Guide:** See [AI CLIs Installation Guide](docs/CLI_INSTALLATION_GUIDE.md).

---

## 📖 Command Reference

### CLI Subcommands
| Command | Action |
| :--- | :--- |
| **`agt-ul`** | Launch interactive REPL & 24/7 background daemon |
| **`agt-ul -v`** | Launch in Verbose debug mode |
| **`agt-ul update`** | Self-update and recompile binary from source |
| **`agt-ul doctor`** | Run 10-point system health check & diagnostics |
| **`agt-ul doctor --fix`** | Run diagnostics and auto-repair missing folders/configs |
| **`agt-ul skill install <repo>`** | Install skill from GitHub (e.g. `Leonxlnx/taste-skill`) |
| **`agt-ul skills`** | List all discovered specialized skills |
| **`agt-ul setup`** | Run interactive configuration wizard |
| **`agt-ul status`** | Display detected CLI tools & system diagnostics |
| **`agt-ul memory`** | Inspect Palace-Mnemosyne memory stats |
| **`agt-ul wiki`** | Browse compiled project LLM-Wiki articles |
| **`agt-ul lcm`** | Lossless Context Management summary & message DAG |
| **`agt-ul cron`** | List active background scheduled tasks |
| **`agt-ul version`** | Display version and build info |

### Interactive REPL Shortcuts
| Command | Action |
| :--- | :--- |
| **`:doctor`** | Run system diagnostics |
| **`:update`** | Trigger self-updater |
| **`:context`** | Visual context window gauge & token breakdown |
| **`:verbose`** | Toggle verbose mode on/off |
| **`:profile`** | View dialectic user persona & coding profile |
| **`:wiki`** | Browse or search project LLM-Wiki knowledge graph (`:wiki list/read/search`) |
| **`:lcm`** | Lossless Context Management DAG inspector (`:lcm describe/grep/expand`) |
| **`:cron`** | Manage background cron tasks (`:cron list`, `:cron add`, `:cron remove`) |
| **`:stats`** | View session execution diagnostics and token metrics |
| **`:drivers`** | List all detected AI CLI tools |
| **`:driver <name>`** | Switch active driver (e.g. `:driver agy`, `:driver claude`, `:driver codex`) |
| **`:memory`** | Browse Palace-Mnemosyne memory drawers |
| **`:skills`** | List learned skill runbooks |
| **`:clear`** | Clear terminal screen |
| **`:exit`** | Exit CLI |

---

## 🔒 Security Model

`agt-ul` drives a coding agent against your workspace, so every remote entry point is authenticated and **fails closed**.

| Surface | Default | How to open it |
| :--- | :--- | :--- |
| **REST `/api/v1/*`** | Token required. Bound to `127.0.0.1`. | Blank `webhook_secret` generates one into `data/rest_api_token`. Send `Authorization: Bearer <token>`. |
| **WebSocket `/ws`** | Token required, checked during the handshake. | Same token, as a header or `?token=<token>`. |
| **Browser origins** | All cross-site requests refused; no wildcard CORS. | Add your UI to `gateways.rest_api.allowed_origins`. |
| **DNS rebinding** | Non-loopback `Host` headers refused on a loopback bind. | Bind a public interface deliberately. |
| **Telegram** | Refuses every chat. | Add IDs to `allowed_chat_ids` (or set `admin_chat_id`). Message the bot and it replies with its chat ID. |
| **Discord** | Refuses every channel. | Add IDs to `allowed_channel_ids` or `guild_ids`. |
| **Permission bypass** | `auto_approve_tools: true` passes `--dangerously-skip-permissions` / `--yes` to the driver. | Set `false` to keep each CLI's own confirmation prompts. |

`/health` and `/healthz` stay unauthenticated for process supervisors and expose nothing about the workspace.

> **Discord setup note:** message content is a privileged intent. Enable **Bot -> Privileged Gateway Intents -> Message Content** in the Discord developer portal, or the bot connects but receives empty messages.

---

## 📊 Architectural Benchmark

| Feature / Dimension | ⚡ Agent-Unleashed (`agt-ul`) | 🌐 OpenClaw | 🧠 Hermes Agent | 🤖 Aider |
| :--- | :--- | :--- | :--- | :--- |
| **Language & Footprint** | **Pure Go (&lt;15MB RAM)** | Node.js (~150MB) | Python (~200MB) | Python (~180MB) |
| **Local CLI Multiplexing** | **✅ agy, claude, codex, aider, ollama** | ❌ API Keys Only | ❌ API / vLLM only | ⚠️ Single-CLI |
| **Lossless Context (LCM)** | **✅ Native DAG + Verbatim Ledger** | ❌ Lossy Sliding Window | ⚠️ Optional Plugin | ❌ Sliding Window |
| **Auto-Compiled LLM-Wiki** | **✅ Native Interlinked Markdown** | ❌ None | ❌ None | ❌ None |
| **Cognitive Long-Term Memory** | **✅ Palace-Mnemosyne + Decay** | Basic Vector Store | ✅ Honcho Dialectic | ❌ Git Session only |
| **Live Context Telemetry Bar** | **✅ Live Bar + :context Meter** | ❌ None | ❌ Logs only | Basic summary |
| **In-Process 24/7 Cron** | **✅ SQLite-Backed 5-Part Cron** | ✅ External cron | ✅ Agent cron | ❌ Interactive only |
| **Diagnostics & Self-Healing** | **✅ agt-ul doctor & update** | Basic status | ✅ hermes doctor | ❌ None |

---

## 📚 Complete Documentation Suite

* 🟢 **[Non-Technical Beginner's Quickstart Guide](docs/NON_TECHNICAL_BEGINNERS_GUIDE.md)** *(Start here if you have zero coding experience!)*
* 🛠️ **[Supported AI CLIs Setup & Auth Guide](docs/CLI_INSTALLATION_GUIDE.md)**
* 🏗️ **[System Architecture & Runtime](docs/ARCHITECTURE.md)**
* 🏛️ **[Palace-Mnemosyne Memory Engine](docs/MEMORY_PALACE.md)**
* 🧠 **[Lossless Context Management (LCM) DAG Guide](docs/LOSSLESS_CONTEXT_MANAGEMENT.md)**
* 🔌 **[Universal CLI Adapters](docs/CLI_ADAPTERS.md)**
* 🌐 **[Multi-Channel Gateways, WebSockets & REST APIs](docs/GATEWAYS.md)**
* ⏰ **[In-Process Autonomous Cron Engine](docs/CRON_AUTOMATIONS.md)**
* ⚡ **[Progressive Skill Engine & Package Manager](docs/SKILLS_AND_PLUGINS.md)**
* 🩺 **[System Doctor & Self-Updater](docs/DOCTOR_AND_UPDATER.md)**

---

## ☕ Support the Project

If you find `agent-unleashed` valuable and would like to support continued development:

[![Buy Me A Coffee](https://cdn.buymeacoffee.com/buttons/v2/default-yellow.png)](https://buymeacoffee.com/subimpact)

👉 **[buymeacoffee.com/subimpact](https://buymeacoffee.com/subimpact)**

---

## 📄 License

MIT License &copy; 2026 subimpact.net
