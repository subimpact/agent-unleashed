package gateways

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"golang.org/x/net/websocket"

	"agent-unleashed/pkg/engine"
)

type WSMessage struct {
	Action    string `json:"action"` // "chat", "get_status", "get_context"
	Prompt    string `json:"prompt,omitempty"`
	SessionID string `json:"session_id,omitempty"`
	Driver    string `json:"driver,omitempty"`
}

type WebSocketHub struct {
	engine  *engine.UnleashedEngine
	clients map[*websocket.Conn]bool
	mu      sync.RWMutex
}

func NewWebSocketHub(eng *engine.UnleashedEngine) *WebSocketHub {
	return &WebSocketHub{
		engine:  eng,
		clients: make(map[*websocket.Conn]bool),
	}
}

func (h *WebSocketHub) Handler() http.Handler {
	return websocket.Handler(func(ws *websocket.Conn) {
		h.mu.Lock()
		h.clients[ws] = true
		h.mu.Unlock()

		defer func() {
			h.mu.Lock()
			delete(h.clients, ws)
			h.mu.Unlock()
			_ = ws.Close()
		}()

		// Send initial welcome message
		_ = websocket.JSON.Send(ws, map[string]interface{}{
			"type":    "welcome",
			"agent":   "Agent-Unleashed (agt-ul)",
			"driver":  h.engine.GetActiveDriverName(),
			"version": "1.0.0",
		})

		for {
			var msg WSMessage
			if err := websocket.JSON.Receive(ws, &msg); err != nil {
				break
			}

			switch msg.Action {
			case "chat":
				sessionID := msg.SessionID
				if sessionID == "" {
					sessionID = "ws_client"
				}

				if msg.Driver != "" {
					h.engine.SetDriver(msg.Driver)
				}

				events := make(chan engine.Event)
				go h.engine.Chat(context.Background(), sessionID, msg.Prompt, "websocket", events)

				for ev := range events {
					if err := websocket.JSON.Send(ws, ev); err != nil {
						break
					}
				}

			case "get_status":
				stats, _ := h.engine.MemoryStore.GetStats()
				_ = websocket.JSON.Send(ws, map[string]interface{}{
					"type":           "status_response",
					"active_driver":  h.engine.GetActiveDriverName(),
					"total_tokens":   h.engine.GetTotalTokens(),
					"total_memories": stats.TotalMemories,
					"rooms":          stats.Rooms,
				})

			case "get_context":
				lastRes := h.engine.GetLastResult()
				_ = websocket.JSON.Send(ws, map[string]interface{}{
					"type":       "context_response",
					"last_stats": lastRes,
				})

			default:
				_ = websocket.JSON.Send(ws, map[string]string{
					"error": fmt.Sprintf("unknown action '%s'", msg.Action),
				})
			}
		}
	})
}
