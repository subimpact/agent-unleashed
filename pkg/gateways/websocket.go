package gateways

import (
	"context"
	"fmt"
	"log"
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
	auth    *GatewayAuth
	clients map[*websocket.Conn]bool
	mu      sync.RWMutex
}

func NewWebSocketHub(eng *engine.UnleashedEngine, auth *GatewayAuth) *WebSocketHub {
	return &WebSocketHub{
		engine:  eng,
		auth:    auth,
		clients: make(map[*websocket.Conn]bool),
	}
}

func (h *WebSocketHub) Handler() http.Handler {
	// websocket.Handler performs no origin check of its own - it only validates
	// that the Origin header parses as a URL - so a cross-site page could
	// otherwise open a socket straight into the agent. Supplying an explicit
	// Handshake is the only place x/net/websocket lets us refuse the upgrade.
	return websocket.Server{
		Handshake: func(_ *websocket.Config, req *http.Request) error {
			if err := h.auth.CheckHTTP(req, ""); err != nil {
				log.Printf("[WS Gateway] Rejected handshake from %s: %v\n", req.RemoteAddr, err)
				return err
			}
			return nil
		},
		Handler: websocket.Handler(h.serve),
	}
}

func (h *WebSocketHub) serve(ws *websocket.Conn) {
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

			ctx, cancel := context.WithCancel(context.Background())
			events := make(chan engine.Event)
			go h.engine.Chat(ctx, sessionID, msg.Prompt, "websocket", events)

			// Chat writes to an unbuffered channel, so abandoning the range on
			// the first send error would leave it blocked forever holding a live
			// CLI subprocess. Cancel the run, then drain to completion.
			failed := false
			for ev := range events {
				if failed {
					continue
				}
				if err := websocket.JSON.Send(ws, ev); err != nil {
					failed = true
					cancel()
				}
			}
			cancel()

		case "get_status":
			payload := map[string]interface{}{
				"type":          "status_response",
				"active_driver": h.engine.GetActiveDriverName(),
				"total_tokens":  h.engine.GetTotalTokens(),
			}
			if stats, err := h.engine.MemoryStore.GetStats(); err == nil && stats != nil {
				payload["total_memories"] = stats.TotalMemories
				payload["rooms"] = stats.Rooms
			} else {
				payload["memory_enabled"] = false
			}
			_ = websocket.JSON.Send(ws, payload)

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
}
