package vectordb

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/go-openapi/strfmt"
	"github.com/weaviate/weaviate-go-client/v4/weaviate"
	"github.com/weaviate/weaviate-go-client/v4/weaviate/auth"
	"github.com/weaviate/weaviate-go-client/v4/weaviate/graphql"
	"github.com/weaviate/weaviate/entities/models"
)

type Weaviate struct {
	endpoint string
	apiKey   string
	client   *weaviate.Client
}

func NewWeaviate(endpoint, apiKey string) *Weaviate {
	return &Weaviate{endpoint: endpoint, apiKey: apiKey}
}

func (w *Weaviate) Name() string { return "weaviate" }

func (w *Weaviate) camelCase(s string) string {
	parts := strings.Split(s, "-")
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, "")
}

func (w *Weaviate) Connect(ctx context.Context) error {
	u, err := url.Parse(w.endpoint)
	if err != nil {
		return fmt.Errorf("parse endpoint: %w", err)
	}
	cfg := weaviate.Config{
		Host:   u.Host,
		Scheme: u.Scheme,
	}
	if w.apiKey != "" {
		cfg.AuthConfig = auth.ApiKey{Value: w.apiKey}
	}
	w.client = weaviate.New(cfg)
	return nil
}

func (w *Weaviate) Heartbeat(ctx context.Context) (map[string]any, error) {
	return map[string]any{"name": "weaviate", "endpoint": w.endpoint}, nil
}

func (w *Weaviate) Tables(ctx context.Context) ([]string, error) {
	schema, err := w.client.Schema().Getter().Do(ctx)
	if err != nil {
		return nil, err
	}
	names := make([]string, len(schema.Classes))
	for i, c := range schema.Classes {
		names[i] = c.Class
	}
	return names, nil
}

func (w *Weaviate) CountVectors(ctx context.Context, namespace string) (int64, error) {
	className := w.camelCase(namespace)
	result, err := w.client.GraphQL().Aggregate().
		WithClassName(className).
		WithFields(graphql.Field{Name: "meta", Fields: []graphql.Field{{Name: "count"}}}).
		Do(ctx)
	if err != nil || result.Errors != nil {
		return 0, nil
	}
	if data, ok := result.Data["Aggregate"].(map[string]interface{}); ok {
		if clsData, ok := data[className].([]interface{}); ok && len(clsData) > 0 {
			if meta, ok := clsData[0].(map[string]interface{}); ok {
				if count, ok := meta["count"].(map[string]interface{}); ok {
					if n, ok := count["count"].(float64); ok {
						return int64(n), nil
					}
				}
			}
		}
	}
	return 0, nil
}

func (w *Weaviate) TotalVectors(ctx context.Context) (int64, error) {
	schema, err := w.client.Schema().Getter().Do(ctx)
	if err != nil {
		return 0, err
	}
	var total int64
	for _, c := range schema.Classes {
		result, err := w.client.GraphQL().Aggregate().
			WithClassName(c.Class).
			WithFields(graphql.Field{Name: "meta", Fields: []graphql.Field{{Name: "count"}}}).
			Do(ctx)
		if err != nil || result.Errors != nil {
			continue
		}
		if data, ok := result.Data["Aggregate"].(map[string]interface{}); ok {
			if clsData, ok := data[c.Class].([]interface{}); ok && len(clsData) > 0 {
				if meta, ok := clsData[0].(map[string]interface{}); ok {
					if count, ok := meta["count"].(map[string]interface{}); ok {
						if n, ok := count["count"].(float64); ok {
							total += int64(n)
						}
					}
				}
			}
		}
	}
	return total, nil
}

func (w *Weaviate) AddVectors(ctx context.Context, namespace string, chunks []VectorChunk) error {
	if len(chunks) == 0 {
		return nil
	}
	className := w.camelCase(namespace)

	// Check if class exists
	exists := false
	schema, _ := w.client.Schema().Getter().Do(ctx)
	for _, c := range schema.Classes {
		if c.Class == className {
			exists = true
			break
		}
	}

	if !exists {
		class := &models.Class{
			Class:      className,
			Vectorizer: "none",
			Properties: []*models.Property{
				{Name: "docId", DataType: []string{"text"}},
				{Name: "text", DataType: []string{"text"}},
				{Name: "metadata", DataType: []string{"text"}},
			},
		}
		if err := w.client.Schema().ClassCreator().WithClass(class).Do(ctx); err != nil {
			return fmt.Errorf("create class: %w", err)
		}
	}

	objects := make([]*models.Object, len(chunks))
	for i, ch := range chunks {
		metaJSON, err := json.Marshal(ch.Metadata)
		if err != nil {
			return fmt.Errorf("marshal metadata for chunk %s: %w", ch.ID, err)
		}
		objects[i] = &models.Object{
			Class:  className,
			ID:     strfmt.UUID(ch.ID),
			Vector: ch.Vector,
			Properties: map[string]interface{}{
				"docId":    getStringFromMap(ch.Metadata, "docId"),
				"text":     getStringFromMap(ch.Metadata, "text"),
				"metadata": string(metaJSON),
			},
		}
	}

	_, err := w.client.Batch().ObjectsBatcher().WithObjects(objects...).Do(ctx)
	return err
}

func (w *Weaviate) DeleteVectors(ctx context.Context, namespace string, vectorIds []string) error {
	if len(vectorIds) == 0 {
		return nil
	}
	className := w.camelCase(namespace)
	for _, id := range vectorIds {
		if err := w.client.Data().Deleter().WithClassName(className).WithID(id).Do(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (w *Weaviate) SimilaritySearch(ctx context.Context, namespace string, queryVector []float32, opts SearchOptions) ([]SearchResult, error) {
	if opts.TopN <= 0 {
		opts.TopN = 4
	}
	className := w.camelCase(namespace)

	nearVector := graphql.NearVectorArgumentBuilder{}
	nearVector.WithVector(queryVector)

	result, err := w.client.GraphQL().Get().
		WithClassName(className).
		WithFields(
			graphql.Field{Name: "docId"},
			graphql.Field{Name: "text"},
			graphql.Field{Name: "metadata"},
		).
		WithNearVector(&nearVector).
		WithLimit(opts.TopN).
		Do(ctx)
	if err != nil {
		return nil, err
	}
	if result.Errors != nil && len(result.Errors) > 0 {
		return nil, fmt.Errorf("graphql error: %s", result.Errors[0].Message)
	}

	var searchResults []SearchResult
	if getData, ok := result.Data["Get"].(map[string]interface{}); ok {
		if clsData, ok := getData[className].([]interface{}); ok {
			for _, item := range clsData {
				obj, _ := item.(map[string]interface{})
				certainty := 0.0
				if add, ok := obj["_additional"].(map[string]interface{}); ok {
					if c, ok := add["certainty"].(float64); ok {
						certainty = c
					}
				}
				if certainty < opts.SimilarityThreshold {
					continue
				}

				meta := map[string]any{}
				if metaStr, ok := obj["metadata"].(string); ok && metaStr != "" {
					json.Unmarshal([]byte(metaStr), &meta)
				}

				text := ""
				if t, ok := obj["text"].(string); ok {
					text = t
				}

				searchResults = append(searchResults, SearchResult{
					DocId:    getStringFromMap(meta, "docId"),
					Text:     text,
					Score:    certainty,
					Distance: 1.0 - certainty,
					Metadata: meta,
				})
			}
		}
	}
	return searchResults, nil
}

func (w *Weaviate) DeleteNamespace(ctx context.Context, namespace string) error {
	return w.client.Schema().ClassDeleter().WithClassName(w.camelCase(namespace)).Do(ctx)
}

func getStringFromMap(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}
