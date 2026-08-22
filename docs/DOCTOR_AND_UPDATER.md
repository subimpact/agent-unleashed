# 🩺 System Doctor, Diagnostics & Self-Updater

Inspired by `hermes doctor` and `openclaw doctor`, `agent-unleashed` provides a complete self-diagnostic suite and self-updater.

---

## 🩺 System Doctor (`pkg/doctor/`)

Run a full 10-point health audit across all subsystems:

```bash
agt-ul doctor
```

### Auto-Remediation Mode (`--fix`):
```bash
agt-ul doctor --fix
```
Automatically repairs missing directories, regenerates template configurations, and repairs SQLite database indexes.

---

## 📋 Diagnostic Subsystem Checks

1. **System & Runtime Environment:** OS architecture, Go version, CPU core count.
2. **Filesystem Permissions:** Validates write permissions on `./data` and `./.agents/skills`.
3. **Configuration Schema:** Checks `config.yaml` syntax, model settings, and gateway endpoints.
4. **AI CLI Drivers:** Tests detection and binary paths for `agy`, `claude`, `aider`, `ollama`, and cloud APIs.
5. **Database & Memory Palace Integrity:** Executes SQLite `PRAGMA integrity_check`, counts memory drawers, and checks cron jobs.
6. **Network Gateway Ports:** Verifies port `8080` availability for REST and WebSocket streaming.
7. **External Gateway Auth:** Tests Telegram Bot API token via `getMe` and Discord credentials.
8. **Skills Discovery:** Validates YAML frontmatter on all specialized `.agents/skills/*/SKILL.md` runbooks.

---

## 🔄 Self-Updater (`pkg/updater/`)

Upgrade and rebuild `agent-unleashed` directly from source with one command:

```bash
# From command line
agt-ul update

# Or inside REPL
:update
```

### 4-Stage Upgrade Pipeline:
1. **Git Sync & Hash Check:** Inspects git status and pulls latest commits from upstream.
2. **Single Binary Compilation:** Compiles an optimized Go binary (`agt-ul.exe`).
3. **Global PATH Installation:** Replaces `agt-ul.exe` and `agy-ul.exe` in `C:\Users\me\AppData\Local\agy\bin\`.
4. **Automated Doctor Verification:** Runs post-update diagnostics to ensure 10/10 subsystems are healthy.
