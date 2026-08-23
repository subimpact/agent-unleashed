package gateways

import (
	"testing"

	"agent-unleashed/pkg/config"
)

// An empty allowlist used to mean "allow everyone", which exposed a
// permission-bypassed coding agent to anyone who found the bot.
func TestTelegramAllowlistFailsClosed(t *testing.T) {
	gw := NewTelegramGateway(nil, config.TelegramGatewayConfig{})
	if gw.isAllowed(12345) {
		t.Error("chat accepted with no allowed_chat_ids and no admin_chat_id")
	}
}

func TestTelegramAllowlist(t *testing.T) {
	gw := NewTelegramGateway(nil, config.TelegramGatewayConfig{
		AllowedChatIDs: []int64{111, 222},
		AdminChatID:    999,
	})

	for _, id := range []int64{111, 222, 999} {
		if !gw.isAllowed(id) {
			t.Errorf("chat %d refused but should be allowed", id)
		}
	}
	for _, id := range []int64{0, 333, -111} {
		if gw.isAllowed(id) {
			t.Errorf("chat %d allowed but should be refused", id)
		}
	}
}

// admin_chat_id defaults to 0; that must not become a wildcard for callers
// whose chat ID somehow resolves to zero.
func TestTelegramZeroAdminIsNotAWildcard(t *testing.T) {
	gw := NewTelegramGateway(nil, config.TelegramGatewayConfig{AllowedChatIDs: []int64{111}})
	if gw.isAllowed(0) {
		t.Error("chat 0 allowed via unset admin_chat_id")
	}
}
