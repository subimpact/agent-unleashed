---
name: self-reflection
description: Guidelines and runbook for evaluating completed tasks, extracting user preferences, and synthesizing new Antigravity skills.
---

# Self-Reflection Skill

This skill guides the agent on how to perform autonomous metacognition.

## When to Trigger Self-Reflection
- After completing a complex multi-step debugging or refactoring task.
- When the user shares personal workflows, preferences, or environment configurations.
- When a new API or tool usage pattern was verified to work.

## Procedures:
1. **Extract Facts**:
   Call `save_memory` with category `fact` or `preference`.
2. **Author New Skills**:
   If the workflow is repeatable, call `save_skill` with the skill name and markdown instructions formatted according to the Antigravity Customization standard.
