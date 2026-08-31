package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Mizerael/terminal_calendar/internal/domain"
	"github.com/Mizerael/terminal_calendar/internal/usecase"
)

type screen int

const (
	screenList screen = iota
	screenForm
	screenConfirm
)

// msgEventsLoaded carries the fetched events for a week (weekStart = Monday).
type msgEventsLoaded struct {
	events    []domain.Event
	weekStart time.Time
}

// msgCalendarsLoaded signals that the user's calendar list has been fetched
// and the model should now load events for the enabled calendars.
type msgCalendarsLoaded struct {
	weekStart time.Time
}

type msgError struct{ err error }
type msgSaved struct{}
type msgDeleted struct{}

// Model is the root bubbletea model. It acts as the controller/presenter of
// the Clean Architecture stack: it owns bubbletea framework state and painting,
// and delegates calendar operations to a usecase.CalendarService.
type Model struct {
	svc *usecase.CalendarService
	sel usecase.Selection
	ctx context.Context

	// weekStart is the Monday (00:00 local) of the displayed week. The week
	// spans [weekStart, weekStart + 7 days).
	weekStart time.Time
	// weekEvents[dayIndex] holds that day's events, sorted by start time.
	weekEvents [7][]domain.Event
	// dayIndex is the focused day within the week (0 = Monday).
	dayIndex int
	// eventIndex is the focused event within the focused day; -1 means none.
	eventIndex int
	// cursorHour is the hour row highlighted in the grid when the focused day
	// has no event (or the selected event's start maps to it otherwise).
	cursorHour int
	// scrollHour is the first hour shown at the top of the grid's scrolling
	// window (0..23). The visible window is [scrollHour, scrollHour+rows).
	scrollHour int

	loading  bool
	loaded   bool
	loadErr  error
	lastSync time.Time

	// popup is a modal detail overlay for a specific event.
	popup      bool
	popupEvent *domain.Event

	screen     screen
	form       *form
	confirm    *domain.Event // event awaiting delete confirmation
	confirmErr error
	help       bool
	width      int
	height     int
	quitting   bool

	// calendars holds the user's calendars; sel selects which are loaded and
	// shown merged and which is the create-target (see usecase.Selection).
	calendars []domain.Calendar
	statePath string
	// picker is the calendar visibility/target overlay.
	picker      bool
	pickerIndex int
	pickerErr   error
}

// New builds the model with a disposed calendar selection and starts loading.
func New(svc *usecase.CalendarService) (*Model, error) {
	m := &Model{
		svc:        svc,
		ctx:        context.Background(),
		weekStart:  domain.MondayOf(today()),
		form:       newForm(),
		cursorHour: time.Now().Hour(),
		scrollHour: defaultScrollHour,
		sel:        usecase.Selection{Enabled: map[string]bool{}},
	}
	m.loadState()
	m.reload()
	return m, nil
}

// today returns midnight (local) today.
func today() time.Time {
	now := time.Now()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
}

func (m *Model) Init() tea.Cmd {
	return m.loadEvents
}

func (m *Model) reload() tea.Cmd {
	return m.loadEvents
}

func (m *Model) loadEvents() tea.Msg {
	calendars, err := m.svc.LoadCalendars(m.ctx)
	if err != nil {
		return msgError{err: err}
	}
	m.calendars = calendars
	return msgCalendarsLoaded{weekStart: m.weekStart}
}

// loadWeekEvents fetches events for every enabled calendar via the use cases
// and returns them as a single flat week-long list.
func (m *Model) loadWeekEvents() tea.Msg {
	events, err := m.svc.LoadWeek(m.ctx, m.weekStart, m.sel.Enabled)
	if err != nil {
		return msgError{err: err}
	}
	return msgEventsLoaded{events: events, weekStart: m.weekStart}
}

// focusedDay returns the time.Time midnight for the focused day.
func (m *Model) focusedDay() time.Time {
	return m.weekStart.AddDate(0, 0, m.dayIndex)
}

// focusedEvent returns a pointer to the focused event, or nil.
func (m *Model) focusedEvent() *domain.Event {
	if m.dayIndex < 0 || m.dayIndex > 6 || m.eventIndex < 0 {
		return nil
	}
	day := m.weekEvents[m.dayIndex]
	if m.eventIndex >= len(day) {
		return nil
	}
	ev := day[m.eventIndex]
	return &ev
}

// totalEvents counts events across all days of the week.
func (m *Model) totalEvents() int {
	n := 0
	for _, d := range m.weekEvents {
		n += len(d)
	}
	return n
}

// ---- updates ----

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		if m.loaded {
			m.ensureVisible(m.effectiveRows())
		}

	case tea.KeyMsg:
		switch {
		case m.screen == screenForm:
			return m.updateForm(msg)
		case m.screen == screenConfirm:
			return m.updateConfirm(msg)
		case m.picker:
			return m.updatePicker(msg)
		case m.popup:
			return m.updatePopup(msg)
		default:
			return m.updateList(msg)
		}

	case msgCalendarsLoaded:
		m.reconcileSelection()
		m.saveState()
		return m, m.loadWeekEvents

	case msgEventsLoaded:
		m.loading = false
		if msg.weekStart.Equal(m.weekStart) {
			m.groupEvents(msg.events)
			m.loaded = true
			m.loadErr = nil
			m.lastSync = time.Now()
			m.clampCursor()
		}

	case msgError:
		m.loading = false
		switch m.screen {
		case screenList:
			m.loadErr = msg.err
		case screenForm:
			m.form.err = "could not save: " + msg.err.Error()
		case screenConfirm:
			m.confirmErr = msg.err
		}

	case msgSaved:
		m.screen = screenList
		m.form.reset(time.Time{})
		return m, m.reload()

	case msgDeleted:
		m.confirm = nil
		m.screen = screenList
		return m, m.reload()
	}
	return m, nil
}

// groupEvents buckets a flat week-long list into per-day slices.
func (m *Model) groupEvents(events []domain.Event) {
	var out [7][]domain.Event
	for _, e := range events {
		idx := domain.DayIndex(&e, m.weekStart)
		if idx >= 0 {
			out[idx] = append(out[idx], e)
		}
	}
	m.weekEvents = out
}

// clampCursor keeps the focused day within range and the event index within
// the focused day (or -1 if the day has no events).
func (m *Model) clampCursor() {
	if m.dayIndex < 0 {
		m.dayIndex = 0
	}
	if m.dayIndex > 6 {
		m.dayIndex = 6
	}
	if m.weekEvents[m.dayIndex] == nil {
		m.weekEvents[m.dayIndex] = []domain.Event{}
	}
	if m.eventIndex >= len(m.weekEvents[m.dayIndex]) {
		m.eventIndex = 0
	}
	if m.eventIndex < 0 || len(m.weekEvents[m.dayIndex]) == 0 {
		m.eventIndex = -1
	}
}

// forEachEvent yields each event in week order as (dayIndex, eventIndex, *Event).
// on is invoked only for non-empty days.
func (m *Model) forEachEvent(on func(dayIdx, evtIdx int, e *domain.Event)) {
	for d := 0; d < 7; d++ {
		for i, e := range m.weekEvents[d] {
			on(d, i, &e)
		}
	}
}

// firstEventIndex returns the (day, event) of the first event in the week, or
// (-1,-1) if there are none.
func (m *Model) firstEventIndex() (int, int) {
	for d := 0; d < 7; d++ {
		if len(m.weekEvents[d]) > 0 {
			return d, 0
		}
	}
	return -1, -1
}

// focusedHour returns the hour row (0..23) the grid should highlight for the
// focused day: the start hour of the focused event when one is selected,
// otherwise the last-known cursor hour.
func (m *Model) focusedHour() int {
	if ev := m.focusedEvent(); ev != nil {
		if t := ev.StartTime(); !t.IsZero() {
			return t.Hour()
		}
	}
	return m.cursorHour
}

// scrollDefaults positions the hour window so the highlighted hour is visible,
// keeping scrollHour within 0..23 for a window of the given number of rows.
// It does not change scrollHour unless the highlight is outside the window.
func (m *Model) ensureVisible(rows int) {
	if rows < 1 {
		return
	}
	h := m.focusedHour()
	if m.scrollHour <= h && h < m.scrollHour+rows {
		return
	}
	if h < m.scrollHour {
		m.scrollHour = h
	} else {
		m.scrollHour = h - rows + 1
	}
	clampHour(&m.scrollHour)
}

// setCursorHour updates the free (no-event) highlight hour and keeps it valid.
func (m *Model) setCursorHour(h int) {
	m.cursorHour = h
	clampHour(&m.cursorHour)
}

func clampHour(p *int) {
	if *p < 0 {
		*p = 0
	}
	if *p > 23 {
		*p = 23
	}
}

// cursorDayBefore finds the event position immediately before the current
// focus across the week (for j/up navigation). Returns false if there is none.
func (m *Model) moveUp() bool {
	if m.eventIndex > 0 {
		m.eventIndex--
		return true
	}
	// move to last event of previous day that has events
	for d := m.dayIndex - 1; d >= 0; d-- {
		if len(m.weekEvents[d]) > 0 {
			m.dayIndex = d
			m.eventIndex = len(m.weekEvents[d]) - 1
			return true
		}
	}
	return false
}

// moveDown moves to the event after the current focus. Returns false at the end.
func (m *Model) moveDown() bool {
	day := m.weekEvents[m.dayIndex]
	if m.eventIndex >= 0 && m.eventIndex < len(day)-1 {
		m.eventIndex++
		return true
	}
	// move to first event of next day that has events
	for d := m.dayIndex + 1; d < 7; d++ {
		if len(m.weekEvents[d]) > 0 {
			m.dayIndex = d
			m.eventIndex = 0
			return true
		}
	}
	return false
}

// moveToDay shifts the focused day by delta and lands as close as possible to
// the current focused time-of-day.
func (m *Model) moveToDay(delta int) {
	target := m.dayIndex + delta
	if target < 0 {
		target = 0
	}
	if target > 6 {
		target = 6
	}
	m.dayIndex = target

	want := time.Time{}
	if ev := m.focusedEvent(); ev != nil {
		want = ev.StartTime()
		if want.IsZero() {
			m.eventIndex = -1
			return
		}
	} else {
		// no current event: default to now's time-of-day
		now := time.Now()
		want = time.Date(2000, 1, 1, now.Hour(), now.Minute(), 0, 0, time.UTC)
	}

	day := m.weekEvents[target]
	if len(day) == 0 {
		m.eventIndex = -1
		return
	}
	// pick the event start closest to `want` (measured as absolute minutes)
	best := 0
	bestGap := int64(1 << 60)
	for i := range day {
		t := day[i].StartTime()
		gap := abs64(t.Unix() - want.Unix())
		if gap < bestGap {
			bestGap = gap
			best = i
		}
	}
	m.eventIndex = best
}

func abs64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

func (m *Model) resetWeek() {
	m.weekEvents = [7][]domain.Event{}
	m.loaded = false
	m.loadErr = nil
	m.eventIndex = -1
	m.loading = true
}

func (m *Model) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		m.quitting = true
		return m, tea.Quit

	case "up", "k":
		m.moveUp()
		m.ensureVisible(m.effectiveRows())
	case "down", "j":
		m.moveDown()
		m.ensureVisible(m.effectiveRows())
	case "g", "home":
		if d, i := m.firstEventIndex(); d >= 0 {
			m.dayIndex, m.eventIndex = d, i
			m.ensureVisible(m.effectiveRows())
		}
	case "G", "end":
		for d := 6; d >= 0; d-- {
			if len(m.weekEvents[d]) > 0 {
				m.dayIndex = d
				m.eventIndex = len(m.weekEvents[d]) - 1
				m.ensureVisible(m.effectiveRows())
				break
			}
		}

	case "left", "h":
		m.moveToDay(-1)
		m.ensureVisible(m.effectiveRows())
	case "right", "l":
		m.moveToDay(1)
		m.ensureVisible(m.effectiveRows())
	case "ctrl+u", "pgup":
		m.scrollHour -= 6
		clampHour(&m.scrollHour)
		m.setCursorHour(m.scrollHour)
	case "ctrl+d", "pgdown":
		m.scrollHour += 6
		clampHour(&m.scrollHour)
		rows := m.effectiveRows()
		if m.scrollHour > 24-rows {
			m.scrollHour = 24 - rows
		}
		clampHour(&m.scrollHour)
		m.setCursorHour(m.scrollHour)
	case "[":
		m.weekStart = m.weekStart.AddDate(0, 0, -7)
		m.resetWeek()
		return m, m.reload()
	case "]":
		m.weekStart = m.weekStart.AddDate(0, 0, 7)
		m.resetWeek()
		return m, m.reload()
	case "t":
		m.weekStart = domain.MondayOf(today())
		m.resetWeek()
		return m, m.reload()

	case "n":
		m.form.reset(m.focusedDay())
		m.screen = screenForm
	case "e":
		if ev := m.focusedEvent(); ev != nil {
			m.form.setEvent(ev)
			m.screen = screenForm
		}
	case "enter":
		if ev := m.focusedEvent(); ev != nil {
			e := *ev
			m.popupEvent = &e
			m.popup = true
		}
	case "d":
		if ev := m.focusedEvent(); ev != nil {
			e := *ev
			m.confirm = &e
			m.confirmErr = nil
			m.screen = screenConfirm
		}
	case "r":
		return m, m.reload()
	case "c":
		m.picker = true
		m.pickerIndex = 0
		m.pickerErr = nil
	case "?":
		m.help = !m.help
	}
	return m, nil
}

// updatePopup handles the modal detail overlay.
func (m *Model) updatePopup(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "enter", "q", " ":
		m.popup = false
		m.popupEvent = nil
	case "e":
		if m.popupEvent != nil {
			m.form.setEvent(m.popupEvent)
			m.popup = false
			m.popupEvent = nil
			m.screen = screenForm
		}
	case "d":
		if m.popupEvent != nil {
			m.confirm = m.popupEvent
			m.popup = false
			m.popupEvent = nil
			m.confirmErr = nil
			m.screen = screenConfirm
		}
	}
	return m, nil
}

func (m *Model) updateConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "y", "enter":
		ev := m.confirm
		m.confirm = nil
		m.screen = screenList
		return m, func() tea.Msg {
			if err := m.svc.Delete(m.ctx, m.sel.Target, ev); err != nil {
				return msgError{err: err}
			}
			return msgDeleted{}
		}
	case "n", "esc", "q":
		m.confirm = nil
		m.confirmErr = nil
		m.screen = screenList
	}
	return m, nil
}

func (m *Model) updateForm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "ctrl+c":
		m.screen = screenList
		m.form.reset(time.Now())
		return m, nil

	case "enter":
		m.form.err = ""
		ev, err := m.form.buildEvent()
		if err == errValidation {
			return m, nil
		}
		if err != nil {
			return m, nil
		}
		m.form.err = "saving…"
		if m.form.editing != nil {
			return m, m.saveEvent(ev, false)
		}
		return m, m.saveEvent(ev, true)

	case "tab", "down":
		m.form.next()
	case "shift+tab", "up":
		m.form.prev()
	default:
		var cmd tea.Cmd
		m.form.inputs[m.form.focus], cmd = m.form.inputs[m.form.focus].Update(msg)
		if cmd != nil {
			return m, cmd
		}
	}
	return m, nil
}

func (m *Model) saveEvent(ev *domain.Event, create bool) tea.Cmd {
	ctx := m.ctx
	return func() tea.Msg {
		var err error
		if create {
			_, err = m.svc.Create(ctx, m.sel.Target, ev)
		} else {
			_, err = m.svc.Update(ctx, m.sel.Target, ev)
		}
		if err != nil {
			return msgError{err: err}
		}
		return msgSaved{}
	}
}

// ---- view ----

func (m *Model) View() string {
	if m.quitting {
		return ""
	}
	switch m.screen {
	case screenForm:
		return m.form.render(m.width, m.height)
	case screenConfirm:
		return m.renderConfirm()
	default:
		v := m.renderList()
		switch {
		case m.picker:
			v = m.renderPicker()
		case m.popup && m.popupEvent != nil:
			v = overlay(v, m.renderPopup(m.popupEvent), m.width, m.height)
		}
		if m.help {
			v = lipgloss.JoinVertical(lipgloss.Center, v, m.renderHelp())
		}
		return v
	}
}

func (m *Model) renderConfirm() string {
	ev := m.confirm
	title := "No event selected"
	if ev != nil {
		title = ev.Summary
	}
	b := titleStyle.SetString("Delete event?").Render() + "\n\n"
	b += confirmText.Render(fmt.Sprintf("Permanently delete “%s” from your calendar?", title)) + "\n\n"
	if m.confirmErr != nil {
		b += formError.Render("✗ could not delete: "+m.confirmErr.Error()) + "\n\n"
	}
	b += formHint.Render("y: delete · n: cancel")
	boxed := formBox.Render(b)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
		confirmBox.Render(boxed))
}

func (m *Model) renderHelp() string {
	lines := []struct{ key, desc string }{
		{"j/k, ↑/↓", "move between events"},
		{"h/l, ←/→", "move between days"},
		{"ctrl+u/ctrl+d", "scroll hour rows"},
		{"[/]", "previous / next week"},
		{"t", "jump to this week"},
		{"enter", "open event detail"},
		{"e", "edit event"},
		{"n", "new event"},
		{"d", "delete event"},
		{"r", "refresh"},
		{"g/G", "first / last event"},
		{"c", "calendars (show/hide # target)"},
		{"?, esc", "close help"},
		{"q, ctrl+c", "quit"},
	}
	var b strings.Builder
	for _, l := range lines {
		b.WriteString(helpKey.Render(fmt.Sprintf("%-14s", l.key)))
		b.WriteString(helpDesc.Render(l.desc))
		b.WriteString("\n")
	}
	return helpBox.Render(b.String())
}

// overlay centers `popup` on top of `base` and blanks/dims the surroundings.
func overlay(base, popup string, w, h int) string {
	// Render the popup box spanning only the lines it occupies.
	placed := lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, popup)
	lines := strings.Split(placed, "\n")
	orig := strings.Split(base, "\n")
	max := len(orig)
	if len(lines) > max {
		max = len(lines)
	}
	out := make([]string, 0, max)
	for i := 0; i < max; i++ {
		var o string
		if i < len(orig) {
			o = orig[i]
		}
		var p string
		if i < len(lines) {
			p = lines[i]
		}
		if strings.TrimSpace(p) == "" {
			out = append(out, o)
		} else {
			out = append(out, p)
		}
	}
	return strings.Join(out, "\n")
}
