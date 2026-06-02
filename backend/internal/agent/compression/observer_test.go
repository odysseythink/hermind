package compression

import (
	"context"
	"testing"

	"github.com/odysseythink/pantheon/core"
	"github.com/stretchr/testify/require"
)

type fakeEngine struct {
	calls int
}

func (f *fakeEngine) Compress(ctx context.Context, history []core.Message) ([]core.Message, error) {
	f.calls++
	return append([]core.Message(nil), history...), nil
}

func TestObserver_ExtractsSummaryAndCallsSave(t *testing.T) {
	inner := &fakeEngine{}
	var saved string
	obs := NewObserver(inner, func(summary string) error {
		saved = summary
		return nil
	})

	msgs := []core.Message{
		{Role: core.MESSAGE_ROLE_USER, Content: core.NewTextContent("hello")},
		{Role: core.MESSAGE_ROLE_ASSISTANT, Content: core.NewTextContent("[Compressed summary of earlier conversation]\nThe user said hello.")},
	}
	out, err := obs.Compress(context.Background(), msgs)
	require.NoError(t, err)
	require.Len(t, out, 2)
	require.Equal(t, "The user said hello.", saved)
	require.Equal(t, 1, inner.calls)
}

func TestObserver_NoSummary_NoSaveCall(t *testing.T) {
	inner := &fakeEngine{}
	var saved string
	obs := NewObserver(inner, func(summary string) error {
		saved = summary
		return nil
	})

	msgs := []core.Message{
		{Role: core.MESSAGE_ROLE_USER, Content: core.NewTextContent("no compression here")},
	}
	out, err := obs.Compress(context.Background(), msgs)
	require.NoError(t, err)
	require.Len(t, out, 1)
	require.Empty(t, saved)
	require.Equal(t, 1, inner.calls)
}

func TestObserver_NilSave_DoesNotPanic(t *testing.T) {
	inner := &fakeEngine{}
	obs := NewObserver(inner, nil)

	msgs := []core.Message{
		{Role: core.MESSAGE_ROLE_ASSISTANT, Content: core.NewTextContent("[Compressed summary of earlier conversation]\nsome summary")},
	}
	out, err := obs.Compress(context.Background(), msgs)
	require.NoError(t, err)
	require.Len(t, out, 1)
}

func TestObserver_DelegatesCompress(t *testing.T) {
	inner := &fakeEngine{}
	obs := NewObserver(inner, nil)

	msgs := []core.Message{
		{Role: core.MESSAGE_ROLE_USER, Content: core.NewTextContent("test")},
	}
	out, err := obs.Compress(context.Background(), msgs)
	require.NoError(t, err)
	require.Len(t, out, 1)
	require.Equal(t, 1, inner.calls)
}
