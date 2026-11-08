package tui

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log"
	"os"
	"path/filepath"

	"github.com/Mizerael/terminal_calendar/internal/usecase"
)

// persistedState is the subset of multi-calendar selection we remember between
// runs: which calendars the user has enabled and which is the create-target.
type persistedState struct {
	Enabled     map[string]bool               `json:"enabled,omitempty"`
	TargetCalID string                        `json:"target,omitempty"`
	Accounts    map[string]persistedSelection `json:"accounts,omitempty"`
}

type persistedSelection struct {
	Enabled     map[string]bool `json:"enabled,omitempty"`
	TargetCalID string          `json:"target,omitempty"`
}

// SetStatePath overrides where the calendar selection is persisted.
func (m *Model) SetStatePath(p string) {
	m.statePath = p
	m.stateDir = ""
	m.stateAccountKey = ""
	m.stateIdentity = ""
	m.stateIdentitySet = true
	m.sel.Enabled = map[string]bool{}
	m.sel.Target = m.initialTarget
	m.loadState()
}

// SetStateDirectory enables account-scoped persistence in dir. The concrete
// file is selected only after the authenticated primary calendar is fetched.
func (m *Model) SetStateDirectory(dir string) {
	m.stateDir = dir
	m.statePath = ""
	m.stateAccountKey = ""
	m.stateIdentity = ""
	m.stateIdentitySet = false
	m.sel.Enabled = map[string]bool{}
	m.sel.Target = m.initialTarget
}

// SetInitialTarget sets the create-target calendar used unless a previously
// persisted selection overrides it. Called by main with GOOGLE_CALENDAR_ID.
func (m *Model) SetInitialTarget(id string) {
	m.initialTarget = id
	if m.sel.Target == "" && id != "" {
		m.sel.Target = id
	}
}

// statePath returns the current state file path, ensuring it is non-empty.
func (m *Model) statePathOrDefault() string {
	if m.statePath == "" {
		return "-"
	}
	return m.statePath
}

func (m *Model) loadAccountState() {
	if m.stateDir == "" {
		return
	}
	primaryID := ""
	for _, calendar := range m.calendars {
		if calendar.Primary {
			primaryID = calendar.ID
			break
		}
	}
	if m.stateIdentitySet && primaryID == m.stateIdentity {
		return
	}
	m.stateIdentity = primaryID
	m.stateIdentitySet = true
	m.statePath = stateFilePath(m.stateDir)
	m.stateAccountKey = ""
	m.sel.Enabled = map[string]bool{}
	m.sel.Target = m.initialTarget
	if primaryID == "" {
		return
	}
	m.stateAccountKey = accountStateKey(primaryID)
	m.loadState()
}

func stateFilePath(dir string) string {
	return filepath.Join(dir, "calendar_state.json")
}

func accountStateKey(primaryCalendarID string) string {
	key := sha256.Sum256([]byte(primaryCalendarID))
	return "sha256:" + hex.EncodeToString(key[:])
}

// loadState reads a previously saved calendar selection, if any. It does not
// reconcile the selection against the fetched calendar list; that happens in
// reconcileSelection once calendars are loaded.
func (m *Model) loadState() {
	path := m.statePathOrDefault()
	if path == "-" || (m.stateDir != "" && m.stateAccountKey == "") {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("warning: could not read calendar selection from %q: %v", path, err)
		}
		return
	}
	var s persistedState
	if err := json.Unmarshal(data, &s); err != nil {
		log.Printf("warning: could not decode calendar selection from %q: %v", path, err)
		return
	}
	if m.stateAccountKey != "" {
		accountSelection, ok := s.Accounts[m.stateAccountKey]
		if !ok {
			// Legacy, unowned selection fields are intentionally left untouched
			// but are not assigned to an account implicitly.
			return
		}
		m.sel.Enabled = accountSelection.Enabled
		m.sel.Target = accountSelection.TargetCalID
		return
	}
	if len(s.Enabled) > 0 {
		m.sel.Enabled = s.Enabled
	}
	m.sel.Target = s.TargetCalID
}

// saveState writes the current calendar selection to disk.
func (m *Model) saveState() {
	path := m.statePathOrDefault()
	if path == "-" || (m.stateDir != "" && m.stateAccountKey == "") {
		return
	}
	selection := persistedSelection{Enabled: m.sel.Enabled, TargetCalID: m.sel.Target}
	if m.sel.Target == "" && len(m.calendars) > 0 {
		selection.TargetCalID = usecase.PrimaryCalID(m.calendars)
	}
	var s persistedState
	if m.stateAccountKey != "" {
		if data, err := os.ReadFile(path); err == nil {
			if err := json.Unmarshal(data, &s); err != nil {
				log.Printf("warning: could not decode calendar selection from %q: %v", path, err)
				return
			}
		} else if !os.IsNotExist(err) {
			log.Printf("warning: could not read calendar selection from %q: %v", path, err)
			return
		}
		if s.Accounts == nil {
			s.Accounts = make(map[string]persistedSelection)
		}
		s.Accounts[m.stateAccountKey] = selection
	} else {
		s.Enabled = selection.Enabled
		s.TargetCalID = selection.TargetCalID
	}
	data, err := json.Marshal(s)
	if err != nil {
		log.Printf("warning: could not encode calendar selection for %q: %v", path, err)
		return
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		log.Printf("warning: could not write calendar selection to %q: %v", path, err)
	}
}

// reconcileSelection brings the calendar selection in line with the fetched
// calendar list (dropping stale ids, defaulting to all and a primary target).
// The policy lives in usecase.Selection; this adapter just applies it.
func (m *Model) reconcileSelection() {
	m.sel.Reconcile(m.calendars)
}
