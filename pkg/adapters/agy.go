package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

type AgyAdapter struct {
	customPath string
	sessions   map[string]string
	mu         sync.RWMutex
}

func NewAgyAdapter(customPath string) *AgyAdapter {
	return &AgyAdapter{
		customPath: customPath,
		sessions:   make(map[string]string),
	}
}

func (a *AgyAdapter) Name() string {
	return "agy"
}

func (a *AgyAdapter) DisplayName() string {
	return "Google Antigravity CLI (agy)"
}

func (a *AgyAdapter) BinaryPath() string {
	if a.customPath != "" {
		if _, err := os.Stat(a.customPath); err == nil {
			return a.customPath
		}
	}

	if p, ok := FindExecutable("agy", "agy.exe"); ok {
		return p
	}

	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData != "" {
		candidate := filepath.Join(localAppData, "agy", "bin", "agy.exe")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	return "agy"
}

func (a *AgyAdapter) Detect() bool {
	p := a.BinaryPath()
	if filepath.IsAbs(p) {
		_, err := os.Stat(p)
		return err == nil
	}
	_, ok := FindExecutable("agy", "agy.exe")
	return ok
}

func (a *AgyAdapter) Capabilities() []string {
	return []string{"subagents", "code_editing", "deep_reasoning", "artifacts", "skills", "zero_api_key"}
}

func (a *AgyAdapter) Execute(ctx context.Context, prompt string, sessionID string, workspaceDir string, options map[string]string) (string, error) {
	bin := a.BinaryPath()

	effort := "high"
	if e, ok := options["effort"]; ok && e != "" {
		effort = e
	}

	args := []string{
		"--output-format", "json",
		fmt.Sprintf("--effort=%s", effort),
		fmt.Sprintf("--print=%s", prompt),
		"--dangerously-skip-permissions",
	}

	a.mu.RLock()
	convID, hasConv := a.sessions[sessionID]
	a.mu.RUnlock()

	if hasConv && convID != "" {
		args = append(args, fmt.Sprintf("--conversation=%s", convID))
	}

	cmd := exec.CommandContext(ctx, bin, args...)
	if workspaceDir != "" {
		cmd.Dir = workspaceDir
	}

	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("agy execution failed (%v): %s", err, string(out))
	}

	var parsed struct {
		ConversationID string `json:"conversation_id"`
		Response       string `json:"response"`
	}

	if jsonErr := json.Unmarshal(out, &parsed); jsonErr == nil && parsed.Response != "" {
		if parsed.ConversationID != "" {
			a.mu.Lock()
			a.sessions[sessionID] = parsed.ConversationID
			a.mu.Unlock()
		}
		return parsed.Response, nil
	}

	return string(out), nil
}
