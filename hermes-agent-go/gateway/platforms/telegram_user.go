package platforms

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/gotd/td/session"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/telegram/message"
	"github.com/gotd/td/tg"

	"github.com/nousresearch/hermes-agent/gateway"
)

// TelegramUser is a Telegram adapter using the gotd/td MTProto library
// for user-account (non-bot) access. It implements gateway.Platform.
type TelegramUser struct {
	appID       int
	appHash     string
	phone       string
	sessionPath string

	codePrompt     func(ctx context.Context) (string, error)
	passwordPrompt func(ctx context.Context) (string, error)

	api    *tg.Client
	sender *message.Sender
}

// TelegramUserConfig holds the configuration for a TelegramUser adapter.
type TelegramUserConfig struct {
	AppID          int
	AppHash        string
	Phone          string
	SessionPath    string
	CodePrompt     func(ctx context.Context) (string, error)
	PasswordPrompt func(ctx context.Context) (string, error)
}

// NewTelegramUser creates a new TelegramUser adapter from the given config.
func NewTelegramUser(cfg TelegramUserConfig) *TelegramUser {
	if cfg.SessionPath == "" {
		cfg.SessionPath = "telegram_session.json"
	}
	return &TelegramUser{
		appID:          cfg.AppID,
		appHash:        cfg.AppHash,
		phone:          cfg.Phone,
		sessionPath:    cfg.SessionPath,
		codePrompt:     cfg.CodePrompt,
		passwordPrompt: cfg.PasswordPrompt,
	}
}

// Name returns the platform name.
func (t *TelegramUser) Name() string { return "telegram_user" }

// Run starts the MTProto client, authenticates if needed, and dispatches
// incoming messages to the handler. It blocks until ctx is cancelled.
func (t *TelegramUser) Run(ctx context.Context, handler gateway.MessageHandler) error {
	if t.appID == 0 {
		return fmt.Errorf("telegram_user: appID is required")
	}
	if t.appHash == "" {
		return fmt.Errorf("telegram_user: appHash is required")
	}
	if t.phone == "" {
		return fmt.Errorf("telegram_user: phone is required")
	}

	dispatcher := tg.NewUpdateDispatcher()
	dispatcher.OnNewMessage(func(ctx context.Context, e tg.Entities, update *tg.UpdateNewMessage) error {
		msg, ok := update.Message.(*tg.Message)
		if !ok {
			return nil
		}
		if msg.Message == "" {
			return nil
		}
		if msg.Out {
			return nil
		}

		userID := t.peerID(msg)
		chatID := t.chatID(msg)

		in := gateway.IncomingMessage{
			Platform:  t.Name(),
			UserID:    strconv.Itoa(userID),
			ChatID:    strconv.Itoa(chatID),
			Text:      msg.Message,
			MessageID: strconv.Itoa(msg.ID),
		}
		gateway.DispatchAndReply(ctx, t, handler, in)
		return nil
	})

	client := telegram.NewClient(t.appID, t.appHash, telegram.Options{
		SessionStorage: &session.FileStorage{Path: t.sessionPath},
		UpdateHandler:  &dispatcher,
	})

	return client.Run(ctx, func(ctx context.Context) error {
		status, err := client.Auth().Status(ctx)
		if err != nil {
			return fmt.Errorf("telegram_user: auth status: %w", err)
		}
		if !status.Authorized {
			if err := t.authenticate(ctx, client); err != nil {
				return fmt.Errorf("telegram_user: auth: %w", err)
			}
		}

		t.api = client.API()
		t.sender = message.NewSender(t.api)

		slog.InfoContext(ctx, "telegram_user: connected and listening for messages")
		<-ctx.Done()
		return ctx.Err()
	})
}

// authenticate performs the phone-based login flow including optional 2FA.
func (t *TelegramUser) authenticate(ctx context.Context, client *telegram.Client) error {
	if t.codePrompt == nil {
		return fmt.Errorf("telegram_user: codePrompt is required for authentication")
	}

	sentCode, err := client.Auth().SendCode(ctx, t.phone, auth.SendCodeOptions{})
	if err != nil {
		return fmt.Errorf("send code: %w", err)
	}

	sc, ok := sentCode.(*tg.AuthSentCode)
	if !ok {
		// AuthSentCodeSuccess means already authorized via future auth tokens.
		return nil
	}

	code, err := t.codePrompt(ctx)
	if err != nil {
		return fmt.Errorf("code prompt: %w", err)
	}

	_, signInErr := client.Auth().SignIn(ctx, t.phone, code, sc.PhoneCodeHash)
	if signInErr == nil {
		return nil
	}

	// If sign-in failed and we have a password prompt, try 2FA.
	if t.passwordPrompt == nil {
		return fmt.Errorf("sign in: %w (no password prompt configured for 2FA)", signInErr)
	}

	password, err := t.passwordPrompt(ctx)
	if err != nil {
		return fmt.Errorf("password prompt: %w", err)
	}

	_, err = client.Auth().Password(ctx, password)
	if err != nil {
		return fmt.Errorf("2FA password: %w", err)
	}
	return nil
}

// peerID extracts the user ID from the message sender.
func (t *TelegramUser) peerID(msg *tg.Message) int {
	if msg.FromID != nil {
		if u, ok := msg.FromID.(*tg.PeerUser); ok {
			return int(u.UserID)
		}
	}
	if u, ok := msg.PeerID.(*tg.PeerUser); ok {
		return int(u.UserID)
	}
	return 0
}

// chatID extracts the chat ID from the message peer.
func (t *TelegramUser) chatID(msg *tg.Message) int {
	switch p := msg.PeerID.(type) {
	case *tg.PeerUser:
		return int(p.UserID)
	case *tg.PeerChat:
		return int(p.ChatID)
	case *tg.PeerChannel:
		return int(p.ChannelID)
	}
	return 0
}

// SendReply sends a text message to the given chat.
func (t *TelegramUser) SendReply(ctx context.Context, out gateway.OutgoingMessage) error {
	if t.sender == nil {
		return fmt.Errorf("telegram_user: not connected")
	}
	chatID, err := strconv.Atoi(out.ChatID)
	if err != nil {
		return fmt.Errorf("telegram_user: invalid chat_id: %w", err)
	}
	_, err = t.sender.To(&tg.InputPeerUser{UserID: int64(chatID)}).Text(ctx, out.Text)
	if err != nil {
		// Try as channel if user send fails.
		_, err = t.sender.To(&tg.InputPeerChannel{ChannelID: int64(chatID)}).Text(ctx, out.Text)
	}
	return err
}
