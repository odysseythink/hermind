// tool/embedding/cosine.go
package embedding

import "math"

// CosineSimilarity returns the cosine similarity between two equal-length
// float32 vectors. Returns 0 if either slice is empty or lengths differ.
func CosineSimilarity(a, b []float32) float32 {
	if len(a) == 0 || len(a) != len(b) {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		na += float64(a[i]) * float64(a[i])
		nb += float64(b[i]) * float64(b[i])
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return float32(dot / (math.Sqrt(na) * math.Sqrt(nb)))
}

// RankedItem pairs a value with its similarity score.
type RankedItem[T any] struct {
	Value T
	Score float32
}

// Rerank returns a slice of (item, score) pairs sorted descending by cosine
// similarity between each item's vector and query. Items without a vector
// (nil) are assigned score 0 and placed at the end.
func Rerank[T any](items []T, getVec func(T) []float32, query []float32) []RankedItem[T] {
	out := make([]RankedItem[T], len(items))
	for i, item := range items {
		v := getVec(item)
		out[i] = RankedItem[T]{Value: item, Score: CosineSimilarity(v, query)}
	}
	// insertion sort (N is small — typically ≤ 60)
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j].Score > out[j-1].Score; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}
