package tui

import (
	"encoding/json"
	"os"

	"github.com/Mizerael/terminal_calendar/internal/usecase"
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
	if m.sel.Target == "" && id != "" {
		m.sel.Target = id
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
// reconcileSelection once calendars are loaded.
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
		m.sel.Enabled = s.Enabled
	}
	m.sel.Target = s.TargetCalID
}

// saveState writes the current calendar selection to disk.
func (m *Model) saveState() {
	s := persistedState{
		Enabled:     m.sel.Enabled,
		TargetCalID: m.sel.Target,
	}
	if m.sel.Target == "" && len(m.calendars) > 0 {
		s.TargetCalID = usecase.PrimaryCalID(m.calendars)
	}
	data, err := json.Marshal(s)
	if err != nil {
		return
	}
	_ = os.WriteFile(m.statePathOrDefault(), data, 0o600)
}

// reconcileSelection brings the calendar selection in line with the fetched
// calendar list (dropping stale ids, defaulting to all and a primary target).
// The policy lives in usecase.Selection; this adapter just applies it.
func (m *Model) reconcileSelection() {
	m.sel.Reconcile(m.calendars)
}
