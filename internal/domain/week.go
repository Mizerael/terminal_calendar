package domain

import (
	"math"
	"time"
)

// MondayOf returns midnight (local) of the Monday of the week containing day.
func MondayOf(day time.Time) time.Time {
	if day.IsZero() {
		return time.Time{}
	}
	wd := (int(day.Weekday()) + 6) % 7 // Mon=0 … Sun=6
	return StartOfDay(day).AddDate(0, 0, -wd)
}

// StartOfDay returns midnight (local) of the given day.
func StartOfDay(day time.Time) time.Time {
	return time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, day.Location())
}

// ClampHour keeps an hour (0..23) in range.
func ClampHour(p *int) {
	if *p < 0 {
		*p = 0
	}
	if *p > 23 {
		*p = 23
	}
}

// DayIndex maps an event to its day index (0=Monday) within the week that
// starts at weekStart, or -1 if the event falls outside that week. Both sides
// are evaluated in weekStart's zone so an offset-carrying event maps to a
// consistent calendar day regardless of the timezone it was stored in.
func DayIndex(e *Event, weekStart time.Time) int {
	start := e.StartTime()
	if start.IsZero() {
		return -1
	}
	refWeek := StartOfDay(weekStart)
	startDay := StartOfDay(start.In(weekStart.Location()))
	diff := int(startDay.Sub(refWeek).Hours()) / 24
	if diff < 0 || diff > 6 {
		return -1
	}
	return diff
}

// HourRange returns the half-open [start, end) hour span of a timed event of
// the given weekStart's day, clamped to the 24-hour day. The end hour is
// inclusive of any sub-hour tail so that a short event (e.g. a recurring
// 09:00-09:30) still fills its starting hour row instead of becoming a
// zero-length, invisible block. ok is false for all-day events or when the
// event has no usable start time.
func HourRange(e *Event) (start, end int, ok bool) {
	if e.AllDay || e.StartTime().IsZero() {
		return 0, 0, false
	}
	s := e.Start
	en := e.EndTime()
	if !en.After(s) {
		en = s.Add(time.Hour)
	}
	sh := s.Hour()
	var eh int
	if !en.After(s) {
		eh = sh + 1
	} else {
		// End hour as minutes past the start's own midnight; ceil so a partial
		// end (e.g. 09:30) occupies hour 9, while an exact hour boundary is
		// exclusive. Values roll naturally past 24 for midnight-crossing events.
		dayStart := StartOfDay(s)
		eh = int(math.Ceil(en.Sub(dayStart).Hours()))
	}
	if eh < sh {
		eh = sh
	}
	if eh > 24 {
		eh = 24
	}
	return sh, eh, true
}
