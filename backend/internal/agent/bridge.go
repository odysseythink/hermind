package agent

import (
	"context"
	"errors"
	"fmt"

	"github.com/odysseythink/pantheon/conversation"
)

func installEventBridges(s *Session) {
	s.conv.OnMessage(func(chat conversation.Chat, _ *conversation.Conversation) {
		if s.muteUser && chat.From == participantUser {
			return
		}
		_ = s.io.Send(ServerFrame{
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
