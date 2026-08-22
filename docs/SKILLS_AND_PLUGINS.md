# ⚡ Progressive Skill Engine & Package Manager

`agent-unleashed` features a **Narrow Waist Progressive Skill Indexer** and built-in skill package manager.

---

## 🎯 Narrow Waist Progressive Indexing (`pkg/skills/indexer.go`)

Traditional agents inject hundreds of pages of documentation into every system prompt, wasting valuable context tokens. 

`agent-unleashed` uses **Progressive Disclosure**:
1. **Lightweight Index Injection (1-line per skill):**
   Only a compact index of discovered skill names and summaries is injected into the initial prompt (~50 tokens total).
2. **On-Demand Hydration:**
   When the agent identifies that a task requires a specific skill, it loads the full `SKILL.md` runbook into context dynamically.

---

## 📦 Built-In Skill Package Manager (`agt-ul skill install`)

Install any skill directly from GitHub with a single command:

```bash
# Install taste-skill (anti-slop frontend design skill)
agt-ul skill install Leonxlnx/taste-skill

# Or install via full GitHub URL
agt-ul skill install https://github.com/Leonxlnx/taste-skill

# List all discovered skills
agt-ul skills
```

---

## 📝 Writing Custom Skills (`SKILL.md`)

Create a folder under `.agents/skills/<skill-name>/` containing a `SKILL.md` file with YAML frontmatter:

```markdown
---
name: custom-db-migrator
description: Automates PostgreSQL to SQLite schema migrations and data export pipelines.
---

# Custom Database Migrator Skill

## 1. Trigger Conditions
Use this skill when the user asks to export schemas, migrate databases, or convert PostgreSQL types.

## 2. Execution Runbook
1. Inspect source schema using `pg_dump --schema-only`.
2. Translate data types to SQLite equivalents (`SERIAL` -> `INTEGER PRIMARY KEY AUTOINCREMENT`).
3. Run verification tests.
```

---

## 🎨 Featured Built-In Skills

### `design-taste-frontend` (`taste-skill`)
* Anti-slop frontend design skill for landing pages, portfolios, and web apps.
* Calibrates 3 visual dials: `DESIGN_VARIANCE` (1–10), `MOTION_INTENSITY` (1–10), `VISUAL_DENSITY` (1–10).
* Eliminates generic AI-purple gradients, centered hero clichés, and templated bento boxes.
