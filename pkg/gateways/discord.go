package gateways

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"agent-unleashed/pkg/config"
	"agent-unleashed/pkg/engine"
)

type DiscordGateway struct {
	engine *engine.UnleashedEngine
	cfg    config.DiscordGatewayConfig
	client *http.Client
}

func NewDiscordGateway(eng *engine.UnleashedEngine, cfg config.DiscordGatewayConfig) *DiscordGateway {
	return &DiscordGateway{
		engine: eng,
		cfg:    cfg,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (d *DiscordGateway) SendMessage(channelID string, content string) error {
	if len(content) > 1950 {
		content = content[:1940] + "\n...(truncated)"
	}

	url := fmt.Sprintf("https://discord.com/api/v10/channels/%s/messages", channelID)
	body, _ := json.Marshal(map[string]string{
		"content": content,
	})

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

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("discord API error: status %d", resp.StatusCode)
	}
	return nil
}

func (d *DiscordGateway) Start(ctx context.Context) error {
	if d.cfg.BotToken == "" {
		return nil
	}

	log.Println("[Discord Gateway] Discord Bot gateway initialized and active.")

	// Keep alive
	<-ctx.Done()
	return nil
}
