"""
Antigravity-Unleashed Tool Execution Engine
Provides native tools for file I/O, terminal execution, web search, and dynamic skill management.
"""

import os
import subprocess
import urllib.request
import urllib.parse
import json
from pathlib import Path
from typing import Dict, Any, List, Optional, Callable


class ToolRegistry:
    def __init__(self, workspace_root: str = ".", memory_store=None):
        self.workspace_root = Path(workspace_root).resolve()
        self.memory_store = memory_store
        self.tools: Dict[str, Dict[str, Any]] = {}
        self._register_default_tools()

    def register(self, name: str, description: str, parameters: Dict[str, Any], handler: Callable):
        self.tools[name] = {
            "name": name,
            "description": description,
            "parameters": parameters,
            "handler": handler,
        }

    def get_schemas(self) -> List[Dict[str, Any]]:
        return [
            {
                "name": t["name"],
                "description": t["description"],
                "parameters": t["parameters"],
            }
            for t in self.tools.values()
        ]

    def execute(self, name: str, args: Dict[str, Any]) -> Any:
        if name not in self.tools:
            return {"error": f"Tool '{name}' not found."}
        try:
            return self.tools[name]["handler"](**args)
        except Exception as e:
            return {"error": f"Tool execution failed: {str(e)}"}

    def _register_default_tools(self):
        # 1. Shell Command Execution
        self.register(
            name="run_command",
            description="Execute a shell command in the workspace directory and return output.",
            parameters={
                "type": "object",
                "properties": {
                    "command": {"type": "string", "description": "The command string to execute."}
                },
                "required": ["command"]
            },
            handler=self._tool_run_command
        )

        # 2. View File
        self.register(
            name="view_file",
            description="Read the contents of a file in the workspace.",
            parameters={
                "type": "object",
                "properties": {
                    "file_path": {"type": "string", "description": "Path to the file."}
                },
                "required": ["file_path"]
            },
            handler=self._tool_view_file
        )

        # 3. Write File
        self.register(
            name="write_file",
            description="Create or overwrite a file with given content.",
            parameters={
                "type": "object",
                "properties": {
                    "file_path": {"type": "string", "description": "Path to the file."},
                    "content": {"type": "string", "description": "Text content to write."}
                },
                "required": ["file_path", "content"]
            },
            handler=self._tool_write_file
        )

        # 4. List Directory
        self.register(
            name="list_dir",
            description="List files and folders in a specified directory.",
            parameters={
                "type": "object",
                "properties": {
                    "dir_path": {"type": "string", "description": "Directory path (defaults to current dir)."}
                }
            },
            handler=self._tool_list_dir
        )

        # 5. Web Search
        self.register(
            name="search_web",
            description="Perform a quick search on the web.",
            parameters={
                "type": "object",
                "properties": {
                    "query": {"type": "string", "description": "Search query."}
                },
                "required": ["query"]
            },
            handler=self._tool_search_web
        )

        # 6. Save Memory
        self.register(
            name="save_memory",
            description="Save a key fact, rule, or preference to persistent long-term vector memory.",
            parameters={
                "type": "object",
                "properties": {
                    "category": {"type": "string", "enum": ["fact", "preference", "lesson"], "description": "Category of memory."},
                    "content": {"type": "string", "description": "The information to remember."}
                },
                "required": ["category", "content"]
            },
            handler=self._tool_save_memory
        )

        # 7. Save Skill
        self.register(
            name="save_skill",
            description="Save a new reusable workflow as an Antigravity SKILL.md in the skills directory.",
            parameters={
                "type": "object",
                "properties": {
                    "skill_name": {"type": "string", "description": "Unique identifier (kebab-case)."},
                    "description": {"type": "string", "description": "What this skill does."},
                    "instructions_md": {"type": "string", "description": "Detailed markdown instructions and runbook."}
                },
                "required": ["skill_name", "description", "instructions_md"]
            },
            handler=self._tool_save_skill
        )

    # --- Tool Implementations ---

    def _tool_run_command(self, command: str) -> Dict[str, Any]:
        try:
            res = subprocess.run(
                command,
                shell=True,
                cwd=str(self.workspace_root),
                capture_output=True,
                text=True,
                timeout=60
            )
            return {
                "exit_code": res.returncode,
                "stdout": res.stdout,
                "stderr": res.stderr
            }
        except subprocess.TimeoutExpired:
            return {"error": "Command timed out after 60 seconds."}
        except Exception as e:
            return {"error": str(e)}

    def _tool_view_file(self, file_path: str) -> Dict[str, Any]:
        target = (self.workspace_root / file_path).resolve()
        if not target.exists():
            return {"error": f"File '{file_path}' not found."}
        try:
            content = target.read_text(encoding="utf-8", errors="replace")
            return {"content": content, "size_bytes": len(content)}
        except Exception as e:
            return {"error": str(e)}

    def _tool_write_file(self, file_path: str, content: str) -> Dict[str, Any]:
        target = (self.workspace_root / file_path).resolve()
        target.parent.mkdir(parents=True, exist_ok=True)
        try:
            target.write_text(content, encoding="utf-8")
            return {"success": True, "path": str(target), "bytes_written": len(content)}
        except Exception as e:
            return {"error": str(e)}

    def _tool_list_dir(self, dir_path: str = ".") -> Dict[str, Any]:
        target = (self.workspace_root / dir_path).resolve()
        if not target.exists() or not target.is_dir():
            return {"error": f"Directory '{dir_path}' not found."}
        try:
            entries = []
            for item in target.iterdir():
                entries.append({
                    "name": item.name,
                    "is_dir": item.is_dir(),
                    "size": item.stat().st_size if item.is_file() else None
                })
            return {"entries": entries}
        except Exception as e:
            return {"error": str(e)}

    def _tool_search_web(self, query: str) -> Dict[str, Any]:
        try:
            url = f"https://html.duckduckgo.com/html/?q={urllib.parse.quote(query)}"
            req = urllib.request.Request(url, headers={"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64)"})
            with urllib.request.urlopen(req, timeout=10) as resp:
                html = resp.read().decode("utf-8", errors="ignore")
            # Simple text extraction
            import re
            snippets = re.findall(r'<a class="result__snippet[^>]*>(.*?)</a>', html)
            clean_snippets = [re.sub(r'<[^>]+>', '', s).strip() for s in snippets[:4]]
            return {"results": clean_snippets or ["No results found."]}
        except Exception as e:
            return {"error": f"Search failed: {str(e)}"}

    def _tool_save_memory(self, category: str, content: str) -> Dict[str, Any]:
        if not self.memory_store:
            return {"error": "Memory store not configured."}
        m_id = self.memory_store.add_memory(category=category, content=content, source="agent_tool")
        return {"success": True, "memory_id": m_id, "saved_content": content}

    def _tool_save_skill(self, skill_name: str, description: str, instructions_md: str) -> Dict[str, Any]:
        skills_dir = self.workspace_root / ".agents" / "skills" / skill_name
        skills_dir.mkdir(parents=True, exist_ok=True)
        skill_file = skills_dir / "SKILL.md"
        
        file_content = f"---\nname: {skill_name}\ndescription: {description}\n---\n\n{instructions_md}\n"
        skill_file.write_text(file_content, encoding="utf-8")
        return {"success": True, "skill_path": str(skill_file)}
