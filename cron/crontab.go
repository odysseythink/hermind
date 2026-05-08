package cron

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// CronSpec is a parsed standard crontab-style schedule with five
// fields: minute (0-59), hour (0-23), day-of-month (1-31),
// month (1-12), day-of-week (0-6, Sunday = 0). Ranges, lists, and
// steps are supported: "*/5 * * * *", "0 9,12,17 * * 1-5".
type CronSpec struct {
	Minute []int
	Hour   []int
	Dom    []int
	Month  []int
	Dow    []int
}

// ParseCrontab parses a 5-field crontab string. It also accepts the
// shortcuts @hourly, @daily, @weekly, @monthly.
func ParseCrontab(s string) (CronSpec, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	switch s {
	case "@hourly":
		return ParseCrontab("0 * * * *")
	case "@daily", "@midnight":
		return ParseCrontab("0 0 * * *")
	case "@weekly":
		return ParseCrontab("0 0 * * 0")
	case "@monthly":
		return ParseCrontab("0 0 1 * *")
	}
	parts := strings.Fields(s)
	if len(parts) != 5 {
		return CronSpec{}, fmt.Errorf("cron: crontab expects 5 fields, got %d", len(parts))
	}
	minute, err := parseField(parts[0], 0, 59)
	if err != nil {
		return CronSpec{}, fmt.Errorf("cron: minute: %w", err)
	}
	hour, err := parseField(parts[1], 0, 23)
	if err != nil {
		return CronSpec{}, fmt.Errorf("cron: hour: %w", err)
	}
	dom, err := parseField(parts[2], 1, 31)
	if err != nil {
		return CronSpec{}, fmt.Errorf("cron: dom: %w", err)
	}
	month, err := parseField(parts[3], 1, 12)
	if err != nil {
		return CronSpec{}, fmt.Errorf("cron: month: %w", err)
	}
	dow, err := parseField(parts[4], 0, 6)
	if err != nil {
		return CronSpec{}, fmt.Errorf("cron: dow: %w", err)
	}
	return CronSpec{Minute: minute, Hour: hour, Dom: dom, Month: month, Dow: dow}, nil
}

// parseField parses one crontab field into the explicit list of
// integers it represents. Supports "*", "n", "n-m", "a,b,c", and
// "*/step" (or "n-m/step").
func parseField(f string, min, max int) ([]int, error) {
	if f == "*" {
		return rangeSlice(min, max, 1), nil
	}
	// Explicit list.
	if strings.Contains(f, ",") {
		var out []int
		for _, part := range strings.Split(f, ",") {
			sub, err := parseField(part, min, max)
			if err != nil {
				return nil, err
			}
			out = append(out, sub...)
		}
		return dedupSort(out), nil
	}
	step := 1
	if idx := strings.Index(f, "/"); idx >= 0 {
		var err error
		step, err = strconv.Atoi(f[idx+1:])
		if err != nil || step <= 0 {
			return nil, fmt.Errorf("invalid step %q", f)
		}
		f = f[:idx]
	}
	if f == "*" || f == "" {
		return rangeSlice(min, max, step), nil
	}
	if strings.Contains(f, "-") {
		parts := strings.Split(f, "-")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid range %q", f)
		}
		from, err1 := strconv.Atoi(parts[0])
		to, err2 := strconv.Atoi(parts[1])
		if err1 != nil || err2 != nil || from < min || to > max || from > to {
			return nil, fmt.Errorf("invalid range %q", f)
		}
		return rangeSlice(from, to, step), nil
	}
	n, err := strconv.Atoi(f)
	if err != nil || n < min || n > max {
		return nil, fmt.Errorf("invalid value %q (expected %d..%d)", f, min, max)
	}
	return []int{n}, nil
}

func rangeSlice(from, to, step int) []int {
	var out []int
	for i := from; i <= to; i += step {
		out = append(out, i)
	}
	return out
}

func dedupSort(in []int) []int {
	seen := map[int]bool{}
	var out []int
	for _, v := range in {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	// Simple insertion sort — inputs are short.
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j-1] > out[j]; j-- {
			out[j-1], out[j] = out[j], out[j-1]
		}
	}
	return out
}

// Next returns the next time after t at which this spec matches.
func (c CronSpec) Next(t time.Time) time.Time {
	// Advance one minute at a time — slow but correct. For Phase 18
	// cron jobs this is more than fine (we call Next at most once
	// per job tick).
	t = t.Add(time.Minute).Truncate(time.Minute)
	for i := 0; i < 60*24*366; i++ {
		if contains(c.Minute, t.Minute()) &&
			contains(c.Hour, t.Hour()) &&
			contains(c.Dom, t.Day()) &&
			contains(c.Month, int(t.Month())) &&
			contains(c.Dow, int(t.Weekday())) {
			return t
		}
		t = t.Add(time.Minute)
	}
	return time.Time{}
}

func contains(xs []int, v int) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}
