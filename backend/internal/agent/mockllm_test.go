package agent_test

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/odysseythink/pantheon/core"
)

const slowReply = "__SLOW__"

type mockLanguageModel struct {
	provider, model string
	replies         []string
	parts           [][]core.ContentParter // if set, overrides replies
	calls           atomic.Int32
	gate            chan struct{} // if non-nil, Generate waits until closed
}

func (m *mockLanguageModel) Generate(ctx context.Context, req *core.Request) (*core.Response, error) {
	if m.gate != nil {
		select {
		case <-m.gate:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	idx := int(m.calls.Add(1)) - 1
	if len(m.parts) > 0 {
		if idx >= len(m.parts) {
			return nil, fmt.Errorf("mock ran out of parts after %d calls", idx)
		}
		return &core.Response{
			Message: core.Message{Content: m.parts[idx]},
			Usage:   core.Usage{TotalTokens: 1},
		}, nil
	}
	if idx >= len(m.replies) {
		return nil, fmt.Errorf("mock ran out of replies after %d calls", idx)
	}
	r := m.replies[idx]
	if r == slowReply {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	return &core.Response{
		Message: core.Message{Content: core.NewTextContent(r)},
		Usage:   core.Usage{TotalTokens: 1},
	}, nil
}

func (m *mockLanguageModel) Stream(ctx context.Context, req *core.Request) (core.StreamResponse, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockLanguageModel) GenerateObject(ctx context.Context, req *core.ObjectRequest) (*core.ObjectResponse, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockLanguageModel) Provider() string { return m.provider }
func (m *mockLanguageModel) Model() string    { return m.model }
