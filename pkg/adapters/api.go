package adapters

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"agent-unleashed/pkg/config"
)

type APIAdapter struct {
	cfg config.ModelConfig
}

func NewAPIAdapter(cfg config.ModelConfig) *APIAdapter {
	return &APIAdapter{cfg: cfg}
}

func (a *APIAdapter) Name() string {
	return "api"
}

func (a *APIAdapter) DisplayName() string {
	return "Cloud Multi-Provider API (OpenRouter / Gemini / Claude / OpenAI)"
}

func (a *APIAdapter) BinaryPath() string {
	return "https://api"
}

func (a *APIAdapter) Detect() bool {
	return a.cfg.OpenRouterAPIKey != "" || a.cfg.GeminiAPIKey != "" || a.cfg.AnthropicAPIKey != "" || a.cfg.OpenAIAPIKey != ""
}

func (a *APIAdapter) Capabilities() []string {
	return []string{"openrouter", "gemini_2_5_pro", "claude_3_7_sonnet", "deepseek_r1", "gpt_4o"}
}

func (a *APIAdapter) Execute(ctx context.Context, prompt string, sessionID string, workspaceDir string, options map[string]string) (*ExecutionResult, error) {
	client := &http.Client{Timeout: 90 * time.Second}
	start := time.Now()

	result := &ExecutionResult{
		ContextLimit: 128000,
		InputTokens:  len(prompt) / 4,
	}

	// 1. OpenRouter
	if a.cfg.OpenRouterAPIKey != "" {
		model := "anthropic/claude-3.7-sonnet"
		if a.cfg.ModelName != "" && a.cfg.ModelName != "auto" {
			model = a.cfg.ModelName
		}
		result.RawCommand = fmt.Sprintf("POST https://openrouter.ai/api/v1/chat/completions (model: %s)", model)

		reqBody, _ := json.Marshal(map[string]interface{}{
			"model": model,
			"messages": []map[string]string{
				{"role": "user", "content": prompt},
			},
		})
		req, _ := http.NewRequestWithContext(ctx, "POST", "https://openrouter.ai/api/v1/chat/completions", bytes.NewBuffer(reqBody))
		req.Header.Set("Authorization", "Bearer "+a.cfg.OpenRouterAPIKey)
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		result.DurationSeconds = time.Since(start).Seconds()

		if err == nil && resp.StatusCode == 200 {
			defer resp.Body.Close()
			var data struct {
				Choices []struct {
					Message struct {
						Content string `json:"content"`
					} `json:"message"`
				} `json:"choices"`
				Usage struct {
					PromptTokens int `json:"prompt_tokens"`
					ComplTokens  int `json:"completion_tokens"`
					TotalTokens  int `json:"total_tokens"`
				} `json:"usage"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&data); err == nil && len(data.Choices) > 0 {
				result.Response = data.Choices[0].Message.Content
				result.InputTokens = data.Usage.PromptTokens
				result.OutputTokens = data.Usage.ComplTokens
				result.TotalTokens = data.Usage.TotalTokens
				return result, nil
			}
		}
	}

	// 2. Google Gemini API
	if a.cfg.GeminiAPIKey != "" {
		url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/gemini-2.0-flash:generateContent?key=%s", a.cfg.GeminiAPIKey)
		result.RawCommand = "POST https://generativelanguage.googleapis.com (model: gemini-2.0-flash)"

		reqBody, _ := json.Marshal(map[string]interface{}{
			"contents": []map[string]interface{}{
				{"parts": []map[string]string{{"text": prompt}}},
			},
		})
		req, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(reqBody))
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		result.DurationSeconds = time.Since(start).Seconds()

		if err == nil && resp.StatusCode == 200 {
			defer resp.Body.Close()
			var data struct {
				Candidates []struct {
					Content struct {
						Parts []struct {
							Text string `json:"text"`
						} `json:"parts"`
					} `json:"content"`
				} `json:"candidates"`
				UsageMetadata struct {
					PromptTokenCount     int `json:"promptTokenCount"`
					CandidatesTokenCount int `json:"candidatesTokenCount"`
					TotalTokenCount      int `json:"totalTokenCount"`
				} `json:"usageMetadata"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&data); err == nil && len(data.Candidates) > 0 && len(data.Candidates[0].Content.Parts) > 0 {
				result.Response = data.Candidates[0].Content.Parts[0].Text
				result.InputTokens = data.UsageMetadata.PromptTokenCount
				result.OutputTokens = data.UsageMetadata.CandidatesTokenCount
				result.TotalTokens = data.UsageMetadata.TotalTokenCount
				result.ContextLimit = 1048576 // 1M tokens
				return result, nil
			}
		}
	}

	return result, fmt.Errorf("no valid cloud API key responded or configured")
}
