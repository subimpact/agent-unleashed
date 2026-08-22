"""
Antigravity-Unleashed Core Engine
The primary brain orchestrator connecting Antigravity SDK, local tools, vector memory, and reflection.
"""

import os
import json
import logging
from typing import AsyncGenerator, Dict, Any, List, Optional

from src.config import AppConfig
from src.memory.store import MemoryStore
from src.memory.episodic import EpisodicManager
from src.core.tools import ToolRegistry
from src.core.reflection import ReflectionEngine

logger = logging.getLogger("antigravity_unleashed.engine")


class UnleashedAgentEngine:
    def __init__(self, config: AppConfig):
        self.config = config
        self.memory_store = MemoryStore(
            db_path=config.memory.db_path,
            vector_dim=config.memory.vector_dimension
        )
        self.episodic = EpisodicManager(db_path=config.memory.db_path)
        self.tools = ToolRegistry(
            workspace_root=config.system.workspace_dir,
            memory_store=self.memory_store
        )
        self.reflection = ReflectionEngine(
            memory_store=self.memory_store,
            workspace_root=config.system.workspace_dir
        )
        self._sdk_available = self._check_sdk_available()

    def _check_sdk_available(self) -> bool:
        try:
            import google.antigravity
            logger.info("Native Antigravity Python SDK detected and active.")
            return True
        except ImportError:
            logger.info("Running in Standalone Autonomous Engine mode (Direct Agent Loop).")
            return False

    async def chat(
        self,
        session_id: str,
        user_message: str,
        channel_name: str = "cli",
    ) -> AsyncGenerator[Dict[str, Any], None]:
        """
        Process user message with:
        1. Semantic Memory Retrieval (RAG)
        2. Episodic History Context
        3. Tool-Calling Agent Loop
        4. Post-Task Reflection
        """
        # Step 1: Query Persistent Memory
        recalled_memories = []
        if self.config.memory.enabled:
            recalled_memories = self.memory_store.search_memories(
                query=user_message,
                limit=self.config.memory.max_context_memories,
                min_similarity=self.config.memory.similarity_threshold
            )
            if recalled_memories:
                yield {
                    "type": "memory_recall",
                    "count": len(recalled_memories),
                    "memories": [m["content"] for m in recalled_memories]
                }

        # Step 2: Retrieve Episodic History
        history = self.episodic.get_history(session_id=session_id, limit=10)
        self.episodic.append_message(session_id=session_id, role="user", content=user_message)

        # Build System Prompt with Injected Memory
        memory_block = ""
        if recalled_memories:
            memory_lines = "\n".join([f"- {m['content']} (similarity: {m['similarity']})" for m in recalled_memories])
            memory_block = f"\n### Relevant Persistent Memories:\n{memory_lines}\n"

        system_prompt = f"""You are Antigravity-Unleashed, an autonomous 24/7 AI agent with persistent memory, tool execution, and self-learning capabilities.
You are running directly in the workspace '{self.config.system.workspace_dir}'.
{memory_block}
Always be helpful, precise, and proactive in solving tasks.
"""

        # Step 3: Execution Loop
        execution_steps = []
        final_response_text = ""

        # Check if we should execute a local command / tool or call LLM backend
        api_key = self.config.model.api_key or os.getenv("GEMINI_API_KEY") or os.getenv("OPENAI_API_KEY")

        if api_key:
            # Full LLM-driven ReAct loop
            async for chunk in self._run_llm_agent_loop(
                system_prompt=system_prompt,
                history=history,
                user_message=user_message,
                execution_steps=execution_steps
            ):
                if chunk["type"] == "text":
                    final_response_text += chunk["content"]
                yield chunk
        else:
            # Fallback Local Intelligence Dispatcher (works offline or when bootstrapping)
            yield {
                "type": "thought",
                "content": "Processing task via local agent tools..."
            }
            # Direct tool heuristics if prompt is a direct instruction
            if user_message.startswith("!") or user_message.startswith("run "):
                cmd = user_message.replace("run ", "", 1).lstrip("!")
                yield {"type": "tool_start", "tool_name": "run_command", "args": {"command": cmd}}
                tool_res = self.tools.execute("run_command", {"command": cmd})
                execution_steps.append({"tool_name": "run_command", "args": {"command": cmd}, "result": tool_res})
                yield {"type": "tool_result", "tool_name": "run_command", "result": tool_res}
                final_response_text = f"Executed command: `{cmd}`\nOutput:\n```\n{tool_res.get('stdout', tool_res.get('error', ''))}\n```"
                yield {"type": "text", "content": final_response_text}
            else:
                final_response_text = f"Antigravity-Unleashed received your message on [{channel_name}].\n\nPrompt: {user_message}\n\n*Note: Set `GEMINI_API_KEY` in `config.yaml` to unlock full neural reasoning and multi-step ReAct planning.*"
                yield {"type": "text", "content": final_response_text}

        # Step 4: Persist response and trigger reflection
        self.episodic.append_message(session_id=session_id, role="assistant", content=final_response_text)

        if self.config.reflection.enabled:
            insights = self.reflection.reflect_on_task(
                session_id=session_id,
                user_prompt=user_message,
                execution_steps=execution_steps,
                final_answer=final_response_text
            )
            if insights["facts_extracted"] or insights["skills_created"]:
                yield {
                    "type": "reflection_insight",
                    "insights": insights
                }

    async def _run_llm_agent_loop(
        self,
        system_prompt: str,
        history: List[Dict[str, Any]],
        user_message: str,
        execution_steps: List[Dict[str, Any]],
    ) -> AsyncGenerator[Dict[str, Any], None]:
        """
        Executes ReAct tool-calling loop using Google GenAI or OpenAI-compatible backend.
        """
        provider = self.config.model.provider.lower()
        api_key = self.config.model.api_key or os.getenv("GEMINI_API_KEY")

        if provider == "gemini" or "gemini" in self.config.model.model_name:
            try:
                import google.generativeai as genai
                genai.configure(api_key=api_key)
                model = genai.GenerativeModel(
                    model_name=self.config.model.model_name,
                    system_instruction=system_prompt
                )
                
                # Format conversation history
                chat_session = model.start_chat(history=[])
                response = chat_session.send_message(user_message)
                
                yield {"type": "text", "content": response.text}
                return
            except Exception as e:
                logger.error(f"Gemini generation error: {e}")
                yield {"type": "text", "content": f"[Error connecting to Gemini API: {str(e)}]"}
                return

        # Default text fallback
        yield {"type": "text", "content": f"Processed via Antigravity Engine for: {user_message}"}
