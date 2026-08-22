# 🌐 24/7 Gateways & Communication Protocols

`agent-unleashed` operates as a 24/7 daemon that ingests user requests from multiple communication channels and multiplexes them to the core orchestrator.

---

## 📡 Gateway Overview

```text
┌─────────────────────────────────────────────────────────────┐
│                 AGENT-UNLEASHED GATEWAY INGRESS             │
│                                                             │
│   ┌───────────────┐ ┌───────────────┐ ┌─────────────────┐   │
│   │ Terminal REPL │ │ Telegram Bot  │ │ Discord Bot     │   │
│   │ (Interactive) │ │ (Callbacks)   │ │ (Pairing & DMs) │   │
│   └───────┬───────┘ └───────┬───────┘ └────────┬────────┘   │
│           │                 │                  │            │
│   ┌───────┴─────────────────┴──────────────────┴────────┐   │
│   │     Real-Time WebSocket Hub (ws://127.0.0.1:8080/ws) │   │
│   │     High-Performance REST API (http://127.0.0.1:8080)│   │
│   └─────────────────────────┬────────────────────────────┘   │
│                             │                                │
│                   ▼ Orchestrator Core ▼                      │
└─────────────────────────────────────────────────────────────┘
```

---

## 1. Real-Time WebSocket Streaming (`ws://127.0.0.1:8080/ws`)

Mounted on the REST API server, the WebSocket endpoint allows browser frontends, desktop companion apps, and IDE sidecars to stream execution progress in real-time.

### Protocol JSON Frames:

#### Client $\longrightarrow$ Server (Send Prompt):
```json
{
  "type": "chat",
  "prompt": "Run security audit on auth package",
  "driver": "agy"
}
```

#### Server $\longrightarrow$ Client (Streaming Events):
```json
{
  "type": "token_chunk",
  "content": "Analyzing dependency tree..."
}
```

```json
{
  "type": "telemetry",
  "input_tokens": 14200,
  "output_tokens": 150,
  "duration_seconds": 2.45
}
```

---

## 2. Telegram Bot Gateway (`pkg/gateways/telegram.go`)

Connects via non-blocking long-polling with interactive `InlineKeyboardMarkup` approval buttons.

### Configuration (`config.yaml`):
```yaml
gateways:
  telegram:
    enabled: true
    token: "YOUR_TELEGRAM_BOT_TOKEN"
    allowed_users:
      - 123456789 # Your Telegram User ID
```

### Key Features:
* **Interactive Tool Approvals:** Sends `[✅ Approve]` and `[⛔ Deny]` inline keyboard buttons for sensitive file writes or terminal commands.
* **Typing Indicator & Markdown Escaping:** Automatically sends `typing` chat actions and chunks long responses safely.

---

## 3. Discord Bot Gateway (`pkg/gateways/discord.go`)

Connects to Discord via Bot Token for DM and Guild channel pairing.

### Configuration (`config.yaml`):
```yaml
gateways:
  discord:
    enabled: true
    token: "YOUR_DISCORD_BOT_TOKEN"
    allowed_channels:
      - "123456789012345678"
```

### Key Features:
* Automatically chunks long markdown replies into safe 1950-character blocks.
* Listens to `@Agent-Unleashed` mentions and direct messages.

---

## 4. Terminal REPL (`pkg/gateways/cli.go`)

Interactive terminal interface supporting command shortcuts:

| Command | Action |
| :--- | :--- |
| **`:doctor`** | Run system health check and diagnostics |
| **`:update`** | Self-update and recompile binary from source |
| **`:context`** | Visual context window gauge and token breakdown |
| **`:verbose`** | Toggle verbose mode on/off |
| **`:profile`** | View dialectic user persona and coding preferences |
| **`:cron`** | List or manage 24/7 background scheduled tasks |
| **`:stats`** | View session token metrics and execution times |
| **`:drivers`** | List all detected AI CLI tools |
| **`:driver <name>`** | Switch active driver (e.g. `:driver agy`, `:driver claude`) |
| **`:memory`** | Browse Palace-Mnemosyne memory stats and rooms |
| **`:skills`** | List discovered `.agents/skills/` runbooks |
| **`:clear`** | Clear terminal screen |
| **`:exit`** | Exit REPL |
