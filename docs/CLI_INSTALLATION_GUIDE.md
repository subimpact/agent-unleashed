# 🛠️ Supported AI CLIs — Installation & Authentication Guide

`agent-unleashed` (`agt-ul`) multiplexes across your local AI CLI tools. You do not need to install all of them — **installing even just one** allows `agt-ul` to start orchestrating immediately!

---

## ⚡ Quick Verification Command

To check which CLI tools are currently installed and detected on your machine:
```bash
agt-ul doctor
# or
agt-ul status
```

---

## 1. 🚀 Google Antigravity CLI (`agy`)

Google Antigravity is Google DeepMind's advanced agentic coding CLI.

### Installation
```bash
# Via npm
npm install -g @google/antigravity-cli

# Or via binary installer (Windows / macOS / Linux)
# Automatically configured when installing the Antigravity Desktop app
```

### Authentication
```bash
# Authenticate with your Google account
agy auth login
```

### Verification
```bash
agy --version
```

---

## 2. 🧠 Anthropic Claude Code (`claude`)

Claude Code is Anthropic's official terminal-based agentic coding tool.

### Installation
```bash
# Via npm (requires Node.js 18+)
npm install -g @anthropic-ai/claude-code
```

### Authentication
```bash
# Run claude once to initiate OAuth browser login
claude
```
*(Follow the interactive terminal prompt to log into your Anthropic Console account).*

### Verification
```bash
claude --version
```

---

## 3. 🤖 Aider Multi-File Architect (`aider`)

Aider is a popular open-source AI pair programmer for terminal and git repositories.

### Installation
```bash
# Using pipx (recommended)
pipx install aider-chat

# Or using uv
uv tool install aider-chat

# Or standard pip
python -m pip install -U aider-chat
```

### Authentication & Configuration
```bash
# Aider can run with your existing environment keys or local Ollama
# e.g., set ANTHROPIC_API_KEY, OPENAI_API_KEY, or GEMINI_API_KEY
```

### Verification
```bash
aider --version
```

---

## 4. 🦙 Local Offline Ollama (`ollama`)

Ollama allows running cutting-edge open-source coding models (DeepSeek-R1, Qwen2.5-Coder, Llama 3.3) 100% offline and locally on your GPU/CPU.

### Installation
* **Windows:** `winget install Ollama.Ollama` or download from [ollama.com/download](https://ollama.com/download)
* **macOS:** `brew install ollama` or download DMG from [ollama.com/download](https://ollama.com/download)
* **Linux:** `curl -fsSL https://ollama.com/install.sh | sh`

### Recommended Coding Models:
```bash
# Fast & highly capable coding model (7B)
ollama pull qwen2.5-coder:7b

# Deep reasoning & mathematical reflection model (7B/14B)
ollama pull deepseek-r1:7b
```

### Verification
```bash
ollama list
```

---

## 5. 🌐 Cloud API Fallback (Optional)

If you don't have any CLI tools installed, `agent-unleashed` can also call cloud APIs directly.

Configure your API keys in `config.yaml`:
```yaml
models:
  provider: "gemini" # or "openai", "anthropic", "openrouter"
  api_key: "YOUR_API_KEY_HERE"
```

---

## 🔄 Switching Drivers in `agt-ul`

Once installed, switch drivers on the fly inside the REPL:
```bash
👤 You > :driver agy       # Switch to Google Antigravity
👤 You > :driver claude    # Switch to Claude Code
👤 You > :driver aider     # Switch to Aider
👤 You > :driver ollama    # Switch to 100% offline local Ollama
```
