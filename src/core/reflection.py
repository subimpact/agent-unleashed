"""
Antigravity-Unleashed Autonomous Reflection Engine
Performs post-task metacognition: extracts persistent memories, identifies recurring workflows,
and synthesizes new Antigravity SKILL.md packages.
"""

import json
import logging
from pathlib import Path
from typing import List, Dict, Any, Optional

logger = logging.getLogger("antigravity_unleashed.reflection")


class ReflectionEngine:
    def __init__(self, memory_store, workspace_root: str = "."):
        self.memory_store = memory_store
        self.workspace_root = Path(workspace_root).resolve()
        self.skills_dir = self.workspace_root / ".agents" / "skills"
        self.skills_dir.mkdir(parents=True, exist_ok=True)

    def reflect_on_task(
        self,
        session_id: str,
        user_prompt: str,
        execution_steps: List[Dict[str, Any]],
        final_answer: str,
    ) -> Dict[str, Any]:
        """
        Evaluate completed task execution for lessons, preferences, and reusable skills.
        """
        insights = {
            "facts_extracted": [],
            "skills_created": []
        }

        # 1. Fact Extraction Heuristic (preferences, environment details)
        lower_prompt = user_prompt.lower()
        if "i prefer" in lower_prompt or "always use" in lower_prompt or "my name is" in lower_prompt or "remember that" in lower_prompt:
            fact_id = self.memory_store.add_memory(
                category="preference",
                content=f"User Preference: {user_prompt}",
                source="reflection_auto",
                metadata={"session_id": session_id}
            )
            insights["facts_extracted"].append(f"Recorded preference (ID: {fact_id})")

        # 2. Reusable Skill Synthesis
        # If the task took multiple steps and was successful, consider distilling it into a skill
        if len(execution_steps) >= 3:
            logger.info(f"[Reflection] Analyzing {len(execution_steps)} steps for skill synthesis...")
            # For demonstration, summarize tool sequence into an autonomous workflow skill if novel
            tool_names = [s.get("tool_name") for s in execution_steps if "tool_name" in s]
            if tool_names:
                unique_workflow_name = f"workflow-{'-'.join(tool_names[:3])}"
                target_skill_dir = self.skills_dir / unique_workflow_name
                
                if not target_skill_dir.exists():
                    target_skill_dir.mkdir(parents=True, exist_ok=True)
                    skill_md = target_skill_dir / "SKILL.md"
                    
                    steps_md = "\n".join([f"- **Step {i+1}**: Call `{s.get('tool_name')}`" for i, s in enumerate(execution_steps)])
                    content = f"""---
name: {unique_workflow_name}
description: Autonomous workflow synthesized by Antigravity-Unleashed from task: {user_prompt[:80]}
---

# Workflow: {unique_workflow_name}

## Objective
Automatically execute the procedure derived from prompt:
> {user_prompt}

## Execution Plan
{steps_md}

## Key Verification
Verify outputs and handle edge cases when repeating this procedure.
"""
                    skill_md.write_text(content, encoding="utf-8")
                    insights["skills_created"].append(unique_workflow_name)
                    logger.info(f"[Reflection] Autonomously created new skill: {unique_workflow_name}")

        return insights
