// Package cron provides a tiny interval-based job scheduler for
// running agent prompts on a recurring schedule. Schedules use a
// small grammar — "every 5m", "every 1h30m", "every 24h" — parsed
// into a time.Duration via ParseSchedule.
package cron

import (
	"fmt"
	"strings"
	"time"
)

// Schedule describes when a Job should fire.
type Schedule struct {
	Every time.Duration
}

// ParseSchedule accepts strings like "every 5m" or "every 1h30m".
func ParseSchedule(s string) (Schedule, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if !strings.HasPrefix(s, "every ") {
		return Schedule{}, fmt.Errorf("cron: schedule must start with 'every ', got %q", s)
	}
	rest := strings.TrimSpace(strings.TrimPrefix(s, "every "))
	if strings.HasSuffix(rest, "d") {
		d := strings.TrimSuffix(rest, "d")
		n, err := time.ParseDuration(d + "h")
		if err != nil {
			return Schedule{}, fmt.Errorf("cron: invalid day interval: %w", err)
		}
		return Schedule{Every: n * 24}, nil
	}
	d, err := time.ParseDuration(rest)
	if err != nil {
		return Schedule{}, fmt.Errorf("cron: invalid duration %q: %w", rest, err)
	}
	if d <= 0 {
		return Schedule{}, fmt.Errorf("cron: interval must be positive, got %s", d)
	}
	return Schedule{Every: d}, nil
}
