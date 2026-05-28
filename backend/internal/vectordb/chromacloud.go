package vectordb

import "context"

type ChromaCloud struct {
	*Chroma
}

func NewChromaCloud(endpoint, apiKey string) *ChromaCloud {
	c := NewChroma(endpoint, "X-Api-Key", apiKey)
	return &ChromaCloud{Chroma: c}
}

func (c *ChromaCloud) Name() string { return "chromacloud" }

func (c *ChromaCloud) Connect(ctx context.Context) error {
	return c.Chroma.Connect(ctx)
}
