// tool/terminal/docker_test.go
package terminal

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDockerRequiresImage(t *testing.T) {
	_, err := NewDocker(Config{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "docker_image")
}

func TestDockerBuildArgsMinimal(t *testing.T) {
	d := &Docker{image: "alpine:3.19"}
	args := d.buildArgs(&ExecOptions{}, "echo hi")
	assert.Equal(t, []string{
		"run", "--rm", "-i",
		"alpine:3.19", "sh", "-c", "echo hi",
	}, args)
}

func TestDockerBuildArgsWithCwd(t *testing.T) {
	d := &Docker{image: "alpine:3.19"}
	args := d.buildArgs(&ExecOptions{Cwd: "/app"}, "pwd")
	assert.Equal(t, []string{
		"run", "--rm", "-i",
		"--workdir", "/app",
		"alpine:3.19", "sh", "-c", "pwd",
	}, args)
}

func TestDockerBuildArgsWithVolumes(t *testing.T) {
	d := &Docker{
		image:   "alpine:3.19",
		volumes: []string{"/host/src:/workspace", "/tmp:/tmp"},
	}
	args := d.buildArgs(&ExecOptions{}, "ls")
	assert.Contains(t, args, "--volume")
	assert.Contains(t, args, "/host/src:/workspace")
	assert.Contains(t, args, "/tmp:/tmp")
}

func TestDockerBuildArgsWithEnv(t *testing.T) {
	d := &Docker{image: "alpine:3.19"}
	args := d.buildArgs(&ExecOptions{Env: map[string]string{"FOO": "bar"}}, "printenv FOO")
	assert.Contains(t, args, "--env")
	assert.Contains(t, args, "FOO=bar")
}

func TestDockerSupportsPersistentShellIsFalse(t *testing.T) {
	d := &Docker{image: "alpine:3.19"}
	assert.False(t, d.SupportsPersistentShell())
	assert.NoError(t, d.Close())
}
