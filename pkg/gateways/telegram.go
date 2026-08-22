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
	UpdateID int64 `json:"update_id"`
	Message  *struct {
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
				t.sendMessage(chatID, "🚀 *Agent-Unleashed Online!* (agt-ul Universal Go Core)\nSend me any coding task or command.")
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
