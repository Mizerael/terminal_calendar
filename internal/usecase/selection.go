package usecase

import "github.com/Mizerael/terminal_calendar/internal/domain"

// Selection is the user's multi-calendar view configuration: which calendars
// are shown merged, and which is the create-target for new events. It is a
// pure value type; the persistence of it is owned by the UI adapter layer.
type Selection struct {
	Enabled map[string]bool
	Target  string
}

// Reconcile brings a (possibly persisted or empty) selection in line with the
// fetched calendar list: it drops enabled ids that no longer exist, defaults
// to showing every calendar when nothing is enabled, and falls the create
// target back to the primary calendar when unset or invalid.
func (s *Selection) Reconcile(calendars []domain.Calendar) {
	if s.Enabled == nil {
		s.Enabled = map[string]bool{}
	}
	has := func(id string) bool {
		for _, c := range calendars {
			if c.ID == id {
				return true
			}
		}
		return false
	}
	for id := range s.Enabled {
		if !has(id) {
			delete(s.Enabled, id)
		}
	}
	if len(s.Enabled) == 0 {
		for _, c := range calendars {
			s.Enabled[c.ID] = true
		}
	}
	if s.Target == "" || !has(s.Target) {
		s.Target = PrimaryCalID(calendars)
	}
}

// PrimaryCalID returns the user's primary calendar id, falling back to the
// first calendar, or "" if there are none.
func PrimaryCalID(calendars []domain.Calendar) string {
	for _, c := range calendars {
		if c.Primary {
			return c.ID
		}
	}
	if len(calendars) > 0 {
		return calendars[0].ID
	}
	return ""
}
