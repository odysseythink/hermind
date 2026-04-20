package bedrock

import (
	"context"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"

	"github.com/odysseythink/hermind/message"
	"github.com/odysseythink/hermind/provider"
)

// eventReader is the subset of *bedrockruntime.ConverseStreamEventStream
// that Stream uses. Extracting it as an interface lets tests inject a
// channel-backed fake without hitting AWS.
type eventReader interface {
	Events() <-chan types.ConverseStreamOutput
	Err() error
	Close() error
}

// Stream opens a ConverseStream request and returns a provider.Stream
// adapter over the resulting AWS event stream.
func (b *Bedrock) Stream(ctx context.Context, req *provider.Request) (provider.Stream, error) {
	in := buildConverseInput(req)
	streamIn := copyToStreamInput(in)
	out, err := b.client.ConverseStream(ctx, streamIn)
	if err != nil {
		return nil, mapAWSError(err)
	}
	return &bedrockStream{
		reader: out.GetStream(),
		model:  req.Model,
	}, nil
}

// bedrockStream is the provider.Stream implementation. Accumulates
// tool-use state across ContentBlockStart/Delta/Stop events so the
// caller sees a single coherent tool_call per invocation.
type bedrockStream struct {
	reader eventReader
	model  string

	// Active tool-use state, keyed by ContentBlockIndex. Reset on each
	// ContentBlockStop event.
	toolCalls map[int32]*toolCallBuilder
}

type toolCallBuilder struct {
	id   string
	name string
	buf  []byte
}

func (s *bedrockStream) Recv() (*provider.StreamEvent, error) {
	ev, ok := <-s.reader.Events()
	if !ok {
		if err := s.reader.Err(); err != nil {
			return nil, mapAWSError(err)
		}
		return &provider.StreamEvent{Type: provider.EventDone}, io.EOF
	}
	return s.dispatch(ev), nil
}

func (s *bedrockStream) Close() error {
	if s.reader == nil {
		return nil
	}
	return s.reader.Close()
}

// dispatch mutates s.toolCalls and returns the provider-level event.
// Package-level streamEventFromChunk is the pure-function variant for
// testing; dispatch is the stateful wrapper the real Recv uses.
func (s *bedrockStream) dispatch(chunk types.ConverseStreamOutput) *provider.StreamEvent {
	if s.toolCalls == nil {
		s.toolCalls = make(map[int32]*toolCallBuilder)
	}
	switch c := chunk.(type) {
	case *types.ConverseStreamOutputMemberContentBlockStart:
		idx := int32(0)
		if c.Value.ContentBlockIndex != nil {
			idx = *c.Value.ContentBlockIndex
		}
		if start, ok := c.Value.Start.(*types.ContentBlockStartMemberToolUse); ok {
			s.toolCalls[idx] = &toolCallBuilder{
				id:   aws.ToString(start.Value.ToolUseId),
				name: aws.ToString(start.Value.Name),
			}
		}
		return &provider.StreamEvent{Type: provider.EventDelta, Delta: &provider.StreamDelta{}}

	case *types.ConverseStreamOutputMemberContentBlockDelta:
		idx := int32(0)
		if c.Value.ContentBlockIndex != nil {
			idx = *c.Value.ContentBlockIndex
		}
		switch d := c.Value.Delta.(type) {
		case *types.ContentBlockDeltaMemberText:
			return &provider.StreamEvent{
				Type:  provider.EventDelta,
				Delta: &provider.StreamDelta{Content: d.Value},
			}
		case *types.ContentBlockDeltaMemberToolUse:
			if tc := s.toolCalls[idx]; tc != nil {
				tc.buf = append(tc.buf, []byte(aws.ToString(d.Value.Input))...)
			}
			return &provider.StreamEvent{Type: provider.EventDelta, Delta: &provider.StreamDelta{}}
		}
		return &provider.StreamEvent{Type: provider.EventDelta, Delta: &provider.StreamDelta{}}

	case *types.ConverseStreamOutputMemberContentBlockStop:
		idx := int32(0)
		if c.Value.ContentBlockIndex != nil {
			idx = *c.Value.ContentBlockIndex
		}
		tc := s.toolCalls[idx]
		delete(s.toolCalls, idx)
		if tc == nil {
			return &provider.StreamEvent{Type: provider.EventDelta, Delta: &provider.StreamDelta{}}
		}
		args := string(tc.buf)
		if args == "" {
			args = "{}"
		}
		return &provider.StreamEvent{
			Type: provider.EventDelta,
			Delta: &provider.StreamDelta{
				ToolCalls: []message.ToolCall{{
					ID:   tc.id,
					Type: "function",
					Function: message.ToolCallFunction{
						Name:      tc.name,
						Arguments: args,
					},
				}},
			},
		}

	case *types.ConverseStreamOutputMemberMessageStop:
		return &provider.StreamEvent{
			Type: provider.EventDone,
			Response: &provider.Response{
				FinishReason: stopReasonToString(c.Value.StopReason),
				Model:        s.model,
			},
		}

	case *types.ConverseStreamOutputMemberMessageStart, *types.ConverseStreamOutputMemberMetadata:
		return &provider.StreamEvent{Type: provider.EventDelta, Delta: &provider.StreamDelta{}}
	}

	return &provider.StreamEvent{Type: provider.EventDelta, Delta: &provider.StreamDelta{}}
}

// streamEventFromChunk is a pure (stateless) mapper used in unit tests.
// It handles the common cases where no cross-event state is needed.
// The real streaming loop uses bedrockStream.dispatch instead.
func streamEventFromChunk(chunk types.ConverseStreamOutput) *provider.StreamEvent {
	switch c := chunk.(type) {
	case *types.ConverseStreamOutputMemberContentBlockDelta:
		switch d := c.Value.Delta.(type) {
		case *types.ContentBlockDeltaMemberText:
			return &provider.StreamEvent{
				Type:  provider.EventDelta,
				Delta: &provider.StreamDelta{Content: d.Value},
			}
		case *types.ContentBlockDeltaMemberToolUse:
			return &provider.StreamEvent{
				Type: provider.EventDelta,
				Delta: &provider.StreamDelta{
					ToolCalls: []message.ToolCall{{
						Type: "function",
						Function: message.ToolCallFunction{
							Arguments: aws.ToString(d.Value.Input),
						},
					}},
				},
			}
		}
	case *types.ConverseStreamOutputMemberMessageStop:
		return &provider.StreamEvent{
			Type: provider.EventDone,
			Response: &provider.Response{
				FinishReason: stopReasonToString(c.Value.StopReason),
			},
		}
	}
	return &provider.StreamEvent{Type: provider.EventDelta, Delta: &provider.StreamDelta{}}
}

// Compile-time assertion that *bedrockruntime.ConverseStreamEventStream
// satisfies the eventReader interface.
var _ eventReader = (*bedrockruntime.ConverseStreamEventStream)(nil)
