// provider/openaicompat/embed.go
package openaicompat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// Embed calls the OpenAI-compatible /embeddings endpoint.
// model is the embedding model name (e.g. "text-embedding-3-small").
func (c *Client) Embed(ctx context.Context, model, text string) ([]float32, error) {
	body, err := json.Marshal(map[string]any{
		"model": model,
		"input": text,
	})
	if err != nil {
		return nil, fmt.Errorf("openaicompat: embed marshal: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.cfg.BaseURL+"/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("openaicompat: embed request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openaicompat: embed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openaicompat: embed: status %d", resp.StatusCode)
	}

	var result struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("openaicompat: embed decode: %w", err)
	}
	if len(result.Data) == 0 {
		return nil, fmt.Errorf("openaicompat: embed: empty response")
	}
	return result.Data[0].Embedding, nil
}
