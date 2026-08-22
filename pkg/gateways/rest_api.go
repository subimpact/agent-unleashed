package gateways

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"agent-unleashed/pkg/config"
	"agent-unleashed/pkg/engine"
)

type RESTAPIGateway struct {
	engine *engine.UnleashedEngine
	cfg    config.RESTAPIGatewayConfig
	server *http.Server
	wsHub  *WebSocketHub
}

type TriggerRequest struct {
	Prompt        string `json:"prompt"`
	SessionID     string `json:"session_id,omitempty"`
	WebhookSecret string `json:"webhook_secret,omitempty"`
}

type TriggerResponse struct {
	SessionID string `json:"session_id"`
	Response  string `json:"response"`
}

func NewRESTAPIGateway(eng *engine.UnleashedEngine, cfg config.RESTAPIGatewayConfig) *RESTAPIGateway {
	return &RESTAPIGateway{
		engine: eng,
		cfg:    cfg,
		wsHub:  NewWebSocketHub(eng),
	}
}

func (r *RESTAPIGateway) Start(ctx context.Context) error {
	mux := http.NewServeMux()

	// 1. Health check endpoints
	healthHandler := func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "online",
			"agent":  "Agent-Unleashed (agt-ul Universal Go)",
			"driver": r.engine.GetActiveDriverName(),
		})
	}
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/healthz", healthHandler)

	// 2. Real-Time WebSocket Streaming Endpoint
	mux.Handle("/ws", r.wsHub.Handler())

	// 3. Status Endpoint
	mux.HandleFunc("/api/v1/status", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		available := r.engine.Registry.ListAvailable()
		var drivers []string
		for _, a := range available {
			drivers = append(drivers, a.Name())
		}

		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"agent_name":        "Agent-Unleashed",
			"active_driver":     r.engine.GetActiveDriverName(),
			"available_drivers": drivers,
			"lcm_enabled":       r.engine.LCMEngine != nil,
			"wiki_enabled":      r.engine.WikiEngine != nil,
		})
	})

	// 4. Wiki API
	mux.HandleFunc("/api/v1/wiki", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		if r.engine.WikiEngine == nil {
			http.Error(w, `{"error": "Wiki engine disabled"}`, http.StatusNotFound)
			return
		}

		pages := r.engine.WikiEngine.ListPages()
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"total_pages": len(pages),
			"pages":       pages,
		})
	})

	// 5. LCM API
	mux.HandleFunc("/api/v1/lcm", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		if r.engine.LCMEngine == nil {
			http.Error(w, `{"error": "LCM engine disabled"}`, http.StatusNotFound)
			return
		}

		sessionID := req.URL.Query().Get("session_id")
		if sessionID == "" {
			sessionID = "default"
		}

		desc, _ := r.engine.LCMEngine.Describe(sessionID)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"session_id": sessionID,
			"summary":    desc,
		})
	})

	// 6. REST API Webhook / Chat Trigger
	chatHandler := func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if req.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		if req.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var body TriggerRequest
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			http.Error(w, "Invalid JSON body", http.StatusBadRequest)
			return
		}

		if r.cfg.WebhookSecret != "" && body.WebhookSecret != r.cfg.WebhookSecret {
			http.Error(w, "Unauthorized: Invalid webhook secret", http.StatusUnauthorized)
			return
		}

		sessionID := body.SessionID
		if sessionID == "" {
			sessionID = "webhook_default"
		}

		events := make(chan engine.Event)
		go r.engine.Chat(ctx, sessionID, body.Prompt, "webhook", events)

		var fullResp strings.Builder
		for ev := range events {
			if ev.Type == engine.EventText {
				fullResp.WriteString(ev.Content)
			}
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(TriggerResponse{
			SessionID: sessionID,
			Response:  fullResp.String(),
		})
	}

	mux.HandleFunc("/api/v1/trigger", chatHandler)
	mux.HandleFunc("/api/v1/chat", chatHandler)

	addr := fmt.Sprintf("%s:%d", r.cfg.Host, r.cfg.Port)
	r.server = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	log.Printf("[REST & WS Gateway] Listening on http://%s (WebSocket at ws://%s/ws)\n", addr, addr)

	go func() {
		<-ctx.Done()
		_ = r.server.Shutdown(context.Background())
	}()

	if err := r.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}
