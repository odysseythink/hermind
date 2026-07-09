package agent

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"

	"github.com/odysseythink/pantheon/conversation"
)

func installEventBridges(s *Session) {
	s.conv.OnMessage(func(chat conversation.Chat, _ *conversation.Conversation) {
		if s.muteUser && chat.From == participantUser {
			return
		}
		// Generate a UUID for assistant messages so citations can attach.
		uuid := ""
		if chat.From != participantUser {
			uuid = uuidV4()
			s.SetCurrentMessageUUID(uuid)
		}
		_ = s.io.Send(ServerFrame{
			UUID:    uuid,
			From:    chat.From,
			To:      chat.To,
			Content: chat.Content,
			State:   string(chat.State),
		})
		logChatSent(s.eventLog, s.UserID, s.UUID, chat.From, chat.To)
	})
	s.conv.OnError(func(err error, _ conversation.Route, _ *conversation.Conversation) {
		content := err.Error()
		if errors.Is(err, context.DeadlineExceeded) || content == "context deadline exceeded" {
			content = "Session reached maximum duration. Ending now."
		}
		_ = s.io.Send(ServerFrame{
			Type:    FrameWSSFailure,
			Content: content,
		})
		s.cancel()
	})
	s.conv.OnInterrupt(func(route conversation.Route, _ *conversation.Conversation) {
		_ = s.io.Send(ServerFrame{
			Type:     FrameWaitingOnInput,
			Question: fmt.Sprintf("Provide feedback to %s as %s.", route.To, route.From),
		})
		go func() {
			select {
			case fb := <-s.feedbackCh:
				_ = s.conv.Continue(s.ctx, fb.Content)
			case <-s.ctx.Done():
				return
			}
		}()
	})
	s.conv.OnTerminate(func(_ string, _ *conversation.Conversation) {
		s.once.Do(func() { close(s.terminated) })
	})
}

func uuidV4() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
