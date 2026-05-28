package processors

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/odysseythink/hermind/backend/internal/collector/core"
	"github.com/odysseythink/hermind/backend/internal/collector/external"
	"github.com/odysseythink/hermind/backend/internal/collector/pipeline"
	"github.com/odysseythink/hermind/backend/internal/collector/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeMinimalWAV creates a minimal valid 16-bit mono WAV file.
func writeMinimalWAV(t *testing.T, path string) {
	t.Helper()
	// WAV header for 1 second of silence at 16000 Hz, 16-bit mono.
	header := []byte{
		'R', 'I', 'F', 'F',
		0x24, 0x00, 0x00, 0x00, // chunk size
		'W', 'A', 'V', 'E',
		'f', 'm', 't', ' ',
		0x10, 0x00, 0x00, 0x00, // subchunk1 size (16)
		0x01, 0x00, // audio format (PCM)
		0x01, 0x00, // num channels (1)
		0x80, 0x3e, 0x00, 0x00, // sample rate (16000)
		0x00, 0x7d, 0x00, 0x00, // byte rate (32000)
		0x02, 0x00, // block align
		0x10, 0x00, // bits per sample (16)
		'd', 'a', 't', 'a',
		0x00, 0x00, 0x00, 0x00, // data chunk size (0 bytes of audio)
	}
	require.NoError(t, os.WriteFile(path, header, 0644))
}

func TestAudioExtractor_Supports(t *testing.T) {
	e := NewAudioExtractor(nil, nil, utils.NewShellRunner())
	assert.True(t, e.Supports(".mp3"))
	assert.True(t, e.Supports(".wav"))
	assert.True(t, e.Supports(".mp4"))
	assert.True(t, e.Supports(".mpeg"))
	assert.True(t, e.Supports(".ogg"))
	assert.True(t, e.Supports(".oga"))
	assert.True(t, e.Supports(".opus"))
	assert.True(t, e.Supports(".m4a"))
	assert.True(t, e.Supports(".webm"))
	assert.False(t, e.Supports(".txt"))
	assert.False(t, e.Supports(".pdf"))
}

func TestAudioExtractor_Extract_NoProvider(t *testing.T) {
	e := NewAudioExtractor(nil, nil, utils.NewShellRunner())
	_, err := e.Extract(context.Background(), pipeline.ExtractInput{FilePath: "test.mp3"})
	assert.ErrorIs(t, err, core.ErrTranscriptionFailed)
}

func TestAudioExtractor_Extract_FFMpegNotInstalled(t *testing.T) {
	shell := utils.NewShellRunner()
	if shell.CheckInstalled("ffmpeg") {
		t.Skip("ffmpeg is installed; skipping not-installed test")
	}

	local := external.NewWhisperLocalAdapter("/tmp/model.bin", shell)
	e := NewAudioExtractor(local, nil, shell)

	_, err := e.Extract(context.Background(), pipeline.ExtractInput{FilePath: "test.mp3"})
	assert.Error(t, err)
}

func TestAudioExtractor_ResolveProvider_Local(t *testing.T) {
	shell := utils.NewShellRunner()
	local := external.NewWhisperLocalAdapter("/tmp/model.bin", shell)
	openAI := external.NewWhisperOpenAIAdapter("key")

	e := NewAudioExtractor(local, openAI, shell)

	provider, err := e.resolveProvider(core.Options{
		WhisperProvider: "local",
	})
	if local.Available() {
		require.NoError(t, err)
		assert.NotNil(t, provider)
	} else {
		assert.ErrorIs(t, err, core.ErrTranscriptionFailed)
	}
}

func TestAudioExtractor_ResolveProvider_OpenAI(t *testing.T) {
	shell := utils.NewShellRunner()
	local := external.NewWhisperLocalAdapter("/tmp/model.bin", shell)
	openAI := external.NewWhisperOpenAIAdapter("key")

	e := NewAudioExtractor(local, openAI, shell)

	provider, err := e.resolveProvider(core.Options{
		WhisperProvider: "openai",
		OpenAiKey:       "test-key",
	})
	require.NoError(t, err)
	assert.NotNil(t, provider)
}

func TestAudioExtractor_Extract_WithWAV(t *testing.T) {
	shell := utils.NewShellRunner()
	if !shell.CheckInstalled("ffmpeg") {
		t.Skip("ffmpeg is not installed; skipping integration test")
	}

	local := external.NewWhisperLocalAdapter("/tmp/model.bin", shell)
	if !local.Available() {
		t.Skip("whisper binary is not installed; skipping integration test")
	}

	e := NewAudioExtractor(local, nil, shell)

	tmpDir := t.TempDir()
	wavPath := filepath.Join(tmpDir, "test.wav")
	writeMinimalWAV(t, wavPath)

	_, err := e.Extract(context.Background(), pipeline.ExtractInput{FilePath: wavPath})
	// Transcription of silence may fail or return empty; either is acceptable.
	if err != nil {
		assert.ErrorIs(t, err, core.ErrTranscriptionFailed)
	}
}
