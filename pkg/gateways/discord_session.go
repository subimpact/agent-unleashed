package gateways

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/websocket"

	"agent-unleashed/pkg/engine"
)

type discordPayload struct {
	Op       int             `json:"op"`
	Data     json.RawMessage `json:"d,omitempty"`
	Sequence *int64          `json:"s,omitempty"`
	Type     string          `json:"t,omitempty"`
}

type discordHello struct {
	HeartbeatInterval int `json:"heartbeat_interval"`
}

type discordReady struct {
	SessionID        string `json:"session_id"`
	ResumeGatewayURL string `json:"resume_gateway_url"`
	User             struct {
		ID       string `json:"id"`
		Username string `json:"username"`
	} `json:"user"`
}

type discordMessage struct {
	ID        string `json:"id"`
	ChannelID string `json:"channel_id"`
	GuildID   string `json:"guild_id"`
	Content   string `json:"content"`
	Author    struct {
		ID       string `json:"id"`
		Bot      bool   `json:"bot"`
		Username string `json:"username"`
	} `json:"author"`
	Mentions []struct {
		ID string `json:"id"`
	} `json:"mentions"`
}

// Start runs the Discord gateway. This previously logged one line and blocked
// on ctx.Done() forever: the bot never connected, never received a message and
// never replied, while the docs described DM pair programming.
func (d *DiscordGateway) Start(ctx context.Context) error {
	if d.cfg.BotToken == "" {
		return nil
	}
	if !d.hasAllowlist() {
		log.Println("[Discord Gateway] WARNING: no guild_ids and no allowed_channel_ids configured - every message will be refused. Add the channel or guild ID to config.yaml.")
	}

	backoff := time.Second
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		err := d.runSession(ctx)
		if ctx.Err() != nil {
			return nil
		}
		if err != nil {
			log.Printf("[Discord Gateway] Session ended: %v (reconnecting in %s)\n", err, backoff)
		}

		select {
		case <-ctx.Done():
			return nil
		case <-time.After(backoff):
		}
		if backoff < 60*time.Second {
			backoff *= 2
		}
	}
}

// resolveGatewayURL asks Discord where to connect. A resume URL from a previous
// session takes priority, as the protocol requires.
func (d *DiscordGateway) resolveGatewayURL() (string, error) {
	d.mu.Lock()
	override, resume := d.gatewayURL, d.resumeURL
	d.mu.Unlock()

	if resume != "" {
		return withGatewayQuery(resume), nil
	}
	if override != "" {
		return withGatewayQuery(override), nil
	}

	req, err := http.NewRequest("GET", d.apiBase+"/gateway/bot", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bot "+d.cfg.BotToken)

	resp, err := d.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return "", fmt.Errorf("discord rejected the bot token (401) - check gateways.discord.bot_token")
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("gateway lookup failed: status %d", resp.StatusCode)
	}

	var body struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", err
	}
	if body.URL == "" {
		return "", fmt.Errorf("gateway lookup returned no url")
	}
	return withGatewayQuery(body.URL), nil
}

// withGatewayQuery adds the API version and encoding without mangling the
// path. Discord hands back a bare host ("wss://gateway.discord.gg"), while a
// resume URL or a test server may carry a path, so string concatenation is not
// safe here.
func withGatewayQuery(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	q := u.Query()
	if q.Get("v") == "" {
		q.Set("v", discordAPIVersion)
	}
	if q.Get("encoding") == "" {
		q.Set("encoding", "json")
	}
	u.RawQuery = q.Encode()
	if u.Path == "" {
		u.Path = "/"
	}
	return u.String()
}

func stripMention(content, selfID string) string {
	if selfID != "" {
		for _, form := range []string{"<@" + selfID + ">", "<@!" + selfID + ">"} {
			content = strings.ReplaceAll(content, form, " ")
		}
	}
	return strings.TrimSpace(content)
}

func mustJSON(v interface{}) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage("null")
	}
	return b
}

func (d *DiscordGateway) clearResume() {
	d.mu.Lock()
	d.sessionID = ""
	d.resumeURL = ""
	d.sequence = 0
	d.mu.Unlock()
}

func (d *DiscordGateway) runSession(ctx context.Context) error {
	gatewayURL, err := d.resolveGatewayURL()
	if err != nil {
		return err
	}

	wsCfg, err := websocket.NewConfig(gatewayURL, "https://discord.com")
	if err != nil {
		return err
	}
	conn, err := websocket.DialConfig(wsCfg)
	if err != nil {
		d.clearResume()
		return fmt.Errorf("dial %s: %w", gatewayURL, err)
	}
	defer conn.Close()

	d.mu.Lock()
	d.conn = conn
	d.mu.Unlock()

	// 1. HELLO carries the heartbeat interval.
	var hello discordPayload
	if err := websocket.JSON.Receive(conn, &hello); err != nil {
		return fmt.Errorf("waiting for HELLO: %w", err)
	}
	if hello.Op != opHello {
		return fmt.Errorf("expected HELLO (op %d), got op %d", opHello, hello.Op)
	}
	var helloData discordHello
	if err := json.Unmarshal(hello.Data, &helloData); err != nil || helloData.HeartbeatInterval <= 0 {
		return fmt.Errorf("malformed HELLO payload")
	}

	// 2. Resume where we can, otherwise identify afresh.
	d.mu.Lock()
	sessionID, seq := d.sessionID, d.sequence
	d.mu.Unlock()

	if sessionID != "" {
		err = d.send(discordPayload{Op: opResume, Data: mustJSON(map[string]interface{}{
			"token":      d.cfg.BotToken,
			"session_id": sessionID,
			"seq":        seq,
		})})
	} else {
		err = d.send(discordPayload{Op: opIdentify, Data: mustJSON(map[string]interface{}{
			"token":   d.cfg.BotToken,
			"intents": discordIntents,
			"properties": map[string]string{
				"os":      "linux",
				"browser": "agent-unleashed",
				"device":  "agt-ul",
			},
		})})
	}
	if err != nil {
		return fmt.Errorf("identify/resume: %w", err)
	}

	sessionCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	go d.heartbeatLoop(sessionCtx, time.Duration(helloData.HeartbeatInterval)*time.Millisecond)

	// Close the socket on shutdown so the blocking read below actually returns.
	go func() {
		<-sessionCtx.Done()
		_ = conn.Close()
	}()

	for {
		var pkt discordPayload
		if err := websocket.JSON.Receive(conn, &pkt); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("read: %w", err)
		}

		if pkt.Sequence != nil {
			d.mu.Lock()
			d.sequence = *pkt.Sequence
			d.mu.Unlock()
		}

		switch pkt.Op {
		case opDispatch:
			d.handleDispatch(sessionCtx, pkt)

		case opHeartbeat:
			_ = d.sendHeartbeat()

		case opHeartbeatACK:
			// nothing to do

		case opReconnect:
			return fmt.Errorf("gateway asked us to reconnect")

		case opInvalidSession:
			var resumable bool
			_ = json.Unmarshal(pkt.Data, &resumable)
			if !resumable {
				d.clearResume()
			}
			return fmt.Errorf("gateway invalidated the session (resumable=%v)", resumable)
		}
	}
}

func (d *DiscordGateway) handleDispatch(ctx context.Context, pkt discordPayload) {
	switch pkt.Type {
	case "READY":
		var ready discordReady
		if err := json.Unmarshal(pkt.Data, &ready); err != nil {
			return
		}
		d.mu.Lock()
		d.sessionID = ready.SessionID
		d.resumeURL = ready.ResumeGatewayURL
		d.selfID = ready.User.ID
		d.mu.Unlock()
		log.Printf("[Discord Gateway] Connected as %s (session %s)\n", ready.User.Username, ready.SessionID)

	case "RESUMED":
		log.Println("[Discord Gateway] Session resumed.")

	case "MESSAGE_CREATE":
		var msg discordMessage
		if err := json.Unmarshal(pkt.Data, &msg); err != nil {
			return
		}
		d.handleMessage(ctx, msg)
	}
}

func (d *DiscordGateway) handleMessage(ctx context.Context, msg discordMessage) {
	d.mu.Lock()
	selfID := d.selfID
	d.mu.Unlock()

	// Never answer ourselves or another bot - that is how loops start.
	if msg.Author.Bot || (selfID != "" && msg.Author.ID == selfID) {
		return
	}
	if strings.TrimSpace(msg.Content) == "" {
		return
	}

	// In a guild, only respond when actually addressed.
	if msg.GuildID != "" {
		mentioned := false
		for _, m := range msg.Mentions {
			if selfID != "" && m.ID == selfID {
				mentioned = true
				break
			}
		}
		if !mentioned {
			return
		}
	}

	if !d.isAllowed(msg.GuildID, msg.ChannelID) {
		log.Printf("[Discord Gateway] Refused message from unlisted channel %s (guild %q)\n", msg.ChannelID, msg.GuildID)
		_ = d.SendMessage(msg.ChannelID, fmt.Sprintf(
			"Unauthorized. To grant access, add this channel ID to gateways.discord.allowed_channel_ids in config.yaml:\n%s", msg.ChannelID))
		return
	}

	prompt := stripMention(msg.Content, selfID)
	if prompt == "" {
		return
	}

	sessionID := "discord_" + msg.ChannelID
	go func() {
		events := make(chan engine.Event)
		go d.engine.Chat(ctx, sessionID, prompt, "discord", events)

		var reply strings.Builder
		for ev := range events {
			if ev.Type == engine.EventText {
				reply.WriteString(ev.Content)
			}
		}

		text := strings.TrimSpace(reply.String())
		if text == "" {
			text = "Task executed."
		}
		if err := d.SendMessage(msg.ChannelID, text); err != nil {
			log.Printf("[Discord Gateway] Could not reply in %s: %v\n", msg.ChannelID, err)
		}
	}()
}

func (d *DiscordGateway) heartbeatLoop(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		return
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := d.sendHeartbeat(); err != nil {
				return
			}
		}
	}
}

func (d *DiscordGateway) sendHeartbeat() error {
	d.mu.Lock()
	seq := d.sequence
	d.mu.Unlock()

	if seq == 0 {
		return d.send(discordPayload{Op: opHeartbeat, Data: json.RawMessage("null")})
	}
	return d.send(discordPayload{Op: opHeartbeat, Data: mustJSON(seq)})
}

// send serialises writes: the heartbeat timer and the dispatch loop both write
// to the socket, and a websocket.Conn is not safe for concurrent writers.
func (d *DiscordGateway) send(p discordPayload) error {
	d.mu.Lock()
	conn := d.conn
	d.mu.Unlock()
	if conn == nil {
		return fmt.Errorf("not connected")
	}

	d.writeMu.Lock()
	defer d.writeMu.Unlock()
	return websocket.JSON.Send(conn, p)
}
