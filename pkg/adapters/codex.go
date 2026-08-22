package adapters

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type CodexAdapter struct {
	binaryPath string
}

func NewCodexAdapter(explicitPath string) *CodexAdapter {
	bin := explicitPath
	if bin == "" {
		if path, found := FindExecutable("codex", "codex-cli"); found {
			bin = path
		} else {
			bin = "codex"
		}
	}
	return &CodexAdapter{binaryPath: bin}
}

func (a *CodexAdapter) Name() string {
	return "codex"
}

func (a *CodexAdapter) DisplayName() string {
	return "OpenAI Codex CLI"
}

func (a *CodexAdapter) Detect() bool {
	_, found := FindExecutable(a.binaryPath, "codex", "codex-cli")
	return found
}

func (a *CodexAdapter) BinaryPath() string {
	return a.binaryPath
}

func (a *CodexAdapter) Capabilities() []string {
	return []string{"code_editing", "cli_execution", "zero_api_key"}
}

func (a *CodexAdapter) Execute(ctx context.Context, prompt string, sessionID string, workspaceDir string, options map[string]string) (*ExecutionResult, error) {
	start := time.Now()

	cmd := exec.CommandContext(ctx, a.binaryPath, "exec", prompt)
	if workspaceDir != "" {
		cmd.Dir = workspaceDir
	}

	out, err := cmd.CombinedOutput()
	duration := time.Since(start).Seconds()

	rawOutput := string(out)
	result := &ExecutionResult{
		Response:        rawOutput,
		RawCommand:      fmt.Sprintf("%s exec <prompt>", a.binaryPath),
		RawOutput:       rawOutput,
		DurationSeconds: duration,
		ContextLimit:    128000,
		NumTurns:        1,
	}

	inputWords := len(strings.Fields(prompt))
	outputWords := len(strings.Fields(rawOutput))
	result.InputTokens = int(float64(inputWords) * 1.33)
	result.OutputTokens = int(float64(outputWords) * 1.33)
	result.TotalTokens = result.InputTokens + result.OutputTokens

	if err != nil {
		if rawOutput != "" {
			return result, fmt.Errorf("codex error: %v\nOutput: %s", err, rawOutput)
		}
		return result, fmt.Errorf("codex execution failed: %w", err)
	}

	return result, nil
}
