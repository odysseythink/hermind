// storage/sqlite/time.go
package sqlite

import "time"

// toEpoch converts a time.Time to a float64 Unix timestamp (seconds).
// This matches the Python hermes state.db storage format.
func toEpoch(t time.Time) float64 {
	return float64(t.UnixNano()) / 1e9
}

// fromEpoch converts a float64 Unix timestamp back to time.Time.
func fromEpoch(f float64) time.Time {
	sec := int64(f)
	nsec := int64((f - float64(sec)) * 1e9)
	return time.Unix(sec, nsec).UTC()
}
