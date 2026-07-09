package providers

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/pantheon/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeSettingsReader struct {
	settings map[string]string
	calls    int
}

func (f *fakeSettingsReader) GetAllSettings(ctx context.Context) (map[string]string, error) {
	f.calls++
	return f.settings, nil
}

type mockLanguageModel struct {
	provider string
	model    string
	delay    <-chan struct{}
}

func (m *mockLanguageModel) Generate(ctx context.Context, req *core.Request) (*core.Response, error) {
	return &core.Response{Message: core.NewTextMessage("assistant", "generated")}, nil
}

func (m *mockLanguageModel) Stream(ctx context.Context, req *core.Request) (core.StreamResponse, error) {
	return func(yield func(*core.StreamPart, error) bool) {
		if m.delay != nil {
			<-m.delay
		}
		yield(&core.StreamPart{TextDelta: m.provider + "-chunk"}, nil)
	}, nil
}

func (m *mockLanguageModel) GenerateObject(ctx context.Context, req *core.ObjectRequest) (*core.ObjectResponse, error) {
	return nil, nil
}

func (m *mockLanguageModel) Provider() string { return m.provider }
func (m *mockLanguageModel) Model() string    { return m.model }

func testBuilder(name string) providerBuilder {
	return func(ctx context.Context, cfg *config.Config, settings map[string]string, modelID string) (core.LanguageModel, error) {
		return &mockLanguageModel{provider: name, model: modelID}, nil
	}
}

func TestManagedLLMProvider_ReloadSwitchesProvider(t *testing.T) {
	providerRegistry["test-a"] = testBuilder("test-a")
	providerRegistry["test-b"] = testBuilder("test-b")
	delete(providerRegistry, "test-a")
	delete(providerRegistry, "test-b")
	// Re-register for cleanup
	providerRegistry["test-a"] = testBuilder("test-a")
	providerRegistry["test-b"] = testBuilder("test-b")
	defer delete(providerRegistry, "test-a")
	defer delete(providerRegistry, "test-b")

	cfg := &config.Config{LLMProvider: "test-a", LLMModel: "model-a"}
	sysSvc := &fakeSettingsReader{settings: map[string]string{"LLMProvider": "test-a"}}
	mgr, err := NewManagedLLMProvider(cfg, sysSvc, sysSvc.settings)
	require.NoError(t, err)

	proxy := mgr.LanguageModel()
	assert.Equal(t, "model-a", proxy.Model())

	sysSvc.settings = map[string]string{"LLMProvider": "test-b"}
	cfg.LLMModel = "model-b"
	require.NoError(t, mgr.Reload(context.Background()))
	assert.Equal(t, "model-b", proxy.Model())
}

func TestManagedLLMProvider_ReloadFailureKeepsOldProvider(t *testing.T) {
	providerRegistry["test-a"] = testBuilder("test-a")
	providerRegistry["test-bad"] = func(ctx context.Context, cfg *config.Config, settings map[string]string, modelID string) (core.LanguageModel, error) {
		return nil, errors.New("bad provider")
	}
	defer delete(providerRegistry, "test-a")
	defer delete(providerRegistry, "test-bad")

	cfg := &config.Config{LLMProvider: "test-a", LLMModel: "model-a"}
	sysSvc := &fakeSettingsReader{settings: map[string]string{"LLMProvider": "test-a"}}
	mgr, err := NewManagedLLMProvider(cfg, sysSvc, sysSvc.settings)
	require.NoError(t, err)

	sysSvc.settings = map[string]string{"LLMProvider": "test-bad"}
	require.Error(t, mgr.Reload(context.Background()))
	assert.Equal(t, "model-a", mgr.LanguageModel().Model())
}

func TestManagedLLMProvider_StreamHoldsOldProviderDuringReload(t *testing.T) {
	doneA := make(chan struct{})
	providerRegistry["test-slow-a"] = func(ctx context.Context, cfg *config.Config, settings map[string]string, modelID string) (core.LanguageModel, error) {
		return &mockLanguageModel{provider: "test-slow-a", model: modelID, delay: doneA}, nil
	}
	providerRegistry["test-b"] = testBuilder("test-b")
	defer delete(providerRegistry, "test-slow-a")
	defer delete(providerRegistry, "test-b")

	cfg := &config.Config{LLMProvider: "test-slow-a", LLMModel: "model-a"}
	sysSvc := &fakeSettingsReader{settings: map[string]string{"LLMProvider": "test-slow-a"}}
	mgr, err := NewManagedLLMProvider(cfg, sysSvc, sysSvc.settings)
	require.NoError(t, err)

	stream, err := mgr.Stream(context.Background(), nil, "", nil)
	require.NoError(t, err)
	// Let the Stream goroutine complete acquire
	time.Sleep(20 * time.Millisecond)

	sysSvc.settings = map[string]string{"LLMProvider": "test-b"}
	cfg.LLMModel = "model-b"
	require.NoError(t, mgr.Reload(context.Background()))

	close(doneA)
	chunk := <-stream
	require.NoError(t, chunk.Err)
	assert.Equal(t, "test-slow-a-chunk", chunk.TextDelta)
}

func TestManagedLLMProvider_OnSettingChanged_IgnoresUnrelatedKey(t *testing.T) {
	providerRegistry["test-a"] = testBuilder("test-a")
	defer delete(providerRegistry, "test-a")

	cfg := &config.Config{LLMProvider: "test-a", LLMModel: "model-a"}
	sysSvc := &fakeSettingsReader{settings: map[string]string{"LLMProvider": "test-a"}}
	mgr, err := NewManagedLLMProvider(cfg, sysSvc, sysSvc.settings)
	require.NoError(t, err)

	require.NoError(t, mgr.OnSettingChanged(context.Background(), "logo_filename", "logo.png"))
	// Unrelated setting should NOT trigger a reload (no calls to GetAllSettings)
	assert.Equal(t, 0, sysSvc.calls, "unrelated setting should not trigger reload")
}

func TestLanguageModelProxy_FollowsReload(t *testing.T) {
	providerRegistry["test-a"] = testBuilder("test-a")
	providerRegistry["test-b"] = testBuilder("test-b")
	defer delete(providerRegistry, "test-a")
	defer delete(providerRegistry, "test-b")

	cfg := &config.Config{LLMProvider: "test-a", LLMModel: "model-a"}
	sysSvc := &fakeSettingsReader{settings: map[string]string{"LLMProvider": "test-a"}}
	mgr, err := NewManagedLLMProvider(cfg, sysSvc, sysSvc.settings)
	require.NoError(t, err)

	proxy := mgr.LanguageModel()
	assert.Equal(t, "model-a", proxy.Model())

	sysSvc.settings = map[string]string{"LLMProvider": "test-b"}
	cfg.LLMModel = "model-b"
	require.NoError(t, mgr.Reload(context.Background()))
	assert.Equal(t, "model-b", proxy.Model())
}

func TestManagedLLMProvider_InitialBuildFailure(t *testing.T) {
	cfg := &config.Config{LLMProvider: "nonexistent", LLMModel: "gpt-4"}
	sysSvc := &fakeSettingsReader{settings: map[string]string{}}
	_, err := NewManagedLLMProvider(cfg, sysSvc, sysSvc.settings)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "initial LLM build failed")
}

func TestMaskKey(t *testing.T) {
	assert.Equal(t, "sk-abc1...xyz9", maskKey("sk-abc1234567890xyz9"))
	assert.Equal(t, "*****", maskKey("short"))
	assert.Equal(t, "***********", maskKey("12345678901"))
}
