package services

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/odysseythink/hermind/backend/internal/dto"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/mlog"
)

func (q *messageQueue) handleMedia(msg *tgbotapi.Message) {
	chatID := msg.Chat.ID
	chatIDStr := fmt.Sprintf("%d", chatID)
	user := q.getUser(chatIDStr)
	if user == nil {
		_ = q.svc.sendText(chatID, "❌ Not authorized.")
		return
	}
	ws, _ := q.resolveWorkspace(user.ActiveWorkspace)
	if ws == nil {
		_ = q.svc.sendText(chatID, "❌ No workspace found.")
		return
	}
	var threadID *int
	if user.ActiveThread != "" {
		if id, err := strconv.Atoi(user.ActiveThread); err == nil {
			threadID = &id
		}
	}

	ctx := context.Background()

	switch {
	case msg.Voice != nil:
		text, err := q.svc.handleVoice(ctx, msg.Voice)
		if err != nil {
			_ = q.svc.sendText(chatID, "❌ Failed to process voice: "+err.Error())
			return
		}
		q.svc.handleChatMessage(ctx, &tgbotapi.Message{Text: text, Chat: msg.Chat}, chatID, chatIDStr, true)
	case msg.Photo != nil && len(msg.Photo) > 0:
		err := q.svc.handlePhoto(ctx, ws, user, threadID, msg)
		if err != nil {
			_ = q.svc.sendText(chatID, "❌ Failed to process photo: "+err.Error())
		}
	case msg.Document != nil:
		err := q.svc.handleDocument(ctx, ws, user, threadID, msg)
		if err != nil {
			_ = q.svc.sendText(chatID, "❌ Failed to process document: "+err.Error())
		}
	default:
		_ = q.svc.sendText(chatID, "❌ Unsupported message type.")
	}
}

func (s *TelegramBotService) handleVoice(ctx context.Context, voice *tgbotapi.Voice) (string, error) {
	file, err := s.bot.GetFile(tgbotapi.FileConfig{FileID: voice.FileID})
	if err != nil {
		return "", fmt.Errorf("get file: %w", err)
	}
	data, err := s.downloadFile(file.Link(s.bot.Token))
	if err != nil {
		return "", fmt.Errorf("download: %w", err)
	}
	mlog.Info("telegram: voice message downloaded, size=", len(data))
	// TODO: STT via Collector when available
	_ = data
	return "[Voice message transcription not yet available]", nil
}

func (s *TelegramBotService) handlePhoto(ctx context.Context, ws *models.Workspace, user *TelegramUser, threadID *int, msg *tgbotapi.Message) error {
	// Get largest photo
	photo := msg.Photo[len(msg.Photo)-1]
	file, err := s.bot.GetFile(tgbotapi.FileConfig{FileID: photo.FileID})
	if err != nil {
		return err
	}
	data, err := s.downloadFile(file.Link(s.bot.Token))
	if err != nil {
		return err
	}
	b64 := base64.StdEncoding.EncodeToString(data)
	dataURL := fmt.Sprintf("data:image/jpeg;base64,%s", b64)

	prompt := msg.Caption
	if prompt == "" {
		prompt = "Describe this image."
	}
	req := dto.ChatRequest{
		Message:     prompt,
		Attachments: []string{dataURL},
	}
	resp, err := s.chatSvc.Complete(ctx, ws, nil, threadID, req)
	if err != nil {
		return err
	}
	if resp.Error != "" {
		return errors.New(resp.Error)
	}
	chatID, _ := strconv.ParseInt(user.ChatID, 10, 64)
	_ = s.sendText(chatID, resp.TextResponse)
	s.maybeSendVoiceReply(chatID, resp.TextResponse, false)
	return nil
}

func (s *TelegramBotService) handleDocument(ctx context.Context, ws *models.Workspace, user *TelegramUser, threadID *int, msg *tgbotapi.Message) error {
	doc := msg.Document
	file, err := s.bot.GetFile(tgbotapi.FileConfig{FileID: doc.FileID})
	if err != nil {
		return err
	}
	data, err := s.downloadFile(file.Link(s.bot.Token))
	if err != nil {
		return err
	}
	mlog.Info("telegram: document downloaded, name=", doc.FileName, " size=", len(data))
	// TODO: parse via Collector when ready
	_ = data

	prompt := msg.Caption
	if prompt == "" {
		prompt = fmt.Sprintf("The user shared a document named %s.", doc.FileName)
	} else {
		prompt = fmt.Sprintf("The user shared a document named %s. User request: %s", doc.FileName, prompt)
	}
	req := dto.ChatRequest{Message: prompt}
	resp, err := s.chatSvc.Complete(ctx, ws, nil, threadID, req)
	if err != nil {
		return err
	}
	if resp.Error != "" {
		return errors.New(resp.Error)
	}
	chatID, _ := strconv.ParseInt(user.ChatID, 10, 64)
	_ = s.sendText(chatID, resp.TextResponse)
	s.maybeSendVoiceReply(chatID, resp.TextResponse, false)
	return nil
}

var telegramHTTPClient = &http.Client{Timeout: 30 * time.Second}

func (s *TelegramBotService) downloadFile(url string) ([]byte, error) {
	if !strings.HasPrefix(url, "https://api.telegram.org/file/") {
		return nil, fmt.Errorf("invalid file URL: not from Telegram API")
	}
	resp, err := telegramHTTPClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}
