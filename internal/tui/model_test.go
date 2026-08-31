package tui

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"gitnub.com/Mizerael/terminal_calendar/internal/gcal"
)

// fakeAPI is a configurable stub backing the TUI.
type fakeAPI struct {
	events  []gcal.Event
	listErr error
	created []*gcal.Event
	updated []*gcal.Event
	deleted []string
}

func (f *fakeAPI) ListEventsRange(ctx context.Context, start, end time.Time) ([]gcal.Event, error) {
	if f.listErr != nil {
		return nil, f.listErr
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

// mkEvent builds an event with an explicit UTC time.
func mkEvent(summary, id string, h, m int) gcal.Event {
	return gcal.Event{
		Summary: summary,
		Id:      id,
		Start:   &gcal.EDT{DateTime: fmt.Sprintf("2026-08-31T%02d:%02d:00Z", h, m)},
		End:     &gcal.EDT{DateTime: fmt.Sprintf("2026-08-31T%02d:%02d:00Z", h, m)},
	}
}

func newTestModel(f *fakeAPI) (*Model, error) {
	return New(f)
}

// load groups the fake events into the model just like the real load does.
func load(t *testing.T, m *Model, f *fakeAPI) *Model {
	t.Helper()
	return upd(t, m, msgEventsLoaded{events: f.events, weekStart: m.weekStart})
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

func TestLoadGroupsEventsByDay(t *testing.T) {
	// 2026-08-31 is a Monday.
	mon := time.Date(2026, 8, 31, 0, 0, 0, 0, time.Local)
	f := &fakeAPI{events: []gcal.Event{
		mkEvent("Mon morning", "a", 9, 0),
		mkEvent("Mon afternoon", "b", 14, 0),
		{Summary: "Tue day", Id: "c", Start: &gcal.EDT{Date: "2026-09-01"}, End: &gcal.EDT{Date: "2026-09-01"}},
	}}
	m, _ := newTestModel(f)
	m = upd(t, m, tea.WindowSizeMsg{Width: 100, Height: 40})
	m.weekStart = mon
	m = load(t, m, f)

	if !m.loaded || m.loadErr != nil {
		t.Fatalf("expected loaded state, err=%v", m.loadErr)
	}
	if got := len(m.weekEvents[0]); got != 2 {
		t.Errorf("Monday events = %d, want 2", got)
	}
	if got := len(m.weekEvents[1]); got != 1 {
		t.Errorf("Tuesday events = %d, want 1", got)
	}
	if m.totalEvents() != 3 {
		t.Errorf("total = %d, want 3", m.totalEvents())
	}
}

func TestKeyNavMovesAcrossDays(t *testing.T) {
	mon := time.Date(2026, 8, 31, 0, 0, 0, 0, time.Local)
	f := &fakeAPI{events: []gcal.Event{
		mkEvent("A", "a", 9, 0),  // Mon
		mkEvent("B", "b", 11, 0), // Mon
		{Summary: "C", Id: "c", Start: &gcal.EDT{DateTime: "2026-09-01T10:00:00Z"}, End: &gcal.EDT{DateTime: "2026-09-01T10:00:00Z"}}, // Tue
	}}
	m, _ := newTestModel(f)
	m.weekStart = mon
	m = load(t, m, f)

	// cursor starts at first event: Mon, event 0 (A)
	if m.dayIndex != 0 || m.eventIndex != 0 {
		t.Fatalf("initial cursor day=%d evt=%d, want 0,0", m.dayIndex, m.eventIndex)
	}
	m = upd(t, m, msgKey("j")) // -> B (Mon evt 1)
	if m.dayIndex != 0 || m.eventIndex != 1 {
		t.Fatalf("after j: day=%d evt=%d, want 0,1", m.dayIndex, m.eventIndex)
	}
	m = upd(t, m, msgKey("j")) // -> C (Tue evt 0)
	if m.dayIndex != 1 || m.eventIndex != 0 {
		t.Fatalf("after jj: day=%d evt=%d, want 1,0", m.dayIndex, m.eventIndex)
	}
	m = upd(t, m, msgKey("j")) // clamp at end
	if m.dayIndex != 1 || m.eventIndex != 0 {
		t.Fatalf("after jjj: day=%d evt=%d, want 1,0", m.dayIndex, m.eventIndex)
	}
	m = upd(t, m, msgKey("k")) // back to Mon evt 1 (B)
	if m.dayIndex != 0 || m.eventIndex != 1 {
		t.Fatalf("after k: day=%d evt=%d, want 0,1", m.dayIndex, m.eventIndex)
	}
	m = upd(t, m, msgKey("k")) // -> Mon evt 0 (A)
	if m.dayIndex != 0 || m.eventIndex != 0 {
		t.Fatalf("after kk: day=%d evt=%d, want 0,0", m.dayIndex, m.eventIndex)
	}
	m = upd(t, m, msgKey("k")) // clamp at start
	if m.dayIndex != 0 || m.eventIndex != 0 {
		t.Fatalf("after kkk: day=%d evt=%d, want 0,0", m.dayIndex, m.eventIndex)
	}
}

func TestDayNavigationLandsNearTime(t *testing.T) {
	mon := time.Date(2026, 8, 31, 0, 0, 0, 0, time.Local)
	f := &fakeAPI{events: []gcal.Event{
		mkEvent("A", "a", 9, 0),  // Mon 09:00
		mkEvent("B", "b", 14, 0), // Mon 14:00
		{Summary: "C", Id: "c", Start: &gcal.EDT{DateTime: "2026-09-01T10:00:00Z"}, End: &gcal.EDT{DateTime: "2026-09-01T10:00:00Z"}}, // Tue 10:00
		{Summary: "D", Id: "d", Start: &gcal.EDT{DateTime: "2026-09-01T15:00:00Z"}, End: &gcal.EDT{DateTime: "2026-09-01T15:00:00Z"}}, // Tue 15:00
	}}
	m, _ := newTestModel(f)
	m.weekStart = mon
	m = load(t, m, f)

	// Focus B on Mon at 14:00; moving right should land on Tue event closest
	// to 14:00 => D (15:00) beats C (10:00).
	m.dayIndex, m.eventIndex = 0, 1
	m = upd(t, m, msgKey("l"))
	if m.dayIndex != 1 {
		t.Fatalf("after l: day=%d, want 1", m.dayIndex)
	}
	if m.eventIndex != 1 {
		t.Fatalf("after l: evt=%d, want 1 (D near 15:00)", m.eventIndex)
	}
	m = upd(t, m, msgKey("h"))
	if m.dayIndex != 0 {
		t.Fatalf("after h: day=%d, want 0", m.dayIndex)
	}
	if m.eventIndex != 1 {
		t.Fatalf("after h: evt=%d, want 1 (B near 14:00)", m.eventIndex)
	}
}

func TestWeekNavigation(t *testing.T) {
	mon := time.Date(2026, 8, 31, 0, 0, 0, 0, time.Local) // a Monday
	f := &fakeAPI{}
	m, _ := newTestModel(f)
	m.weekStart = mon

	m = upd(t, m, msgKey("]"))
	if !m.weekStart.Equal(mon.AddDate(0, 0, 7)) {
		t.Fatalf("after ]: weekStart = %v, want +1 week", m.weekStart)
	}
	if !m.loading || m.loaded {
		t.Fatalf("week change should start loading")
	}
	m = upd(t, m, msgKey("["))
	if !m.weekStart.Equal(mon) {
		t.Fatalf("after [: weekStart = %v, want back to mon", m.weekStart)
	}
	m = upd(t, m, msgKey("]"))
	m = upd(t, m, msgKey("t"))
	if !m.weekStart.Equal(mondayOf(today())) {
		t.Fatalf("after t: weekStart = %v, want this week", m.weekStart)
	}
}

func TestModalPopupFlow(t *testing.T) {
	mon := time.Date(2026, 8, 31, 0, 0, 0, 0, time.Local)
	f := &fakeAPI{events: []gcal.Event{mkEvent("Detail target", "x", 9, 0)}}
	m, _ := newTestModel(f)
	m.weekStart = mon
	m = load(t, m, f)

	// focus is on the only event
	m = upd(t, m, msgKey("enter"))
	if !m.popup || m.popupEvent == nil {
		t.Fatal("enter should open the modal popup")
	}
	if m.popupEvent.Summary != "Detail target" {
		t.Errorf("popup event = %q", m.popupEvent.Summary)
	}

	// popup is modal: j should NOT move the underlying cursor
	m = upd(t, m, msgKey("j"))
	if m.dayIndex != 0 || m.eventIndex != 0 {
		t.Fatalf("popup should block nav, cursor moved to %d,%d", m.dayIndex, m.eventIndex)
	}

	m = upd(t, m, msgKey("esc"))
	if m.popup || m.popupEvent != nil {
		t.Fatal("esc should close the popup")
	}
}

func TestPopupEditAndDelete(t *testing.T) {
	mon := time.Date(2026, 8, 31, 0, 0, 0, 0, time.Local)
	f := &fakeAPI{events: []gcal.Event{mkEvent("Pop", "1", 9, 0)}}
	m, _ := newTestModel(f)
	m.weekStart = mon
	m = load(t, m, f)

	m = upd(t, m, msgKey("enter"))
	m = upd(t, m, msgKey("e"))
	if m.screen != screenForm || m.form.editing == nil {
		t.Fatal("e from popup should open edit form")
	}
	m = upd(t, m, msgKey("esc"))
	m = upd(t, m, msgKey("enter"))
	m = upd(t, m, msgKey("d"))
	if m.screen != screenConfirm || m.confirm == nil {
		t.Fatal("d from popup should open delete confirmation")
	}
	m = run(t, m, msgKey("n"))
	if len(f.deleted) != 0 {
		t.Fatal("should not delete on cancel")
	}
}

func TestCreateEventFromForm(t *testing.T) {
	f := &fakeAPI{}
	m, _ := newTestModel(f)
	m = upd(t, m, tea.WindowSizeMsg{Width: 100, Height: 40})

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
	start, err := parseFormTime(ev.Start.DateTime)
	if err != nil {
		t.Fatalf("bad start datetime %q: %v", ev.Start.DateTime, err)
	}
	if start.Format("2006-01-02 15:04") != "2026-09-01 10:30" {
		t.Errorf("unexpected start: %v", start)
	}
	end, err := parseFormTime(ev.End.DateTime)
	if err != nil {
		t.Fatalf("bad end datetime %q: %v", ev.End.DateTime, err)
	}
	if end.Format("2006-01-02 15:04") != "2026-09-01 11:00" {
		t.Errorf("unexpected end: %v", end)
	}
	if tz := ev.Start.TimeZone; tz != "" {
		if _, err := time.LoadLocation(tz); err != nil {
			t.Errorf("start timeZone %q is not a valid IANA zone", tz)
		}
	}
}

func TestNewEventPrefillsFocusedDay(t *testing.T) {
	mon := time.Date(2026, 8, 31, 0, 0, 0, 0, time.Local)
	f := &fakeAPI{}
	m, _ := newTestModel(f)
	m.weekStart = mon
	m.dayIndex = 2 // Wednesday = 2026-09-02

	m = upd(t, m, msgKey("n"))
	if got := m.form.value(fieldStartDate); got != "2026-09-02" {
		t.Errorf("start date prefilled = %q, want 2026-09-02", got)
	}
}

// parseFormTime accepts both layouts we emit: with an explicit timeZone
// ("2006-01-02T15:04:05") or with an embedded offset (RFC3339).
func parseFormTime(v string) (time.Time, error) {
	if t, err := time.Parse("2006-01-02T15:04:05", v); err == nil {
		return t, nil
	}
	return time.Parse(time.RFC3339, v)
}

func TestAllDayEventFromForm(t *testing.T) {
	f := &fakeAPI{}
	m, _ := newTestModel(f)

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
	m, _ := newTestModel(f)
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
	m, _ := newTestModel(f)
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
	m, _ := newTestModel(f)
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
	mon := time.Date(2026, 8, 31, 0, 0, 0, 0, time.Local)
	f := &fakeAPI{events: []gcal.Event{{
		Summary:     "Old",
		Location:    "Room 1",
		Description: "notes",
		Start:       &gcal.EDT{DateTime: "2026-08-31T09:00:00Z", TimeZone: "Europe/Berlin"},
		End:         &gcal.EDT{DateTime: "2026-08-31T10:00:00Z", TimeZone: "Europe/Berlin"},
	}}}
	m, _ := newTestModel(f)
	m.weekStart = mon
	m = load(t, m, f)

	m = run(t, m, msgKey("e"))
	if m.screen != screenForm || m.form.editing == nil {
		t.Fatal("e should open the form in edit mode")
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
	mon := time.Date(2026, 8, 31, 0, 0, 0, 0, time.Local)
	f := &fakeAPI{events: []gcal.Event{mkEvent("doomed", "abc", 9, 0)}}
	m, _ := newTestModel(f)
	m.weekStart = mon
	m = load(t, m, f)

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
	mon := time.Date(2026, 8, 31, 0, 0, 0, 0, time.Local)
	f := &fakeAPI{events: []gcal.Event{mkEvent("survivor", "abc", 9, 0)}}
	m, _ := newTestModel(f)
	m.weekStart = mon
	m = load(t, m, f)

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
	m := &Model{loadErr: nil}
	m = upd(t, m, msgError{err: fmt.Errorf("boom")})
	if m.loadErr == nil {
		t.Fatal("expected load error to be stored")
	}
}

func TestMondayOf(t *testing.T) {
	cases := []struct {
		day  time.Time
		want time.Time
	}{
		{time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC), time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC)}, // Monday -> itself
		{time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC), time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC)},  // Wednesday -> Monday
		{time.Date(2026, 9, 6, 0, 0, 0, 0, time.UTC), time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC)},  // Sunday -> Monday
	}
	for _, c := range cases {
		if got := mondayOf(c.day); !got.Equal(c.want) {
			t.Errorf("mondayOf(%v) = %v, want %v", c.day, got, c.want)
		}
	}
}

// TestWeekRendersSmoke ensures View() does not panic for the week and popup
// states (catching layout bugs) and produces non-empty output.
func TestWeekRendersSmoke(t *testing.T) {
	mon := time.Date(2026, 8, 31, 0, 0, 0, 0, time.Local)
	f := &fakeAPI{events: []gcal.Event{
		mkEvent("A", "a", 9, 0),
		mkEvent("B", "b", 14, 0),
		{Summary: "C", Id: "c", Start: &gcal.EDT{Date: "2026-09-01"}, End: &gcal.EDT{Date: "2026-09-01"}},
	}}
	m, _ := newTestModel(f)
	m.weekStart = mon
	m = upd(t, m, tea.WindowSizeMsg{Width: 100, Height: 24})
	m = load(t, m, f)

	base := m.View()
	if base == "" {
		t.Fatal("View() returned empty for week")
	}

	m = upd(t, m, msgKey("enter"))
	if !m.popup {
		t.Fatal("expected popup open")
	}
	withPopup := m.View()
	if withPopup == "" {
		t.Fatal("View() returned empty with popup")
	}
	if !strings.Contains(withPopup, "A") {
		t.Errorf("popup view should include event title")
	}

	// narrow terminal should not panic either
	m2, _ := newTestModel(f)
	m2.weekStart = mon
	m2 = upd(t, m2, tea.WindowSizeMsg{Width: 40, Height: 10})
	m2 = load(t, m2, f)
	if m2.View() == "" {
		t.Fatal("View() empty on narrow terminal")
	}
}
