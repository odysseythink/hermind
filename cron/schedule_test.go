package cron

import (
	"testing"
	"time"
)

func TestParseSchedule(t *testing.T) {
	cases := []struct {
		in   string
		want time.Duration
	}{
		{"every 5m", 5 * time.Minute},
		{"every 1h30m", 90 * time.Minute},
		{"every 24h", 24 * time.Hour},
		{"every 1d", 24 * time.Hour},
		{"every 7d", 7 * 24 * time.Hour},
	}
	for _, c := range cases {
		s, err := ParseSchedule(c.in)
		if err != nil {
			t.Errorf("ParseSchedule(%q): unexpected error: %v", c.in, err)
			continue
		}
		if s.Every != c.want {
			t.Errorf("ParseSchedule(%q): got %s, want %s", c.in, s.Every, c.want)
		}
	}
}

func TestParseScheduleErrors(t *testing.T) {
	bad := []string{"5m", "every now", "every -1m", "every 0s"}
	for _, in := range bad {
		if _, err := ParseSchedule(in); err == nil {
			t.Errorf("expected error for %q", in)
		}
	}
}
