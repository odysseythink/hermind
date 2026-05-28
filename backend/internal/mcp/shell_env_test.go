package mcp

import (
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func skipWindows(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("posix-only")
	}
}

func TestShellEnv_FallbackOnEmptyShellVar(t *testing.T) {
	skipWindows(t)
	t.Setenv("SHELL", "")
	m := shellEnv(t.Context())
	require.NotNil(t, m)
	assert.NotEmpty(t, m)
}

func TestShellEnv_FallbackOnShellError(t *testing.T) {
	skipWindows(t)
	t.Setenv("SHELL", "/nonexistent/bin/sh")
	m := shellEnv(t.Context())
	require.NotNil(t, m)
	assert.NotEmpty(t, m)
}

func TestShellEnv_Timeout(t *testing.T) {
	skipWindows(t)
	start := time.Now()
	t.Setenv("SHELL", "./testdata/slow-shell.sh")
	m := shellEnv(t.Context())
	elapsed := time.Since(start)
	require.NotNil(t, m)
	assert.Less(t, elapsed, 6*time.Second, "should return within 5s timeout + slack")
	assert.NotContains(t, m, "should-not-be-reached")
}

func TestParseEnvOutput_Standard(t *testing.T) {
	m := parseEnvOutput("PATH=/bin\nFOO=bar\n")
	assert.Equal(t, map[string]string{"PATH": "/bin", "FOO": "bar"}, m)
}

func TestParseEnvOutput_MultilineValue(t *testing.T) {
	// For now we only handle simple single-line values; this test documents current behaviour.
	m := parseEnvOutput("SINGLE=line\nMULTI=first\nsecond\n")
	assert.Equal(t, "line", m["SINGLE"])
	assert.Equal(t, "first", m["MULTI"])
}

func TestParseEnvOutput_SkipEmpty(t *testing.T) {
	m := parseEnvOutput("\n\nA=1\n\nB=2\n")
	assert.Equal(t, map[string]string{"A": "1", "B": "2"}, m)
}

func TestParseEnvOutput_SkipMalformed(t *testing.T) {
	m := parseEnvOutput("NOEQUALSIGN\nVALID=yes\n")
	assert.Equal(t, map[string]string{"VALID": "yes"}, m)
}

func TestBuildServerEnv_UserEnvOverridesShell(t *testing.T) {
	srv := &ServerConfig{
		Env: map[string]string{"PATH": "/custom/bin"},
	}
	out := buildServerEnv(srv)
	m := envSliceToMap(out)
	assert.Equal(t, "/custom/bin", m["PATH"])
}

func TestBuildServerEnv_DockerOverrides(t *testing.T) {
	t.Setenv("HERMIND_RUNTIME", "docker")
	// Start from empty shell env by using a server with no env.
	srv := &ServerConfig{Env: map[string]string{}}
	out := buildServerEnv(srv)
	m := envSliceToMap(out)
	assert.NotEmpty(t, m["PATH"])
	assert.NotEmpty(t, m["NODE_PATH"])
}

func TestBuildServerEnv_PassthroughOSEnv(t *testing.T) {
	srv := &ServerConfig{}
	out := buildServerEnv(srv)
	m := envSliceToMap(out)
	// Should contain at least some of the current process environment.
	assert.NotEmpty(t, m)
}

func envSliceToMap(slice []string) map[string]string {
	m := make(map[string]string, len(slice))
	for _, kv := range slice {
		if i := indexByte(kv, '='); i > 0 {
			m[kv[:i]] = kv[i+1:]
		}
	}
	return m
}

func indexByte(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}
