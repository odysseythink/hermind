package workers

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockJob struct {
	name      string
	schedule  string
	enabled   bool
	runFn     func(ctx context.Context) error
	runCount  atomic.Int32
	panicFlag bool
}

func (m *mockJob) Name() string                     { return m.name }
func (m *mockJob) Schedule() string                 { return m.schedule }
func (m *mockJob) Enabled(ctx context.Context) bool { return m.enabled }
func (m *mockJob) Run(ctx context.Context) error {
	m.runCount.Add(1)
	if m.panicFlag {
		panic("intentional panic")
	}
	if m.runFn != nil {
		return m.runFn(ctx)
	}
	return nil
}

func TestManager_StartStop(t *testing.T) {
	cfg := &config.Config{}
	mgr := NewManager(nil, cfg)
	job := &mockJob{name: "test-job", schedule: "@every 1s", enabled: true}
	mgr.Register(job)

	err := mgr.Start()
	require.NoError(t, err)
	assert.Equal(t, StateRunning, mgr.state)

	// Let it run at least once
	time.Sleep(1100 * time.Millisecond)
	assert.GreaterOrEqual(t, job.runCount.Load(), int32(1))

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err = mgr.Stop(ctx)
	require.NoError(t, err)
	assert.Equal(t, StateStopped, mgr.state)
}

func TestManager_DisabledJobSkipped(t *testing.T) {
	cfg := &config.Config{}
	mgr := NewManager(nil, cfg)
	job := &mockJob{name: "disabled-job", schedule: "@every 100ms", enabled: false}
	mgr.Register(job)

	err := mgr.Start()
	require.NoError(t, err)

	time.Sleep(250 * time.Millisecond)
	assert.Equal(t, int32(0), job.runCount.Load())

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = mgr.Stop(ctx)
}

func TestManager_PanicRecovery(t *testing.T) {
	cfg := &config.Config{}
	mgr := NewManager(nil, cfg)
	job := &mockJob{name: "panic-job", schedule: "@every 100ms", enabled: true, panicFlag: true}
	mgr.Register(job)

	err := mgr.Start()
	require.NoError(t, err)

	// Manager should survive the panic
	time.Sleep(250 * time.Millisecond)
	assert.Equal(t, StateRunning, mgr.state)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = mgr.Stop(ctx)
}

func TestManager_Trigger(t *testing.T) {
	cfg := &config.Config{}
	mgr := NewManager(nil, cfg)
	job := &mockJob{name: "ondemand-job", schedule: "", enabled: true}
	mgr.Register(job)

	err := mgr.Start()
	require.NoError(t, err)

	err = mgr.Trigger("ondemand-job")
	require.NoError(t, err)

	// Allow time for execution
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, int32(1), job.runCount.Load())

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = mgr.Stop(ctx)
}

func TestManager_TriggerUnknownJob(t *testing.T) {
	cfg := &config.Config{}
	mgr := NewManager(nil, cfg)
	err := mgr.Trigger("nonexistent")
	assert.Error(t, err)
}
