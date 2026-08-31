package tui

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Mizerael/terminal_calendar/internal/domain"
	"github.com/Mizerael/terminal_calendar/internal/usecase"
)

// fakeAPI is a configurable stub for the usecase.CalendarGateway port, backing
// the TUI.
type fakeAPI struct {
	calendars []domain.Calendar
	// events is the flat fallback list used when mapEvents is empty.
	events    []domain.Event
	mapEvents map[string][]domain.Event
	calErr    error
	listErr   error
	created   []*domain.Event
	updated   []*domain.Event
	deleted   []string
}

func (f *fakeAPI) ListCalendars(ctx context.Context) ([]domain.Calendar, error) {
	if f.calErr != nil {
		return nil, f.calErr
	}
	return f.calendars, nil
}

func (f *fakeAPI) ListEventsRangeIn(ctx context.Context, calID string, start, end time.Time) ([]domain.Event, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	var src []domain.Event
	if f.mapEvents != nil {
		src = f.mapEvents[calID]
	} else {
		src = f.events
	}
	out := make([]domain.Event, len(src))
	for i, e := range src {
		e.CalendarID = calID
		if e.CalendarSummary == "" {
			e.CalendarSummary = calID
		}
		out[i] = e
	}
	return out, nil
}
func (f *fakeAPI) CreateEventIn(ctx context.Context, calID string, e *domain.Event) (*domain.Event, error) {
	f.created = append(f.created, e)
	return e, nil
}
func (f *fakeAPI) UpdateEventIn(ctx context.Context, calID string, e *domain.Event) (*domain.Event, error) {
	f.updated = append(f.updated, e)
	return e, nil
}
func (f *fakeAPI) DeleteEventIn(ctx context.Context, calID, id string) error {
	f.deleted = append(f.deleted, id)
	return nil
}

// makeEvent builds a timed event anchored to 2026-08-31 (a Monday) at the
// given UTC hour/min.
func makeEvent(summary, id string, h, m int) domain.Event {
	return eventAt(summary, id,
		fmt.Sprintf("2026-08-31T%02d:%02d:00Z", h, m),
		fmt.Sprintf("2026-08-31T%02d:%02d:00Z", h, m))
}

// eventAt builds a timed event from RFC3339 start/end strings.
func eventAt(summary, id, startRFC, endRFC string) domain.Event {
	start, _ := time.Parse(time.RFC3339, startRFC)
	end, _ := time.Parse(time.RFC3339, endRFC)
	return domain.Event{ID: id, Summary: summary, Start: start, End: end}
}

// eventOn builds an event on the given date (RFC3339 date, UTC) at hour/min.
func eventOn(summary, id, date string, h, m int) domain.Event {
	s, _ := time.Parse(time.RFC3339, fmt.Sprintf("%sT%02d:%02d:00Z", date, h, m))
	return domain.Event{ID: id, Summary: summary, Start: s, End: s}
}

// alldayOn builds an all-day event on the given date (local midnight).
func alldayOn(summary, id, date string) domain.Event {
	start, _ := time.Parse("2006-01-02", date)
	return domain.Event{ID: id, Summary: summary, Start: start, End: start.AddDate(0, 0, 1), AllDay: true}
}

func newTestModel(f *fakeAPI) (*Model, error) {
	return New(usecase.NewCalendarService(f))
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
	f := &fakeAPI{events: []domain.Event{
		makeEvent("Mon morning", "a", 9, 0),
		makeEvent("Mon afternoon", "b", 14, 0),
		alldayOn("Tue day", "c", "2026-09-01"),
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
	f := &fakeAPI{events: []domain.Event{
		makeEvent("A", "a", 9, 0),              // Mon
		makeEvent("B", "b", 11, 0),             // Mon
		eventOn("C", "c", "2026-09-01", 10, 0), // Tue
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
	f := &fakeAPI{events: []domain.Event{
		makeEvent("A", "a", 9, 0),              // Mon 09:00
		makeEvent("B", "b", 14, 0),             // Mon 14:00
		eventOn("C", "c", "2026-09-01", 10, 0), // Tue 10:00
		eventOn("D", "d", "2026-09-01", 15, 0), // Tue 15:00
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
	if !m.weekStart.Equal(domain.MondayOf(today())) {
		t.Fatalf("after t: weekStart = %v, want this week", m.weekStart)
	}
}

func TestModalPopupFlow(t *testing.T) {
	mon := time.Date(2026, 8, 31, 0, 0, 0, 0, time.Local)
	f := &fakeAPI{events: []domain.Event{makeEvent("Detail target", "x", 9, 0)}}
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
	f := &fakeAPI{events: []domain.Event{makeEvent("Pop", "1", 9, 0)}}
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
	start := ev.StartTime()
	if start.Format("2006-01-02 15:04") != "2026-09-01 10:30" {
		t.Errorf("unexpected start: %v", start)
	}
	end := ev.EndTime()
	if end.Format("2006-01-02 15:04") != "2026-09-01 11:00" {
		t.Errorf("unexpected end: %v", end)
	}
	if tz := ev.Timezone(); tz != "" {
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
	if !ev.AllDay {
		t.Fatal("expected all-day event")
	}
	if ev.StartTime().Format("2006-01-02") != "2026-12-24" || ev.EndTime().Format("2006-01-02") != "2026-12-25" {
		t.Errorf("unexpected dates: %v / %v", ev.StartTime(), ev.EndTime())
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
	berlin, _ := time.LoadLocation("Europe/Berlin")
	start := time.Date(2026, 8, 31, 9, 0, 0, 0, berlin)
	f := &fakeAPI{events: []domain.Event{{
		ID:          "e1",
		Summary:     "Old",
		Location:    "Room 1",
		Description: "notes",
		Start:       start,
		End:         start.Add(time.Hour),
		TimeZone:    "Europe/Berlin",
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
	f := &fakeAPI{events: []domain.Event{makeEvent("doomed", "abc", 9, 0)}}
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
	f := &fakeAPI{events: []domain.Event{makeEvent("survivor", "abc", 9, 0)}}
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
		if got := domain.MondayOf(c.day); !got.Equal(c.want) {
			t.Errorf("MondayOf(%v) = %v, want %v", c.day, got, c.want)
		}
	}
}

// TestGridRender checks the grid lays out hour spans and clash markers, and
// prints the rendered output for visual inspection.
func TestGridRender(t *testing.T) {
	mon := time.Date(2026, 8, 31, 0, 0, 0, 0, time.Local)
	// events: Mon 09-10 "Standup", Mon 09:30-11 "Deep", Tue all-day "Holiday",
	// Wed 14-16 "Workshop"
	f := &fakeAPI{events: []domain.Event{
		makeEvent("Standup", "a", 9, 0),
		eventAt("Deep", "b", "2026-08-31T09:30:00Z", "2026-08-31T11:00:00Z"),
		alldayOn("Holiday", "c", "2026-09-01"),
		eventAt("Workshop", "d", "2026-09-02T14:00:00Z", "2026-09-02T16:00:00Z"),
	}}
	m, _ := newTestModel(f)
	m.weekStart = mon
	m = upd(t, m, tea.WindowSizeMsg{Width: 100, Height: 28})
	m = load(t, m, f)
	m.dayIndex = 0
	m.eventIndex = 0

	grid := m.renderGrid()
	t.Logf("grid output:\n---\n%s\n---", grid)
	if !strings.Contains(grid, "Standup") {
		t.Errorf("grid should contain Standup")
	}
	if !strings.Contains(grid, "Workshop") {
		t.Errorf("grid should contain Workshop")
	}
	if !strings.Contains(grid, "all-day") {
		t.Errorf("grid should contain the all-day banner")
	}
	if !strings.Contains(grid, "2x") {
		t.Errorf("grid should mark the 09:00 clash with 2x")
	}
}

// TestGridRenderShortEvent ensures a short same-hour event (typical of a
// recurring series, e.g. 09:00-09:30) is actually drawn in the grid rather
// than vanishing as a zero-length block.
func TestGridRenderShortEvent(t *testing.T) {
	mon := time.Date(2026, 8, 31, 0, 0, 0, 0, time.Local)
	f := &fakeAPI{events: []domain.Event{
		eventAt("Quick sync", "a", "2026-08-31T09:00:00Z", "2026-08-31T09:30:00Z"),
	}}
	m, _ := newTestModel(f)
	m.weekStart = mon
	m = upd(t, m, tea.WindowSizeMsg{Width: 100, Height: 28})
	m = load(t, m, f)
	m.dayIndex = 0

	grid := m.renderGrid()
	if !strings.Contains(grid, "Quick sync") {
		t.Errorf("short same-hour event should render; grid output:\n%s", grid)
	}
}

// TestWeekRendersSmoke ensures View() does not panic for the week and popup
// states (catching layout bugs) and produces non-empty output.
func TestWeekRendersSmoke(t *testing.T) {
	mon := time.Date(2026, 8, 31, 0, 0, 0, 0, time.Local)
	f := &fakeAPI{events: []domain.Event{
		makeEvent("A", "a", 9, 0),
		makeEvent("B", "b", 14, 0),
		alldayOn("C", "c", "2026-09-01"),
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

func TestHourRange(t *testing.T) {
	// timed event 09:00-11:00
	e := eventAt("x", "1", "2026-08-31T09:00:00Z", "2026-08-31T11:00:00Z")
	s, en, ok := hourRange(&e)
	if !ok || s != 9 || en != 11 {
		t.Errorf("hourRange = %d,%d,%v, want 9,11,true", s, en, ok)
	}

	// all-day event -> not ok
	ad := alldayOn("ad", "2", "2026-08-31")
	if _, _, ok := hourRange(&ad); ok {
		t.Error("all-day event should report ok=false")
	}

	// zero-length event -> at least one hour
	zl := eventAt("zl", "3", "2026-08-31T10:00:00Z", "2026-08-31T10:00:00Z")
	s, en, ok = hourRange(&zl)
	if !ok || s != 10 || en != 11 {
		t.Errorf("zero-length hourRange = %d,%d,%v, want 10,11,true", s, en, ok)
	}

	// event crossing local midnight (23:30 -> 00:30 next day). Construct the
	// wall-clock times in the machine's own zone so the crossing is guaranteed
	// regardless of where the test runs.
	loc := time.Local
	sLoc := time.Date(2026, 9, 1, 23, 30, 0, 0, loc)
	eLoc := time.Date(2026, 9, 2, 0, 30, 0, 0, loc)
	cross := &domain.Event{Start: sLoc, End: eLoc}
	s, en, ok = hourRange(cross)
	if !ok {
		t.Fatal("crossing event should parse")
	}
	if s != 23 {
		t.Errorf("crossing event start hour = %d, want 23", s)
	}
	if en != 24 {
		t.Errorf("crossing event end hour = %d, want 24 (past local midnight)", en)
	}
}

// TestHourRangeSameHour ensures a short event that ends mid-hour (common for
// recurring series, e.g. a 09:00-09:30 standup) still fills its start-hour row
// instead of becoming an invisible zero-length block.
func TestHourRangeSameHour(t *testing.T) {
	e := eventAt("x", "1", "2026-08-31T09:00:00Z", "2026-08-31T09:30:00Z")
	s, en, ok := hourRange(&e)
	if !ok || s != 9 || en != 10 {
		t.Errorf("hourRange = %d,%d,%v, want 9,10,true", s, en, ok)
	}

	// 09:00-10:30 spans two full rows.
	e2 := eventAt("y", "2", "2026-08-31T09:00:00Z", "2026-08-31T10:30:00Z")
	s2, en2, ok2 := hourRange(&e2)
	if !ok2 || s2 != 9 || en2 != 11 {
		t.Errorf("hourRange = %d,%d,%v, want 9,11,true", s2, en2, ok2)
	}
}

// TestEmptyCellCursorRequiresSelection ensures the free-hour cell is only
// highlighted when an actual event is selected, so an empty day/free hour is
// never falsely painted with the cursor color.
func TestEmptyCellCursorRequiresSelection(t *testing.T) {
	m := &Model{dayIndex: 1, eventIndex: -1}
	m.setCursorHour(12)

	// No event selected -> never highlighted.
	if m.emptyCellCursor(1, 12) {
		t.Error("empty cell cursor should be off when no event is selected")
	}

	// A focused event does enable highlighting (the grid never draws it on an
	// empty cell because the focused event occupies that hour, but guard the
	// prerequisite).
	m.eventIndex = 0
	if !m.emptyCellCursor(1, 12) {
		t.Error("empty cell cursor should be on when an event is focused")
	}
}

// TestFocusedEventMarker ensures the selected event gets a distinct ▸ marker
// in the grid so the selection is obvious in a merged view.
func TestFocusedEventMarker(t *testing.T) {
	mon := time.Date(2026, 8, 31, 0, 0, 0, 0, time.Local)
	f := &fakeAPI{events: []domain.Event{
		makeEvent("Standup", "a", 9, 0),
		makeEvent("Other", "b", 14, 0),
	}}
	m, _ := newTestModel(f)
	m.weekStart = mon
	m = upd(t, m, tea.WindowSizeMsg{Width: 100, Height: 40})
	m = load(t, m, f)
	m.dayIndex = 0
	m.eventIndex = 0

	grid := m.renderGrid()
	if !strings.Contains(grid, "▸") {
		t.Errorf("focused event should be marked with ▸ in the grid")
	}
}

func TestColWidth(t *testing.T) {
	m := &Model{}
	m.width = 100
	if got := m.colW(); got != 13 {
		t.Errorf("colW(100) = %d, want 13", got)
	}
	m.width = 40
	if got := m.colW(); got != 8 {
		t.Errorf("colW(40) = %d, want 8 (clamped to min)", got)
	}
	m.width = 200
	if got := m.colW(); got != 27 {
		t.Errorf("colW(200) = %d, want 27 (stretches with width)", got)
	}
}

func TestWindowResizeFillsTerminal(t *testing.T) {
	m, _ := newTestModel(&fakeAPI{events: []domain.Event{makeEvent("A", "a", 9, 0)}})

	// small terminal
	m = upd(t, m, tea.WindowSizeMsg{Width: 40, Height: 12})
	if got := m.effectiveRows(); got > 24 || got < 1 {
		t.Errorf("effectiveRows on small terminal = %d, want within [1,24]", got)
	}
	if got := m.colW(); got < colWMin {
		t.Errorf("colW on small terminal = %d, want >= %d", got, colWMin)
	}

	// large terminal: grid should stretch (more rows, wider columns)
	m = upd(t, m, tea.WindowSizeMsg{Width: 200, Height: 60})
	// gridRows (capacity) is capped at the 24 hours of a day
	if got := m.gridRows(); got != 24 {
		t.Errorf("gridRows on tall terminal = %d, want 24", got)
	}
	// the visible window follows scrollHour + remaining hours
	// (default scrollHour=6 => hours 6..23, i.e. 18 rows)
	if got, want := m.effectiveRows(), 24-m.scrollHour; got != want {
		t.Errorf("effectiveRows on tall terminal = %d, want %d", got, want)
	}
	// columns stretch with width
	if got := m.colW(); got != 27 {
		t.Errorf("colW on wide terminal = %d, want 27", got)
	}
}

func TestResizeKeepsFocusVisible(t *testing.T) {
	m, _ := newTestModel(&fakeAPI{events: []domain.Event{makeEvent("late", "a", 22, 0)}})
	m = upd(t, m, tea.WindowSizeMsg{Width: 100, Height: 60})
	m = load(t, m, &fakeAPI{events: []domain.Event{makeEvent("late", "a", 22, 0)}})
	m.dayIndex, m.eventIndex = 0, 0

	// On a tall window all 24 hours fit, focus hour 22 is visible.
	m = upd(t, m, tea.WindowSizeMsg{Width: 100, Height: 60})
	if h := m.focusedHour(); h != 22 {
		t.Fatalf("focusedHour = %d, want 22", h)
	}
	// On a short window scrollHour should shift so hour 22 stays visible.
	m = upd(t, m, tea.WindowSizeMsg{Width: 100, Height: 12})
	rows := m.effectiveRows()
	if !(m.scrollHour <= 22 && 22 < m.scrollHour+rows) {
		t.Errorf("focus hour 22 not within window [%d,%d)", m.scrollHour, m.scrollHour+rows)
	}
}

func TestScrollKeysAdjustWindow(t *testing.T) {
	m := &Model{scrollHour: 6, cursorHour: 6}
	m = upd(t, m, tea.WindowSizeMsg{Width: 100, Height: 28})
	// rows = gridRows = (28-4)=24 -> capped 20
	m = upd(t, m, msgKey("ctrl+d"))
	if m.scrollHour != 12 {
		t.Errorf("after ctrl+d scrollHour = %d, want 12", m.scrollHour)
	}
	m = upd(t, m, msgKey("ctrl+d"))
	m = upd(t, m, msgKey("ctrl+d"))
	m = upd(t, m, msgKey("ctrl+d"))
	// expect to clamp at 24 - effectiveRows
	rows := m.effectiveRows()
	if m.scrollHour > 24-rows {
		t.Errorf("scrollHour %d exceeds max %d", m.scrollHour, 24-rows)
	}
	// scroll up all the way
	for i := 0; i < 10; i++ {
		m = upd(t, m, msgKey("ctrl+u"))
	}
	if m.scrollHour != 0 {
		t.Errorf("after repeated ctrl+u scrollHour = %d, want 0", m.scrollHour)
	}
}

func TestEnsureVisible(t *testing.T) {
	m := &Model{scrollHour: 6, cursorHour: 6}
	m.weekEvents[0] = []domain.Event{makeEvent("late", "x", 22, 0)}
	m.dayIndex = 0
	m.eventIndex = 0
	m.ensureVisible(10) // window [6,16)
	if m.scrollHour != 13 {
		t.Errorf("ensureVisible scrolled to %d, want 13 (hour 22-10+1)", m.scrollHour)
	}
}

func TestGridNavigationByEvent(t *testing.T) {
	mon := time.Date(2026, 8, 31, 0, 0, 0, 0, time.Local)
	f := &fakeAPI{events: []domain.Event{
		makeEvent("Early", "a", 8, 0), // Mon 08
		makeEvent("Late", "b", 20, 0), // Mon 20
		eventAt("Night", "c", "2026-09-02T23:00:00+04:00", "2026-09-03T00:30:00+04:00"), // Wed 23 local
	}}
	m, _ := newTestModel(f)
	m.weekStart = mon
	m = upd(t, m, tea.WindowSizeMsg{Width: 100, Height: 28})
	m = load(t, m, f)

	// focus first event: Mon Early at 08:00
	if m.dayIndex != 0 || m.eventIndex != 0 {
		t.Fatalf("initial %d,%d want 0,0", m.dayIndex, m.eventIndex)
	}
	if got := m.focusedHour(); got != 8 {
		t.Errorf("focusedHour = %d, want 8", got)
	}

	// j -> Mon Late (20:00)
	m = upd(t, m, msgKey("j"))
	if m.dayIndex != 0 || m.eventIndex != 1 || m.focusedHour() != 20 {
		t.Fatalf("after j: %d,%d hour=%d want 0,1 hour=20", m.dayIndex, m.eventIndex, m.focusedHour())
	}

	// j -> Wed Night (23:00) (Mon has no more events -> next day with events)
	m = upd(t, m, msgKey("j"))
	if m.dayIndex != 2 || m.eventIndex != 0 || m.focusedHour() != 23 {
		t.Fatalf("after jj: %d,%d hour=%d want 2,0 hour=23", m.dayIndex, m.eventIndex, m.focusedHour())
	}
}

// TestResizeRenderDoesNotPanicOrOverflow renders the grid at several terminal
// sizes and ensures it never panics nor produces content wider than the
// available width.
func TestResizeRenderDoesNotPanicOrOverflow(t *testing.T) {
	mon := time.Date(2026, 8, 31, 0, 0, 0, 0, time.Local)
	f := &fakeAPI{events: []domain.Event{
		makeEvent("Standup morning", "a", 9, 0),
		makeEvent("Afternoon review", "b", 14, 0),
		alldayOn("Holiday", "c", "2026-09-01"),
	}}
	for _, sz := range [][2]int{{80, 20}, {100, 24}, {160, 40}, {220, 50}} {
		m, _ := newTestModel(f)
		m.weekStart = mon
		m = upd(t, m, tea.WindowSizeMsg{Width: sz[0], Height: sz[1]})
		m = load(t, m, f)
		v := m.View()
		if v == "" {
			t.Fatalf("size %dx%d: empty view", sz[0], sz[1])
		}
		for _, line := range strings.Split(v, "\n") {
			if w := lipgloss.Width(line); w > sz[0] {
				t.Errorf("size %dx%d: line width %d exceeds terminal width", sz[0], sz[1], w)
			}
		}
	}
}
