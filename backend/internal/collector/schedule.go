package collector

import (
	"fmt"
	"slices"
	"strings"
	"time"
)

// Schedule is a set of times of day at which the collector runs, every day,
// in a given time zone.
type Schedule struct {
	minutes []int // minutes after midnight, sorted, unique
	loc     *time.Location
}

// ParseSchedule parses a comma-separated list of 24-hour "HH:MM" times, e.g.
// "00:00,08:00,16:00".
func ParseSchedule(spec string, loc *time.Location) (Schedule, error) {
	var minutes []int

	for part := range strings.SplitSeq(spec, ",") {
		t, err := time.Parse("15:04", strings.TrimSpace(part))
		if err != nil {
			return Schedule{}, fmt.Errorf("schedule time %q must be HH:MM (24-hour)", strings.TrimSpace(part))
		}

		minutes = append(minutes, t.Hour()*60+t.Minute())
	}

	slices.Sort(minutes)
	minutes = slices.Compact(minutes)

	return Schedule{minutes: minutes, loc: loc}, nil
}

// Next returns the first scheduled time strictly after after.
func (s Schedule) Next(after time.Time) time.Time {
	local := after.In(s.loc)
	year, month, day := local.Date()

	// A scheduled time always exists within today or tomorrow; the third day
	// only matters when a DST jump swallows tomorrow's last slot.
	for offset := range 3 {
		for _, m := range s.minutes {
			candidate := time.Date(year, month, day+offset, m/60, m%60, 0, 0, s.loc)
			if candidate.After(after) {
				return candidate
			}
		}
	}

	panic("unreachable: schedule has no times")
}

// String formats the schedule as it was configured, e.g.
// "00:00,08:00,16:00 UTC".
func (s Schedule) String() string {
	times := make([]string, len(s.minutes))
	for i, m := range s.minutes {
		times[i] = fmt.Sprintf("%02d:%02d", m/60, m%60)
	}

	return strings.Join(times, ",") + " " + s.loc.String()
}
