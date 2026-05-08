// tool/terminal/singularity_test.go
package terminal

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSingularityRequiresImage(t *testing.T) {
	_, err := NewSingularity(Config{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "singularity_image")
}

func TestSingularityBuildArgsMinimal(t *testing.T) {
	s := &Singularity{image: "/opt/ubuntu.sif"}
	args := s.buildArgs(&ExecOptions{}, "echo hi")
	assert.Equal(t, []string{
		"exec",
		"/opt/ubuntu.sif", "sh", "-c", "echo hi",
	}, args)
}

func TestSingularityBuildArgsWithCwd(t *testing.T) {
	s := &Singularity{image: "/opt/ubuntu.sif"}
	args := s.buildArgs(&ExecOptions{Cwd: "/workspace"}, "pwd")
	assert.Contains(t, args, "--pwd")
	assert.Contains(t, args, "/workspace")
}

func TestSingularityBuildArgsWithEnv(t *testing.T) {
	s := &Singularity{image: "/opt/ubuntu.sif"}
	args := s.buildArgs(&ExecOptions{Env: map[string]string{"FOO": "bar"}}, "printenv FOO")
	assert.Contains(t, args, "--env")
	assert.Contains(t, args, "FOO=bar")
}

func TestSingularitySupportsPersistentShellIsFalse(t *testing.T) {
	s := &Singularity{image: "/opt/ubuntu.sif"}
	assert.False(t, s.SupportsPersistentShell())
	assert.NoError(t, s.Close())
}
