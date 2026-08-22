package adapters

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type OllamaAdapter struct {
	endpoint string
}

func NewOllamaAdapter(endpoint string) *OllamaAdapter {
	if endpoint == "" {
		endpoint = "http://localhost:11434"
	}
	return &OllamaAdapter{endpoint: endpoint}
}

func (o *OllamaAdapter) Name() string {
	return "ollama"
}

func (o *OllamaAdapter) DisplayName() string {
	return "Local Ollama / vLLM (Offline)"
}

func (o *OllamaAdapter) BinaryPath() string {
	if p, ok := FindExecutable("ollama", "ollama.exe"); ok {
		return p
	}
	return o.endpoint
}

func (o *OllamaAdapter) Detect() bool {
	client := &http.Client{Timeout: 1 * time.Second}
	resp, err := client.Get(o.endpoint + "/api/tags")
	if err == nil && resp.StatusCode == 200 {
		_ = resp.Body.Close()
		return true
	}
	_, ok := FindExecutable("ollama", "ollama.exe")
	return ok
}

func (o *OllamaAdapter) Capabilities() []string {
	return []string{"100_percent_offline", "privacy", "hermes_3", "qwen_2_5_coder", "llama_3_3"}
}

func (o *OllamaAdapter) Execute(ctx context.Context, prompt string, sessionID string, workspaceDir string, options map[string]string) (*ExecutionResult, error) {
	model := "hermes3"
	if m, ok := options["model"]; ok && m != "" && m != "auto" {
		model = m
	}

	reqBody, _ := json.Marshal(map[string]interface{}{
		"model":  model,
		"prompt": prompt,
		"stream": false,
	})

	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, "POST", o.endpoint+"/api/generate", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	duration := time.Since(start).Seconds()

	result := &ExecutionResult{
		ContextLimit:    128000,
		DurationSeconds: duration,
		RawCommand:      fmt.Sprintf("POST %s/api/generate (model: %s)", o.endpoint, model),
	}

	if err != nil {
		return result, fmt.Errorf("could not connect to Ollama at %s: %w", o.endpoint, err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	result.RawOutput = string(body)

	var parsed struct {
		Response      string `json:"response"`
		PromptEvalCnt int    `json:"prompt_eval_count"`
		EvalCount     int    `json:"eval_count"`
		Error         string `json:"error"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		result.Response = string(body)
		return result, nil
	}
	if parsed.Error != "" {
		return result, fmt.Errorf("ollama error: %s", parsed.Error)
	}

	result.Response = parsed.Response
	result.InputTokens = parsed.PromptEvalCnt
	result.OutputTokens = parsed.EvalCount
	result.TotalTokens = parsed.PromptEvalCnt + parsed.EvalCount
	return result, nil
}
