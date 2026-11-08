package tui

import (
	"bytes"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Mizerael/terminal_calendar/internal/domain"
	"github.com/Mizerael/terminal_calendar/internal/usecase"
)

func TestStatePathLoadsAndSavesSelection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "selection.json")
	if err := os.WriteFile(path, []byte(`{"enabled":{"work":true,"personal":false},"target":"work"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	m := &Model{}
	m.SetStatePath(path)
	if !m.sel.Enabled["work"] || m.sel.Enabled["personal"] || m.sel.Target != "work" {
		t.Fatalf("loaded selection = %#v, want work enabled and target", m.sel)
	}
	m.calendars = []domain.Calendar{{ID: "work", Primary: true}}
	m.saveState()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var got persistedState
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if !got.Enabled["work"] || got.TargetCalID != "work" {
		t.Errorf("saved selection = %#v, want work enabled and target", got)
	}
}

func TestLoadStateReportsMalformedFileAndKeepsRunning(t *testing.T) {
	path := filepath.Join(t.TempDir(), "selection.json")
	if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	previousWriter, previousFlags := log.Writer(), log.Flags()
	log.SetOutput(&output)
	log.SetFlags(0)
	t.Cleanup(func() {
		log.SetOutput(previousWriter)
		log.SetFlags(previousFlags)
	})

	m := &Model{}
	m.SetStatePath(path)
	if len(m.sel.Enabled) != 0 || m.sel.Target != "" {
		t.Fatalf("malformed state changed selection: %#v", m.sel)
	}
	if !strings.Contains(output.String(), "could not decode calendar selection") {
		t.Errorf("expected malformed state warning, got %q", output.String())
	}
}

func TestSaveStateReportsWriteErrorWithoutPanicking(t *testing.T) {
	dir := t.TempDir()
	var output bytes.Buffer
	previousWriter, previousFlags := log.Writer(), log.Flags()
	log.SetOutput(&output)
	log.SetFlags(0)
	t.Cleanup(func() {
		log.SetOutput(previousWriter)
		log.SetFlags(previousFlags)
	})

	m := &Model{calendars: []domain.Calendar{{ID: "work", Primary: true}}}
	m.SetStatePath(dir)
	m.saveState()
	if !strings.Contains(output.String(), "could not write calendar selection") {
		t.Errorf("expected write warning, got %q", output.String())
	}
}

func TestDisabledStatePersistenceDoesNotReadOrWrite(t *testing.T) {
	m := &Model{}
	m.SetStatePath("-")
	m.sel = usecase.Selection{Enabled: map[string]bool{"work": true}, Target: "work"}
	m.saveState()
	if !m.sel.Enabled["work"] || m.sel.Target != "work" {
		t.Fatalf("disabled persistence changed selection: %#v", m.sel)
	}
}

func TestAccountSwitchLoadsOnlyMatchingPrimaryCalendarState(t *testing.T) {
	dir := t.TempDir()
	path := stateFilePath(dir)
	data, err := json.Marshal(persistedState{
		Enabled: map[string]bool{"legacy": true}, TargetCalID: "legacy",
		Accounts: map[string]persistedSelection{
			accountStateKey("primary-a"): {Enabled: map[string]bool{"shared": true}, TargetCalID: "shared"},
			accountStateKey("primary-b"): {Enabled: map[string]bool{"primary-b": true}, TargetCalID: "primary-b"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}

	m := &Model{}
	m.SetStateDirectory(dir)
	m.calendars = []domain.Calendar{{ID: "primary-a", Primary: true}, {ID: "shared"}}
	_, _ = m.Update(msgCalendarsLoaded{})
	if !m.sel.Enabled["shared"] || m.sel.Enabled["legacy"] || m.sel.Target != "shared" {
		t.Fatalf("account A's saved selection not loaded: %#v", m.sel)
	}

	// The same model can receive refreshed calendars for a different account.
	m.calendars = []domain.Calendar{{ID: "primary-b", Primary: true}, {ID: "shared"}}
	_, _ = m.Update(msgCalendarsLoaded{})
	if !m.sel.Enabled["primary-b"] || m.sel.Enabled["shared"] {
		t.Fatalf("account B did not load its own selection: %#v", m.sel.Enabled)
	}
	if m.sel.Target != "primary-b" {
		t.Errorf("account B target = %q, want primary-b", m.sel.Target)
	}

	// An unknown identity must not save, but a later known identity must still
	// load its own selection even in this same model.
	m.calendars = []domain.Calendar{{ID: "shared"}}
	_, _ = m.Update(msgCalendarsLoaded{})
	if m.stateAccountKey != "" {
		t.Fatalf("unknown identity retained account key %q", m.stateAccountKey)
	}
	m.calendars = []domain.Calendar{{ID: "primary-b", Primary: true}, {ID: "shared"}}
	_, _ = m.Update(msgCalendarsLoaded{})
	if !m.sel.Enabled["primary-b"] || m.sel.Enabled["shared"] || m.sel.Target != "primary-b" {
		t.Fatalf("unknown-to-known identity failed to restore B selection: %#v", m.sel)
	}

	data, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var saved persistedState
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatal(err)
	}
	if !saved.Enabled["legacy"] || saved.TargetCalID != "legacy" {
		t.Errorf("legacy unowned selection was not preserved: %#v", saved)
	}
	if _, ok := saved.Accounts[accountStateKey("primary-a")]; !ok {
		t.Error("account A selection was lost when saving account B")
	}
	if _, ok := saved.Accounts[accountStateKey("primary-b")]; !ok {
		t.Error("account B selection was not retained")
	}
}

func TestUnknownAccountDoesNotLoadOrSaveState(t *testing.T) {
	dir := t.TempDir()
	path := stateFilePath(dir)
	malformed := []byte("{")
	if err := os.WriteFile(path, malformed, 0o600); err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	previousWriter, previousFlags := log.Writer(), log.Flags()
	log.SetOutput(&output)
	log.SetFlags(0)
	t.Cleanup(func() {
		log.SetOutput(previousWriter)
		log.SetFlags(previousFlags)
	})
	m := &Model{}
	m.SetStateDirectory(dir)
	m.calendars = []domain.Calendar{{ID: "shared", Primary: false}}
	_, _ = m.Update(msgCalendarsLoaded{})
	if m.statePath != path || m.sel.Target != "shared" || !m.sel.Enabled["shared"] {
		t.Fatalf("unknown account loaded persisted state: path=%q selection=%#v", m.statePath, m.sel)
	}
	if strings.Contains(output.String(), "could not decode calendar selection") {
		t.Errorf("unknown account attempted to read state: %q", output.String())
	}
	m.sel.Enabled = map[string]bool{"shared": true}
	m.saveState()
	actual, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(actual) != string(malformed) {
		t.Errorf("unknown account changed state file: %q", actual)
	}
}
