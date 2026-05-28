package dto

type VectorSearchRequest struct {
	Query          string   `json:"query"`
	TopN           *int     `json:"topN,omitempty"`
	ScoreThreshold *float64 `json:"scoreThreshold,omitempty"`
}

type VectorSearchResult struct {
	ID       string         `json:"id"`
	Text     string         `json:"text"`
	Metadata map[string]any `json:"metadata"`
	Distance float64        `json:"distance"`
	Score    float64        `json:"score"`
}
