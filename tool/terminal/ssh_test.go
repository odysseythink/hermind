// tool/terminal/ssh_test.go
package terminal

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSSHRequiresHost(t *testing.T) {
	_, err := NewSSH(Config{SSHUser: "u", SSHKey: "/tmp/k"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ssh_host")
}

func TestNewSSHRequiresUser(t *testing.T) {
	_, err := NewSSH(Config{SSHHost: "h", SSHKey: "/tmp/k"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ssh_user")
}

func TestNewSSHRequiresKey(t *testing.T) {
	_, err := NewSSH(Config{SSHHost: "h", SSHUser: "u"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ssh_key")
}

func TestNewSSHParsesHostPort(t *testing.T) {
	s, err := NewSSH(Config{SSHHost: "h:2222", SSHUser: "u", SSHKey: "/tmp/k"})
	require.NoError(t, err)
	assert.Equal(t, "h", s.host)
	assert.Equal(t, 2222, s.port)
}

func TestNewSSHDefaultPort(t *testing.T) {
	s, err := NewSSH(Config{SSHHost: "h", SSHUser: "u", SSHKey: "/tmp/k"})
	require.NoError(t, err)
	assert.Equal(t, 22, s.port)
}

func TestSSHSupportsPersistentShellIsFalse(t *testing.T) {
	s, err := NewSSH(Config{SSHHost: "h", SSHUser: "u", SSHKey: "/tmp/k"})
	require.NoError(t, err)
	assert.False(t, s.SupportsPersistentShell())
	assert.NoError(t, s.Close())
}

func TestBuildWrappedCommandSimple(t *testing.T) {
	got := buildWrappedCommand("echo hi", &ExecOptions{})
	assert.Equal(t, "echo hi", got)
}

func TestBuildWrappedCommandWithCwd(t *testing.T) {
	got := buildWrappedCommand("echo hi", &ExecOptions{Cwd: "/tmp"})
	assert.Equal(t, "cd '/tmp' && echo hi", got)
}

func TestBuildWrappedCommandWithEnv(t *testing.T) {
	got := buildWrappedCommand("echo $FOO", &ExecOptions{
		Env: map[string]string{"FOO": "bar"},
	})
	assert.Contains(t, got, "export FOO='bar'")
	assert.Contains(t, got, "echo $FOO")
}

func TestShellQuoteEscapesQuotes(t *testing.T) {
	assert.Equal(t, `'hello'`, shellQuote("hello"))
	assert.Equal(t, `'it'"'"'s'`, shellQuote("it's"))
}
