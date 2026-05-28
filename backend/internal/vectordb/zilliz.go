package vectordb

import (
	"context"
	"fmt"

	"github.com/milvus-io/milvus-sdk-go/v2/client"
)

type Zilliz struct {
	*Milvus
}

func NewZilliz(endpoint, token string) *Zilliz {
	// Store token in Milvus.password so Zilliz can reuse Milvus address
	m := NewMilvus(endpoint, "", token)
	return &Zilliz{Milvus: m}
}

func (z *Zilliz) Name() string { return "zilliz" }

func (z *Zilliz) Connect(ctx context.Context) error {
	cfg := client.Config{
		Address: z.address,
		APIKey:  z.password,
	}
	c, err := client.NewClient(ctx, cfg)
	if err != nil {
		return fmt.Errorf("zilliz client: %w", err)
	}
	z.client = c
	return nil
}
