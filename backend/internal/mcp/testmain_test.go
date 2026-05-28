package mcp

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestMain(m *testing.M) {
	tmp, err := os.MkdirTemp("", "mcp-echo-bin-")
	if err != nil {
		os.Exit(1)
	}
	defer os.RemoveAll(tmp)

	binPath := filepath.Join(tmp, "echo-mcp")
	if runtime.GOOS == "windows" {
		binPath += ".exe"
	}

	echoDir, err := filepath.Abs("./testdata/echo-mcp")
	if err != nil {
		os.Exit(1)
	}
	cmd := exec.Command("go", "build", "-C", echoDir, "-o", binPath, ".")
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		os.Exit(1)
	}
	os.Setenv("MCP_ECHO_BIN", binPath)
	os.Exit(m.Run())
}
