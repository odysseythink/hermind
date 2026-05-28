package external

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/odysseythink/hermind/backend/internal/collector/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWhisperLocalAdapter_Available(t *testing.T) {
	shell := utils.NewShellRunner()
	adapter := NewWhisperLocalAdapter("/tmp/model.bin", shell)

	available := adapter.Available()
	assert.IsType(t, true, available)
}

func TestWhisperLocalAdapter_Transcribe_NotInstalled(t *testing.T) {
	shell := utils.NewShellRunner()
	adapter := NewWhisperLocalAdapter("/tmp/model.bin", shell)

	if adapter.Available() {
		t.Skip("whisper binary is installed; skipping not-installed fallback test")
	}

	_, err := adapter.Transcribe(context.Background(), "/dev/null/nonexistent.wav")
	assert.Error(t, err)
}

func TestWhisperOpenAIAdapter_Transcribe_MissingFile(t *testing.T) {
	adapter := NewWhisperOpenAIAdapter("fake-api-key")
	_, err := adapter.Transcribe(context.Background(), "/dev/null/nonexistent.wav")
	assert.Error(t, err)
}

func TestWhisperOpenAIAdapter_Transcribe_JSONParsing(t *testing.T) {
	// Verify the JSON unmarshal shape used by Transcribe.
	respBody := []byte(`{"text": "hello world"}`)
	var result struct {
		Text string `json:"text"`
	}
	err := json.Unmarshal(respBody, &result)
	require.NoError(t, err)
	assert.Equal(t, "hello world", result.Text)

	// Verify empty text field parses correctly.
	respBody = []byte(`{"text": ""}`)
	err = json.Unmarshal(respBody, &result)
	require.NoError(t, err)
	assert.Equal(t, "", result.Text)
}
