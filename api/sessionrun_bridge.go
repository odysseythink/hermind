package api

import "github.com/odysseythink/hermind/api/sessionrun"

// hubPublisher adapts a StreamHub to the sessionrun.EventPublisher
// contract. The runner package lives in a child directory and cannot
// import the api package; this adapter converts between the two event
// shapes (they carry the same fields under different types).
type hubPublisher struct{ hub StreamHub }

func (h *hubPublisher) Publish(e sessionrun.Event) {
	h.hub.Publish(StreamEvent{
		Type:      e.Type,
		SessionID: e.SessionID,
		Data:      e.Data,
	})
}
