package tts

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/odysseythink/hermind/backend/internal/config"
)

var testElevenLabsBaseURL string

// SetTestElevenLabsBaseURL overrides the ElevenLabs API base URL for tests.
func SetTestElevenLabsBaseURL(u string) { testElevenLabsBaseURL = u }

type elevenLabsProvider struct {
	apiKey  string
	voiceID string
	model   string
	http    *http.Client
}

// NewElevenLabsProvider creates an ElevenLabs TTS provider.
func NewElevenLabsProvider(cfg *config.Config, settings map[string]string) Provider {
	return &elevenLabsProvider{
		apiKey:  pick("ElevenLabsApiKey", settings, cfg.ElevenLabsAPIKey),
		voiceID: pick("ElevenLabsVoiceID", settings, cfg.ElevenLabsVoiceID),
		model:   pick("ElevenLabsModel", settings, cfg.ElevenLabsModel),
		http:    &http.Client{Timeout: 30 * time.Second},
	}
}

func (p *elevenLabsProvider) Name() string    { return "elevenlabs" }
func (p *elevenLabsProvider) Available() bool { return p.apiKey != "" && p.voiceID != "" }

func (p *elevenLabsProvider) Synthesize(ctx context.Context, text string) (*Synthesis, error) {
	base := "https://api.elevenlabs.io"
	if testElevenLabsBaseURL != "" {
		base = testElevenLabsBaseURL
	}
	url := fmt.Sprintf("%s/v1/text-to-speech/%s", base, p.voiceID)
	body, _ := json.Marshal(map[string]any{
		"text": text,
		"model_id": p.model,
		"voice_settings": map[string]float64{
			"stability":       0.5,
			"similarity_boost": 0.5,
		},
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("elevenlabs: %w", err)
	}
	req.Header.Set("xi-api-key", p.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("elevenlabs: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("elevenlabs HTTP %d: %s", resp.StatusCode, msg)
	}

	audio, err := io.ReadAll(io.LimitReader(resp.Body, 50<<20)) // 50 MiB cap
	if err != nil {
		return nil, fmt.Errorf("elevenlabs read: %w", err)
	}
	return &Synthesis{Audio: audio, ContentType: "audio/mpeg"}, nil
}
