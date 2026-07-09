package providers

import "context"

// SettingObserver is notified by SystemService after a setting is written.
type SettingObserver interface {
	OnSettingChanged(ctx context.Context, key, value string) error
}

// SettingsReader is the minimal interface ManagedLLMProvider needs to read
// all settings for a reload. Implemented by services.SystemService to avoid
// a providers → services import cycle.
type SettingsReader interface {
	GetAllSettings(ctx context.Context) (map[string]string, error)
}
