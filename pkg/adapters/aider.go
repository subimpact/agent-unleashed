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

type AiderAdapter struct {
	customPath string
}

func NewAiderAdapter(customPath string) *AiderAdapter {
	return &AiderAdapter{customPath: customPath}
}

func (a *AiderAdapter) Name() string {
	return "aider"
}

func (a *AiderAdapter) DisplayName() string {
	return "Aider Coding Assistant (aider)"
}

func (a *AiderAdapter) BinaryPath() string {
	if a.customPath != "" {
		if _, err := os.Stat(a.customPath); err == nil {
			return a.customPath
		}
	}
	if p, ok := FindExecutable("aider", "aider.exe"); ok {
		return p
	}
	return "aider"
}

func (a *AiderAdapter) Detect() bool {
	p := a.BinaryPath()
	if filepath.IsAbs(p) {
		_, err := os.Stat(p)
		return err == nil
	}
	_, ok := FindExecutable("aider", "aider.exe")
	return ok
}

func (a *AiderAdapter) Capabilities() []string {
	return []string{"git_diffs", "repo_mapping", "multi_model", "zero_api_key"}
}

func (a *AiderAdapter) Execute(ctx context.Context, prompt string, sessionID string, workspaceDir string, options map[string]string) (*ExecutionResult, error) {
	bin := a.BinaryPath()

	args := []string{
		"--message", prompt,
		"--no-git",
		"--yes",
	}

	start := time.Now()
	cmd := exec.CommandContext(ctx, bin, args...)
	if workspaceDir != "" {
		cmd.Dir = workspaceDir
	}

	out, err := cmd.CombinedOutput()
	duration := time.Since(start).Seconds()

	result := &ExecutionResult{
		ContextLimit:    128000,
		DurationSeconds: duration,
		Response:        string(out),
		RawCommand:      fmt.Sprintf("%s %s", bin, strings.Join(args, " ")),
		RawOutput:       string(out),
		InputTokens:     len(prompt) / 4,
		OutputTokens:    len(out) / 4,
		TotalTokens:     (len(prompt) + len(out)) / 4,
	}

	if err != nil {
		return result, fmt.Errorf("aider CLI execution failed (%v): %s", err, string(out))
	}

	return result, nil
}
