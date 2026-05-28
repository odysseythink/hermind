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

var testOpenAITTSBaseURL string

// SetTestOpenAITTSBaseURL overrides the OpenAI TTS API base URL for tests.
func SetTestOpenAITTSBaseURL(u string) { testOpenAITTSBaseURL = u }

type openAIProvider struct {
	apiKey string
	model  string
	voice  string
	http   *http.Client
}

// NewOpenAIProvider creates an OpenAI TTS provider.
func NewOpenAIProvider(cfg *config.Config, settings map[string]string) Provider {
	return &openAIProvider{
		apiKey: pick("OpenAiKey", settings, cfg.OpenAiKey),
		model:  pick("OpenAITTSModel", settings, cfg.OpenAITTSModel),
		voice:  pick("OpenAITTSVoice", settings, cfg.OpenAITTSVoice),
		http:   &http.Client{Timeout: 30 * time.Second},
	}
}

func (p *openAIProvider) Name() string    { return "openai" }
func (p *openAIProvider) Available() bool { return p.apiKey != "" }

func (p *openAIProvider) Synthesize(ctx context.Context, text string) (*Synthesis, error) {
	url := "https://api.openai.com/v1/audio/speech"
	if testOpenAITTSBaseURL != "" {
		url = testOpenAITTSBaseURL + "/v1/audio/speech"
	}
	body, _ := json.Marshal(map[string]any{
		"model": p.model,
		"input": text,
		"voice": p.voice,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("openai tts: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openai tts: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("openai tts HTTP %d: %s", resp.StatusCode, msg)
	}

	audio, err := io.ReadAll(io.LimitReader(resp.Body, 50<<20)) // 50 MiB cap
	if err != nil {
		return nil, fmt.Errorf("openai tts read: %w", err)
	}
	return &Synthesis{Audio: audio, ContentType: resp.Header.Get("Content-Type")}, nil
}
