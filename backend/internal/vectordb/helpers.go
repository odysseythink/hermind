package vectordb

import "math"

// distanceToSimilarity converts cosine distance to similarity score.
// Used by LanceDB and Chroma providers.
func distanceToSimilarity(distance float64) float64 {
	if distance >= 1.0 {
		return 1.0
	}
	if distance < 0 {
		return 1.0 - math.Abs(distance)
	}
	return 1.0 - distance
}
