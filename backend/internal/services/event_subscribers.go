package services

import (
	"context"
	"encoding/json"
	"time"

	"github.com/odysseythink/mlog"
)

// EventEnvelope is what subscribers receive — a parsed view of the row that
// was just persisted, with metadata decoded once for all handlers.
type EventEnvelope struct {
	Event      string
	Metadata   map[string]any
	UserID     *int
	OccurredAt time.Time
}

type eventHandler = func(ctx context.Context, e EventEnvelope)

func (s *EventLogService) ensureRegistry() {
	s.subInit.Do(func() {
		s.subscribers = map[string][]eventHandler{}
	})
}

// Subscribe registers a handler for an event type. Handlers run in their own
// goroutines so a slow handler never blocks LogEvent or another handler.
func (s *EventLogService) Subscribe(eventType string, h eventHandler) {
	s.ensureRegistry()
	s.subMu.Lock()
	defer s.subMu.Unlock()
	s.subscribers[eventType] = append(s.subscribers[eventType], h)
}

// notifySubscribers fans out an event to all registered handlers.
// Called by LogEvent after the DB row is written.
func (s *EventLogService) notifySubscribers(eventType string, metaJSON *string, userID *int, occurredAt time.Time) {
	s.ensureRegistry()
	s.subMu.RLock()
	handlers := append([]eventHandler(nil), s.subscribers[eventType]...)
	s.subMu.RUnlock()
	if len(handlers) == 0 {
		return
	}
	var meta map[string]any
	if metaJSON != nil {
		if err := json.Unmarshal([]byte(*metaJSON), &meta); err != nil {
			mlog.Warning("eventlog: failed to unmarshal metadata for notify", mlog.Err(err))
		}
	}
	env := EventEnvelope{Event: eventType, Metadata: meta, UserID: userID, OccurredAt: occurredAt}
	for _, h := range handlers {
		go func(h eventHandler) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			defer func() { _ = recover() }()
			h(ctx, env)
		}(h)
	}
}
