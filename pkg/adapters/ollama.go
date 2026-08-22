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

func (o *OllamaAdapter) Execute(ctx context.Context, prompt string, sessionID string, workspaceDir string, options map[string]string) (string, error) {
	model := "hermes3"
	if m, ok := options["model"]; ok && m != "" && m != "auto" {
		model = m
	}

	reqBody, _ := json.Marshal(map[string]interface{}{
		"model":  model,
		"prompt": prompt,
		"stream": false,
	})

	req, err := http.NewRequestWithContext(ctx, "POST", o.endpoint+"/api/generate", bytes.NewBuffer(reqBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("could not connect to Ollama at %s: %w", o.endpoint, err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var parsed struct {
		Response string `json:"response"`
		Error    string `json:"error"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return string(body), nil
	}
	if parsed.Error != "" {
		return "", fmt.Errorf("ollama error: %s", parsed.Error)
	}

	return parsed.Response, nil
}
