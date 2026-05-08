package citesink_test

import (
	"context"
	"testing"

	"github.com/odysseythink/hermind/tool/memory/memprovider/citesink"
	"github.com/stretchr/testify/assert"
)

func TestCiteSink_Roundtrip(t *testing.T) {
	var ids []string
	ctx := citesink.WithSink(context.Background(), func(id string) { ids = append(ids, id) })
	citesink.Cite(ctx, "mc_1")
	citesink.Cite(ctx, "mc_2")
	assert.Equal(t, []string{"mc_1", "mc_2"}, ids)
}

func TestCiteSink_Absent(t *testing.T) {
	citesink.Cite(context.Background(), "mc_1")
}
