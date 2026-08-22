# 🏛️ Palace-Mnemosyne Cognitive Memory Engine

`agent-unleashed` features **Palace-Mnemosyne** — an advanced hybrid long-term memory engine combining spatial method-of-loci architecture with mathematical Ebbinghaus cognitive decay.

---

## 📐 Conceptual Model

Unlike traditional flat vector databases that suffer from semantic drift and retrieval clutter, Palace-Mnemosyne structures knowledge into three distinct dimensions:

```text
┌─────────────────────────────────────────────────────────────┐
│                    PALACE-MNEMOSYNE STORE                   │
│                                                             │
│   ┌─────────────────────────────────────────────────────┐   │
│   │ WING: Default / Project Identifier                  │   │
│   │                                                     │   │
│   │   ┌───────────────┐ ┌───────────────┐ ┌───────────┐ │   │
│   │   │ ROOM:         │ │ ROOM:         │ │ ROOM:     │ │   │
│   │   │ preferences   │ │ architecture  │ │ profile   │ │   │
│   │   │               │ │               │ │           │ │   │
│   │   │ • Drawer 1    │ │ • Drawer 1    │ │ • Drawer 1│ │   │
│   │   │ • Drawer 2    │ │ • Drawer 2    │ │           │ │   │
│   │   └───────────────┘ └───────────────┘ └───────────┘ │   │
│   └─────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
```

1. **Wings (Top-Level Scope):** Broad project or repository boundaries (e.g., `default`, `frontend-v2`, `agent-unleashed`).
2. **Rooms (Semantic Corridors):** Domain-specific categories (e.g., `preferences`, `architecture`, `api_conventions`, `user_profile`, `deployments`).
3. **Drawers / Loci (Verbatim Facts):** Granular, atomic factual records with source attribution and metadata.

---

## 📉 Mathematical Ebbinghaus Cognitive Decay

To prevent outdated transient context from polluting future queries, memories experience temporal retention decay based on the **Ebbinghaus Forgetting Curve**:

$$R = e^{-\lambda \Delta t}$$

* $R$: Retention Score ($0.0 \le R \le 1.0$)
* $\lambda$: Decay Rate Constant ($\lambda = 0.005 \text{ hr}^{-1}$)
* $\Delta t$: Elapsed time in hours since last access or creation.

### 🧠 Spaced-Repetition Reinforcement
When a memory fact is recalled during a task, its `access_count` increments, resetting $\Delta t = 0$ and boosting its cognitive weight $W$:

$$W = R \times (1.0 + 0.1 \times \min(\text{access\_count}, 10))$$

---

## 🔍 Hybrid Search & RAG Scoring

Memory retrieval blends **SQLite FTS5 Full-Text Match** and **384-dimensional Normalized Cosine Vector Similarity**:

$$\text{FinalScore} = (0.5 \times \text{VectorSimilarity} + 0.5 \times \text{FTS5Score}) \times W$$

### SQLite Schema (`data/memory.sqlite`)

```sql
CREATE TABLE IF NOT EXISTS memories (
    id TEXT PRIMARY KEY,
    wing TEXT NOT NULL,
    room TEXT NOT NULL,
    hall TEXT NOT NULL,
    content TEXT NOT NULL,
    source TEXT NOT NULL,
    created_at DATETIME NOT NULL,
    access_count INTEGER DEFAULT 1,
    last_accessed DATETIME NOT NULL,
    embedding BLOB,
    metadata_json TEXT
);

CREATE VIRTUAL TABLE IF NOT EXISTS memories_fts USING fts5(
    id UNINDEXED,
    content,
    room,
    hall
);
```

---

## 👤 Dialectic User Persona Tracking (`pkg/memory/profile.go`)

Agent-Unleashed maintains an evolving user profile stored in `room:user_profile`:
* **Preferred Languages:** Tracks coding languages (Go, Python, TypeScript, Rust).
* **Paradigms:** Tracks architectural choices (Functional, OOP, Clean Architecture, Microservices).
* **Communication Style:** Conciseness preference, feedback depth.

### CLI Inspection:
```bash
# In Terminal REPL
👤 You [agy] > :profile
```
