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

// SanitizeName reduces arbitrary text to one safe path segment, for anywhere a
// caller-supplied name becomes a directory.
func SanitizeName(raw string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(raw)) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '_', r == '-', r == ' ', r == '.', r == '/':
			b.WriteRune('-')
		}
	}
	name := b.String()
	for strings.Contains(name, "--") {
		name = strings.ReplaceAll(name, "--", "-")
	}
	name = strings.Trim(name, "-")
	if len(name) > 48 {
		name = strings.Trim(name[:48], "-")
	}
	return name
}

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

// resolve joins a caller-supplied path onto the workspace root and refuses
// anything that escapes it. filepath.Join alone cleans "../.." into a real
// parent path, so every file tool needs this check.
func (t *ToolRunner) resolve(rel string) (string, error) {
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("absolute paths are not allowed: %s", rel)
	}
	target := filepath.Clean(filepath.Join(t.workspaceRoot, rel))
	root := filepath.Clean(t.workspaceRoot)
	if target != root && !strings.HasPrefix(target, root+string(os.PathSeparator)) {
		return "", fmt.Errorf("path %s escapes the workspace", rel)
	}
	return target, nil
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
	target, err := t.resolve(filePath)
	if err != nil {
		return ToolResult{Error: err.Error()}
	}
	content, err := os.ReadFile(target)
	if err != nil {
		return ToolResult{Error: err.Error()}
	}
	return ToolResult{Output: string(content)}
}

func (t *ToolRunner) WriteFile(filePath, content string) ToolResult {
	target, err := t.resolve(filePath)
	if err != nil {
		return ToolResult{Error: err.Error()}
	}
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return ToolResult{Error: err.Error()}
	}
	if err := os.WriteFile(target, []byte(content), 0644); err != nil {
		return ToolResult{Error: err.Error()}
	}
	return ToolResult{Output: fmt.Sprintf("Successfully written %d bytes to %s", len(content), filePath)}
}

func (t *ToolRunner) ListDir(dirPath string) ToolResult {
	if dirPath == "" {
		dirPath = "."
	}
	target, err := t.resolve(dirPath)
	if err != nil {
		return ToolResult{Error: err.Error()}
	}
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
	clean := SanitizeName(skillName)
	if clean == "" {
		return ToolResult{Error: fmt.Sprintf("skill name %q contains no usable characters", skillName)}
	}
	skillDir := filepath.Join(t.workspaceRoot, ".agents", "skills", clean)
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		return ToolResult{Error: err.Error()}
	}

	skillMD := filepath.Join(skillDir, "SKILL.md")
	content := fmt.Sprintf("---\nname: %s\ndescription: %s\n---\n\n%s\n", clean, description, instructionsMD)
	if err := os.WriteFile(skillMD, []byte(content), 0644); err != nil {
		return ToolResult{Error: err.Error()}
	}

	return ToolResult{Output: fmt.Sprintf("Created skill %s at %s", clean, skillMD)}
}
