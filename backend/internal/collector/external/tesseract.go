package external

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/odysseythink/hermind/backend/internal/collector/utils"
)

// TesseractAdapter wraps the tesseract CLI for OCR.
type TesseractAdapter struct {
	cacheDir    string
	shellRunner *utils.ShellRunner
}

// NewTesseractAdapter creates a new TesseractAdapter.
func NewTesseractAdapter(cacheDir string, shellRunner *utils.ShellRunner) *TesseractAdapter {
	return &TesseractAdapter{
		cacheDir:    cacheDir,
		shellRunner: shellRunner,
	}
}

// Available returns true if tesseract is installed and in PATH.
func (t *TesseractAdapter) Available() bool {
	return t.shellRunner.CheckInstalled("tesseract")
}

// Run executes tesseract on the given image file with the specified languages.
func (t *TesseractAdapter) Run(ctx context.Context, imagePath string, langs []string) (string, error) {
	args := []string{imagePath, "stdout"}
	if len(langs) > 0 {
		args = append(args, "-l", strings.Join(langs, "+"))
	}

	env := os.Environ()
	if t.cacheDir != "" {
		env = append(env, fmt.Sprintf("TESSDATA_PREFIX=%s", t.cacheDir))
	}
	return t.shellRunner.RunWithEnvAndTimeout(ctx, utils.DefaultTimeoutTesseract, env, "tesseract", args...)
}
