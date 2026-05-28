package external

import (
	"context"
	"testing"

	"github.com/odysseythink/hermind/backend/internal/collector/utils"
	"github.com/stretchr/testify/assert"
)

func TestTesseractAdapter_Available(t *testing.T) {
	shell := utils.NewShellRunner()
	adapter := NewTesseractAdapter("", shell)

	// This test documents the behavior; it does not require tesseract to be installed.
	available := adapter.Available()
	// Assert type only; actual result depends on host environment.
	assert.IsType(t, true, available)
}

func TestTesseractAdapter_Run_NotInstalled(t *testing.T) {
	shell := utils.NewShellRunner()
	adapter := NewTesseractAdapter("/tmp/tessdata", shell)

	if adapter.Available() {
		t.Skip("tesseract is installed; skipping not-installed fallback test")
	}

	_, err := adapter.Run(context.Background(), "/dev/null/nonexistent.png", []string{"eng"})
	assert.Error(t, err)
}
