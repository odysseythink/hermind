package services

import (
	"context"
	"fmt"
	"sync"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/odysseythink/mlog"
)

type messageQueue struct {
	chatID   int64
	svc      *TelegramBotService
	updates  chan tgbotapi.Update
	stopOnce sync.Once
	stopCh   chan struct{}
}

func newMessageQueue(chatID int64, svc *TelegramBotService) *messageQueue {
	return &messageQueue{
		chatID:  chatID,
		svc:     svc,
		updates: make(chan tgbotapi.Update, 16),
		stopCh:  make(chan struct{}),
	}
}

func (q *messageQueue) enqueue(u tgbotapi.Update) {
	select {
	case q.updates <- u:
	case <-q.stopCh:
	}
}

func (q *messageQueue) stop() {
	q.stopOnce.Do(func() {
		close(q.stopCh)
		close(q.updates)
	})
}

func (q *messageQueue) run() {
	defer q.svc.queues.Delete(q.chatID)
	for {
		select {
		case <-q.stopCh:
			return
		case u, ok := <-q.updates:
			if !ok {
				return
			}
			q.process(u)
		}
	}
}

func (q *messageQueue) process(u tgbotapi.Update) {
	defer func() {
		if r := recover(); r != nil {
			mlog.Error("telegram queue panic: ", r)
		}
	}()

	msg := u.Message
	chatIDStr := fmt.Sprintf("%d", q.chatID)

	// Pairing check
	if !q.isApproved(chatIDStr) {
		if msg.IsCommand() && msg.Command() == "start" {
			q.svc.handleStart(msg)
		} else {
			_ = q.svc.sendText(q.chatID, "Please send /start to begin pairing.")
		}
		return
	}

	if msg.IsCommand() {
		q.handleCommand(msg)
		return
	}

	// Media messages
	if msg.Voice != nil || (msg.Photo != nil && len(msg.Photo) > 0) || msg.Document != nil {
		q.handleMedia(msg)
		return
	}

	// Regular text
	q.svc.handleChatMessage(context.Background(), msg, q.chatID, chatIDStr)
}

func (q *messageQueue) isApproved(chatIDStr string) bool {
	_, ok := q.svc.approved.Load(chatIDStr)
	return ok
}

// Stub for PR5 media handling.
func (q *messageQueue) handleMedia(msg *tgbotapi.Message) {
	_ = q.svc.sendText(msg.Chat.ID, "❌ Media processing is not yet available.")
}
