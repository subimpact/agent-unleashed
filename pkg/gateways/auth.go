package gateways

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// GatewayAuth guards the locally-bound HTTP/WebSocket surface. A daemon on
// 127.0.0.1 is not private: any page the user happens to have open can reach it
// with fetch() or a WebSocket. Since every request here can run a coding agent
// against the workspace, three independent checks have to pass.
//
//  1. Token      - shared secret, generated on first run if none is configured.
//  2. Origin     - blocks cross-site requests from a browser outright.
//  3. Host       - blocks DNS-rebinding a public name onto the loopback bind.
type GatewayAuth struct {
	secret         string
	generated      bool
	tokenPath      string
	allowedOrigins []string
	boundHost      string
}

const tokenFileName = "rest_api_token"

// NewGatewayAuth resolves the gateway secret. An explicitly configured secret
// always wins; otherwise the token persisted under dataDir is reused, and only
// if that is missing too is a fresh one minted. Config is never rewritten.
func NewGatewayAuth(secret string, allowedOrigins []string, dataDir, boundHost string) (*GatewayAuth, error) {
	if dataDir == "" {
		dataDir = "./data"
	}
	a := &GatewayAuth{
		tokenPath:      filepath.Join(dataDir, tokenFileName),
		allowedOrigins: normalizeOrigins(allowedOrigins),
		boundHost:      boundHost,
	}

	if strings.TrimSpace(secret) != "" {
		a.secret = strings.TrimSpace(secret)
		return a, nil
	}

	if existing, err := os.ReadFile(a.tokenPath); err == nil {
		if tok := strings.TrimSpace(string(existing)); tok != "" {
			a.secret = tok
			a.generated = true
			return a, nil
		}
	}

	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return nil, fmt.Errorf("could not generate gateway token: %w", err)
	}
	a.secret = hex.EncodeToString(buf)
	a.generated = true

	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("could not create data dir for gateway token: %w", err)
	}
	if err := os.WriteFile(a.tokenPath, []byte(a.secret+"\n"), 0o600); err != nil {
		return nil, fmt.Errorf("could not persist gateway token: %w", err)
	}
	return a, nil
}

// Banner describes how a client should authenticate, for the startup log.
func (a *GatewayAuth) Banner() string {
	if a.generated {
		return fmt.Sprintf("auth: token required (auto-generated, stored in %s)", a.tokenPath)
	}
	return "auth: token required (gateways.rest_api.webhook_secret)"
}

// Token returns the active shared secret.
func (a *GatewayAuth) Token() string { return a.secret }

// CheckHTTP runs all three checks against an incoming request. bodySecret is
// the legacy webhook_secret field carried in the JSON body, still accepted so
// existing webhook callers keep working.
func (a *GatewayAuth) CheckHTTP(req *http.Request, bodySecret string) error {
	if err := a.CheckHost(req.Host); err != nil {
		return err
	}
	if err := a.CheckOrigin(req.Header.Get("Origin")); err != nil {
		return err
	}
	return a.CheckToken(firstNonEmpty(tokenFromRequest(req), bodySecret))
}

// CheckToken compares a presented token against the secret in constant time.
func (a *GatewayAuth) CheckToken(presented string) error {
	if subtle.ConstantTimeCompare([]byte(presented), []byte(a.secret)) != 1 {
		return fmt.Errorf("invalid or missing gateway token")
	}
	return nil
}

// CheckOrigin allows requests that carry no Origin header - curl, scripts and
// native clients never send one - and otherwise requires an explicit match.
// With no allowlist configured, every browser origin is refused, which is the
// intended default for a machine-local agent daemon.
func (a *GatewayAuth) CheckOrigin(origin string) error {
	if origin == "" {
		return nil
	}
	normalized := normalizeOrigin(origin)
	for _, allowed := range a.allowedOrigins {
		if allowed == normalized {
			return nil
		}
	}
	return fmt.Errorf("origin %q is not allowed", origin)
}

// AllowedOrigin echoes an origin back only when it is on the allowlist, so no
// response ever carries a wildcard Access-Control-Allow-Origin.
func (a *GatewayAuth) AllowedOrigin(origin string) string {
	if origin == "" || a.CheckOrigin(origin) != nil {
		return ""
	}
	return origin
}

// CheckHost rejects a Host header that does not name the loopback interface,
// which is how a rebound DNS record reaches a 127.0.0.1 listener. When the
// daemon is deliberately bound to a public interface the check is skipped.
func (a *GatewayAuth) CheckHost(host string) error {
	if a.boundHost != "127.0.0.1" && a.boundHost != "localhost" && a.boundHost != "::1" {
		return nil
	}
	name := host
	if h, _, err := net.SplitHostPort(host); err == nil {
		name = h
	}
	switch strings.ToLower(strings.Trim(name, "[]")) {
	case "localhost", "127.0.0.1", "::1", "":
		return nil
	}
	if ip := net.ParseIP(strings.Trim(name, "[]")); ip != nil && ip.IsLoopback() {
		return nil
	}
	return fmt.Errorf("host %q is not a loopback address", host)
}

// WriteCORS sets the response headers for an allowed origin only.
func (a *GatewayAuth) WriteCORS(w http.ResponseWriter, req *http.Request) {
	if origin := a.AllowedOrigin(req.Header.Get("Origin")); origin != "" {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Agt-Token")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Vary", "Origin")
	}
}

// tokenFromRequest reads the token from a header, or from the query string for
// browser WebSocket clients, which cannot set headers on the handshake.
func tokenFromRequest(req *http.Request) string {
	if auth := req.Header.Get("Authorization"); auth != "" {
		if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
			return strings.TrimSpace(auth[7:])
		}
		return strings.TrimSpace(auth)
	}
	if tok := req.Header.Get("X-Agt-Token"); tok != "" {
		return strings.TrimSpace(tok)
	}
	if tok := req.Header.Get("X-Webhook-Secret"); tok != "" {
		return strings.TrimSpace(tok)
	}
	if req.URL != nil {
		return strings.TrimSpace(req.URL.Query().Get("token"))
	}
	return ""
}

func normalizeOrigins(origins []string) []string {
	var out []string
	for _, o := range origins {
		if n := normalizeOrigin(o); n != "" {
			out = append(out, n)
		}
	}
	return out
}

func normalizeOrigin(origin string) string {
	o := strings.TrimSpace(strings.ToLower(origin))
	o = strings.TrimSuffix(o, "/")
	if u, err := url.Parse(o); err == nil && u.Host != "" {
		return u.Scheme + "://" + u.Host
	}
	return o
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
