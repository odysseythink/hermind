package extensions

import (
	"context"
	"testing"

	"github.com/odysseythink/hermind/backend/internal/collector/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockExtension struct {
	name string
}

func (m *mockExtension) Name() string { return m.name }

func (m *mockExtension) Handle(ctx context.Context, endpoint string, method string, body []byte) (*core.ExtensionResponse, error) {
	return &core.ExtensionResponse{Success: true, Data: map[string]interface{}{"name": m.name}}, nil
}

func TestRegistry_RegisterAndHandle(t *testing.T) {
	r := NewRegistry()
	mock := &mockExtension{name: "test"}
	r.Register("/ext/test", mock)

	resp, err := r.Handle(context.Background(), "/ext/test", "POST", nil)
	require.NoError(t, err)
	assert.True(t, resp.Success)
	assert.Equal(t, "test", resp.Data["name"])
}

func TestRegistry_Handle_NotFound(t *testing.T) {
	r := NewRegistry()
	_, err := r.Handle(context.Background(), "/ext/missing", "POST", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}
