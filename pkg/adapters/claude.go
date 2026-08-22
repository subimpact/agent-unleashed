package adapters

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type ClaudeAdapter struct {
	customPath string
}

func NewClaudeAdapter(customPath string) *ClaudeAdapter {
	return &ClaudeAdapter{customPath: customPath}
}

func (c *ClaudeAdapter) Name() string {
	return "claude"
}

func (c *ClaudeAdapter) DisplayName() string {
	return "Claude Code CLI (claude)"
}

func (c *ClaudeAdapter) BinaryPath() string {
	if c.customPath != "" {
		if _, err := os.Stat(c.customPath); err == nil {
			return c.customPath
		}
	}
	if p, ok := FindExecutable("claude", "claude.exe", "claude.cmd"); ok {
		return p
	}
	return "claude"
}

func (c *ClaudeAdapter) Detect() bool {
	p := c.BinaryPath()
	if filepath.IsAbs(p) {
		_, err := os.Stat(p)
		return err == nil
	}
	_, ok := FindExecutable("claude", "claude.exe", "claude.cmd")
	return ok
}

func (c *ClaudeAdapter) Capabilities() []string {
	return []string{"claude_3_7_sonnet", "terminal_tools", "codebase_edits", "zero_api_key"}
}

func (c *ClaudeAdapter) Execute(ctx context.Context, prompt string, sessionID string, workspaceDir string, options map[string]string) (*ExecutionResult, error) {
	bin := c.BinaryPath()

	args := []string{
		"-p", prompt,
		"--dangerously-skip-permissions",
	}

	start := time.Now()
	cmd := exec.CommandContext(ctx, bin, args...)
	if workspaceDir != "" {
		cmd.Dir = workspaceDir
	}

	out, err := cmd.CombinedOutput()
	duration := time.Since(start).Seconds()

	result := &ExecutionResult{
		ContextLimit:    200000, // 200k for Claude 3.7
		DurationSeconds: duration,
		Response:        string(out),
		RawCommand:      fmt.Sprintf("%s %s", bin, strings.Join(args, " ")),
		RawOutput:       string(out),
		InputTokens:     len(prompt) / 4,
		OutputTokens:    len(out) / 4,
		TotalTokens:     (len(prompt) + len(out)) / 4,
	}

	if err != nil {
		return result, fmt.Errorf("claude CLI execution failed (%v): %s", err, string(out))
	}

	return result, nil
}
