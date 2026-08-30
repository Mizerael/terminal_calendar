package tui

import (
	"context"
	"fmt"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"gitnub.com/Mizerael/terminal_calendar/internal/gcal"
)

// fakeAPI is a configurable stub backing the TUI.
type fakeAPI struct {
	events   []gcal.Event
	listErrs map[time.Time]error
	created  []*gcal.Event
	updated  []*gcal.Event
	deleted  []string
}

func (f *fakeAPI) ListEvents(ctx context.Context, day time.Time) ([]gcal.Event, error) {
	if err := f.listErrs[day]; err != nil {
		return nil, err
	}
	return f.events, nil
}
func (f *fakeAPI) CreateEvent(ctx context.Context, e *gcal.Event) (*gcal.Event, error) {
	f.created = append(f.created, e)
	return e, nil
}
func (f *fakeAPI) UpdateEvent(ctx context.Context, e *gcal.Event) (*gcal.Event, error) {
	f.updated = append(f.updated, e)
	return e, nil
}
func (f *fakeAPI) DeleteEvent(ctx context.Context, id string) error {
	f.deleted = append(f.deleted, id)
	return nil
}

func newTestModel(f *fakeAPI, day time.Time) (*Model, error) {
	m, err := New(f)
	if err != nil {
		return nil, err
	}
	m.day = day
	return m, nil
}

func msgKey(s string) tea.Msg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

// upd runs m.Update and returns the resulting *Model.
func upd(t *testing.T, m *Model, msg tea.Msg) *Model {
	t.Helper()
	im, _ := m.Update(msg)
	nm, ok := im.(*Model)
	if !ok {
		t.Fatalf("Update returned %T, want *Model", im)
	}
	return nm
}

// run is upd plus execution of any returned commands (and their messages),
// so async flows like create/update/delete complete in-test.
func run(t *testing.T, m *Model, msg tea.Msg) *Model {
	t.Helper()
	im, cmd := m.Update(msg)
	m, ok := im.(*Model)
	if !ok {
		t.Fatalf("Update returned %T, want *Model", im)
	}
	for i := 0; cmd != nil && i < 10; i++ {
		res := cmd()
		if res == nil {
			break
		}
		im, cmd = m.Update(res)
		m, ok = im.(*Model)
		if !ok {
			t.Fatalf("Update returned %T, want *Model", im)
		}
	}
	return m
}

func TestLoadEventsPopulatesList(t *testing.T) {
	f := &fakeAPI{
		events: []gcal.Event{
			{Summary: "Standup", Start: &gcal.EDT{DateTime: "2026-08-30T09:00:00Z"}},
			{Summary: "Review", Start: &gcal.EDT{DateTime: "2026-08-30T13:00:00Z"}},
		},
	}
	m, err := newTestModel(f, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	m = upd(t, m, msgEventsLoaded{events: f.events, day: m.day})
	if len(m.events) != 2 {
		t.Fatalf("want 2 events, got %d", len(m.events))
	}
	if !m.loaded || m.loadErr != nil {
		t.Fatalf("expected loaded state without error")
	}
}

func TestKeyNavMovesCursor(t *testing.T) {
	f := &fakeAPI{
		events: []gcal.Event{
			{Summary: "A", Start: &gcal.EDT{DateTime: "2026-08-30T09:00:00Z"}},
			{Summary: "B", Start: &gcal.EDT{DateTime: "2026-08-30T10:00:00Z"}},
			{Summary: "C", Start: &gcal.EDT{DateTime: "2026-08-30T11:00:00Z"}},
		},
	}
	m, _ := newTestModel(f, time.Now())
	m = upd(t, m, msgEventsLoaded{events: f.events, day: m.day})

	cases := []struct {
		key  string
		want int
	}{
		{"down", 1},
		{"j", 2},
		{"down", 2}, // clamped at bottom
		{"up", 1},
		{"k", 0},
		{"up", 0}, // clamped at top
		{"G", 2},
		{"g", 0},
	}
	for _, c := range cases {
		m = upd(t, m, msgKey(c.key))
		if m.cursor != c.want {
			t.Errorf("key %q: cursor = %d, want %d", c.key, m.cursor, c.want)
		}
	}
}

func TestDayNavigation(t *testing.T) {
	f := &fakeAPI{events: []gcal.Event{
		{Summary: "X", Start: &gcal.EDT{DateTime: "2026-08-30T09:00:00Z"}},
	}}
	m, _ := newTestModel(f, time.Date(2026, 8, 30, 0, 0, 0, 0, time.Local))
	m = upd(t, m, msgEventsLoaded{events: f.events, day: m.day})

	before := m.day
	m = upd(t, m, msgKey("right"))
	if !m.day.After(before) {
		t.Fatal("right should advance the day")
	}
	if !m.loading {
		t.Fatal("changing day should set loading")
	}
	m = upd(t, m, msgKey("left"))
	if !m.day.Equal(before) {
		t.Fatal("left should go back to the previous day")
	}
	m = upd(t, m, msgKey("t"))
	if !m.day.Equal(today()) {
		t.Fatal("t should jump to today")
	}
}

func TestCreateEventFromForm(t *testing.T) {
	f := &fakeAPI{}
	m, _ := newTestModel(f, time.Now())

	m = upd(t, m, msgKey("n"))
	if m.screen != screenForm {
		t.Fatal("n should open the form")
	}

	setForm := func(id fieldID, v string) {
		m.form.inputs[id].SetValue(v)
	}
	setForm(fieldTitle, "Retro")
	setForm(fieldStartDate, "2026-09-01")
	setForm(fieldStartTime, "10:30")
	setForm(fieldEndDate, "2026-09-01")
	setForm(fieldEndTime, "11:00")

	m = run(t, m, msgKey("enter"))
	if len(f.created) != 1 {
		t.Fatalf("expected 1 created event, got %d", len(f.created))
	}
	ev := f.created[0]
	if ev.Summary != "Retro" {
		t.Errorf("summary = %q", ev.Summary)
	}
	start, err := time.Parse(time.RFC3339, ev.Start.DateTime)
	if err != nil {
		t.Fatalf("bad start datetime %q: %v", ev.Start.DateTime, err)
	}
	if start.Format("2006-01-02 15:04") != "2026-09-01 10:30" {
		t.Errorf("unexpected start: %v", start)
	}
	end, err := time.Parse(time.RFC3339, ev.End.DateTime)
	if err != nil {
		t.Fatalf("bad end datetime %q: %v", ev.End.DateTime, err)
	}
	if end.Format("2006-01-02 15:04") != "2026-09-01 11:00" {
		t.Errorf("unexpected end: %v", end)
	}
}

func TestAllDayEventFromForm(t *testing.T) {
	f := &fakeAPI{}
	m, _ := newTestModel(f, time.Now())

	m = upd(t, m, msgKey("n"))
	m.form.inputs[fieldTitle].SetValue("Holiday")
	m.form.inputs[fieldStartDate].SetValue("2026-12-24")
	m.form.inputs[fieldEndDate].SetValue("2026-12-25")

	m = run(t, m, msgKey("enter"))
	if len(f.created) != 1 {
		t.Fatalf("expected 1 created event, got %d", len(f.created))
	}
	ev := f.created[0]
	if !ev.AllDay() {
		t.Fatal("expected all-day event")
	}
	if ev.Start.Date != "2026-12-24" || ev.End.Date != "2026-12-25" {
		t.Errorf("unexpected dates: %+v / %+v", ev.Start, ev.End)
	}
}

func TestFormValidationRequiredTitle(t *testing.T) {
	f := &fakeAPI{}
	m, _ := newTestModel(f, time.Now())
	m = upd(t, m, msgKey("n"))
	m = run(t, m, msgKey("enter"))
	if len(f.created) != 0 {
		t.Fatal("should not create an event without a title")
	}
	if m.form.err == "" {
		t.Fatal("expected validation error message")
	}
}

func TestFormValidationBadTime(t *testing.T) {
	f := &fakeAPI{}
	m, _ := newTestModel(f, time.Now())
	m = upd(t, m, msgKey("n"))
	m.form.inputs[fieldTitle].SetValue("X")
	m.form.inputs[fieldStartTime].SetValue("not a time")
	m = run(t, m, msgKey("enter"))
	if len(f.created) != 0 {
		t.Fatal("should not create an event with an invalid time")
	}
}

func TestFormValidationEndBeforeStart(t *testing.T) {
	f := &fakeAPI{}
	m, _ := newTestModel(f, time.Now())
	m = upd(t, m, msgKey("n"))
	m.form.inputs[fieldTitle].SetValue("X")
	m.form.inputs[fieldStartTime].SetValue("15:00")
	m.form.inputs[fieldEndTime].SetValue("14:00")
	m = run(t, m, msgKey("enter"))
	if len(f.created) != 0 {
		t.Fatal("should reject end before start")
	}
}

func TestEditEventPrefillsForm(t *testing.T) {
	f := &fakeAPI{
		events: []gcal.Event{{
			Summary:     "Old",
			Location:    "Room 1",
			Description: "notes",
			Start:       &gcal.EDT{DateTime: "2026-08-30T09:00:00Z", TimeZone: "Europe/Berlin"},
			End:         &gcal.EDT{DateTime: "2026-08-30T10:00:00Z", TimeZone: "Europe/Berlin"},
		}},
	}
	m, _ := newTestModel(f, time.Now())
	m = upd(t, m, msgEventsLoaded{events: f.events, day: m.day})
	m = run(t, m, msgKey("enter"))
	if m.screen != screenForm || m.form.editing == nil {
		t.Fatal("enter should open the form in edit mode")
	}
	if got := m.form.value(fieldTitle); got != "Old" {
		t.Errorf("title prefilled = %q", got)
	}
	if got := m.form.value(fieldStartTime); got != "09:00" {
		t.Errorf("start time prefilled = %q", got)
	}

	// change title and save → update, not create
	m.form.inputs[fieldTitle].SetValue("New")
	m = run(t, m, msgKey("enter"))
	if len(f.updated) != 1 || len(f.created) != 0 {
		t.Fatalf("updated=%d created=%d, want update=1 create=0", len(f.updated), len(f.created))
	}
	if f.updated[0].Summary != "New" {
		t.Errorf("updated summary = %q", f.updated[0].Summary)
	}
}

func TestDeleteFlow(t *testing.T) {
	f := &fakeAPI{
		events: []gcal.Event{{
			Id:      "abc",
			Summary: "doomed",
			Start:   &gcal.EDT{DateTime: "2026-08-30T09:00:00Z"},
		}},
	}
	m, _ := newTestModel(f, time.Now())
	m = upd(t, m, msgEventsLoaded{events: f.events, day: m.day})

	m = upd(t, m, msgKey("d"))
	if m.screen != screenConfirm {
		t.Fatal("d should open the confirmation screen")
	}
	m = run(t, m, msgKey("y"))
	if len(f.deleted) != 1 || f.deleted[0] != "abc" {
		t.Fatalf("deleted = %v", f.deleted)
	}
	if m.screen != screenList {
		t.Fatal("should return to the list after delete")
	}
}

func TestDeleteCancel(t *testing.T) {
	f := &fakeAPI{
		events: []gcal.Event{{
			Id:      "abc",
			Summary: "survivor",
			Start:   &gcal.EDT{DateTime: "2026-08-30T09:00:00Z"},
		}},
	}
	m, _ := newTestModel(f, time.Now())
	m = upd(t, m, msgEventsLoaded{events: f.events, day: m.day})

	m = upd(t, m, msgKey("d"))
	m = upd(t, m, msgKey("n"))
	if len(f.deleted) != 0 {
		t.Fatal("cancel must not delete")
	}
	if m.screen != screenList {
		t.Fatal("should return to the list after cancel")
	}
}

func TestLoadErrorSetsError(t *testing.T) {
	f := &fakeAPI{listErrs: map[time.Time]error{
		time.Now(): fmt.Errorf("boom"),
	}}
	m, _ := newTestModel(f, time.Now())
	m = upd(t, m, msgError{err: fmt.Errorf("boom")})
	if m.loadErr == nil {
		t.Fatal("expected load error to be stored")
	}
}
