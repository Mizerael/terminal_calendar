package tui

import (
	"encoding/json"
	"os"
)

// statePath is the default location of the persisted calendar selection. It
// can be overridden via SetStatePath (wired from main).
const defaultStatePath = "calendar_state.json"

// persistedState is the subset of multi-calendar selection we remember between
// runs: which calendars the user has enabled and which is the create-target.
type persistedState struct {
	Enabled     map[string]bool `json:"enabled,omitempty"`
	TargetCalID string          `json:"target,omitempty"`
}

// SetStatePath overrides where the calendar selection is persisted.
func (m *Model) SetStatePath(p string) {
	m.statePath = p
}

// SetInitialTarget sets the create-target calendar used unless a previously
// persisted selection overrides it. Called by main with GOOGLE_CALENDAR_ID.
func (m *Model) SetInitialTarget(id string) {
	if m.targetCalID == "" && id != "" {
		m.targetCalID = id
	}
}

// statePath returns the current state file path, ensuring it is non-empty.
func (m *Model) statePathOrDefault() string {
	if m.statePath == "" {
		return defaultStatePath
	}
	return m.statePath
}

// loadState reads a previously saved calendar selection, if any. It does not
// reconcile the selection against the fetched calendar list; that happens in
// applyDefaultCalendarSelection once calendars are loaded.
func (m *Model) loadState() {
	data, err := os.ReadFile(m.statePathOrDefault())
	if err != nil {
		return
	}
	var s persistedState
	if err := json.Unmarshal(data, &s); err != nil {
		return
	}
	if len(s.Enabled) > 0 {
		m.isEnabled = s.Enabled
	}
	m.targetCalID = s.TargetCalID
}

// saveState writes the current calendar selection to disk.
func (m *Model) saveState() {
	s := persistedState{
		Enabled:     m.isEnabled,
		TargetCalID: m.targetCalID,
	}
	if m.targetCalID == "" && len(m.calendars) > 0 {
		s.TargetCalID = m.primaryCalID()
	}
	data, err := json.Marshal(s)
	if err != nil {
		return
	}
	_ = os.WriteFile(m.statePathOrDefault(), data, 0o600)
}

// applyDefaultCalendarSelection reconciles the persisted (or empty) selection
// against the freshly loaded calendar list. Called once whenever the calendar
// list is fetched.
func (m *Model) applyDefaultCalendarSelection() {
	if m.isEnabled == nil {
		m.isEnabled = map[string]bool{}
	}
	// Drop enabled ids that no longer exist.
	for id := range m.isEnabled {
		if !m.hasCalendar(id) {
			delete(m.isEnabled, id)
		}
	}
	// If nothing is enabled yet, default to showing every calendar.
	if len(m.isEnabled) == 0 {
		for _, cal := range m.calendars {
			m.isEnabled[cal.ID] = true
		}
	}
	// Default the create-target to the primary calendar when unset or invalid.
	if m.targetCalID == "" || !m.hasCalendar(m.targetCalID) {
		m.targetCalID = m.primaryCalID()
	}
}

// primaryCalID returns the user's primary calendar id, falling back to the
// first calendar, or "" if there are none.
func (m *Model) primaryCalID() string {
	for _, cal := range m.calendars {
		if cal.Primary {
			return cal.ID
		}
	}
	if len(m.calendars) > 0 {
		return m.calendars[0].ID
	}
	return ""
}

// hasCalendar reports whether id is present in the calendar list.
func (m *Model) hasCalendar(id string) bool {
	for _, cal := range m.calendars {
		if cal.ID == id {
			return true
		}
	}
	return false
}
