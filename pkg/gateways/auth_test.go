package gateways

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func newAuth(t *testing.T, secret string, origins []string) *GatewayAuth {
	t.Helper()
	a, err := NewGatewayAuth(secret, origins, t.TempDir(), "127.0.0.1")
	if err != nil {
		t.Fatalf("NewGatewayAuth: %v", err)
	}
	return a
}

func TestTokenIsGeneratedAndPersisted(t *testing.T) {
	dir := t.TempDir()

	a, err := NewGatewayAuth("", nil, dir, "127.0.0.1")
	if err != nil {
		t.Fatalf("NewGatewayAuth: %v", err)
	}
	if len(a.Token()) < 32 {
		t.Fatalf("generated token too short: %q", a.Token())
	}

	path := filepath.Join(dir, tokenFileName)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("token file not written: %v", err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Errorf("token file mode = %v, want 0600", info.Mode().Perm())
	}

	// A restart must reuse the same token rather than locking out clients.
	b, err := NewGatewayAuth("", nil, dir, "127.0.0.1")
	if err != nil {
		t.Fatalf("second NewGatewayAuth: %v", err)
	}
	if b.Token() != a.Token() {
		t.Errorf("token not reused across restarts: %q vs %q", a.Token(), b.Token())
	}
}

func TestConfiguredSecretWins(t *testing.T) {
	dir := t.TempDir()
	a, err := NewGatewayAuth("  hunter2  ", nil, dir, "127.0.0.1")
	if err != nil {
		t.Fatalf("NewGatewayAuth: %v", err)
	}
	if a.Token() != "hunter2" {
		t.Errorf("Token() = %q, want %q", a.Token(), "hunter2")
	}
	if _, err := os.Stat(filepath.Join(dir, tokenFileName)); !os.IsNotExist(err) {
		t.Error("token file written even though a secret was configured")
	}
}

func TestCheckToken(t *testing.T) {
	a := newAuth(t, "s3cret", nil)
	if err := a.CheckToken("s3cret"); err != nil {
		t.Errorf("correct token rejected: %v", err)
	}
	for _, bad := range []string{"", "s3cre", "s3crett", "S3CRET"} {
		if err := a.CheckToken(bad); err == nil {
			t.Errorf("token %q accepted", bad)
		}
	}
}

func TestCheckOriginRefusesBrowsersByDefault(t *testing.T) {
	a := newAuth(t, "s3cret", nil)

	// No Origin header: curl, scripts, native clients.
	if err := a.CheckOrigin(""); err != nil {
		t.Errorf("empty origin rejected: %v", err)
	}
	// Any page the user has open must not be able to reach the daemon.
	for _, origin := range []string{"https://evil.example", "http://localhost:3000", "null"} {
		if err := a.CheckOrigin(origin); err == nil {
			t.Errorf("origin %q accepted with empty allowlist", origin)
		}
	}
}

func TestCheckOriginHonoursAllowlist(t *testing.T) {
	a := newAuth(t, "s3cret", []string{"http://localhost:3000/"})
	if err := a.CheckOrigin("http://localhost:3000"); err != nil {
		t.Errorf("allowlisted origin rejected: %v", err)
	}
	if err := a.CheckOrigin("http://localhost:3001"); err == nil {
		t.Error("neighbouring port accepted")
	}
}

func TestAllowedOriginNeverWildcards(t *testing.T) {
	a := newAuth(t, "s3cret", []string{"https://app.example"})
	if got := a.AllowedOrigin("https://evil.example"); got != "" {
		t.Errorf("AllowedOrigin returned %q for a disallowed origin", got)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/chat", nil)
	req.Header.Set("Origin", "https://evil.example")
	a.WriteCORS(rec, req)
	if v := rec.Header().Get("Access-Control-Allow-Origin"); v != "" {
		t.Errorf("Access-Control-Allow-Origin = %q, want unset", v)
	}
}

func TestCheckHostBlocksDNSRebinding(t *testing.T) {
	a := newAuth(t, "s3cret", nil)
	for _, host := range []string{"127.0.0.1:8080", "localhost:8080", "[::1]:8080", "localhost"} {
		if err := a.CheckHost(host); err != nil {
			t.Errorf("loopback host %q rejected: %v", host, err)
		}
	}
	if err := a.CheckHost("rebind.evil.example:8080"); err == nil {
		t.Error("rebound public hostname accepted")
	}

	// Bound to a public interface: the operator opted in, so skip the check.
	pub, err := NewGatewayAuth("s3cret", nil, t.TempDir(), "0.0.0.0")
	if err != nil {
		t.Fatalf("NewGatewayAuth: %v", err)
	}
	if err := pub.CheckHost("agent.example.com:8080"); err != nil {
		t.Errorf("public bind rejected host: %v", err)
	}
}

func TestCheckHTTPAcceptsTokenFromHeaders(t *testing.T) {
	a := newAuth(t, "s3cret", nil)

	cases := map[string]func(*http.Request){
		"bearer":        func(r *http.Request) { r.Header.Set("Authorization", "Bearer s3cret") },
		"x-agt-token":   func(r *http.Request) { r.Header.Set("X-Agt-Token", "s3cret") },
		"legacy-header": func(r *http.Request) { r.Header.Set("X-Webhook-Secret", "s3cret") },
		"query":         func(r *http.Request) { r.URL.RawQuery = "token=s3cret" },
	}
	for name, set := range cases {
		req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:8080/api/v1/chat", nil)
		set(req)
		if err := a.CheckHTTP(req, ""); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}

	// Legacy JSON body field still works.
	req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:8080/api/v1/chat", nil)
	if err := a.CheckHTTP(req, "s3cret"); err != nil {
		t.Errorf("body secret rejected: %v", err)
	}
	// And an unauthenticated request does not.
	req = httptest.NewRequest(http.MethodPost, "http://127.0.0.1:8080/api/v1/chat", nil)
	if err := a.CheckHTTP(req, ""); err == nil {
		t.Error("unauthenticated request accepted")
	}
}

// A cross-site page cannot set headers on a simple POST, but it can send one.
// Origin must stop it even before the token check.
func TestCheckHTTPRejectsCrossSitePostWithValidToken(t *testing.T) {
	a := newAuth(t, "s3cret", nil)
	req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:8080/api/v1/chat", nil)
	req.Header.Set("Origin", "https://evil.example")
	req.Header.Set("X-Agt-Token", "s3cret")
	if err := a.CheckHTTP(req, ""); err == nil {
		t.Error("cross-site request accepted despite a valid token")
	}
}
