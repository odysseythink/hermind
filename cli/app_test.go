package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewApp_HermindHomeOverride(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HERMIND_HOME", dir)

	cfgPath := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte("model: anthropic/claude-opus-4-6\n"), 0o644))

	app, err := NewApp()
	require.NoError(t, err)
	defer app.Close()

	assert.Equal(t, cfgPath, app.ConfigPath)
	assert.Equal(t, dir, app.InstanceRoot)
}

func TestNewApp_CwdFirstRunWritesDefault(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HERMIND_HOME", "")
	t.Chdir(tmp)
	// Also isolate HOME so the migration-notice path in NewApp doesn't read
	// the developer's real ~/.hermind.
	t.Setenv("HOME", t.TempDir())

	app, err := NewApp()
	require.NoError(t, err)
	defer app.Close()

	expected := filepath.Join(app.InstanceRoot, "config.yaml")
	assert.Equal(t, expected, app.ConfigPath)

	_, err = os.Stat(expected)
	assert.NoError(t, err, "first-run should write default config.yaml")
}

func TestNewApp_MigrationNoticeFiresOnceWhenHomeHermindExists(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HERMIND_HOME", "")
	t.Chdir(tmp)

	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)
	require.NoError(t, os.MkdirAll(filepath.Join(fakeHome, ".hermind"), 0o755))

	app, err := NewApp()
	require.NoError(t, err)
	root := app.InstanceRoot
	app.Close()

	marker := filepath.Join(root, ".migration_notice_shown")
	_, err = os.Stat(marker)
	assert.NoError(t, err, "first boot should create .migration_notice_shown marker")

	app2, err := NewApp()
	require.NoError(t, err)
	app2.Close()
}
