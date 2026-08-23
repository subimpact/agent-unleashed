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
	auth   *GatewayAuth
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

func NewRESTAPIGateway(eng *engine.UnleashedEngine, cfg config.RESTAPIGatewayConfig, dataDir string) (*RESTAPIGateway, error) {
	auth, err := NewGatewayAuth(cfg.WebhookSecret, cfg.AllowedOrigins, dataDir, cfg.Host)
	if err != nil {
		return nil, err
	}
	return &RESTAPIGateway{
		engine: eng,
		cfg:    cfg,
		auth:   auth,
		wsHub:  NewWebSocketHub(eng, auth),
	}, nil
}

// guard wraps a handler with the full origin/host/token check. Every endpoint
// goes through it: even the read-only ones expose the workspace's driver and
// knowledge-base contents to whoever can reach them.
func (r *RESTAPIGateway) guard(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		r.auth.WriteCORS(w, req)

		if req.Method == http.MethodOptions {
			// Only a pre-approved origin gets a usable preflight response.
			if r.auth.AllowedOrigin(req.Header.Get("Origin")) == "" {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
			w.WriteHeader(http.StatusOK)
			return
		}

		// Host and Origin are settled before the body is touched, so an
		// unauthorised caller gets 403 rather than a parser error that would
		// tell it the endpoint exists.
		if err := r.auth.CheckHost(req.Host); err != nil {
			r.reject(w, req, err)
			return
		}
		if err := r.auth.CheckOrigin(req.Header.Get("Origin")); err != nil {
			r.reject(w, req, err)
			return
		}

		var bodySecret string
		if req.Method == http.MethodPost && req.Body != nil {
			// Decode once here so the token can travel in the legacy body field,
			// then hand the parsed request down via the context.
			var body TriggerRequest
			if err := json.NewDecoder(http.MaxBytesReader(w, req.Body, 1<<20)).Decode(&body); err != nil {
				http.Error(w, "Invalid JSON body", http.StatusBadRequest)
				return
			}
			bodySecret = body.WebhookSecret
			req = req.WithContext(context.WithValue(req.Context(), triggerBodyKey{}, body))
		}

		if err := r.auth.CheckToken(firstNonEmpty(tokenFromRequest(req), bodySecret)); err != nil {
			r.reject(w, req, err)
			return
		}

		h(w, req)
	}
}

type triggerBodyKey struct{}

func (r *RESTAPIGateway) reject(w http.ResponseWriter, req *http.Request, err error) {
	log.Printf("[REST Gateway] Rejected %s %s from %s: %v\n", req.Method, req.URL.Path, req.RemoteAddr, err)
	http.Error(w, "Forbidden", http.StatusForbidden)
}

func (r *RESTAPIGateway) Start(ctx context.Context) error {
	mux := http.NewServeMux()

	// 1. Health check endpoints. Unauthenticated on purpose so process
	//    supervisors can probe them, and deliberately free of any detail
	//    about the workspace, driver or configuration.
	healthHandler := func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "online",
			"agent":  "Agent-Unleashed (agt-ul Universal Go)",
		})
	}
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/healthz", healthHandler)

	// 2. Real-Time WebSocket Streaming Endpoint (authenticated in its handshake)
	mux.Handle("/ws", r.wsHub.Handler())

	// 3. Status Endpoint
	mux.HandleFunc("/api/v1/status", r.guard(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")

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
	}))

	// 4. Wiki API
	mux.HandleFunc("/api/v1/wiki", r.guard(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.engine.WikiEngine == nil {
			http.Error(w, `{"error": "Wiki engine disabled"}`, http.StatusNotFound)
			return
		}

		pages := r.engine.WikiEngine.ListPages()
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"total_pages": len(pages),
			"pages":       pages,
		})
	}))

	// 5. LCM API
	mux.HandleFunc("/api/v1/lcm", r.guard(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")

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
	}))

	// 6. REST API Webhook / Chat Trigger
	chatHandler := r.guard(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		body, ok := req.Context().Value(triggerBodyKey{}).(TriggerRequest)
		if !ok {
			http.Error(w, "Invalid JSON body", http.StatusBadRequest)
			return
		}

		sessionID := body.SessionID
		if sessionID == "" {
			sessionID = "webhook_default"
		}

		// Tie the run to the request so a disconnecting client cancels the
		// underlying CLI subprocess instead of orphaning it.
		runCtx, cancel := context.WithCancel(req.Context())
		defer cancel()

		events := make(chan engine.Event)
		go r.engine.Chat(runCtx, sessionID, body.Prompt, "webhook", events)

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

	mux.HandleFunc("/api/v1/trigger", chatHandler)
	mux.HandleFunc("/api/v1/chat", chatHandler)

	addr := fmt.Sprintf("%s:%d", r.cfg.Host, r.cfg.Port)
	r.server = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	log.Printf("[REST & WS Gateway] Listening on http://%s (WebSocket at ws://%s/ws) - %s\n", addr, addr, r.auth.Banner())

	go func() {
		<-ctx.Done()
		_ = r.server.Shutdown(context.Background())
	}()

	if err := r.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}
