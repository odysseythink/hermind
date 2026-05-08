package cron

import (
	"testing"
	"time"
)

func TestParseCrontabStar(t *testing.T) {
	c, err := ParseCrontab("* * * * *")
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Minute) != 60 || len(c.Hour) != 24 {
		t.Errorf("bad expansion: minute=%d hour=%d", len(c.Minute), len(c.Hour))
	}
}

func TestParseCrontabShortcuts(t *testing.T) {
	c, err := ParseCrontab("@daily")
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Minute) != 1 || c.Minute[0] != 0 {
		t.Errorf("@daily minute = %v", c.Minute)
	}
	if len(c.Hour) != 1 || c.Hour[0] != 0 {
		t.Errorf("@daily hour = %v", c.Hour)
	}
}

func TestParseCrontabStep(t *testing.T) {
	c, err := ParseCrontab("*/15 * * * *")
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Minute) != 4 {
		t.Errorf("minute = %v", c.Minute)
	}
}

func TestParseCrontabList(t *testing.T) {
	c, err := ParseCrontab("0 9,12,17 * * 1-5")
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Hour) != 3 {
		t.Errorf("hour = %v", c.Hour)
	}
	if len(c.Dow) != 5 {
		t.Errorf("dow = %v", c.Dow)
	}
}

func TestParseCrontabInvalid(t *testing.T) {
	bad := []string{"", "* * *", "99 * * * *", "* * 32 * *", "* * * 13 *", "@never"}
	for _, b := range bad {
		if _, err := ParseCrontab(b); err == nil {
			t.Errorf("expected error for %q", b)
		}
	}
}

func TestCronSpecNextHourly(t *testing.T) {
	c, _ := ParseCrontab("0 * * * *")
	t0 := time.Date(2026, 4, 11, 12, 15, 0, 0, time.UTC)
	next := c.Next(t0)
	if next.Hour() != 13 || next.Minute() != 0 {
		t.Errorf("next = %v", next)
	}
}

func TestCronSpecNextDailyMidnight(t *testing.T) {
	c, _ := ParseCrontab("@daily")
	t0 := time.Date(2026, 4, 11, 10, 0, 0, 0, time.UTC)
	next := c.Next(t0)
	if next.Year() != 2026 || next.Month() != 4 || next.Day() != 12 || next.Hour() != 0 || next.Minute() != 0 {
		t.Errorf("next = %v", next)
	}
}
