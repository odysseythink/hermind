package processors

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/odysseythink/hermind/backend/internal/collector/core"
	"github.com/odysseythink/hermind/backend/internal/collector/external"
	"github.com/odysseythink/hermind/backend/internal/collector/pipeline"
	"github.com/odysseythink/hermind/backend/internal/collector/utils"
)

// AudioExtractor extracts text from audio/video files via speech-to-text.
type AudioExtractor struct {
	localAdapter  *external.WhisperLocalAdapter
	openAIAdapter *external.WhisperOpenAIAdapter
	shellRunner   *utils.ShellRunner
}

// NewAudioExtractor creates a new AudioExtractor.
func NewAudioExtractor(local *external.WhisperLocalAdapter, openAI *external.WhisperOpenAIAdapter, shellRunner *utils.ShellRunner) *AudioExtractor {
	return &AudioExtractor{
		localAdapter:  local,
		openAIAdapter: openAI,
		shellRunner:   shellRunner,
	}
}

// Supports returns true for supported audio/video extensions.
func (e *AudioExtractor) Supports(ext string) bool {
	switch ext {
	case ".mp3", ".wav", ".mp4", ".mpeg", ".ogg", ".oga", ".opus", ".m4a", ".webm":
		return true
	}
	return false
}

// Extract converts the audio to WAV and transcribes it.
func (e *AudioExtractor) Extract(ctx context.Context, input pipeline.ExtractInput) (*pipeline.ExtractOutput, error) {
	provider, err := e.resolveProvider(input.Options)
	if err != nil {
		return nil, err
	}

	// Convert input to 16kHz mono WAV.
	tmpWav, err := os.CreateTemp("", "collector-audio-*.wav")
	if err != nil {
		return nil, fmt.Errorf("create temp wav: %w", err)
	}
	tmpWavPath := tmpWav.Name()
	tmpWav.Close()
	defer os.Remove(tmpWavPath)

	_, err = e.shellRunner.RunWithTimeout(ctx, utils.DefaultTimeoutFFmpeg, "ffmpeg", "-i", input.FilePath, "-ar", "16000", "-ac", "1", "-sample_fmt", "s16", "-y", tmpWavPath)
	if err != nil {
		return nil, fmt.Errorf("ffmpeg conversion failed: %w", err)
	}

	text, err := provider.Transcribe(ctx, tmpWavPath)
	if err != nil {
		return nil, core.ErrTranscriptionFailed
	}

	return &pipeline.ExtractOutput{
		Content: strings.TrimSpace(text),
	}, nil
}

type whisperProvider interface {
	Transcribe(ctx context.Context, filePath string) (string, error)
}

func (e *AudioExtractor) resolveProvider(opts core.Options) (whisperProvider, error) {
	if opts.WhisperProvider == "" {
		// No options configured; try local first, then fallback.
		if e.localAdapter != nil && e.localAdapter.Available() {
			return e.localAdapter, nil
		}
		if e.openAIAdapter != nil {
			return e.openAIAdapter, nil
		}
		return nil, core.ErrTranscriptionFailed
	}

	switch opts.WhisperProvider {
	case "local":
		if e.localAdapter != nil && e.localAdapter.Available() {
			return e.localAdapter, nil
		}
		return nil, core.ErrTranscriptionFailed
	case "openai":
		if e.openAIAdapter == nil || opts.OpenAiKey == "" {
			return nil, core.ErrTranscriptionFailed
		}
		return e.openAIAdapter, nil
	default:
		// Auto-detect: prefer local if available, else openai if configured.
		if e.localAdapter != nil && e.localAdapter.Available() {
			return e.localAdapter, nil
		}
		if e.openAIAdapter != nil && opts.OpenAiKey != "" {
			return e.openAIAdapter, nil
		}
		return nil, core.ErrTranscriptionFailed
	}
}
