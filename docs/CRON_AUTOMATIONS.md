# ⏰ In-Process Autonomous Cron Engine

`agent-unleashed` features a built-in, in-process 24/7 cron scheduler running directly on background Goroutines, backed by persistent SQLite storage.

---

## ⚙️ Cron Architecture

```text
┌─────────────────────────────────────────────────────────────┐
│                 IN-PROCESS CRON ENGINE                      │
│                                                             │
│   ┌─────────────────────────────────────────────────────┐   │
│   │ Goroutine Loop: Evaluates every 60 seconds          │   │
│   └──────────────────────────┬──────────────────────────┘   │
│                              │                              │
│   ┌──────────────────────────▼──────────────────────────┐   │
│   │ Match Standard 5-Part Cron Schedule (* * * * *)     │   │
│   └──────────────────────────┬──────────────────────────┘   │
│                              │                              │
│   ┌──────────────────────────▼──────────────────────────┐   │
│   │ Execute Orchestrator Task                           │   │
│   └──────────────────────────┬──────────────────────────┘   │
│                              │                              │
│   ┌──────────────────────────▼──────────────────────────┐   │
│   │ Dispatch Output to Channel (Telegram/Discord/WS/CLI)│   │
│   └─────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
```

---

## 📝 5-Part Cron Syntax

Standard 5-part cron format: `minute hour day-of-month month day-of-week`

| Field | Range | Special Characters |
| :--- | :--- | :--- |
| **Minute** | `0–59` | `*`, `,`, `-`, `*/N` |
| **Hour** | `0–23` | `*`, `,`, `-`, `*/N` |
| **Day of Month** | `1–31` | `*`, `,`, `-`, `*/N` |
| **Month** | `1–12` | `*`, `,`, `-`, `*/N` |
| **Day of Week** | `0–6` (0 = Sunday) | `*`, `,`, `-`, `*/N` |

### Common Schedule Examples:
* `0 9 * * *` — Every day at 9:00 AM
* `*/15 * * * *` — Every 15 minutes
* `0 0 * * 1` — Every Monday at midnight
* `0 3 * * *` — Every night at 3:00 AM (Database cleanup / backups)

---

## 💻 CLI & REPL Commands

### List Scheduled Cron Jobs:
```bash
# From command line
agt-ul cron

# Inside REPL
:cron
# or
:cron list
```

### Add a New Scheduled Job:
```bash
# Inside REPL
:cron add "0 9 * * *" "Perform daily git status and dependency audit"
```

### Remove a Scheduled Job:
```bash
# Inside REPL
:cron remove <job-id>
```

---

## 🗄️ Database Persistence (`data/memory.sqlite`)

Cron jobs are automatically persisted in SQLite table `cron_jobs` and survive daemon restarts:

```sql
CREATE TABLE IF NOT EXISTS cron_jobs (
    id TEXT PRIMARY KEY,
    schedule TEXT NOT NULL,
    prompt TEXT NOT NULL,
    channel TEXT NOT NULL,
    target_id TEXT,
    enabled INTEGER DEFAULT 1,
    last_run DATETIME,
    created_at DATETIME NOT NULL
);
```
