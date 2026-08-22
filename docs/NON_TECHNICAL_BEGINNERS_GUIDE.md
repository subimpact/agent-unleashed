# 🟢 Non-Technical Beginner's Guide: Getting Started with Agent-Unleashed

Welcome! This guide is written specifically for anyone who wants an **autonomous AI assistant that runs 24/7 on their computer, phone, or Telegram**, even if you have zero coding experience.

---

## 🌟 What is Agent-Unleashed?

Think of **Agent-Unleashed (`agt-ul`)** as a **personal AI butler** that lives on your computer:
* 🆓 **Zero Expensive API Bills:** It can tap into the free coding sessions of AI tools you already have (like Google Antigravity or Claude Code).
* 🧠 **Remembers Everything:** It doesn't forget your preferences, project rules, or past instructions even across reboots.
* 📱 **Talk from Anywhere:** Message your agent on **Telegram** or **Discord** while you're away from your desk.
* ⏰ **Runs Automatically:** Schedule tasks (like "check my server every night at 3 AM" or "summarize the news every morning").

---

## 🚀 Step 1: Install in 10 Seconds

You only need to copy and paste **one command** into your terminal.

### If you are on Windows:
1. Open **PowerShell** (press the Windows key, type `PowerShell`, and hit Enter).
2. Copy and paste this command, then press Enter:
```powershell
irm https://agent.subimpact.net/install.ps1 | iex
```

### If you are on Mac or Linux:
1. Open **Terminal**.
2. Copy and paste this command, then press Enter:
```bash
curl -fsSL https://agent.subimpact.net/install.sh | bash
```

That's it! `agt-ul` is now installed on your computer.

---

## 🛠️ Step 2: Set Up at Least One Free AI Tool

`agent-unleashed` works with your favorite AI tool. You only need to have **one** of these:

| Tool | How to install it |
| :--- | :--- |
| **🚀 Google Antigravity** | Run `npm install -g @google/antigravity-cli` then `agy auth login` |
| **🧠 Anthropic Claude Code** | Run `npm install -g @anthropic-ai/claude-code` then type `claude` |
| **⚡ OpenAI Codex** | Run `npm install -g @openai/codex` then `codex auth login` |
| **🦙 Ollama (100% Offline)** | Download installer from [ollama.com](https://ollama.com) |

---

## 🧙 Step 3: Run the Easy Setup Wizard

In your terminal, type:
```bash
agt-ul setup
```

The wizard will guide you through simple questions:
1. It automatically detects which AI tool is on your computer.
2. It asks if you want to turn on **Telegram** or **Discord** (optional).
3. It saves your settings automatically.

---

## 💬 Step 4: Talk to Your Agent

To start chatting with your agent in your terminal:
```bash
agt-ul
```

Now you can type anything you want! For example:
* *"Create a simple website with a dark theme."*
* *"Explain how the memory system works."*
* *"Remind me to check the database tomorrow."*

### Handy Shortcuts (Type inside the chat):
* **`:help`** — Show all available commands.
* **`:doctor`** — Check if everything is working properly.
* **`:wiki`** — See the knowledge base your agent has automatically written.
* **`:driver claude`** or **`:driver agy`** — Switch between AI tools instantly.
* **`:exit`** — Close the chat.

---

## 📱 Step 5: Connect Telegram (Optional)

Want to message your agent from your phone?

1. Open Telegram and search for **`@BotFather`**.
2. Send `/newbot`, choose a name (e.g. `MyCoolAgentBot`), and copy the **Bot Token** it gives you.
3. Open `config.yaml` or run `agt-ul setup`, turn on Telegram, and paste your Bot Token.
4. Run `agt-ul` on your computer.
5. Open Telegram, find your new bot, press **Start**, and send your tasks!

---

## ❓ Frequently Asked Questions

### 1. Does this cost money?
No! `agent-unleashed` is 100% free and open-source under the MIT license. It uses your existing logged-in CLI accounts.

### 2. What if my computer restarts?
Everything your agent learned is safely stored in a local SQLite file in `./data/`. When you start `agt-ul` again, it remembers all past context!

### 3. How do I update to the newest version?
Simply run:
```bash
agt-ul update
```
It will automatically download the latest updates and recompile itself!
