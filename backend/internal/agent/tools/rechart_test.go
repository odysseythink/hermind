package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/pantheon/tool"
	"github.com/stretchr/testify/require"
)

func TestRechart_BasicLineChart_ReturnsJSON(t *testing.T) {
	tc := &ToolContext{
		Ctx:       context.Background(),
		Workspace: &models.Workspace{ID: 1},
		Emit:      func(string) {},
	}
	entry := NewRechartSkill(tc)
	args := json.RawMessage(`{"type":"line","data":{"labels":["A","B"],"values":[1,2]}}`)
	result, err := entry.Handler(context.Background(), args)
	require.NoError(t, err)
	require.Contains(t, result, "line")
	require.Contains(t, result, "renderable")
	require.Contains(t, result, "true")
}

func TestRechart_InvalidType_ReturnsError(t *testing.T) {
	tc := &ToolContext{
		Ctx:       context.Background(),
		Workspace: &models.Workspace{ID: 1},
		Emit:      func(string) {},
	}
	entry := NewRechartSkill(tc)
	args := json.RawMessage(`{"type":"donut","data":{"x":1}}`)
	result, err := entry.Handler(context.Background(), args)
	require.NoError(t, err)
	require.Contains(t, result, "error")
	require.Contains(t, result, "donut")
}

func TestRechart_MissingData_ReturnsError(t *testing.T) {
	tc := &ToolContext{
		Ctx:       context.Background(),
		Workspace: &models.Workspace{ID: 1},
		Emit:      func(string) {},
	}
	entry := NewRechartSkill(tc)
	args := json.RawMessage(`{"type":"bar"}`)
	result, err := entry.Handler(context.Background(), args)
	require.NoError(t, err)
	require.Contains(t, result, "error")
	require.Contains(t, result, "data is required")
}

func TestRechart_DispatchViaRegistry(t *testing.T) {
	tc := &ToolContext{
		Ctx:       context.Background(),
		Workspace: &models.Workspace{ID: 1},
		Emit:      func(string) {},
	}
	entry := NewRechartSkill(tc)
	reg := tool.NewRegistry()
	reg.Register(entry)

	result, err := reg.Dispatch(context.Background(), "rechart", json.RawMessage(`{"type":"pie","data":{"slices":[1,2,3]}}`))
	require.NoError(t, err)
	require.Contains(t, result, "pie")
}
