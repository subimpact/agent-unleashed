"""
Antigravity-Unleashed Persistent Memory Store
Combines SQLite relational metadata with vector similarity search for semantic RAG.
"""

import os
import json
import sqlite3
import hashlib
import numpy as np
from pathlib import Path
from datetime import datetime
from typing import List, Dict, Any, Optional


class MemoryStore:
    def __init__(self, db_path: str = "./data/memory.sqlite", vector_dim: int = 384):
        self.db_path = db_path
        self.vector_dim = vector_dim
        Path(self.db_path).parent.mkdir(parents=True, exist_ok=True)
        self._init_db()

    def _init_db(self):
        with sqlite3.connect(self.db_path) as conn:
            cursor = conn.cursor()
            # Relational metadata & episodic memories table
            cursor.execute("""
                CREATE TABLE IF NOT EXISTS memories (
                    id TEXT PRIMARY KEY,
                    category TEXT NOT NULL,       -- 'fact', 'preference', 'lesson', 'conversation', 'skill_ref'
                    content TEXT NOT NULL,
                    source TEXT,                  -- 'telegram', 'discord', 'cli', 'reflection', 'manual'
                    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
                    access_count INTEGER DEFAULT 0,
                    last_accessed TIMESTAMP,
                    embedding BLOB,
                    metadata_json TEXT
                )
            """)
            cursor.execute("""
                CREATE INDEX IF NOT EXISTS idx_memories_category ON memories(category);
            """)
            conn.commit()

    def _compute_embedding(self, text: str) -> np.ndarray:
        """
        Deterministic, lightweight embedding projection using char n-gram hashing
        if external neural embedding library is not loaded.
        Can be upgraded to sentence-transformers or Gemini text-embedding-004.
        """
        vec = np.zeros(self.vector_dim, dtype=np.float32)
        words = text.lower().split()
        if not words:
            return vec

        for word in words:
            # 3-gram feature hashing
            for i in range(max(1, len(word) - 2)):
                gram = word[i:i+3]
                h = int(hashlib.md5(gram.encode()).hexdigest(), 16)
                idx = h % self.vector_dim
                val = 1.0 if (h >> 8) % 2 == 0 else -1.0
                vec[idx] += val

        norm = np.linalg.norm(vec)
        if norm > 0:
            vec = vec / norm
        return vec

    def add_memory(
        self,
        category: str,
        content: str,
        source: str = "system",
        metadata: Optional[Dict[str, Any]] = None,
    ) -> str:
        mem_id = hashlib.sha256(f"{category}:{content}:{datetime.utcnow().isoformat()}".encode()).hexdigest()[:16]
        embedding = self._compute_embedding(content).tobytes()
        metadata_json = json.dumps(metadata or {})

        with sqlite3.connect(self.db_path) as conn:
            cursor = conn.cursor()
            cursor.execute("""
                INSERT OR REPLACE INTO memories (id, category, content, source, embedding, metadata_json)
                VALUES (?, ?, ?, ?, ?, ?)
            """, (mem_id, category, content, source, embedding, metadata_json))
            conn.commit()

        return mem_id

    def search_memories(
        self,
        query: str,
        category: Optional[str] = None,
        limit: int = 5,
        min_similarity: float = 0.4,
    ) -> List[Dict[str, Any]]:
        query_vec = self._compute_embedding(query)
        results = []

        with sqlite3.connect(self.db_path) as conn:
            cursor = conn.cursor()
            if category:
                cursor.execute(
                    "SELECT id, category, content, source, created_at, embedding, metadata_json FROM memories WHERE category = ?",
                    (category,)
                )
            else:
                cursor.execute(
                    "SELECT id, category, content, source, created_at, embedding, metadata_json FROM memories"
                )

            rows = cursor.fetchall()

            for row in rows:
                m_id, m_cat, m_content, m_source, m_created, m_emb_bytes, m_meta = row
                if not m_emb_bytes:
                    continue

                emb = np.frombuffer(m_emb_bytes, dtype=np.float32)
                sim = float(np.dot(query_vec, emb))
                if sim >= min_similarity:
                    results.append({
                        "id": m_id,
                        "category": m_cat,
                        "content": m_content,
                        "source": m_source,
                        "created_at": m_created,
                        "similarity": round(sim, 4),
                        "metadata": json.loads(m_meta) if m_meta else {},
                    })

            # Update access timestamps for top matched memories
            results.sort(key=lambda x: x["similarity"], reverse=True)
            top_results = results[:limit]

            for item in top_results:
                cursor.execute(
                    "UPDATE memories SET access_count = access_count + 1, last_accessed = CURRENT_TIMESTAMP WHERE id = ?",
                    (item["id"],)
                )
            conn.commit()

        return top_results

    def get_recent_memories(self, limit: int = 10) -> List[Dict[str, Any]]:
        with sqlite3.connect(self.db_path) as conn:
            cursor = conn.cursor()
            cursor.execute(
                "SELECT id, category, content, source, created_at, metadata_json FROM memories ORDER BY created_at DESC LIMIT ?",
                (limit,)
            )
            rows = cursor.fetchall()
            return [
                {
                    "id": r[0],
                    "category": r[1],
                    "content": r[2],
                    "source": r[3],
                    "created_at": r[4],
                    "metadata": json.loads(r[5]) if r[5] else {},
                }
                for r in rows
            ]
