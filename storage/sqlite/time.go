// storage/sqlite/time.go
package sqlite

import "time"

// toEpoch converts a time.Time to a float64 Unix timestamp (seconds).
// The zero time.Time maps to 0.0 so memories with never-set timestamps
// (e.g., LastUsedAt before first reinforcement) round-trip as zero.
func toEpoch(t time.Time) float64 {
	if t.IsZero() {
		return 0
	}
	return float64(t.UnixNano()) / 1e9
}

// fromEpoch converts a float64 Unix timestamp back to time.Time.
// 0.0 is interpreted as the zero time so IsZero() stays true after
// a full DB round-trip.
func fromEpoch(f float64) time.Time {
	if f == 0 {
		return time.Time{}
	}
	sec := int64(f)
	nsec := int64((f - float64(sec)) * 1e9)
	return time.Unix(sec, nsec).UTC()
}
