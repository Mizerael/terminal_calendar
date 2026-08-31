package tui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Mizerael/terminal_calendar/internal/domain"
	"github.com/Mizerael/terminal_calendar/internal/usecase"
)

// loadCalendars drives the full async load path (Init -> ListCalendars ->
// loadWeekEvents) and returns after events are grouped into the model.
func loadCalendars(t *testing.T, m *Model, f *fakeAPI) *Model {
	t.Helper()
	cmd := m.Init()
	res := cmd()
	im, cmd := m.Update(res)
	m, ok := im.(*Model)
	if !ok {
		t.Fatalf("calendars update returned %T, want *Model", im)
	}
	if cmd == nil {
		t.Fatal("expected loadWeekEvents command after calendars loaded")
	}
	res = cmd()
	im, _ = m.Update(res)
	m, ok = im.(*Model)
	if !ok {
		t.Fatalf("events update returned %T, want *Model", im)
	}
	return m
}

func TestMergedLoadAcrossEnabledCalendars(t *testing.T) {
	mon := time.Date(2026, 8, 31, 0, 0, 0, 0, time.Local)
	f := &fakeAPI{
		calendars: []domain.Calendar{
			{ID: "work", Summary: "Work", Primary: true},
			{ID: "personal", Summary: "Personal"},
		},
		mapEvents: map[string][]domain.Event{
			"work":     {makeEvent("Standup", "a", 9, 0)},
			"personal": {makeEvent("Yoga", "b", 18, 0)},
		},
	}
	m, _ := newTestModel(f)
	m = upd(t, m, tea.WindowSizeMsg{Width: 100, Height: 40})
	m.weekStart = mon
	m = loadCalendars(t, m, f)

	if !m.loaded {
		t.Fatalf("expected loaded state")
	}
	if m.totalEvents() != 2 {
		t.Fatalf("total events = %d, want 2", m.totalEvents())
	}
	evs := m.weekEvents[0]
	got := map[string]bool{}
	for _, e := range evs {
		got[e.CalendarID] = true
	}
	if !got["work"] || !got["personal"] {
		t.Errorf("expected both calendars' events merged, got %v", got)
	}
}

func TestCalendarColorStable(t *testing.T) {
	a := calendarColor("work")
	b := calendarColor("work")
	if a != b {
		t.Errorf("calendarColor not stable: %q != %q", a, b)
	}
	_ = calendarColor("personal")
}

func TestPickerToggleAndTarget(t *testing.T) {
	f := &fakeAPI{
		calendars: []domain.Calendar{
			{ID: "work", Summary: "Work", Primary: true},
			{ID: "personal", Summary: "Personal"},
		},
	}
	m, _ := newTestModel(f)
	m.calendars = f.calendars
	m.sel.Enabled = map[string]bool{"work": true, "personal": true}
	m.picker = true

	m.pickerIndex = 1
	m = upd(t, m, msgKey("enter"))
	if m.sel.Enabled["personal"] {
		t.Errorf("personal should be disabled after toggle")
	}
	m = upd(t, m, msgKey("t"))
	if m.sel.Target != "personal" {
		t.Errorf("target = %q, want personal", m.sel.Target)
	}
	if !m.sel.Enabled["work"] {
		t.Errorf("work should remain enabled")
	}
}

func TestCreateGoesToTargetCalendar(t *testing.T) {
	f := &fakeAPI{
		calendars: []domain.Calendar{{ID: "work", Summary: "Work", Primary: true}},
		events:    []domain.Event{},
	}
	m, _ := newTestModel(f)
	m.calendars = f.calendars
	m.sel.Target = "work"

	m = upd(t, m, tea.WindowSizeMsg{Width: 100, Height: 40})
	m = upd(t, m, msgKey("n"))
	if m.screen != screenForm {
		t.Fatal("n should open the form")
	}

	m.form.inputs[fieldTitle].SetValue("New meeting")
	m.form.inputs[fieldStartDate].SetValue("2026-08-31")
	m.form.inputs[fieldStartTime].SetValue("")
	m.form.inputs[fieldEndDate].SetValue("2026-08-31")
	m.form.inputs[fieldEndTime].SetValue("")

	m = run(t, m, msgKey("enter"))
	if len(f.created) != 1 {
		t.Fatalf("created events = %d, want 1", len(f.created))
	}
	if f.created[0].Summary != "New meeting" {
		t.Errorf("created summary = %q", f.created[0].Summary)
	}
}

func TestStateRoundTrip(t *testing.T) {
	path := t.TempDir() + "/state.json"

	f := &fakeAPI{
		calendars: []domain.Calendar{
			{ID: "work", Summary: "Work", Primary: true},
			{ID: "personal", Summary: "Personal"},
		},
	}
	m, _ := New(usecase.NewCalendarService(f))
	m.SetStatePath(path)
	m.calendars = f.calendars
	m.sel.Enabled = map[string]bool{"work": true, "personal": false}
	m.sel.Target = "personal"
	m.saveState()

	m2, _ := New(usecase.NewCalendarService(f))
	m2.SetStatePath(path)
	m2.loadState()
	m2.calendars = f.calendars
	if !m2.sel.Enabled["work"] || m2.sel.Enabled["personal"] {
		t.Errorf("enabled state not persisted: %v", m2.sel.Enabled)
	}
	if m2.sel.Target != "personal" {
		t.Errorf("target not persisted: %q", m2.sel.Target)
	}
}
