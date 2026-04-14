package platforms

import (
	"context"
	"testing"

	"github.com/gotd/td/tg"

	"github.com/odysseythink/hermind/gateway"
)

func TestTelegramUserName(t *testing.T) {
	tu := NewTelegramUser(TelegramUserConfig{})
	if tu.Name() != "telegram_user" {
		t.Errorf("Name() = %q", tu.Name())
	}
}

func TestTelegramUserMissingConfig(t *testing.T) {
	tu := NewTelegramUser(TelegramUserConfig{})
	err := tu.Run(context.Background(), func(_ context.Context, _ gateway.IncomingMessage) (*gateway.OutgoingMessage, error) {
		return nil, nil
	})
	if err == nil {
		t.Error("expected error for missing config")
	}
}

func TestTelegramUserSendReplyNotConnected(t *testing.T) {
	tu := NewTelegramUser(TelegramUserConfig{})
	err := tu.SendReply(context.Background(), gateway.OutgoingMessage{ChatID: "123", Text: "hi"})
	if err == nil {
		t.Error("expected error when not connected")
	}
}

func TestTelegramUserPeerID(t *testing.T) {
	tu := NewTelegramUser(TelegramUserConfig{})
	msg := &tg.Message{
		FromID: &tg.PeerUser{UserID: 42},
		PeerID: &tg.PeerChat{ChatID: 99},
	}
	if id := tu.peerID(msg); id != 42 {
		t.Errorf("peerID = %d, want 42", id)
	}
}

func TestTelegramUserChatID(t *testing.T) {
	tu := NewTelegramUser(TelegramUserConfig{})
	tests := []struct {
		name   string
		peerID tg.PeerClass
		want   int
	}{
		{"user", &tg.PeerUser{UserID: 42}, 42},
		{"chat", &tg.PeerChat{ChatID: 99}, 99},
		{"channel", &tg.PeerChannel{ChannelID: 123}, 123},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := &tg.Message{PeerID: tt.peerID}
			if id := tu.chatID(msg); id != tt.want {
				t.Errorf("chatID = %d, want %d", id, tt.want)
			}
		})
	}
}

// Compile-time interface check.
var _ gateway.Platform = (*TelegramUser)(nil)
