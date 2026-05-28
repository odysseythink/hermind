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

type AstraDB struct {
	applicationToken string
	endpoint         string
	client           *http.Client
}

func NewAstraDB(applicationToken, endpoint string) *AstraDB {
	return &AstraDB{
		applicationToken: applicationToken,
		endpoint:         endpoint,
		client:           &http.Client{Timeout: 30 * time.Second},
	}
}

func (a *AstraDB) Name() string { return "astra" }

func (a *AstraDB) Connect(ctx context.Context) error {
	return nil // HTTP client is stateless
}

func (a *AstraDB) Heartbeat(ctx context.Context) (map[string]any, error) {
	return map[string]any{"name": "astra", "endpoint": a.endpoint}, nil
}

// sanitize ensures the collection name is valid for Astra DB:
//   - must start with "ns_" (prepended if missing)
//   - illegal characters are replaced with "_"
func (a *AstraDB) sanitize(name string) string {
	sanitized := regexp.MustCompile(`[^a-zA-Z0-9_]`).ReplaceAllString(name, "_")
	if !regexp.MustCompile(`^ns_`).MatchString(sanitized) {
		sanitized = "ns_" + sanitized
	}
	return sanitized
}

func (a *AstraDB) Tables(ctx context.Context) ([]string, error) {
	respBody, err := a.doRequest(ctx, a.endpoint, map[string]any{
		"findCollections": map[string]any{},
	})
	if err != nil {
		return nil, fmt.Errorf("astra: list collections: %w", err)
	}

	var resp struct {
		Status struct {
			Collections []string `json:"collections"`
		} `json:"status"`
	}
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("astra: unmarshal collections: %w", err)
	}
	return resp.Status.Collections, nil
}

func (a *AstraDB) CountVectors(ctx context.Context, namespace string) (int64, error) {
	return a.collectionCount(ctx, a.sanitize(namespace))
}

func (a *AstraDB) TotalVectors(ctx context.Context) (int64, error) {
	collections, err := a.Tables(ctx)
	if err != nil {
		return 0, err
	}

	var total int64
	for _, col := range collections {
		count, err := a.collectionCount(ctx, col)
		if err != nil {
			continue
		}
		total += count
	}
	return total, nil
}

func (a *AstraDB) collectionCount(ctx context.Context, name string) (int64, error) {
	respBody, err := a.doRequest(ctx, a.endpoint+"/"+name, map[string]any{
		"countDocuments": map[string]any{},
	})
	if err != nil {
		return 0, err
	}

	var resp struct {
		Status struct {
			Count int64 `json:"count"`
		} `json:"status"`
	}
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return 0, fmt.Errorf("astra: unmarshal count: %w", err)
	}
	return resp.Status.Count, nil
}

func (a *AstraDB) AddVectors(ctx context.Context, namespace string, chunks []VectorChunk) error {
	if len(chunks) == 0 {
		return nil
	}

	dims := len(chunks[0].Vector)
	colName := a.sanitize(namespace)

	exists, err := a.collectionExists(ctx, colName)
	if err != nil {
		return fmt.Errorf("astra: check collection exists: %w", err)
	}

	if !exists {
		if err := a.createCollection(ctx, colName, dims); err != nil {
			return fmt.Errorf("astra: create collection: %w", err)
		}
	}

	// Astra batch insert limit is 20 documents per request.
	const batchSize = 20
	for i := 0; i < len(chunks); i += batchSize {
		end := i + batchSize
		if end > len(chunks) {
			end = len(chunks)
		}
		batch := chunks[i:end]

		docs := make([]map[string]any, 0, len(batch))
		for _, ch := range batch {
			doc := map[string]any{
				"_id":     ch.ID,
				"$vector": ch.Vector,
			}
			for k, v := range ch.Metadata {
				doc[k] = v
			}
			docs = append(docs, doc)
		}

		_, err := a.doRequest(ctx, a.endpoint+"/"+colName, map[string]any{
			"insertMany": map[string]any{
				"documents": docs,
			},
		})
		if err != nil {
			return fmt.Errorf("astra: insert vectors: %w", err)
		}
	}

	return nil
}

func (a *AstraDB) DeleteVectors(ctx context.Context, namespace string, vectorIds []string) error {
	if len(vectorIds) == 0 {
		return nil
	}

	colName := a.sanitize(namespace)
	_, err := a.doRequest(ctx, a.endpoint+"/"+colName, map[string]any{
		"deleteMany": map[string]any{
			"filter": map[string]any{
				"_id": map[string]any{"$in": vectorIds},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("astra: delete vectors: %w", err)
	}
	return nil
}

func (a *AstraDB) SimilaritySearch(ctx context.Context, namespace string, queryVector []float32, opts SearchOptions) ([]SearchResult, error) {
	if opts.TopN <= 0 {
		opts.TopN = 4
	}

	colName := a.sanitize(namespace)

	reqBody := map[string]any{
		"find": map[string]any{
			"sort":  map[string]any{"$vector": queryVector},
			"limit": opts.TopN,
			"options": map[string]any{
				"includeSimilarity": true,
			},
		},
	}

	respBody, err := a.doRequest(ctx, a.endpoint+"/"+colName, reqBody)
	if err != nil {
		return nil, fmt.Errorf("astra: similarity search: %w", err)
	}

	var resp struct {
		Data struct {
			Documents []map[string]any `json:"documents"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("astra: unmarshal search results: %w", err)
	}

	var results []SearchResult
	for _, doc := range resp.Data.Documents {
		similarity := 0.0
		if s, ok := doc["$similarity"].(float64); ok {
			similarity = s
		} else if s, ok := doc["$similarity"].(float32); ok {
			similarity = float64(s)
		}

		if similarity < opts.SimilarityThreshold {
			continue
		}

		meta := make(map[string]any)
		for k, v := range doc {
			if k == "_id" || k == "$vector" || k == "$similarity" {
				continue
			}
			meta[k] = v
		}

		docId := ""
		if v, ok := meta["docId"].(string); ok {
			docId = v
		}
		text := ""
		if v, ok := meta["text"].(string); ok {
			text = v
		}

		results = append(results, SearchResult{
			DocId:    docId,
			Text:     text,
			Score:    similarity,
			Distance: 1.0 - similarity,
			Metadata: meta,
		})
	}

	return results, nil
}

func (a *AstraDB) DeleteNamespace(ctx context.Context, namespace string) error {
	colName := a.sanitize(namespace)
	_, err := a.doRequest(ctx, a.endpoint+"/"+colName, map[string]any{
		"deleteCollection": map[string]any{},
	})
	if err != nil {
		return fmt.Errorf("astra: delete namespace: %w", err)
	}
	return nil
}

func (a *AstraDB) collectionExists(ctx context.Context, name string) (bool, error) {
	collections, err := a.Tables(ctx)
	if err != nil {
		return false, err
	}
	for _, c := range collections {
		if c == name {
			return true, nil
		}
	}
	return false, nil
}

func (a *AstraDB) createCollection(ctx context.Context, name string, dimension int) error {
	_, err := a.doRequest(ctx, a.endpoint, map[string]any{
		"createCollection": map[string]any{
			"name": name,
			"options": map[string]any{
				"vector": map[string]any{
					"dimension": dimension,
					"metric":    "cosine",
				},
			},
		},
	})
	return err
}

func (a *AstraDB) doRequest(ctx context.Context, url string, body map[string]any) ([]byte, error) {
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bodyReader)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Token", a.applicationToken)

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("astra api error %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}
