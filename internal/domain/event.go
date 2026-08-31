// Package domain holds the framework-independent entities and pure business
// rules of the calendar TUI. It has no dependencies on the UI, the Google API
// or any other package in the repo, satisfying the innermost layer of Clean
// Architecture.
package domain

import "time"

// Calendar is a calendar the user can view events from.
type Calendar struct {
	ID      string
	Summary string
	Primary bool
}

// Event is a calendar event, free of any transport/API representation. Times
// are parsed, concrete time.Time values in the event's own location. For a
// full-day event Start/End hold the inclusive start / exclusive end dates at
// local midnight and AllDay is true.
type Event struct {
	ID               string
	Summary          string
	Location         string
	Description      string
	RecurringEventID string

	Start time.Time
	End   time.Time
	// AllDay marks a date-only (full-day) event.
	AllDay bool
	// TimeZone is the IANA zone name attached to the event when known, kept so
	// the event round-trips without losing zone fidelity (Google accepts a
	// naive dateTime together with an explicit timeZone).
	TimeZone string

	// CalendarID and CalendarSummary identify the owning calendar. They are
	// filled in when the event is fetched so a merged multi-calendar view can
	// color and route it.
	CalendarID      string
	CalendarSummary string
}

// StartTime returns the event's start instant.
func (e *Event) StartTime() time.Time { return e.Start }

// EndTime returns the event's end instant, falling back to its start when end
// is absent.
func (e *Event) EndTime() time.Time {
	if e.End.IsZero() {
		return e.Start
	}
	return e.End
}

// Timezone returns the event's IANA timezone name, or "" when none is known.
func (e *Event) Timezone() string { return e.TimeZone }
