"""
Antigravity-Unleashed Persistent Memory Store
Combines SQLite relational metadata, FTS5 full-text keyword indexing, and vector similarity for Hybrid RAG.
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
            # 1. Relational metadata & episodic memories table
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

            # 2. SQLite FTS5 Full-Text Search Table for Hybrid BM25 Matching
            try:
                cursor.execute("""
                    CREATE VIRTUAL TABLE IF NOT EXISTS memories_fts USING fts5(
                        id UNINDEXED,
                        content,
                        category
                    );
                """)
            except sqlite3.OperationalError:
                pass  # Fallback if FTS5 not enabled in certain environments

            conn.commit()

    def _compute_embedding(self, text: str) -> np.ndarray:
        """
        Deterministic, lightweight embedding projection using char n-gram hashing.
        """
        vec = np.zeros(self.vector_dim, dtype=np.float32)
        words = text.lower().split()
        if not words:
            return vec

        for word in words:
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

            # Ingest into FTS5 virtual table
            try:
                cursor.execute("DELETE FROM memories_fts WHERE id = ?", (mem_id,))
                cursor.execute(
                    "INSERT INTO memories_fts (id, content, category) VALUES (?, ?, ?)",
                    (mem_id, content, category)
                )
            except sqlite3.OperationalError:
                pass

            conn.commit()

        return mem_id

    def search_memories(
        self,
        query: str,
        category: Optional[str] = None,
        limit: int = 5,
        min_similarity: float = 0.4,
    ) -> List[Dict[str, Any]]:
        """
        Hybrid search combining Vector Cosine Similarity with FTS5 Keyword BM25 boosts.
        """
        query_vec = self._compute_embedding(query)
        scored_memories: Dict[str, Dict[str, Any]] = {}

        with sqlite3.connect(self.db_path) as conn:
            cursor = conn.cursor()

            # 1. Keyword FTS5 Search
            clean_query = "".join([c for c in query if c.isalnum() or c.isspace()]).strip()
            if clean_query:
                fts_query = " OR ".join(clean_query.split()[:5])
                try:
                    cursor.execute("""
                        SELECT id, rank FROM memories_fts WHERE memories_fts MATCH ? ORDER BY rank LIMIT 10
                    """, (fts_query,))
                    for row in cursor.fetchall():
                        m_id, rank = row
                        # Convert FTS rank into a positive score boost
                        boost = max(0.1, min(0.3, abs(rank) * 0.05))
                        scored_memories[m_id] = {"fts_boost": boost}
                except sqlite3.OperationalError:
                    pass

            # 2. Vector Cosine Search
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
            results = []

            for row in rows:
                m_id, m_cat, m_content, m_source, m_created, m_emb_bytes, m_meta = row
                if not m_emb_bytes:
                    continue

                emb = np.frombuffer(m_emb_bytes, dtype=np.float32)
                vec_sim = float(np.dot(query_vec, emb))
                fts_boost = scored_memories.get(m_id, {}).get("fts_boost", 0.0)
                
                # Hybrid combined score
                final_score = vec_sim + fts_boost

                if final_score >= min_similarity:
                    results.append({
                        "id": m_id,
                        "category": m_cat,
                        "content": m_content,
                        "source": m_source,
                        "created_at": m_created,
                        "similarity": round(final_score, 4),
                        "vector_similarity": round(vec_sim, 4),
                        "fts_boost": round(fts_boost, 4),
                        "metadata": json.loads(m_meta) if m_meta else {},
                    })

            # Sort and update access stats
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
