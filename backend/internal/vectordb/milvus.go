package vectordb

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
)

type Milvus struct {
	address  string
	username string
	password string
	client   client.Client
}

func NewMilvus(address, username, password string) *Milvus {
	return &Milvus{address: address, username: username, password: password}
}

func (m *Milvus) Name() string { return "milvus" }

func (m *Milvus) Connect(ctx context.Context) error {
	cfg := client.Config{
		Address:  m.address,
		Username: m.username,
		Password: m.password,
	}
	c, err := client.NewClient(ctx, cfg)
	if err != nil {
		return fmt.Errorf("milvus: create client: %w", err)
	}
	m.client = c
	return nil
}

func (m *Milvus) Heartbeat(ctx context.Context) (map[string]any, error) {
	return map[string]any{"name": "milvus", "address": m.address}, nil
}

func (m *Milvus) normalize(input string) string {
	re := regexp.MustCompile(`[^a-zA-Z0-9_]`)
	normalized := re.ReplaceAllString(input, "_")
	if len(normalized) > 0 && normalized[0] >= '0' && normalized[0] <= '9' {
		normalized = "c_" + normalized
	}
	return normalized
}

func (m *Milvus) AddVectors(ctx context.Context, namespace string, chunks []VectorChunk) error {
	if len(chunks) == 0 {
		return nil
	}

	collName := m.normalize(namespace)
	dims := len(chunks[0].Vector)

	exists, err := m.client.HasCollection(ctx, collName)
	if err != nil {
		return fmt.Errorf("milvus: check collection exists: %w", err)
	}

	if !exists {
		schema := entity.NewSchema().
			WithName(collName).
			WithField(entity.NewField().WithName("id").WithDataType(entity.FieldTypeVarChar).WithMaxLength(255).WithIsPrimaryKey(true)).
			WithField(entity.NewField().WithName("vector").WithDataType(entity.FieldTypeFloatVector).WithDim(int64(dims))).
			WithField(entity.NewField().WithName("metadata").WithDataType(entity.FieldTypeJSON))

		if err := m.client.CreateCollection(ctx, schema, 1); err != nil {
			return fmt.Errorf("milvus: create collection: %w", err)
		}

		idx, err := entity.NewIndexFlat(entity.COSINE)
		if err != nil {
			return fmt.Errorf("milvus: create index object: %w", err)
		}

		if err := m.client.CreateIndex(ctx, collName, "vector", idx, false); err != nil {
			return fmt.Errorf("milvus: create index: %w", err)
		}

		if err := m.client.LoadCollection(ctx, collName, false); err != nil {
			return fmt.Errorf("milvus: load collection: %w", err)
		}
	}

	ids := make([]string, len(chunks))
	vectors := make([][]float32, len(chunks))
	metadataBytes := make([][]byte, len(chunks))

	for i, ch := range chunks {
		ids[i] = ch.ID
		vectors[i] = ch.Vector
		meta, err := json.Marshal(ch.Metadata)
		if err != nil {
			return fmt.Errorf("milvus: marshal metadata: %w", err)
		}
		metadataBytes[i] = meta
	}

	idCol := entity.NewColumnVarChar("id", ids)
	vectorCol := entity.NewColumnFloatVector("vector", dims, vectors)
	metaCol := entity.NewColumnJSONBytes("metadata", metadataBytes)

	_, err = m.client.Insert(ctx, collName, "", idCol, vectorCol, metaCol)
	if err != nil {
		return fmt.Errorf("milvus: insert: %w", err)
	}

	return nil
}

func (m *Milvus) DeleteVectors(ctx context.Context, namespace string, vectorIds []string) error {
	if len(vectorIds) == 0 {
		return nil
	}

	collName := m.normalize(namespace)

	quoted := make([]string, len(vectorIds))
	for i, id := range vectorIds {
		// Milvus expression: id in ["id1", "id2", ...]
		// Escape backslash first, then double quote
		safe := strings.ReplaceAll(id, `\`, `\\`)
		safe = strings.ReplaceAll(safe, `"`, `\"`)
		quoted[i] = fmt.Sprintf(`"%s"`, safe)
	}
	expr := fmt.Sprintf("id in [%s]", strings.Join(quoted, ","))

	if err := m.client.Delete(ctx, collName, "", expr); err != nil {
		return fmt.Errorf("milvus: delete: %w", err)
	}
	return nil
}

func (m *Milvus) SimilaritySearch(ctx context.Context, namespace string, queryVector []float32, opts SearchOptions) ([]SearchResult, error) {
	if opts.TopN <= 0 {
		opts.TopN = 4
	}

	collName := m.normalize(namespace)

	sp, err := entity.NewIndexFlatSearchParam()
	if err != nil {
		return nil, fmt.Errorf("milvus: create search param: %w", err)
	}

	vectors := []entity.Vector{entity.FloatVector(queryVector)}

	results, err := m.client.Search(
		ctx,
		collName,
		nil,
		"",
		[]string{"metadata"},
		vectors,
		"vector",
		entity.COSINE,
		opts.TopN,
		sp,
	)
	if err != nil {
		return nil, fmt.Errorf("milvus: search: %w", err)
	}

	var searchResults []SearchResult
	for _, result := range results {
		for i := 0; i < result.ResultCount; i++ {
			score := float64(result.Scores[i])
			if score < opts.SimilarityThreshold {
				continue
			}

			idStr, _ := result.IDs.GetAsString(i)

			meta := map[string]any{}
			if metaCol := result.Fields.GetColumn("metadata"); metaCol != nil {
				if jsonStr, err := metaCol.GetAsString(i); err == nil {
					_ = json.Unmarshal([]byte(jsonStr), &meta)
				}
			}

			text := ""
			if v, ok := meta["text"].(string); ok {
				text = v
			}

			searchResults = append(searchResults, SearchResult{
				DocId:    idStr,
				Text:     text,
				Score:    score,
				Distance: 1.0 - score,
				Metadata: meta,
			})
		}
	}

	return searchResults, nil
}

func (m *Milvus) DeleteNamespace(ctx context.Context, namespace string) error {
	collName := m.normalize(namespace)
	if err := m.client.DropCollection(ctx, collName); err != nil {
		return fmt.Errorf("milvus: drop collection: %w", err)
	}
	return nil
}

func (m *Milvus) Tables(ctx context.Context) ([]string, error) {
	collections, err := m.client.ListCollections(ctx)
	if err != nil {
		return nil, fmt.Errorf("milvus: list collections: %w", err)
	}

	names := make([]string, len(collections))
	for i, col := range collections {
		names[i] = col.Name
	}
	return names, nil
}

func (m *Milvus) CountVectors(ctx context.Context, namespace string) (int64, error) {
	collName := m.normalize(namespace)
	stats, err := m.client.GetCollectionStatistics(ctx, collName)
	if err != nil {
		return 0, err
	}
	if countStr, ok := stats["row_count"]; ok {
		if count, err := strconv.ParseInt(countStr, 10, 64); err == nil {
			return count, nil
		}
	}
	return 0, nil
}

func (m *Milvus) TotalVectors(ctx context.Context) (int64, error) {
	collections, err := m.client.ListCollections(ctx)
	if err != nil {
		return 0, fmt.Errorf("milvus: list collections: %w", err)
	}

	var total int64
	for _, col := range collections {
		stats, err := m.client.GetCollectionStatistics(ctx, col.Name)
		if err != nil {
			continue
		}
		if countStr, ok := stats["row_count"]; ok {
			if count, err := strconv.ParseInt(countStr, 10, 64); err == nil {
				total += count
			}
		}
	}
	return total, nil
}
