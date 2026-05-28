package vectordb

import (
	"context"
	"fmt"
	"net/url"
	"strconv"

	"github.com/qdrant/go-client/qdrant"
)

type Qdrant struct {
	endpoint string
	apiKey   string
	client   *qdrant.Client
}

func NewQdrant(endpoint, apiKey string) *Qdrant {
	return &Qdrant{endpoint: endpoint, apiKey: apiKey}
}

func (q *Qdrant) Name() string { return "qdrant" }

func (q *Qdrant) Connect(ctx context.Context) error {
	host, port, err := parseEndpoint(q.endpoint)
	if err != nil {
		return fmt.Errorf("qdrant: parse endpoint: %w", err)
	}

	cfg := &qdrant.Config{
		Host:   host,
		Port:   port,
		APIKey: q.apiKey,
	}

	client, err := qdrant.NewClient(cfg)
	if err != nil {
		return fmt.Errorf("qdrant: create client: %w", err)
	}

	q.client = client
	return nil
}

func (q *Qdrant) Heartbeat(ctx context.Context) (map[string]any, error) {
	return map[string]any{"name": "qdrant", "endpoint": q.endpoint}, nil
}

func (q *Qdrant) Tables(ctx context.Context) ([]string, error) {
	return q.client.ListCollections(ctx)
}

func (q *Qdrant) CountVectors(ctx context.Context, namespace string) (int64, error) {
	info, err := q.client.GetCollectionInfo(ctx, namespace)
	if err != nil {
		return 0, err
	}
	return int64(info.GetPointsCount()), nil
}

func (q *Qdrant) TotalVectors(ctx context.Context) (int64, error) {
	names, err := q.client.ListCollections(ctx)
	if err != nil {
		return 0, err
	}

	var total int64
	for _, name := range names {
		info, err := q.client.GetCollectionInfo(ctx, name)
		if err != nil {
			continue
		}
		total += int64(info.GetPointsCount())
	}
	return total, nil
}

func (q *Qdrant) AddVectors(ctx context.Context, namespace string, chunks []VectorChunk) error {
	if len(chunks) == 0 {
		return nil
	}

	dims := len(chunks[0].Vector)

	exists, err := q.client.CollectionExists(ctx, namespace)
	if err != nil {
		return fmt.Errorf("qdrant: check collection exists: %w", err)
	}

	if !exists {
		err = q.client.CreateCollection(ctx, &qdrant.CreateCollection{
			CollectionName: namespace,
			VectorsConfig: qdrant.NewVectorsConfig(&qdrant.VectorParams{
				Size:     uint64(dims),
				Distance: qdrant.Distance_Cosine,
			}),
		})
		if err != nil {
			return fmt.Errorf("qdrant: create collection: %w", err)
		}
	}

	points := make([]*qdrant.PointStruct, 0, len(chunks))
	for _, ch := range chunks {
		payload, err := toPayload(ch.Metadata)
		if err != nil {
			return fmt.Errorf("qdrant: convert payload: %w", err)
		}

		points = append(points, &qdrant.PointStruct{
			Id:      qdrant.NewIDUUID(ch.ID),
			Payload: payload,
			Vectors: qdrant.NewVectorsDense(ch.Vector),
		})
	}

	_, err = q.client.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: namespace,
		Points:         points,
	})
	if err != nil {
		return fmt.Errorf("qdrant: upsert: %w", err)
	}
	return nil
}

func (q *Qdrant) DeleteVectors(ctx context.Context, namespace string, vectorIds []string) error {
	if len(vectorIds) == 0 {
		return nil
	}

	ids := make([]*qdrant.PointId, 0, len(vectorIds))
	for _, id := range vectorIds {
		ids = append(ids, qdrant.NewIDUUID(id))
	}

	_, err := q.client.Delete(ctx, &qdrant.DeletePoints{
		CollectionName: namespace,
		Points:         qdrant.NewPointsSelectorIDs(ids),
	})
	if err != nil {
		return fmt.Errorf("qdrant: delete: %w", err)
	}
	return nil
}

func (q *Qdrant) SimilaritySearch(ctx context.Context, namespace string, queryVector []float32, opts SearchOptions) ([]SearchResult, error) {
	if opts.TopN <= 0 {
		opts.TopN = 4
	}

	limit := uint64(opts.TopN)
	query := qdrant.NewQueryDense(queryVector)

	req := &qdrant.QueryPoints{
		CollectionName: namespace,
		Query:          query,
		Limit:          &limit,
		WithPayload:    qdrant.NewWithPayloadEnable(true),
	}

	if opts.SimilarityThreshold > 0 {
		threshold := float32(opts.SimilarityThreshold)
		req.ScoreThreshold = &threshold
	}

	scored, err := q.client.Query(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("qdrant: query: %w", err)
	}

	var results []SearchResult
	for _, sp := range scored {
		meta := fromPayload(sp.GetPayload())
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
			Score:    float64(sp.GetScore()),
			Distance: 1.0 - float64(sp.GetScore()),
			Metadata: meta,
		})
	}

	return results, nil
}

func (q *Qdrant) DeleteNamespace(ctx context.Context, namespace string) error {
	err := q.client.DeleteCollection(ctx, namespace)
	if err != nil {
		return fmt.Errorf("qdrant: delete collection: %w", err)
	}
	return nil
}

// parseEndpoint parses a Qdrant endpoint into host and port.
// Supported formats: "host:port", "http://host:port", "https://host:port".
func parseEndpoint(endpoint string) (string, int, error) {
	if endpoint == "" {
		return "localhost", 6334, nil
	}

	u, err := url.Parse(endpoint)
	if err == nil && u.Host != "" {
		host := u.Hostname()
		portStr := u.Port()
		if portStr == "" {
			if u.Scheme == "https" {
				return host, 6334, nil
			}
			return host, 6334, nil
		}
		port, err := strconv.Atoi(portStr)
		if err != nil {
			return "", 0, fmt.Errorf("invalid port: %w", err)
		}
		return host, port, nil
	}

	// Fallback: try host:port directly
	host, portStr, err := splitHostPort(endpoint)
	if err == nil {
		port, err := strconv.Atoi(portStr)
		if err != nil {
			return "", 0, fmt.Errorf("invalid port: %w", err)
		}
		return host, port, nil
	}

	// No port specified, use default
	return endpoint, 6334, nil
}

// splitHostPort splits a "host:port" string.
func splitHostPort(hostport string) (string, string, error) {
	for i := len(hostport) - 1; i >= 0; i-- {
		if hostport[i] == ':' {
			return hostport[:i], hostport[i+1:], nil
		}
		if hostport[i] == ']' {
			break
		}
	}
	return "", "", fmt.Errorf("missing port in address")
}

// toPayload converts a Go map[string]any into Qdrant's payload format.
func toPayload(m map[string]any) (map[string]*qdrant.Value, error) {
	if m == nil {
		return nil, nil
	}
	payload := make(map[string]*qdrant.Value, len(m))
	for k, v := range m {
		qv, err := toQdrantValue(v)
		if err != nil {
			return nil, err
		}
		payload[k] = qv
	}
	return payload, nil
}

// toQdrantValue converts a Go value into a Qdrant protobuf Value.
func toQdrantValue(v any) (*qdrant.Value, error) {
	switch val := v.(type) {
	case nil:
		return &qdrant.Value{Kind: &qdrant.Value_NullValue{NullValue: qdrant.NullValue_NULL_VALUE}}, nil
	case bool:
		return &qdrant.Value{Kind: &qdrant.Value_BoolValue{BoolValue: val}}, nil
	case int:
		return &qdrant.Value{Kind: &qdrant.Value_IntegerValue{IntegerValue: int64(val)}}, nil
	case int8:
		return &qdrant.Value{Kind: &qdrant.Value_IntegerValue{IntegerValue: int64(val)}}, nil
	case int16:
		return &qdrant.Value{Kind: &qdrant.Value_IntegerValue{IntegerValue: int64(val)}}, nil
	case int32:
		return &qdrant.Value{Kind: &qdrant.Value_IntegerValue{IntegerValue: int64(val)}}, nil
	case int64:
		return &qdrant.Value{Kind: &qdrant.Value_IntegerValue{IntegerValue: val}}, nil
	case uint:
		return &qdrant.Value{Kind: &qdrant.Value_IntegerValue{IntegerValue: int64(val)}}, nil
	case uint8:
		return &qdrant.Value{Kind: &qdrant.Value_IntegerValue{IntegerValue: int64(val)}}, nil
	case uint16:
		return &qdrant.Value{Kind: &qdrant.Value_IntegerValue{IntegerValue: int64(val)}}, nil
	case uint32:
		return &qdrant.Value{Kind: &qdrant.Value_IntegerValue{IntegerValue: int64(val)}}, nil
	case uint64:
		return &qdrant.Value{Kind: &qdrant.Value_IntegerValue{IntegerValue: int64(val)}}, nil
	case float32:
		return &qdrant.Value{Kind: &qdrant.Value_DoubleValue{DoubleValue: float64(val)}}, nil
	case float64:
		return &qdrant.Value{Kind: &qdrant.Value_DoubleValue{DoubleValue: val}}, nil
	case string:
		return &qdrant.Value{Kind: &qdrant.Value_StringValue{StringValue: val}}, nil
	case []any:
		list := make([]*qdrant.Value, 0, len(val))
		for _, item := range val {
			qv, err := toQdrantValue(item)
			if err != nil {
				return nil, err
			}
			list = append(list, qv)
		}
		return &qdrant.Value{Kind: &qdrant.Value_ListValue{ListValue: &qdrant.ListValue{Values: list}}}, nil
	case map[string]any:
		fields := make(map[string]*qdrant.Value, len(val))
		for k, item := range val {
			qv, err := toQdrantValue(item)
			if err != nil {
				return nil, err
			}
			fields[k] = qv
		}
		return &qdrant.Value{Kind: &qdrant.Value_StructValue{StructValue: &qdrant.Struct{Fields: fields}}}, nil
	default:
		return &qdrant.Value{Kind: &qdrant.Value_StringValue{StringValue: fmt.Sprintf("%v", val)}}, nil
	}
}

// fromPayload converts Qdrant's payload format back into a Go map[string]any.
func fromPayload(payload map[string]*qdrant.Value) map[string]any {
	if payload == nil {
		return nil
	}
	m := make(map[string]any, len(payload))
	for k, v := range payload {
		m[k] = fromQdrantValue(v)
	}
	return m
}

// fromQdrantValue converts a Qdrant protobuf Value back into a Go value.
func fromQdrantValue(v *qdrant.Value) any {
	if v == nil {
		return nil
	}
	switch val := v.Kind.(type) {
	case *qdrant.Value_NullValue:
		return nil
	case *qdrant.Value_BoolValue:
		return val.BoolValue
	case *qdrant.Value_IntegerValue:
		return val.IntegerValue
	case *qdrant.Value_DoubleValue:
		return val.DoubleValue
	case *qdrant.Value_StringValue:
		return val.StringValue
	case *qdrant.Value_ListValue:
		if val.ListValue == nil {
			return nil
		}
		list := make([]any, 0, len(val.ListValue.Values))
		for _, item := range val.ListValue.Values {
			list = append(list, fromQdrantValue(item))
		}
		return list
	case *qdrant.Value_StructValue:
		if val.StructValue == nil {
			return nil
		}
		m := make(map[string]any, len(val.StructValue.Fields))
		for k, item := range val.StructValue.Fields {
			m[k] = fromQdrantValue(item)
		}
		return m
	default:
		return nil
	}
}
