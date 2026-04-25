package presence

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// SleepWindowConfig is the YAML-shaped input. Lives here (not in
// config/) so the source's tests can construct it without pulling the
// whole config package; config/ re-exports the same struct via type
// alias.
type SleepWindowConfig struct {
	Enabled  bool   `yaml:"enabled,omitempty"`
	Start    string `yaml:"start,omitempty"`    // "HH:MM"
	End      string `yaml:"end,omitempty"`      // "HH:MM"
	Timezone string `yaml:"timezone,omitempty"` // IANA name; empty = process local
}

// SleepWindow votes Absent when the wall clock falls inside a
// configured time-of-day window. Never votes Present — sleep hours
// don't prove user is awake (insomnia / night-shift case).
type SleepWindow struct {
	enabled    bool
	start, end timeOfDay
	tz         *time.Location
}

type timeOfDay struct {
	hour   int
	minute int
}

// NewSleepWindow validates and constructs a SleepWindow. Returns an
// error for malformed HH:MM, unknown timezone, or empty endpoints
// when enabled.
func NewSleepWindow(cfg SleepWindowConfig) (*SleepWindow, error) {
	if !cfg.Enabled {
		// Disabled is always-Unknown. Skip validation; allow nil-safe
		// construction so callers can build it unconditionally.
		return &SleepWindow{enabled: false, tz: time.Local}, nil
	}
	if cfg.Start == "" || cfg.End == "" {
		return nil, fmt.Errorf("presence.sleep_window: start and end are required when enabled")
	}
	start, err := parseTimeOfDay(cfg.Start)
	if err != nil {
		return nil, fmt.Errorf("presence.sleep_window: start %q: %w", cfg.Start, err)
	}
	end, err := parseTimeOfDay(cfg.End)
	if err != nil {
		return nil, fmt.Errorf("presence.sleep_window: end %q: %w", cfg.End, err)
	}
	tz := time.Local
	if cfg.Timezone != "" {
		loc, err := time.LoadLocation(cfg.Timezone)
		if err != nil {
			return nil, fmt.Errorf("presence.sleep_window: timezone %q: %w", cfg.Timezone, err)
		}
		tz = loc
	}
	return &SleepWindow{enabled: true, start: start, end: end, tz: tz}, nil
}

// Name implements Source.
func (s *SleepWindow) Name() string { return "sleep_window" }

// Vote implements Source. Returns Absent if `now` (in the configured
// TZ) falls inside the window, Unknown otherwise.
func (s *SleepWindow) Vote(now time.Time) Vote {
	if !s.enabled {
		return Unknown
	}
	local := now.In(s.tz)
	if inWindow(local, s.start, s.end) {
		return Absent
	}
	return Unknown
}

func parseTimeOfDay(s string) (timeOfDay, error) {
	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		return timeOfDay{}, fmt.Errorf("expected HH:MM")
	}
	h, err := strconv.Atoi(parts[0])
	if err != nil || h < 0 || h > 23 {
		return timeOfDay{}, fmt.Errorf("hour must be 0–23")
	}
	m, err := strconv.Atoi(parts[1])
	if err != nil || m < 0 || m > 59 {
		return timeOfDay{}, fmt.Errorf("minute must be 0–59")
	}
	return timeOfDay{hour: h, minute: m}, nil
}

func (t timeOfDay) total() int { return t.hour*60 + t.minute }

func inWindow(now time.Time, start, end timeOfDay) bool {
	cur := timeOfDay{now.Hour(), now.Minute()}.total()
	s, e := start.total(), end.total()
	if s == e {
		return false // zero-length window
	}
	if s < e {
		// Same-day window: [start, end)
		return cur >= s && cur < e
	}
	// Crosses midnight: cur >= start OR cur < end
	return cur >= s || cur < e
}
