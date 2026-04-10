// agent/engine_test.go
package agent

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/nousresearch/hermes-agent/config"
	"github.com/nousresearch/hermes-agent/message"
	"github.com/nousresearch/hermes-agent/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeProvider returns a canned response for tests.
type fakeProvider struct {
	name     string
	response *provider.Response
	err      error
	streamFn func() (provider.Stream, error)
}

func (f *fakeProvider) Name() string { return f.name }
func (f *fakeProvider) Complete(ctx context.Context, req *provider.Request) (*provider.Response, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.response, nil
}
func (f *fakeProvider) Stream(ctx context.Context, req *provider.Request) (provider.Stream, error) {
	if f.streamFn != nil {
		return f.streamFn()
	}
	return nil, errors.New("not implemented")
}
func (f *fakeProvider) ModelInfo(string) *provider.ModelInfo       { return nil }
func (f *fakeProvider) EstimateTokens(string, string) (int, error) { return 0, nil }
func (f *fakeProvider) Available() bool                             { return true }

// fakeStream returns a single delta then Done.
type fakeStream struct {
	events []*provider.StreamEvent
	idx    int
}

func (s *fakeStream) Recv() (*provider.StreamEvent, error) {
	if s.idx >= len(s.events) {
		return nil, io.EOF
	}
	ev := s.events[s.idx]
	s.idx++
	return ev, nil
}
func (s *fakeStream) Close() error { return nil }

func newFakeStreamingProvider(text string) *fakeProvider {
	return &fakeProvider{
		name: "fake",
		streamFn: func() (provider.Stream, error) {
			return &fakeStream{
				events: []*provider.StreamEvent{
					{Type: provider.EventDelta, Delta: &provider.StreamDelta{Content: text}},
					{
						Type: provider.EventDone,
						Response: &provider.Response{
							Message: message.Message{
								Role:    message.RoleAssistant,
								Content: message.TextContent(text),
							},
							FinishReason: "end_turn",
							Usage:        message.Usage{InputTokens: 5, OutputTokens: 3},
						},
					},
				},
			}, nil
		},
	}
}

func TestEngineSingleTurn(t *testing.T) {
	p := newFakeStreamingProvider("Hello back!")
	e := NewEngine(p, nil, config.AgentConfig{MaxTurns: 10}, "cli")

	result, err := e.RunConversation(context.Background(), &RunOptions{
		UserMessage: "hi",
		SessionID:   "test-session",
	})
	require.NoError(t, err)
	assert.Equal(t, "Hello back!", result.Response.Content.Text())
	assert.Equal(t, 1, result.Iterations)
	require.Len(t, result.Messages, 2)
	assert.Equal(t, message.RoleUser, result.Messages[0].Role)
	assert.Equal(t, message.RoleAssistant, result.Messages[1].Role)
}

func TestEngineRespectsContextCancellation(t *testing.T) {
	p := newFakeStreamingProvider("should not get here")
	e := NewEngine(p, nil, config.AgentConfig{MaxTurns: 10}, "cli")

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before run

	_, err := e.RunConversation(ctx, &RunOptions{
		UserMessage: "hi",
		SessionID:   "cancelled-session",
	})
	assert.ErrorIs(t, err, context.Canceled)
}
