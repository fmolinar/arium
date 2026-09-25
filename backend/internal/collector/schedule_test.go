package collector

import (
	"testing"
	"time"
)

func TestParseSchedule(t *testing.T) {
	s, err := ParseSchedule(" 16:00, 00:00,08:00,08:00", time.UTC)
	if err != nil {
		t.Fatalf("ParseSchedule: %v", err)
	}
	if got := s.String(); got != "00:00,08:00,16:00 UTC" {
		t.Errorf("String() = %q, want sorted and de-duplicated", got)
	}

	for _, bad := range []string{"", "8am", "24:00", "12:60", "08:00,", "8:00:00"} {
		if _, err := ParseSchedule(bad, time.UTC); err == nil {
			t.Errorf("ParseSchedule(%q) succeeded, want error", bad)
		}
	}
}

func TestScheduleNext(t *testing.T) {
	s, _ := ParseSchedule("00:00,08:00,16:00", time.UTC)
	day := func(d, h, m int) time.Time { return time.Date(2026, 9, d, h, m, 0, 0, time.UTC) }

	tests := []struct {
		after, want time.Time
	}{
		{day(24, 7, 59), day(24, 8, 0)},
		{day(24, 8, 0), day(24, 16, 0)}, // strictly after: a run at 08:00 schedules 16:00 next
		{day(24, 12, 0), day(24, 16, 0)},
		{day(24, 16, 30), day(25, 0, 0)}, // wraps to tomorrow
		{day(30, 23, 0), time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)},
	}

	for _, tt := range tests {
		if got := s.Next(tt.after); !got.Equal(tt.want) {
			t.Errorf("Next(%v) = %v, want %v", tt.after, got, tt.want)
		}
	}
}

func TestScheduleNextInTimeZone(t *testing.T) {
	la, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		t.Skip("tzdata unavailable:", err)
	}

	s, _ := ParseSchedule("06:00", la)

	// 2026-09-24 12:00 UTC is 05:00 PDT, so the next run is 06:00 PDT = 13:00 UTC.
	got := s.Next(time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC))
	if want := time.Date(2026, 9, 24, 13, 0, 0, 0, time.UTC); !got.Equal(want) {
		t.Errorf("Next = %v, want %v", got, want)
	}

	// Across the DST change (2026-11-01), 06:00 local stays 06:00 local.
	got = s.Next(time.Date(2026, 11, 1, 6, 0, 0, 0, la))
	if local := got.In(la); local.Hour() != 6 || local.Day() != 2 {
		t.Errorf("Next across DST = %v, want 06:00 on Nov 2 local", local)
	}
}
