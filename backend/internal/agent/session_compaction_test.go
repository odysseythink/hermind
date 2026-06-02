package agent_test

import (
	"context"
	"testing"

	"github.com/odysseythink/hermind/backend/internal/agent"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/pantheon/core"
	"github.com/stretchr/testify/require"
)

type noopCompressor struct{}

func (n *noopCompressor) Compress(ctx context.Context, history []core.Message) ([]core.Message, error) {
	return history, nil
}

func TestNewSession_AcceptsCompressor(t *testing.T) {
	serverConn, _ := newPipedWS(t)
	wc := agent.NewWSConnForTesting(serverConn)
	defer wc.Close()

	ws := &models.Workspace{ID: 1}
	comp := &noopCompressor{}
	sess := agent.NewSessionForTesting(context.Background(), "test-uuid", ws, nil,
		&mockLanguageModel{provider: "mock", model: "mock-model", replies: []string{"TERMINATE"}},
		"sys", nil, wc, comp)
	require.NotNil(t, sess)
	require.Equal(t, comp, sess.CompressorForTesting())
}

func TestNewSession_NilCompressor(t *testing.T) {
	serverConn, _ := newPipedWS(t)
	wc := agent.NewWSConnForTesting(serverConn)
	defer wc.Close()

	ws := &models.Workspace{ID: 1}
	sess := agent.NewSessionForTesting(context.Background(), "test-uuid", ws, nil,
		&mockLanguageModel{provider: "mock", model: "mock-model", replies: []string{"TERMINATE"}},
		"sys", nil, wc, nil)
	require.NotNil(t, sess)
	require.Nil(t, sess.CompressorForTesting())
}

func TestSession_initAgent_WiresCompressor(t *testing.T) {
	serverConn, _ := newPipedWS(t)
	wc := agent.NewWSConnForTesting(serverConn)
	defer wc.Close()

	ws := &models.Workspace{ID: 1}
	comp := &noopCompressor{}
	sess := agent.NewSessionForTesting(context.Background(), "test-uuid", ws, nil,
		&mockLanguageModel{provider: "mock", model: "mock-model", replies: []string{"TERMINATE"}},
		"sys", nil, wc, comp)
	require.NotNil(t, sess)
	require.NotNil(t, sess.PantheonAgent())
}
