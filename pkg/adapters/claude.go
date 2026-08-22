package adapters

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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

func (c *ClaudeAdapter) Execute(ctx context.Context, prompt string, sessionID string, workspaceDir string, options map[string]string) (string, error) {
	bin := c.BinaryPath()

	args := []string{
		"-p", prompt,
		"--dangerously-skip-permissions",
	}

	cmd := exec.CommandContext(ctx, bin, args...)
	if workspaceDir != "" {
		cmd.Dir = workspaceDir
	}

	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("claude CLI execution failed (%v): %s", err, string(out))
	}

	return string(out), nil
}
