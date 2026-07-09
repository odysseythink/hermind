package services

import (
	"context"
	"errors"
	"testing"

	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/providers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordingObserver struct {
	calls []string
	err   error
}

func (o *recordingObserver) OnSettingChanged(ctx context.Context, key, value string) error {
	o.calls = append(o.calls, key+"="+value)
	return o.err
}

func setupSystemService(t *testing.T) *SystemService {
	cfg := &config.Config{StorageDir: t.TempDir()}
	db, err := NewDB(cfg)
	require.NoError(t, err)
	t.Cleanup(func() {
		sqlDB, _ := db.DB()
		if sqlDB != nil {
			sqlDB.Close()
		}
	})
	require.NoError(t, AutoMigrate(db))
	return NewSystemService(db)
}

func TestSystemService_ObserverNotified(t *testing.T) {
	svc := setupSystemService(t)
	obs := &recordingObserver{}
	svc.RegisterObserver(obs)

	require.NoError(t, svc.SetSetting(context.Background(), "OpenAiKey", "new-key"))
	require.Len(t, obs.calls, 1)
	assert.Equal(t, "OpenAiKey=new-key", obs.calls[0])
}

func TestSystemService_ObserverErrorPropagated(t *testing.T) {
	svc := setupSystemService(t)
	obs := &recordingObserver{err: errors.New("reload failed")}
	svc.RegisterObserver(obs)

	err := svc.SetSetting(context.Background(), "OpenAiKey", "new-key")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reload failed")

	// DB still persisted even though observer returned an error
	val, err := svc.GetSetting(context.Background(), "OpenAiKey")
	require.NoError(t, err)
	assert.Equal(t, "new-key", val)
}

func TestSystemService_ImplementsSettingsReader(t *testing.T) {
	svc := setupSystemService(t)
	// Compile-time check: SystemService satisfies providers.SettingsReader
	var _ providers.SettingsReader = svc
}
