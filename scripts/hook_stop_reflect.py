#!/usr/bin/env python3
"""
Antigravity Stop Lifecycle Hook
Performs post-task reflection to record completed tasks into persistent memory.
"""

import sys
import json
from pathlib import Path

# Add project root to sys.path
sys.path.insert(0, str(Path(__file__).parent.parent.resolve()))

from src.memory.store import MemoryStore


def main():
    try:
        raw_input = sys.stdin.read()
        if not raw_input.strip():
            print(json.dumps({}))
            return

        payload = json.loads(raw_input)
        workspace_dir = Path(__file__).parent.parent
        db_path = workspace_dir / "data" / "memory.sqlite"

        store = MemoryStore(db_path=str(db_path))
        conv_id = payload.get("conversationId", "unknown")
        term_reason = payload.get("terminationReason", "completed")

        store.add_memory(
            category="lesson",
            content=f"Completed task turn in conversation {conv_id[:8]} with status '{term_reason}'.",
            source="antigravity_stop_hook",
            metadata=payload
        )

        # Allow stop to proceed normally
        print(json.dumps({}))

    except Exception:
        print(json.dumps({}))


if __name__ == "__main__":
    main()
