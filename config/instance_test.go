package config

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInstanceRoot_CwdDefault(t *testing.T) {
	t.Setenv("HERMIND_HOME", "")
	tmp := t.TempDir()
	t.Chdir(tmp)

	got, err := InstanceRoot()
	require.NoError(t, err)
	// macOS tmp dirs may be reported as /private/var/... — resolve both sides.
	wantAbs, err := filepath.EvalSymlinks(tmp)
	require.NoError(t, err)
	gotAbs, err := filepath.EvalSymlinks(filepath.Dir(got))
	require.NoError(t, err)
	assert.Equal(t, wantAbs, gotAbs)
	assert.Equal(t, ".hermind", filepath.Base(got))
}

func TestInstanceRoot_HermindHomeOverride(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HERMIND_HOME", tmp)

	got, err := InstanceRoot()
	require.NoError(t, err)
	assert.Equal(t, tmp, got)
}

func TestInstanceRoot_HermindHomeTrimsWhitespace(t *testing.T) {
	t.Setenv("HERMIND_HOME", "   ")
	tmp := t.TempDir()
	t.Chdir(tmp)

	got, err := InstanceRoot()
	require.NoError(t, err)
	wantAbs, err := filepath.EvalSymlinks(tmp)
	require.NoError(t, err)
	gotAbs, err := filepath.EvalSymlinks(filepath.Dir(got))
	require.NoError(t, err)
	assert.Equal(t, wantAbs, gotAbs)
	assert.Equal(t, ".hermind", filepath.Base(got))
}

func TestInstancePath_JoinsComponents(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HERMIND_HOME", tmp)

	got, err := InstancePath("skills", "enabled.yaml")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(tmp, "skills", "enabled.yaml"), got)
}
