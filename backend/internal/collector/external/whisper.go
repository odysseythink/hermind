package external

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/odysseythink/hermind/backend/internal/collector/utils"
)

// WhisperLocalAdapter wraps a local whisper.cpp binary.
type WhisperLocalAdapter struct {
	modelPath   string
	shellRunner *utils.ShellRunner
}

// NewWhisperLocalAdapter creates a new WhisperLocalAdapter.
func NewWhisperLocalAdapter(modelPath string, shellRunner *utils.ShellRunner) *WhisperLocalAdapter {
	return &WhisperLocalAdapter{
		modelPath:   modelPath,
		shellRunner: shellRunner,
	}
}

// Available returns true if a whisper binary (main or whisper) is in PATH.
func (w *WhisperLocalAdapter) Available() bool {
	return w.shellRunner.CheckInstalled("main") || w.shellRunner.CheckInstalled("whisper")
}

// Transcribe runs the local whisper binary on the given WAV file.
func (w *WhisperLocalAdapter) Transcribe(ctx context.Context, wavPath string) (string, error) {
	bin := "main"
	if !w.shellRunner.CheckInstalled(bin) {
		if w.shellRunner.CheckInstalled("whisper") {
			bin = "whisper"
		} else {
			return "", fmt.Errorf("no whisper binary found in PATH")
		}
	}

	args := []string{"-m", w.modelPath, "-f", wavPath}
	out, err := w.shellRunner.RunWithTimeout(ctx, utils.DefaultTimeoutWhisper, bin, args...)
	if err != nil {
		return "", fmt.Errorf("whisper transcription failed: %w", err)
	}
	return out, nil
}

// WhisperOpenAIAdapter wraps the OpenAI Whisper API.
type WhisperOpenAIAdapter struct {
	apiKey     string
	httpClient *http.Client
}

// NewWhisperOpenAIAdapter creates a new WhisperOpenAIAdapter.
func NewWhisperOpenAIAdapter(apiKey string) *WhisperOpenAIAdapter {
	return &WhisperOpenAIAdapter{
		apiKey:     apiKey,
		httpClient: &http.Client{},
	}
}

// Transcribe sends the audio file to the OpenAI transcription endpoint.
func (w *WhisperOpenAIAdapter) Transcribe(ctx context.Context, filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("open audio file: %w", err)
	}
	defer file.Close()

	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)

	go func() {
		defer pw.Close()
		part, err := writer.CreateFormFile("file", filepath.Base(filePath))
		if err != nil {
			pw.CloseWithError(fmt.Errorf("create form file: %w", err))
			return
		}
		if _, err := io.Copy(part, file); err != nil {
			pw.CloseWithError(fmt.Errorf("copy file to form: %w", err))
			return
		}
		_ = writer.WriteField("model", "whisper-1")
		_ = writer.WriteField("temperature", "0")
		if err := writer.Close(); err != nil {
			pw.CloseWithError(fmt.Errorf("close multipart writer: %w", err))
			return
		}
	}()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.openai.com/v1/audio/transcriptions", pr)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+w.apiKey)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := w.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("openai api error %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("unmarshal OpenAI response: %w", err)
	}
	return strings.TrimSpace(result.Text), nil
}
