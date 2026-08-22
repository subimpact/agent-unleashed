"""
Antigravity-Unleashed Core Engine
The primary brain orchestrator connecting Antigravity CLI session (Zero-API-Key), local tools, vector memory, and reflection.
"""

import os
import json
import logging
import asyncio
import urllib.request
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
        # Session map: maps channel session_id to Antigravity CLI conversation IDs
        self.agy_conversations: Dict[str, str] = {}

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
        3. Multi-Backend Reasoning (agy CLI / Ollama / Gemini / Local Tools)
        4. Post-Task Reflection & Self-Evolution
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
            memory_block = f"\n[Persistent Memory Context]:\n{memory_lines}\n"

        augmented_prompt = f"{memory_block}\n{user_message}" if memory_block else user_message

        # Step 3: Dispatch to Selected Backend
        execution_steps = []
        final_response_text = ""
        provider = self.config.model.provider.lower()

        if provider == "agy":
            # 🚀 ZERO API KEY: Uses your active local Antigravity session
            async for chunk in self._run_agy_agent_loop(
                session_id=session_id,
                prompt=augmented_prompt,
                execution_steps=execution_steps
            ):
                if chunk["type"] == "text":
                    final_response_text += chunk["content"]
                yield chunk

        elif provider == "ollama":
            # 🚀 ZERO API KEY: Uses local Ollama instance (e.g. Hermes 3 / Qwen)
            async for chunk in self._run_ollama_agent_loop(
                prompt=augmented_prompt,
                execution_steps=execution_steps
            ):
                if chunk["type"] == "text":
                    final_response_text += chunk["content"]
                yield chunk

        elif provider in ["gemini", "openai"]:
            # Optional API Key Backend
            async for chunk in self._run_api_agent_loop(
                prompt=augmented_prompt,
                history=history,
                execution_steps=execution_steps
            ):
                if chunk["type"] == "text":
                    final_response_text += chunk["content"]
                yield chunk

        else:
            # Fallback Local Command Dispatcher
            yield {"type": "thought", "content": "Running via local tool dispatcher..."}
            if user_message.startswith("!") or user_message.startswith("run "):
                cmd = user_message.replace("run ", "", 1).lstrip("!")
                yield {"type": "tool_start", "tool_name": "run_command", "args": {"command": cmd}}
                tool_res = self.tools.execute("run_command", {"command": cmd})
                execution_steps.append({"tool_name": "run_command", "args": {"command": cmd}, "result": tool_res})
                yield {"type": "tool_result", "tool_name": "run_command", "result": tool_res}
                final_response_text = f"Output:\n```\n{tool_res.get('stdout', tool_res.get('error', ''))}\n```"
                yield {"type": "text", "content": final_response_text}
            else:
                final_response_text = f"Antigravity-Unleashed [{channel_name}] received: {user_message}"
                yield {"type": "text", "content": final_response_text}

        # Step 4: Persist response & trigger self-reflection
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

    def _resolve_agy_path(self) -> str:
        if self.config.model.agy_binary_path and os.path.exists(self.config.model.agy_binary_path):
            return self.config.model.agy_binary_path

        # Check standard PATH
        import shutil
        found = shutil.which("agy") or shutil.which("agy.exe")
        if found:
            return found

        # Check Windows default AppData location
        local_app_data = os.getenv("LOCALAPPDATA", "")
        fallback_win = os.path.join(local_app_data, "agy", "bin", "agy.exe")
        if os.path.exists(fallback_win):
            return fallback_win

        return "agy"

    async def _run_agy_agent_loop(
        self,
        session_id: str,
        prompt: str,
        execution_steps: List[Dict[str, Any]],
    ) -> AsyncGenerator[Dict[str, Any], None]:
        """
        Executes prompt through the local authenticated Antigravity CLI (agy).
        Directly uses your active account and CLI session with ZERO standalone API keys!
        """
        agy_bin = self._resolve_agy_path()
        yield {"type": "thought", "content": f"Invoking Antigravity CLI ({agy_bin})..."}

        cmd = [
            agy_bin,
            "--output-format", "json",
            f"--effort={self.config.model.effort}",
            f"--print={prompt}"
        ]

        if self.config.model.auto_approve_tools:
            cmd.append("--dangerously-skip-permissions")

        # Resume conversation if exists for this session
        if session_id in self.agy_conversations:
            cmd.append(f"--conversation={self.agy_conversations[session_id]}")

        try:
            process = await asyncio.create_subprocess_exec(
                *cmd,
                cwd=self.config.system.workspace_dir,
                stdout=asyncio.subprocess.PIPE,
                stderr=asyncio.subprocess.PIPE
            )
            stdout, stderr = await process.communicate()

            if process.returncode != 0:
                err_text = stderr.decode("utf-8", errors="replace")
                logger.error(f"agy CLI error: {err_text}")
                yield {"type": "text", "content": f"⚠️ Antigravity CLI execution error: {err_text}"}
                return

            raw_out = stdout.decode("utf-8", errors="replace").strip()
            try:
                data = json.loads(raw_out)
                resp_text = data.get("response", "")
                conv_id = data.get("conversation_id")
                if conv_id:
                    self.agy_conversations[session_id] = conv_id

                yield {"type": "text", "content": resp_text}
            except json.JSONDecodeError:
                yield {"type": "text", "content": raw_out}

        except Exception as e:
            logger.error(f"Failed to execute agy CLI: {e}")
            yield {"type": "text", "content": f"❌ Error invoking Antigravity CLI: {str(e)}"}

    async def _run_ollama_agent_loop(
        self,
        prompt: str,
        execution_steps: List[Dict[str, Any]],
    ) -> AsyncGenerator[Dict[str, Any], None]:
        """
        Executes prompt through a local Ollama server (e.g. Nous Hermes 3 / Qwen).
        Requires ZERO API keys and runs 100% offline.
        """
        model_name = self.config.model.model_name if self.config.model.model_name != "auto" else "hermes3"
        yield {"type": "thought", "content": f"Querying local Ollama model ({model_name})..."}

        req_body = json.dumps({
            "model": model_name,
            "prompt": prompt,
            "stream": False
        }).encode("utf-8")

        try:
            req = urllib.request.Request(
                "http://localhost:11434/api/generate",
                data=req_body,
                headers={"Content-Type": "application/json"}
            )
            loop = asyncio.get_event_loop()
            resp = await loop.run_in_executor(None, lambda: urllib.request.urlopen(req, timeout=120).read().decode("utf-8"))
            data = json.loads(resp)
            yield {"type": "text", "content": data.get("response", "")}
        except Exception as e:
            yield {"type": "text", "content": f"❌ Could not connect to local Ollama at localhost:11434 ({str(e)}). Make sure Ollama is running."}

    async def _run_api_agent_loop(
        self,
        prompt: str,
        history: List[Dict[str, Any]],
        execution_steps: List[Dict[str, Any]],
    ) -> AsyncGenerator[Dict[str, Any], None]:
        """
        Optional cloud API runner (Gemini / OpenAI).
        """
        api_key = self.config.model.api_key or os.getenv("GEMINI_API_KEY")
        if not api_key:
            yield {"type": "text", "content": "⚠️ No API key found. Switch to `provider: 'agy'` in `config.yaml` to use Antigravity without an API key!"}
            return

        try:
            import google.generativeai as genai
            genai.configure(api_key=api_key)
            model = genai.GenerativeModel(model_name=self.config.model.model_name or "gemini-2.5-pro")
            res = model.generate_content(prompt)
            yield {"type": "text", "content": res.text}
        except Exception as e:
            yield {"type": "text", "content": f"API Error: {str(e)}"}
