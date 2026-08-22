package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"antigravity-unleashed/pkg/memory"
)

type ReflectionEngine struct {
	workspaceRoot string
	skillsDir     string
	memoryStore   *memory.MemoryStore
}

func NewReflectionEngine(workspaceRoot, skillsDir string, store *memory.MemoryStore) *ReflectionEngine {
	if skillsDir == "" {
		skillsDir = filepath.Join(workspaceRoot, ".agents", "skills")
	}
	_ = os.MkdirAll(skillsDir, 0755)

	return &ReflectionEngine{
		workspaceRoot: workspaceRoot,
		skillsDir:     skillsDir,
		memoryStore:   store,
	}
}

func (r *ReflectionEngine) Reflect(sessionID, userPrompt, responseText string, toolSteps []string) []string {
	var insights []string

	lower := strings.ToLower(userPrompt)
	if strings.Contains(lower, "i prefer") || strings.Contains(lower, "always use") || strings.Contains(lower, "remember that") || strings.Contains(lower, "my name is") {
		if r.memoryStore != nil {
			id, err := r.memoryStore.AddMemory("preference", "User Preference: "+userPrompt, "reflection_auto", map[string]interface{}{
				"session_id": sessionID,
			})
			if err == nil {
				insights = append(insights, fmt.Sprintf("Recorded preference memory (ID: %s)", id))
			}
		}
	}

	if len(toolSteps) >= 3 {
		workflowName := fmt.Sprintf("workflow-%s", strings.Join(toolSteps[:2], "-"))
		targetDir := filepath.Join(r.skillsDir, workflowName)
		if _, err := os.Stat(targetDir); os.IsNotExist(err) {
			_ = os.MkdirAll(targetDir, 0755)
			skillMD := filepath.Join(targetDir, "SKILL.md")

			var stepsMD strings.Builder
			for i, step := range toolSteps {
				stepsMD.WriteString(fmt.Sprintf("- **Step %d**: `%s`\n", i+1, step))
			}

			content := fmt.Sprintf(`---
name: %s
description: Synthesized by Antigravity-Unleashed from task: %s
---

# Workflow: %s

## Task Context
> %s

## Execution Steps
%s
`, workflowName, userPrompt, workflowName, userPrompt, stepsMD.String())

			if err := os.WriteFile(skillMD, []byte(content), 0644); err == nil {
				insights = append(insights, fmt.Sprintf("Autonomously authored new skill: %s", workflowName))
			}
		}
	}

	return insights
}
