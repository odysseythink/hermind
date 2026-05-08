package copilot

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// buildFakeCopilot compiles the test helper program located at
// testdata/fake_copilot/main.go and returns its path. The helper
// acts like the Copilot CLI for protocol testing: it reads
// newline-delimited JSON-RPC from stdin and replies with canned
// frames from testdata/.
func buildFakeCopilot(t *testing.T) string {
	t.Helper()
	src := filepath.Join("testdata", "fake_copilot", "main.go")
	bin := filepath.Join(t.TempDir(), "fake_copilot"+exeSuffix())
	cmd := exec.Command("go", "build", "-o", bin, src)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Skipf("cannot build fake copilot binary: %v", err)
	}
	return bin
}

func exeSuffix() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}
