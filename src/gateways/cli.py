"""
Antigravity-Unleashed Interactive CLI Gateway
Rich terminal interface for local testing and interactive pair programming.
"""

import sys
import asyncio
import logging

logger = logging.getLogger("antigravity_unleashed.gateway.cli")


class CliGateway:
    def __init__(self, engine):
        self.engine = engine

    async def start(self):
        print("\n" + "=" * 60)
        print("  🚀 ANTIGRAVITY-UNLEASHED (Interactive CLI Mode)")
        print("  Autonomous Agent with Persistent Memory & Self-Learning")
        print("=" * 60)
        print("Commands: ':memory' to view memories, ':skills' to view skills, ':exit' to quit.\n")

        session_id = "cli_main"

        while True:
            try:
                user_input = input("👤 You > ").strip()
                if not user_input:
                    continue

                if user_input.lower() in [":exit", "exit", "quit"]:
                    print("\n👋 Shutting down CLI...")
                    break

                if user_input.lower() == ":memory":
                    recent = self.engine.memory_store.get_recent_memories(limit=10)
                    print("\n🧠 Persistent Memories:")
                    for m in recent:
                        print(f"  • [{m['category']}] {m['content']} ({m['source']})")
                    print()
                    continue

                if user_input.lower() == ":skills":
                    skills_dir = self.engine.reflection.skills_dir
                    skills = [s.name for s in skills_dir.iterdir() if s.is_dir()]
                    print(f"\n⚡ Learned Skills ({len(skills)}):")
                    for s in skills:
                        print(f"  • {s}")
                    print()
                    continue

                print("\n🤖 Antigravity > ", end="", flush=True)
                async for event in self.engine.chat(session_id=session_id, user_message=user_input, channel_name="cli"):
                    if event["type"] == "text":
                        sys.stdout.write(event["content"])
                        sys.stdout.flush()
                    elif event["type"] == "memory_recall":
                        print(f"\n[🧠 Injected {event['count']} relevant memories into context]", flush=True)
                    elif event["type"] == "tool_start":
                        print(f"\n[🔧 Calling tool: {event['tool_name']} with args: {event.get('args')}]", flush=True)
                    elif event["type"] == "tool_result":
                        print(f"[✅ Tool output received]", flush=True)
                    elif event["type"] == "reflection_insight":
                        print(f"\n[✨ Self-Learning: {event['insights']}]", flush=True)
                print("\n")

            except (KeyboardInterrupt, EOFError):
                print("\n👋 Exiting...")
                break
            except Exception as e:
                print(f"\n❌ Error: {str(e)}\n")
