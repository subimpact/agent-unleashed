#!/usr/bin/env python3
"""
Antigravity PreInvocation Lifecycle Hook
Queries persistent SQLite/vector memory and injects relevant context before the model generates responses.
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
            print(json.dumps({"injectSteps": []}))
            return

        payload = json.loads(raw_input)
        workspace_dir = Path(__file__).parent.parent
        db_path = workspace_dir / "data" / "memory.sqlite"

        if not db_path.exists():
            print(json.dumps({"injectSteps": []}))
            return

        store = MemoryStore(db_path=str(db_path))
        recent_memories = store.get_recent_memories(limit=3)

        if not recent_memories:
            print(json.dumps({"injectSteps": []}))
            return

        memory_bullets = "\n".join([f"- [{m['category']}] {m['content']}" for m in recent_memories])
        injected_msg = f"🧠 [Antigravity-Unleashed Persistent Memory]:\n{memory_bullets}"

        output = {
            "injectSteps": [
                {
                    "ephemeralMessage": injected_msg
                }
            ]
        }
        print(json.dumps(output))

    except Exception as e:
        # Never crash the hook, return empty steps
        print(json.dumps({"injectSteps": []}))


if __name__ == "__main__":
    main()
