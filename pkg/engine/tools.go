package engine

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"agent-unleashed/pkg/memory"
)

type ToolResult struct {
	Output string `json:"output,omitempty"`
	Error  string `json:"error,omitempty"`
}

type ToolRunner struct {
	workspaceRoot string
	memoryStore   *memory.MemoryStore
}

func NewToolRunner(workspaceRoot string, store *memory.MemoryStore) *ToolRunner {
	abs, err := filepath.Abs(workspaceRoot)
	if err != nil {
		abs = workspaceRoot
	}
	return &ToolRunner{
		workspaceRoot: abs,
		memoryStore:   store,
	}
}

func (t *ToolRunner) RunCommand(cmdStr string) ToolResult {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	var cmd *exec.Cmd
	if os.PathSeparator == '\\' {
		cmd = exec.CommandContext(ctx, "powershell", "-NoProfile", "-Command", cmdStr)
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", cmdStr)
	}
	cmd.Dir = t.workspaceRoot

	out, err := cmd.CombinedOutput()
	if err != nil {
		return ToolResult{
			Output: string(out),
			Error:  fmt.Sprintf("Command failed: %v", err),
		}
	}
	return ToolResult{Output: string(out)}
}

func (t *ToolRunner) ViewFile(filePath string) ToolResult {
	target := filepath.Join(t.workspaceRoot, filePath)
	content, err := os.ReadFile(target)
	if err != nil {
		return ToolResult{Error: err.Error()}
	}
	return ToolResult{Output: string(content)}
}

func (t *ToolRunner) WriteFile(filePath, content string) ToolResult {
	target := filepath.Join(t.workspaceRoot, filePath)
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return ToolResult{Error: err.Error()}
	}
	if err := os.WriteFile(target, []byte(content), 0644); err != nil {
		return ToolResult{Error: err.Error()}
	}
	return ToolResult{Output: fmt.Sprintf("Successfully written %d bytes to %s", len(content), filePath)}
}

func (t *ToolRunner) ListDir(dirPath string) ToolResult {
	target := filepath.Join(t.workspaceRoot, dirPath)
	entries, err := os.ReadDir(target)
	if err != nil {
		return ToolResult{Error: err.Error()}
	}

	var lines []string
	for _, e := range entries {
		typeStr := "FILE"
		if e.IsDir() {
			typeStr = "DIR "
		}
		lines = append(lines, fmt.Sprintf("[%s] %s", typeStr, e.Name()))
	}
	return ToolResult{Output: strings.Join(lines, "\n")}
}

func (t *ToolRunner) SearchWeb(query string) ToolResult {
	searchURL := fmt.Sprintf("https://html.duckduckgo.com/html/?q=%s", url.QueryEscape(query))
	req, err := http.NewRequest("GET", searchURL, nil)
	if err != nil {
		return ToolResult{Error: err.Error()}
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return ToolResult{Error: err.Error()}
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	re := regexp.MustCompile(`<a class="result__snippet[^>]*>(.*?)</a>`)
	matches := re.FindAllStringSubmatch(string(body), 4)

	stripTag := regexp.MustCompile(`<[^>]+>`)
	var results []string
	for _, m := range matches {
		if len(m) > 1 {
			clean := strings.TrimSpace(stripTag.ReplaceAllString(m[1], ""))
			if clean != "" {
				results = append(results, clean)
			}
		}
	}

	if len(results) == 0 {
		return ToolResult{Output: "No search results found."}
	}
	return ToolResult{Output: strings.Join(results, "\n\n")}
}

func (t *ToolRunner) SaveMemory(category, content string) ToolResult {
	if t.memoryStore == nil {
		return ToolResult{Error: "Memory store not configured"}
	}
	id, err := t.memoryStore.AddMemory(category, content, "agent_tool", nil)
	if err != nil {
		return ToolResult{Error: err.Error()}
	}
	return ToolResult{Output: fmt.Sprintf("Saved memory [%s] (ID: %s)", category, id)}
}

func (t *ToolRunner) SaveSkill(skillName, description, instructionsMD string) ToolResult {
	skillDir := filepath.Join(t.workspaceRoot, ".agents", "skills", skillName)
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		return ToolResult{Error: err.Error()}
	}

	skillMD := filepath.Join(skillDir, "SKILL.md")
	content := fmt.Sprintf("---\nname: %s\ndescription: %s\n---\n\n%s\n", skillName, description, instructionsMD)
	if err := os.WriteFile(skillMD, []byte(content), 0644); err != nil {
		return ToolResult{Error: err.Error()}
	}

	return ToolResult{Output: fmt.Sprintf("Created skill %s at %s", skillName, skillMD)}
}
