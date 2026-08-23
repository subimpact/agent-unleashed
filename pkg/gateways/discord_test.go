package gateways

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/net/websocket"

	"agent-unleashed/pkg/config"
	"agent-unleashed/pkg/engine"
)

// fakeEngine stands in for UnleashedEngine so the protocol loop can be tested
// without adapters, a memory store or a real CLI.
type fakeEngine struct {
	mu      sync.Mutex
	prompts []string
	reply   string
}

func (f *fakeEngine) Chat(ctx context.Context, sessionID, userMessage, channelName string, out chan<- engine.Event) {
	defer close(out)
	f.mu.Lock()
	f.prompts = append(f.prompts, userMessage)
	reply := f.reply
	f.mu.Unlock()
	out <- engine.Event{Type: engine.EventText, Content: reply}
}

func (f *fakeEngine) seen() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.prompts...)
}

// fakeDiscord serves both halves of the API the gateway talks to: the REST
// endpoint for sending messages and the gateway WebSocket.
type fakeDiscord struct {
	server *httptest.Server

	mu       sync.Mutex
	posted   []string
	identify map[string]interface{}
	beats    int

	dispatch  chan discordPayload
	ready     chan struct{}
	readyOnce sync.Once
}

func newFakeDiscord(t *testing.T) *fakeDiscord {
	t.Helper()
	f := &fakeDiscord{
		dispatch: make(chan discordPayload, 8),
		ready:    make(chan struct{}),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v10/gateway/bot", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bot test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{
			"url": "ws://" + r.Host + "/gateway",
		})
	})
	mux.HandleFunc("/api/v10/channels/", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Content string `json:"content"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.mu.Lock()
		f.posted = append(f.posted, body.Content)
		f.mu.Unlock()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"1"}`))
	})
	mux.Handle("/gateway", websocket.Handler(f.serveGateway))

	f.server = httptest.NewServer(mux)
	t.Cleanup(f.server.Close)
	return f
}

func (f *fakeDiscord) serveGateway(ws *websocket.Conn) {
	defer ws.Close()

	_ = websocket.JSON.Send(ws, discordPayload{Op: opHello, Data: mustJSON(map[string]int{"heartbeat_interval": 50})})

	var first discordPayload
	if err := websocket.JSON.Receive(ws, &first); err != nil {
		return
	}
	if first.Op == opIdentify {
		var ident map[string]interface{}
		_ = json.Unmarshal(first.Data, &ident)
		f.mu.Lock()
		f.identify = ident
		f.mu.Unlock()
	}

	seq := int64(1)
	_ = websocket.JSON.Send(ws, discordPayload{
		Op:       opDispatch,
		Type:     "READY",
		Sequence: &seq,
		Data: mustJSON(map[string]interface{}{
			"session_id":         "sess-1",
			"resume_gateway_url": "",
			"user":               map[string]string{"id": "bot-42", "username": "agt-ul"},
		}),
	})
	f.readyOnce.Do(func() { close(f.ready) })

	// Count heartbeats on a reader goroutine while dispatching test events.
	go func() {
		for {
			var pkt discordPayload
			if err := websocket.JSON.Receive(ws, &pkt); err != nil {
				return
			}
			if pkt.Op == opHeartbeat {
				f.mu.Lock()
				f.beats++
				f.mu.Unlock()
				_ = websocket.JSON.Send(ws, discordPayload{Op: opHeartbeatACK})
			}
		}
	}()

	for pkt := range f.dispatch {
		seq++
		s := seq
		pkt.Sequence = &s
		if err := websocket.JSON.Send(ws, pkt); err != nil {
			return
		}
	}
}

func (f *fakeDiscord) sendMessage(msg map[string]interface{}) {
	f.dispatch <- discordPayload{Op: opDispatch, Type: "MESSAGE_CREATE", Data: mustJSON(msg)}
}

func (f *fakeDiscord) postedMessages() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.posted...)
}

func (f *fakeDiscord) heartbeats() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.beats
}

func startGateway(t *testing.T, f *fakeDiscord, cfg config.DiscordGatewayConfig, eng chatEngine) {
	t.Helper()
	gw := NewDiscordGateway(nil, cfg)
	gw.engine = eng
	gw.apiBase = f.server.URL + "/api/v10"
	gw.gatewayURL = "ws" + strings.TrimPrefix(f.server.URL, "http") + "/gateway"

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() { _ = gw.Start(ctx) }()

	select {
	case <-f.ready:
	case <-time.After(5 * time.Second):
		t.Fatal("gateway never completed the READY handshake")
	}
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

// End to end: connect, identify, receive a DM, run it through the engine and
// post the reply back. None of this happened before - Start() was a stub.
func TestDiscordDirectMessageRoundTrip(t *testing.T) {
	f := newFakeDiscord(t)
	eng := &fakeEngine{reply: "the adapter registry maps names onto drivers"}

	startGateway(t, f, config.DiscordGatewayConfig{
		BotToken:          "test-token",
		AllowedChannelIDs: []int64{555},
	}, eng)

	f.sendMessage(map[string]interface{}{
		"id":         "m1",
		"channel_id": "555",
		"content":    "how does the adapter registry work",
		"author":     map[string]interface{}{"id": "user-1", "bot": false},
	})

	waitFor(t, "the engine to receive the prompt", func() bool { return len(eng.seen()) > 0 })
	if got := eng.seen()[0]; got != "how does the adapter registry work" {
		t.Errorf("engine saw %q", got)
	}

	waitFor(t, "a reply to be posted", func() bool { return len(f.postedMessages()) > 0 })
	if got := f.postedMessages()[0]; got != eng.reply {
		t.Errorf("posted %q, want %q", got, eng.reply)
	}
}

func TestDiscordIdentifyRequestsMessageContentIntent(t *testing.T) {
	f := newFakeDiscord(t)
	startGateway(t, f, config.DiscordGatewayConfig{
		BotToken:          "test-token",
		AllowedChannelIDs: []int64{555},
	}, &fakeEngine{reply: "ok"})

	f.mu.Lock()
	ident := f.identify
	f.mu.Unlock()

	if ident == nil {
		t.Fatal("no IDENTIFY was sent")
	}
	if ident["token"] != "test-token" {
		t.Errorf("IDENTIFY token = %v", ident["token"])
	}
	intents, ok := ident["intents"].(float64)
	if !ok {
		t.Fatalf("IDENTIFY intents missing: %v", ident["intents"])
	}
	if int(intents)&(1<<15) == 0 {
		t.Error("MESSAGE_CONTENT intent not requested; the bot would receive empty content")
	}
	if int(intents)&(1<<12) == 0 {
		t.Error("DIRECT_MESSAGES intent not requested")
	}
}

func TestDiscordHeartbeats(t *testing.T) {
	f := newFakeDiscord(t)
	startGateway(t, f, config.DiscordGatewayConfig{
		BotToken:          "test-token",
		AllowedChannelIDs: []int64{555},
	}, &fakeEngine{reply: "ok"})

	// The fake advertises a 50ms interval, so several should land quickly.
	waitFor(t, "heartbeats", func() bool { return f.heartbeats() >= 2 })
}

// Fail closed, exactly like Telegram.
func TestDiscordRefusesUnlistedChannel(t *testing.T) {
	f := newFakeDiscord(t)
	eng := &fakeEngine{reply: "should never run"}

	startGateway(t, f, config.DiscordGatewayConfig{
		BotToken:          "test-token",
		AllowedChannelIDs: []int64{555},
	}, eng)

	f.sendMessage(map[string]interface{}{
		"id":         "m1",
		"channel_id": "999",
		"content":    "run rm -rf /",
		"author":     map[string]interface{}{"id": "user-1", "bot": false},
	})

	waitFor(t, "the refusal notice", func() bool { return len(f.postedMessages()) > 0 })
	if !strings.Contains(f.postedMessages()[0], "Unauthorized") {
		t.Errorf("expected a refusal, got %q", f.postedMessages()[0])
	}
	if len(eng.seen()) != 0 {
		t.Errorf("engine ran for an unlisted channel: %v", eng.seen())
	}
}

func TestDiscordAllowlistFailsClosed(t *testing.T) {
	gw := NewDiscordGateway(nil, config.DiscordGatewayConfig{BotToken: "t"})
	if gw.isAllowed("123", "456") {
		t.Error("message accepted with no guild_ids and no allowed_channel_ids")
	}
	if gw.hasAllowlist() {
		t.Error("hasAllowlist true with nothing configured")
	}

	gw = NewDiscordGateway(nil, config.DiscordGatewayConfig{GuildIDs: []int64{123}})
	if !gw.isAllowed("123", "456") {
		t.Error("guild allowlist not honoured")
	}
	if gw.isAllowed("789", "456") {
		t.Error("wrong guild accepted")
	}
}

// A bot answering a bot is how a message loop starts.
func TestDiscordIgnoresBotsAndItself(t *testing.T) {
	f := newFakeDiscord(t)
	eng := &fakeEngine{reply: "ok"}
	startGateway(t, f, config.DiscordGatewayConfig{
		BotToken:          "test-token",
		AllowedChannelIDs: []int64{555},
	}, eng)

	f.sendMessage(map[string]interface{}{
		"id": "m1", "channel_id": "555", "content": "hello",
		"author": map[string]interface{}{"id": "other-bot", "bot": true},
	})
	f.sendMessage(map[string]interface{}{
		"id": "m2", "channel_id": "555", "content": "echo",
		"author": map[string]interface{}{"id": "bot-42", "bot": false},
	})
	// A real message afterwards proves the two above were processed and skipped.
	f.sendMessage(map[string]interface{}{
		"id": "m3", "channel_id": "555", "content": "genuine question",
		"author": map[string]interface{}{"id": "user-1", "bot": false},
	})

	waitFor(t, "the human message", func() bool { return len(eng.seen()) > 0 })
	seen := eng.seen()
	if len(seen) != 1 || seen[0] != "genuine question" {
		t.Errorf("engine saw %v, want only the human message", seen)
	}
}

// In a guild the bot must only answer when addressed, and the mention is
// stripped before the prompt reaches the engine.
func TestDiscordGuildRequiresMention(t *testing.T) {
	f := newFakeDiscord(t)
	eng := &fakeEngine{reply: "ok"}
	startGateway(t, f, config.DiscordGatewayConfig{
		BotToken: "test-token",
		GuildIDs: []int64{777},
	}, eng)

	f.sendMessage(map[string]interface{}{
		"id": "m1", "channel_id": "555", "guild_id": "777",
		"content": "just chatting in the channel",
		"author":  map[string]interface{}{"id": "user-1", "bot": false},
	})
	f.sendMessage(map[string]interface{}{
		"id": "m2", "channel_id": "555", "guild_id": "777",
		"content":  "<@bot-42> summarise the repo",
		"author":   map[string]interface{}{"id": "user-1", "bot": false},
		"mentions": []map[string]string{{"id": "bot-42"}},
	})

	waitFor(t, "the mentioned message", func() bool { return len(eng.seen()) > 0 })
	seen := eng.seen()
	if len(seen) != 1 {
		t.Fatalf("engine saw %v, want only the mention", seen)
	}
	if seen[0] != "summarise the repo" {
		t.Errorf("mention was not stripped: %q", seen[0])
	}
}

func TestChunkMessage(t *testing.T) {
	if got := chunkMessage("", 100); len(got) != 1 || got[0] != "(empty response)" {
		t.Errorf("empty content = %v", got)
	}
	if got := chunkMessage("short", 100); len(got) != 1 || got[0] != "short" {
		t.Errorf("short content = %v", got)
	}

	// Long replies are split, not truncated.
	long := strings.Repeat("abcde\n", 500)
	chunks := chunkMessage(long, 100)
	if len(chunks) < 2 {
		t.Fatalf("long content produced %d chunk(s)", len(chunks))
	}
	for i, c := range chunks {
		if len([]rune(c)) > 100 {
			t.Errorf("chunk %d has %d runes, over the limit", i, len([]rune(c)))
		}
	}

	// Multi-byte characters must not be sliced in half.
	emoji := strings.Repeat("🚀", 300)
	for _, c := range chunkMessage(emoji, 100) {
		if !isValidUTF8(c) {
			t.Error("chunking produced invalid UTF-8")
		}
	}
}

func isValidUTF8(s string) bool {
	for _, r := range s {
		if r == '�' {
			return false
		}
	}
	return true
}
