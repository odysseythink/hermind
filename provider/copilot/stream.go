package copilot

import (
	"context"

	"github.com/odysseythink/hermind/provider"
)

// Stream for Copilot is a thin adapter: we call Complete (which
// blocks until the child finishes), then fabricate a two-event
// stream (Delta with the full text, then Done). This keeps the
// semantics consistent for callers that plug Copilot into
// fallback chains while Stream-native support lands.
func (c *Copilot) Stream(ctx context.Context, req *provider.Request) (provider.Stream, error) {
	resp, err := c.Complete(ctx, req)
	if err != nil {
		return nil, err
	}
	return &syntheticStream{resp: resp}, nil
}

type syntheticStream struct {
	resp *provider.Response
	sent bool
	done bool
}

func (s *syntheticStream) Recv() (*provider.StreamEvent, error) {
	if !s.sent {
		s.sent = true
		return &provider.StreamEvent{
			Type: provider.EventDelta,
			Delta: &provider.StreamDelta{
				Content: s.resp.Message.Content.Text(),
			},
		}, nil
	}
	if !s.done {
		s.done = true
		return &provider.StreamEvent{Type: provider.EventDone, Response: s.resp}, nil
	}
	return &provider.StreamEvent{Type: provider.EventDone, Response: s.resp}, nil
}

func (s *syntheticStream) Close() error { return nil }
