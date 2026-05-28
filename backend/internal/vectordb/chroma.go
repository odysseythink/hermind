package vectordb

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"time"
)

var (
	chromaInvalidCharRegex  = regexp.MustCompile(`[^a-zA-Z0-9_-]`)
	chromaDotRegex          = regexp.MustCompile(`\.+`)
	chromaStartEndCharRegex = regexp.MustCompile(`^[a-zA-Z0-9]$`)
	chromaIPAddressRegex    = regexp.MustCompile(`^\d+\.\d+\.\d+\.\d+$`)
)

// isValidChromaCollectionName mirrors the logic of the original PCRE regex:
// ^(?!\d+\.\d+\.\d+\.\d+$)(?!.*\.\.)(?=^[a-zA-Z0-9][a-zA-Z0-9_-]{1,61}[a-zA-Z0-9]$).{3,63}$
func isValidChromaCollectionName(name string) bool {
	if len(name) < 3 || len(name) > 63 {
		return false
	}
	if chromaIPAddressRegex.MatchString(name) {
		return false
	}
	if bytes.Contains([]byte(name), []byte("..")) {
		return false
	}
	first := name[0]
	last := name[len(name)-1]
	if !isChromaValidChar(first) || !isChromaValidChar(last) {
		return false
	}
	for i := 1; i < len(name)-1; i++ {
		c := name[i]
		if !isChromaValidChar(c) && c != '_' && c != '-' {
			return false
		}
	}
	return true
}

func isChromaValidChar(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

type Chroma struct {
	endpoint  string
	apiHeader string
	apiKey    string
	client    *http.Client
}

func NewChroma(endpoint, apiHeader, apiKey string) *Chroma {
	return &Chroma{
		endpoint:  endpoint,
		apiHeader: apiHeader,
		apiKey:    apiKey,
		client:    &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Chroma) Name() string { return "chroma" }

func (c *Chroma) Connect(ctx context.Context) error {
	return nil // HTTP client is stateless
}

func (c *Chroma) Heartbeat(ctx context.Context) (map[string]any, error) {
	return map[string]any{"name": "chroma", "endpoint": c.endpoint}, nil
}

func (c *Chroma) normalize(input string) string {
	if isValidChromaCollectionName(input) {
		return input
	}
	normalized := chromaInvalidCharRegex.ReplaceAllString(input, "-")
	normalized = chromaDotRegex.ReplaceAllString(normalized, ".")
	if len(normalized) > 0 && !chromaStartEndCharRegex.MatchString(normalized[:1]) {
		normalized = "anythingllm-" + normalized[1:]
	}
	if len(normalized) > 0 && !chromaStartEndCharRegex.MatchString(normalized[len(normalized)-1:]) {
		normalized = normalized[:len(normalized)-1]
	}
	if len(normalized) < 3 {
		normalized = "anythingllm-" + normalized
	}
	if len(normalized) > 63 {
		normalized = c.normalize(normalized[:63])
	}
	if chromaIPAddressRegex.MatchString(normalized) {
		normalized = "-" + normalized[1:]
	}
	return normalized
}

func (c *Chroma) Tables(ctx context.Context) ([]string, error) {
	resp, err := c.doRequest(ctx, "GET", c.endpoint+"/api/v1/collections", nil)
	if err != nil {
		return nil, err
	}
	var collections []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(resp, &collections); err != nil {
		return nil, err
	}
	names := make([]string, len(collections))
	for i, col := range collections {
		names[i] = col.Name
	}
	return names, nil
}

func (c *Chroma) CountVectors(ctx context.Context, namespace string) (int64, error) {
	col, err := c.getCollection(ctx, namespace)
	if err != nil {
		return 0, nil
	}
	count, err := c.collectionCount(ctx, col.Name)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (c *Chroma) TotalVectors(ctx context.Context) (int64, error) {
	collections, err := c.Tables(ctx)
	if err != nil {
		return 0, err
	}
	var total int64
	for _, col := range collections {
		count, err := c.collectionCount(ctx, col)
		if err != nil {
			continue
		}
		total += count
	}
	return total, nil
}

func (c *Chroma) collectionCount(ctx context.Context, name string) (int64, error) {
	col, err := c.getCollection(ctx, name)
	if err != nil {
		return 0, err
	}
	resp, err := c.doRequest(ctx, "GET", fmt.Sprintf("%s/api/v1/collections/%s/count", c.endpoint, col.ID), nil)
	if err != nil {
		return 0, err
	}
	var count int64
	json.Unmarshal(resp, &count)
	return count, nil
}

func (c *Chroma) getCollection(ctx context.Context, name string) (*chromaCollection, error) {
	resp, err := c.doRequest(ctx, "GET", c.endpoint+"/api/v1/collections", nil)
	if err != nil {
		return nil, err
	}
	var collections []chromaCollection
	if err := json.Unmarshal(resp, &collections); err != nil {
		return nil, err
	}
	normalized := c.normalize(name)
	for _, col := range collections {
		if col.Name == normalized {
			return &col, nil
		}
	}
	return nil, fmt.Errorf("collection not found: %s", normalized)
}

func (c *Chroma) createCollection(ctx context.Context, name string, dims int) (*chromaCollection, error) {
	body, err := json.Marshal(map[string]any{
		"name":     c.normalize(name),
		"metadata": map[string]any{"hnsw:space": "cosine"},
	})
	if err != nil {
		return nil, fmt.Errorf("marshal create collection body: %w", err)
	}
	resp, err := c.doRequest(ctx, "POST", c.endpoint+"/api/v1/collections", body)
	if err != nil {
		return nil, err
	}
	var col chromaCollection
	if err := json.Unmarshal(resp, &col); err != nil {
		return nil, err
	}
	return &col, nil
}

func (c *Chroma) AddVectors(ctx context.Context, namespace string, chunks []VectorChunk) error {
	if len(chunks) == 0 {
		return nil
	}

	col, err := c.getCollection(ctx, namespace)
	if err != nil {
		col, err = c.createCollection(ctx, namespace, len(chunks[0].Vector))
		if err != nil {
			return fmt.Errorf("create collection: %w", err)
		}
	}

	ids := make([]string, len(chunks))
	embeddings := make([][]float32, len(chunks))
	metadatas := make([]map[string]any, len(chunks))
	documents := make([]string, len(chunks))

	for i, ch := range chunks {
		ids[i] = ch.ID
		embeddings[i] = ch.Vector
		metadatas[i] = ch.Metadata
		text, _ := ch.Metadata["text"].(string)
		documents[i] = text
	}

	body, err := json.Marshal(map[string]any{
		"ids":        ids,
		"embeddings": embeddings,
		"metadatas":  metadatas,
		"documents":  documents,
	})
	if err != nil {
		return fmt.Errorf("marshal add vectors body: %w", err)
	}
	_, err = c.doRequest(ctx, "POST", fmt.Sprintf("%s/api/v1/collections/%s/add", c.endpoint, col.ID), body)
	return err
}

func (c *Chroma) DeleteVectors(ctx context.Context, namespace string, vectorIds []string) error {
	if len(vectorIds) == 0 {
		return nil
	}
	col, err := c.getCollection(ctx, namespace)
	if err != nil {
		return nil
	}
	body, err := json.Marshal(map[string]any{"ids": vectorIds})
	if err != nil {
		return fmt.Errorf("marshal delete vectors body: %w", err)
	}
	_, err = c.doRequest(ctx, "POST", fmt.Sprintf("%s/api/v1/collections/%s/delete", c.endpoint, col.ID), body)
	return err
}

func (c *Chroma) SimilaritySearch(ctx context.Context, namespace string, queryVector []float32, opts SearchOptions) ([]SearchResult, error) {
	if opts.TopN <= 0 {
		opts.TopN = 4
	}
	col, err := c.getCollection(ctx, namespace)
	if err != nil {
		return nil, err
	}

	body, err := json.Marshal(map[string]any{
		"query_embeddings": [][]float32{queryVector},
		"n_results":        opts.TopN,
		"include":          []string{"metadatas", "documents", "distances"},
	})
	if err != nil {
		return nil, fmt.Errorf("marshal query body: %w", err)
	}
	resp, err := c.doRequest(ctx, "POST", fmt.Sprintf("%s/api/v1/collections/%s/query", c.endpoint, col.ID), body)
	if err != nil {
		return nil, err
	}

	var result struct {
		IDs       [][]string         `json:"ids"`
		Documents [][]string         `json:"documents"`
		Metadatas [][]map[string]any `json:"metadatas"`
		Distances [][]float64        `json:"distances"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	var searchResults []SearchResult
	if len(result.IDs) > 0 {
		for i := 0; i < len(result.IDs[0]); i++ {
			distance := 0.0
			if len(result.Distances) > 0 && len(result.Distances[0]) > i {
				distance = result.Distances[0][i]
			}
			score := distanceToSimilarity(distance)
			if score < opts.SimilarityThreshold {
				continue
			}

			meta := map[string]any{}
			if len(result.Metadatas) > 0 && len(result.Metadatas[0]) > i && result.Metadatas[0][i] != nil {
				meta = result.Metadatas[0][i]
			}

			text := ""
			if len(result.Documents) > 0 && len(result.Documents[0]) > i {
				text = result.Documents[0][i]
			}

			searchResults = append(searchResults, SearchResult{
				DocId:    getStringMeta(meta, "docId"),
				Text:     text,
				Score:    score,
				Distance: distance,
				Metadata: meta,
			})
		}
	}
	return searchResults, nil
}

func (c *Chroma) DeleteNamespace(ctx context.Context, namespace string) error {
	col, err := c.getCollection(ctx, namespace)
	if err != nil {
		return nil
	}
	_, err = c.doRequest(ctx, "DELETE", fmt.Sprintf("%s/api/v1/collections/%s", c.endpoint, col.ID), nil)
	return err
}

func (c *Chroma) doRequest(ctx context.Context, method, url string, body []byte) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	if c.apiKey != "" && c.apiHeader != "" {
		req.Header.Set(c.apiHeader, c.apiKey)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("chroma api error %d: %s", resp.StatusCode, string(respBody))
	}
	return respBody, nil
}

type chromaCollection struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func getStringMeta(meta map[string]any, key string) string {
	if v, ok := meta[key].(string); ok {
		return v
	}
	return ""
}
