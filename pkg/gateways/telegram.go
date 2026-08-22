package gateways

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"agent-unleashed/pkg/config"
	"agent-unleashed/pkg/engine"
)

type TelegramGateway struct {
	engine  *engine.UnleashedEngine
	cfg     config.TelegramGatewayConfig
	client  *http.Client
	offset  int64
	apiBase string
}

type TGUpdate struct {
	UpdateID      int64 `json:"update_id"`
	CallbackQuery *struct {
		ID   string `json:"id"`
		Data string `json:"data"`
		From struct {
			ID int64 `json:"id"`
		} `json:"from"`
		Message *struct {
			MessageID int64 `json:"message_id"`
			Chat      struct {
				ID int64 `json:"id"`
			} `json:"chat"`
		} `json:"message"`
	} `json:"callback_query"`
	Message *struct {
		MessageID int64 `json:"message_id"`
		Chat      struct {
			ID int64 `json:"id"`
		} `json:"chat"`
		Text string `json:"text"`
	} `json:"message"`
}

type TGResponse struct {
	OK     bool       `json:"ok"`
	Result []TGUpdate `json:"result"`
}

func NewTelegramGateway(eng *engine.UnleashedEngine, cfg config.TelegramGatewayConfig) *TelegramGateway {
	return &TelegramGateway{
		engine:  eng,
		cfg:     cfg,
		client:  &http.Client{Timeout: 35 * time.Second},
		apiBase: fmt.Sprintf("https://api.telegram.org/bot%s", cfg.BotToken),
	}
}

func (t *TelegramGateway) isAllowed(chatID int64) bool {
	if len(t.cfg.AllowedChatIDs) == 0 {
		return true
	}
	for _, id := range t.cfg.AllowedChatIDs {
		if id == chatID {
			return true
		}
	}
	return false
}

func (t *TelegramGateway) sendMessage(chatID int64, text string) {
	if len(text) > 4000 {
		text = text[:3990] + "\n...(truncated)"
	}

	payload := map[string]interface{}{
		"chat_id": chatID,
		"text":    text,
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", t.apiBase+"/sendMessage", bytes.NewBuffer(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := t.client.Do(req)
	if err == nil {
		_ = resp.Body.Close()
	}
}

func (t *TelegramGateway) SendInteractiveApproval(chatID int64, actionDescription string, actionID string) {
	payload := map[string]interface{}{
		"chat_id": chatID,
		"text":    fmt.Sprintf("⚠️ **[Approval Required]**\nAction: `%s`", actionDescription),
		"reply_markup": map[string]interface{}{
			"inline_keyboard": [][]map[string]string{
				{
					{"text": "✅ Approve", "callback_data": fmt.Sprintf("approve_%s", actionID)},
					{"text": "⛔ Deny", "callback_data": fmt.Sprintf("deny_%s", actionID)},
				},
			},
		},
	}
	body, _ := json.Marshal(payload)

	req, _ := http.NewRequest("POST", t.apiBase+"/sendMessage", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := t.client.Do(req)
	if err == nil {
		_ = resp.Body.Close()
	}
}

func (t *TelegramGateway) answerCallbackQuery(callbackID string, text string) {
	payload := map[string]string{
		"callback_query_id": callbackID,
		"text":              text,
	}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", t.apiBase+"/answerCallbackQuery", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := t.client.Do(req)
	if err == nil {
		_ = resp.Body.Close()
	}
}

func (t *TelegramGateway) Start(ctx context.Context) error {
	if t.cfg.BotToken == "" {
		return nil
	}

	log.Println("[Telegram Gateway] Starting Telegram Bot polling loop...")

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		pollURL := fmt.Sprintf("%s/getUpdates?timeout=25&offset=%d", t.apiBase, t.offset)
		resp, err := t.client.Get(pollURL)
		if err != nil {
			time.Sleep(2 * time.Second)
			continue
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		var tgResp TGResponse
		if err := json.Unmarshal(body, &tgResp); err != nil || !tgResp.OK {
			time.Sleep(2 * time.Second)
			continue
		}

		for _, update := range tgResp.Result {
			if update.UpdateID >= t.offset {
				t.offset = update.UpdateID + 1
			}

			// Handle Interactive Button Callbacks
			if update.CallbackQuery != nil {
				cb := update.CallbackQuery
				if cb.Data != "" {
					if strings.HasPrefix(cb.Data, "approve_") {
						t.answerCallbackQuery(cb.ID, "Action Approved! Executing...")
						if cb.Message != nil {
							t.sendMessage(cb.Message.Chat.ID, "✅ Action approved by user.")
						}
					} else if strings.HasPrefix(cb.Data, "deny_") {
						t.answerCallbackQuery(cb.ID, "Action Denied.")
						if cb.Message != nil {
							t.sendMessage(cb.Message.Chat.ID, "⛔ Action cancelled by user.")
						}
					}
				}
				continue
			}

			if update.Message == nil || update.Message.Text == "" {
				continue
			}

			chatID := update.Message.Chat.ID
			if !t.isAllowed(chatID) {
				t.sendMessage(chatID, "⛔ Unauthorized.")
				continue
			}

			text := strings.TrimSpace(update.Message.Text)
			sessionID := fmt.Sprintf("telegram_%d", chatID)

			if text == "/start" {
				t.sendMessage(chatID, "🚀 *Agent-Unleashed Online!* (agt-ul Universal Go Core)\nSend me any coding task, question, or command.")
				continue
			}

			if text == "/memory" && t.engine.MemoryStore != nil {
				stats, _ := t.engine.MemoryStore.GetStats()
				var out strings.Builder
				out.WriteString(fmt.Sprintf("🧠 Palace-Mnemosyne Memory (%d Total):\n", stats.TotalMemories))
				for room, count := range stats.Rooms {
					out.WriteString(fmt.Sprintf("• Room [%s]: %d drawers\n", room, count))
				}
				t.sendMessage(chatID, out.String())
				continue
			}

			if text == "/context" {
				stats := t.engine.GetLastResult()
				used := 0
				limit := 1048576
				if stats != nil {
					used = stats.TotalTokens
					if stats.ContextLimit > 0 {
						limit = stats.ContextLimit
					}
				}
				pctLeft := 100.0 - (float64(used)/float64(limit))*100.0
				t.sendMessage(chatID, fmt.Sprintf("📊 Context Budget:\n• Used: %d tok\n• Limit: %d tok\n• Available: %.1f%%\n• Driver: %s",
					used, limit, pctLeft, t.engine.GetActiveDriverName()))
				continue
			}

			// Run chat query asynchronously
			go func(cID int64, prompt, sID string) {
				events := make(chan engine.Event)
				go t.engine.Chat(ctx, sID, prompt, "telegram", events)

				var fullResponse strings.Builder
				for ev := range events {
					if ev.Type == engine.EventText {
						fullResponse.WriteString(ev.Content)
					}
				}

				reply := fullResponse.String()
				if reply == "" {
					reply = "Task executed."
				}
				t.sendMessage(cID, reply)
			}(chatID, text, sessionID)
		}
	}
}
