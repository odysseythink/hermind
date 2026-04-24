package embedding_test

import (
	"testing"

	"github.com/odysseythink/hermind/tool/embedding"
)

func TestCosineSimilarity(t *testing.T) {
	a := []float32{1, 0, 0}
	b := []float32{1, 0, 0}
	if got := embedding.CosineSimilarity(a, b); got < 0.999 {
		t.Fatalf("identical vectors: want ~1.0, got %f", got)
	}

	c := []float32{0, 1, 0}
	if got := embedding.CosineSimilarity(a, c); got > 0.001 {
		t.Fatalf("orthogonal vectors: want ~0.0, got %f", got)
	}
}

func TestRerankEmpty(t *testing.T) {
	scores := embedding.Rerank([]string{}, func(s string) []float32 { return nil }, []float32{1, 0, 0})
	if len(scores) != 0 {
		t.Fatalf("expected empty, got %v", scores)
	}
}
