"""
Antigravity-Unleashed Episodic Context Manager
Manages message histories, token budgets, and session persistence across multiple channels.
"""

import json
import sqlite3
from pathlib import Path
from typing import List, Dict, Any, Optional


class EpisodicManager:
    def __init__(self, db_path: str = "./data/memory.sqlite"):
        self.db_path = db_path
        self._init_db()

    def _init_db(self):
        with sqlite3.connect(self.db_path) as conn:
            cursor = conn.cursor()
            cursor.execute("""
                CREATE TABLE IF NOT EXISTS session_messages (
                    id INTEGER PRIMARY KEY AUTOINCREMENT,
                    session_id TEXT NOT NULL,      -- e.g. 'telegram_12345', 'discord_67890', 'cli_main'
                    role TEXT NOT NULL,            -- 'user', 'assistant', 'system', 'tool'
                    content TEXT NOT NULL,
                    tool_calls_json TEXT,
                    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
                )
            """)
            cursor.execute("""
                CREATE INDEX IF NOT EXISTS idx_session_messages ON session_messages(session_id, created_at);
            """)
            conn.commit()

    def append_message(
        self,
        session_id: str,
        role: str,
        content: str,
        tool_calls: Optional[List[Dict[str, Any]]] = None,
    ):
        with sqlite3.connect(self.db_path) as conn:
            cursor = conn.cursor()
            cursor.execute("""
                INSERT INTO session_messages (session_id, role, content, tool_calls_json)
                VALUES (?, ?, ?, ?)
            """, (session_id, role, content, json.dumps(tool_calls or [])))
            conn.commit()

    def get_history(self, session_id: str, limit: int = 20) -> List[Dict[str, Any]]:
        with sqlite3.connect(self.db_path) as conn:
            cursor = conn.cursor()
            cursor.execute("""
                SELECT role, content, tool_calls_json, created_at
                FROM session_messages
                WHERE session_id = ?
                ORDER BY id ASC
            """, (session_id,))
            rows = cursor.fetchall()
            
            # Return up to last `limit` messages
            recent_rows = rows[-limit:] if limit > 0 else rows
            return [
                {
                    "role": r[0],
                    "content": r[1],
                    "tool_calls": json.loads(r[2]) if r[2] else [],
                    "created_at": r[3],
                }
                for r in recent_rows
            ]

    def clear_history(self, session_id: str):
        with sqlite3.connect(self.db_path) as conn:
            cursor = conn.cursor()
            cursor.execute("DELETE FROM session_messages WHERE session_id = ?", (session_id,))
            conn.commit()
