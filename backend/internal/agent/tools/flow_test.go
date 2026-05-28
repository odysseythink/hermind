package tools

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/odysseythink/hermind/backend/internal/agent/flow"
	"github.com/odysseythink/hermind/backend/internal/services"
	"github.com/stretchr/testify/require"
)

func TestFlowProjection_ActiveFlowsBecomeEntries(t *testing.T) {
	tmpDir := t.TempDir()
	fsvc := services.NewAgentFlowService(tmpDir)

	active := []byte(`{"name":"Active Flow","active":true,"steps":[{"type":"noop"}]}`)
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "plugins", "agent-flows", "active.json"), active, 0644))

	flows, err := fsvc.ListFlows()
	require.NoError(t, err)
	require.Len(t, flows, 1)
	require.True(t, flows[0].Active)

	var emitted []string
	emit := func(m string) { emitted = append(emitted, m) }
	exec := flow.New(nil, true)
	e := flowToEntry(flows[0], fsvc, exec, emit)

	require.Equal(t, "flow-active-flow", e.Name)
	require.Contains(t, e.Description, "Active Flow")
	require.Equal(t, "flow", e.Toolset)
}

func TestFlowProjection_ExecutorNil_ReturnsError(t *testing.T) {
	tmpDir := t.TempDir()
	fsvc := services.NewAgentFlowService(tmpDir)

	active := []byte(`{"name":"Active Flow","active":true,"steps":[]}`)
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "plugins", "agent-flows", "active.json"), active, 0644))

	flows, err := fsvc.ListFlows()
	require.NoError(t, err)
	require.Len(t, flows, 1)

	emit := func(string) {}
	e := flowToEntry(flows[0], fsvc, nil, emit)

	result, err := e.Handler(context.Background(), nil)
	require.NoError(t, err)
	require.Contains(t, result, "flow execution requires AgentFlowExecutor")
}

func TestFlowProjection_ExecutorRunsFlow_ReturnsOutput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("flow-result"))
	}))
	defer srv.Close()

	tmpDir := t.TempDir()
	fsvc := services.NewAgentFlowService(tmpDir)

	f := []byte(fmt.Sprintf(`{"name":"Test Flow","active":true,"steps":[{"type":"apiCall","config":{"url":"%s","method":"GET"}}]}`, srv.URL))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "plugins", "agent-flows", "test-flow.json"), f, 0644))

	flows, err := fsvc.ListFlows()
	require.NoError(t, err)
	require.Len(t, flows, 1)

	var emitted []string
	emit := func(m string) { emitted = append(emitted, m) }
	exec := flow.New(nil, true)
	e := flowToEntry(flows[0], fsvc, exec, emit)

	result, err := e.Handler(context.Background(), nil)
	require.NoError(t, err)
	require.Contains(t, result, "Test Flow")
	require.Contains(t, result, "output")
	require.Contains(t, result, "flow-result")
	require.Len(t, emitted, 3)
	require.Contains(t, emitted[0], "Test Flow")
}
