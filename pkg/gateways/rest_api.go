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
	}
}

func (r *RESTAPIGateway) Start(ctx context.Context) error {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status": "online",
			"agent":  "Agent-Unleashed (agt-ul Universal Go)",
		})
	})

	mux.HandleFunc("/api/v1/trigger", func(w http.ResponseWriter, req *http.Request) {
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
	})

	addr := fmt.Sprintf("%s:%d", r.cfg.Host, r.cfg.Port)
	r.server = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	log.Printf("[REST API Gateway] Listening on http://%s\n", addr)

	go func() {
		<-ctx.Done()
		_ = r.server.Shutdown(context.Background())
	}()

	if err := r.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}
