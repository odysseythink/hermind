package tts

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/odysseythink/hermind/backend/internal/config"
)

var testOpenAIGenericBaseURL string

// SetTestOpenAIGenericBaseURL overrides the OpenAI Generic TTS API base URL for tests.
func SetTestOpenAIGenericBaseURL(u string) { testOpenAIGenericBaseURL = u }

type openAIGenericProvider struct {
	apiKey   string
	endpoint string
	model    string
	voice    string
	http     *http.Client
}

// NewOpenAIGenericProvider creates an OpenAI-compatible TTS provider.
func NewOpenAIGenericProvider(cfg *config.Config, settings map[string]string) Provider {
	return &openAIGenericProvider{
		apiKey:   pick("TTSOpenAICompatKey", settings, cfg.TTSOpenAICompatKey),
		endpoint: pick("TTSOpenAICompatEndpoint", settings, cfg.TTSOpenAICompatEndpoint),
		model:    pick("TTSOpenAICompatModel", settings, cfg.TTSOpenAICompatModel),
		voice:    pick("TTSOpenAICompatVoice", settings, cfg.TTSOpenAICompatVoice),
		http:     &http.Client{Timeout: 30 * time.Second},
	}
}

func (p *openAIGenericProvider) Name() string    { return "openai-generic" }
func (p *openAIGenericProvider) Available() bool { return p.endpoint != "" }

func (p *openAIGenericProvider) Synthesize(ctx context.Context, text string) (*Synthesis, error) {
	base := p.endpoint
	if testOpenAIGenericBaseURL != "" {
		base = testOpenAIGenericBaseURL
	}
	base = strings.TrimRight(base, "/")
	url := base + "/audio/speech"

	body, _ := json.Marshal(map[string]any{
		"model": p.model,
		"input": text,
		"voice": p.voice,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("openai-generic tts: %w", err)
	}
	if p.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.apiKey)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openai-generic tts: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("openai-generic tts HTTP %d: %s", resp.StatusCode, msg)
	}

	audio, err := io.ReadAll(io.LimitReader(resp.Body, 50<<20)) // 50 MiB cap
	if err != nil {
		return nil, fmt.Errorf("openai-generic tts read: %w", err)
	}
	return &Synthesis{Audio: audio, ContentType: resp.Header.Get("Content-Type")}, nil
}
