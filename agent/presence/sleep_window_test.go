package presence

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func mustTZ(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	require.NoError(t, err)
	return loc
}

func TestSleepWindow_DisabledAlwaysUnknown(t *testing.T) {
	sw, err := NewSleepWindow(SleepWindowConfig{Enabled: false})
	require.NoError(t, err)
	require.Equal(t, Unknown, sw.Vote(time.Now()))
}

func TestSleepWindow_NotCrossingMidnight(t *testing.T) {
	utc := time.UTC
	sw, err := NewSleepWindow(SleepWindowConfig{
		Enabled:  true,
		Start:    "14:00",
		End:      "18:00",
		Timezone: "UTC",
	})
	require.NoError(t, err)

	// 14:00 inclusive
	require.Equal(t, Absent, sw.Vote(time.Date(2026, 1, 1, 14, 0, 0, 0, utc)))
	// 16:30 inside
	require.Equal(t, Absent, sw.Vote(time.Date(2026, 1, 1, 16, 30, 0, 0, utc)))
	// 17:59 still inside
	require.Equal(t, Absent, sw.Vote(time.Date(2026, 1, 1, 17, 59, 0, 0, utc)))
	// 18:00 boundary — exclusive (end is exclusive)
	require.Equal(t, Unknown, sw.Vote(time.Date(2026, 1, 1, 18, 0, 0, 0, utc)))
	// 13:59 outside
	require.Equal(t, Unknown, sw.Vote(time.Date(2026, 1, 1, 13, 59, 0, 0, utc)))
}

func TestSleepWindow_CrossingMidnight(t *testing.T) {
	utc := time.UTC
	sw, err := NewSleepWindow(SleepWindowConfig{
		Enabled:  true,
		Start:    "23:00",
		End:      "07:00",
		Timezone: "UTC",
	})
	require.NoError(t, err)

	require.Equal(t, Absent, sw.Vote(time.Date(2026, 1, 1, 23, 0, 0, 0, utc)))   // start
	require.Equal(t, Absent, sw.Vote(time.Date(2026, 1, 2, 3, 0, 0, 0, utc)))    // overnight
	require.Equal(t, Absent, sw.Vote(time.Date(2026, 1, 2, 6, 59, 0, 0, utc)))   // just before end
	require.Equal(t, Unknown, sw.Vote(time.Date(2026, 1, 2, 7, 0, 0, 0, utc)))   // end exclusive
	require.Equal(t, Unknown, sw.Vote(time.Date(2026, 1, 1, 22, 59, 0, 0, utc))) // before start
	require.Equal(t, Unknown, sw.Vote(time.Date(2026, 1, 1, 12, 0, 0, 0, utc)))  // midday
}

func TestSleepWindow_NonDefaultTimezone(t *testing.T) {
	pacific := mustTZ(t, "America/Los_Angeles")
	sw, err := NewSleepWindow(SleepWindowConfig{
		Enabled:  true,
		Start:    "23:00",
		End:      "07:00",
		Timezone: "America/Los_Angeles",
	})
	require.NoError(t, err)

	// 06:30 in LA = 14:30 UTC the same date in winter (UTC-8). Inside.
	winterMorningUTC := time.Date(2026, 1, 2, 14, 30, 0, 0, time.UTC)
	require.Equal(t, Absent, sw.Vote(winterMorningUTC))

	// 12:00 in LA = 20:00 UTC — outside the sleep window.
	winterNoonUTC := time.Date(2026, 1, 2, 20, 0, 0, 0, time.UTC)
	require.Equal(t, Unknown, sw.Vote(winterNoonUTC))

	// Sanity: same wall-clock time computed in pacific TZ matches.
	require.Equal(t, "06:30", winterMorningUTC.In(pacific).Format("15:04"))
}

func TestSleepWindow_ZeroLengthAlwaysUnknown(t *testing.T) {
	sw, err := NewSleepWindow(SleepWindowConfig{
		Enabled: true,
		Start:   "10:00",
		End:     "10:00",
	})
	require.NoError(t, err)
	require.Equal(t, Unknown, sw.Vote(time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)))
	require.Equal(t, Unknown, sw.Vote(time.Date(2026, 1, 1, 10, 30, 0, 0, time.UTC)))
}

func TestSleepWindow_RejectsMalformedStart(t *testing.T) {
	_, err := NewSleepWindow(SleepWindowConfig{Enabled: true, Start: "25:99", End: "07:00"})
	require.Error(t, err)
}

func TestSleepWindow_RejectsUnknownTimezone(t *testing.T) {
	_, err := NewSleepWindow(SleepWindowConfig{
		Enabled: true, Start: "23:00", End: "07:00", Timezone: "Mars/Olympus",
	})
	require.Error(t, err)
}

func TestSleepWindow_RejectsEmptyEndpointsWhenEnabled(t *testing.T) {
	_, err := NewSleepWindow(SleepWindowConfig{Enabled: true, Start: "", End: "07:00"})
	require.Error(t, err)
	_, err = NewSleepWindow(SleepWindowConfig{Enabled: true, Start: "23:00", End: ""})
	require.Error(t, err)
}

func TestSleepWindow_NeverReturnsPresent(t *testing.T) {
	sw, err := NewSleepWindow(SleepWindowConfig{
		Enabled: true, Start: "23:00", End: "07:00", Timezone: "UTC",
	})
	require.NoError(t, err)
	for h := 0; h < 24; h++ {
		v := sw.Vote(time.Date(2026, 1, 1, h, 0, 0, 0, time.UTC))
		require.NotEqual(t, Present, v, "SleepWindow must never vote Present (h=%d)", h)
	}
}

func TestSleepWindow_Name(t *testing.T) {
	sw, err := NewSleepWindow(SleepWindowConfig{Enabled: false})
	require.NoError(t, err)
	require.Equal(t, "sleep_window", sw.Name())
}
