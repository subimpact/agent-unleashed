package gateways

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/websocket"

	"agent-unleashed/pkg/config"
	"agent-unleashed/pkg/engine"
)

// Discord Gateway opcodes (v10).
const (
	opDispatch            = 0
	opHeartbeat           = 1
	opIdentify            = 2
	opResume              = 6
	opReconnect           = 7
	opInvalidSession      = 9
	opHello               = 10
	opHeartbeatACK        = 11
	discordAPIVersion     = "10"
	discordMaxMessageRune = 1900
)

// discordIntents: GUILDS | GUILD_MESSAGES | DIRECT_MESSAGES | MESSAGE_CONTENT.
// MESSAGE_CONTENT is privileged - enable it under Bot > Privileged Gateway
// Intents in the Discord developer portal or the bot receives empty content.
const discordIntents = (1 << 0) | (1 << 9) | (1 << 12) | (1 << 15)

// chatEngine is the slice of the engine the gateway needs, so the protocol loop
// can be exercised in tests without standing up adapters and a memory store.
type chatEngine interface {
	Chat(ctx context.Context, sessionID, userMessage, channelName string, out chan<- engine.Event)
}

type DiscordGateway struct {
	engine chatEngine
	cfg    config.DiscordGatewayConfig
	client *http.Client

	// apiBase and gatewayURL are overridden by tests to point at a local server.
	apiBase    string
	gatewayURL string

	mu sync.Mutex
	// writeMu serialises socket writes: the heartbeat timer and the dispatch
	// loop both send, and a websocket.Conn has no internal write lock.
	writeMu   sync.Mutex
	conn      *websocket.Conn
	sequence  int64
	sessionID string
	resumeURL string
	selfID    string
}

func NewDiscordGateway(eng *engine.UnleashedEngine, cfg config.DiscordGatewayConfig) *DiscordGateway {
	return &DiscordGateway{
		engine:  eng,
		cfg:     cfg,
		client:  &http.Client{Timeout: 30 * time.Second},
		apiBase: "https://discord.com/api/v" + discordAPIVersion,
	}
}

// --- outbound ---------------------------------------------------------------

func (d *DiscordGateway) SendMessage(channelID string, content string) error {
	for _, chunk := range chunkMessage(content, discordMaxMessageRune) {
		if err := d.postMessage(channelID, chunk); err != nil {
			return err
		}
	}
	return nil
}

func (d *DiscordGateway) postMessage(channelID, content string) error {
	url := fmt.Sprintf("%s/channels/%s/messages", d.apiBase, channelID)
	body, _ := json.Marshal(map[string]string{"content": content})

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bot "+d.cfg.BotToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := d.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode == http.StatusTooManyRequests {
		return fmt.Errorf("discord rate limited (429)")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("discord API error: status %d", resp.StatusCode)
	}
	return nil
}

// chunkMessage splits on rune boundaries, preferring a line break, so a reply
// longer than Discord's limit arrives whole instead of being truncated - and
// never with a multi-byte character sliced in half.
func chunkMessage(content string, limit int) []string {
	runes := []rune(content)
	if len(runes) == 0 {
		return []string{"(empty response)"}
	}
	if len(runes) <= limit {
		return []string{content}
	}

	var out []string
	for len(runes) > limit {
		cut := limit
		for i := limit; i > limit/2; i-- {
			if runes[i] == '\n' {
				cut = i
				break
			}
		}
		out = append(out, strings.TrimRight(string(runes[:cut]), "\n"))
		runes = runes[cut:]
		for len(runes) > 0 && runes[0] == '\n' {
			runes = runes[1:]
		}
	}
	if len(runes) > 0 {
		out = append(out, string(runes))
	}
	return out
}

// --- access control ---------------------------------------------------------

// isAllowed fails closed, matching the Telegram gateway: with no guilds and no
// channels configured the bot answers nobody, rather than handing a
// permission-bypassed coding agent to any server it happens to be invited to.
func (d *DiscordGateway) isAllowed(guildID, channelID string) bool {
	for _, id := range d.cfg.AllowedChannelIDs {
		if strconv.FormatInt(id, 10) == channelID {
			return true
		}
	}
	if guildID != "" {
		for _, id := range d.cfg.GuildIDs {
			if strconv.FormatInt(id, 10) == guildID {
				return true
			}
		}
	}
	return false
}

func (d *DiscordGateway) hasAllowlist() bool {
	return len(d.cfg.AllowedChannelIDs) > 0 || len(d.cfg.GuildIDs) > 0
}
