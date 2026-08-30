package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"gitnub.com/Mizerael/terminal_calendar/internal/gcal"
)

type screen int

const (
	screenList screen = iota
	screenForm
	screenConfirm
)

// msgEventsLoaded carries the fetched events for a day.
type msgEventsLoaded struct {
	events []gcal.Event
	day    time.Time
}

type msgError struct{ err error }
type msgSaved struct{}
type msgDeleted struct{}

// eventAPI is the subset of the calendar API the TUI needs.
type eventAPI interface {
	ListEvents(ctx context.Context, day time.Time) ([]gcal.Event, error)
	CreateEvent(ctx context.Context, e *gcal.Event) (*gcal.Event, error)
	UpdateEvent(ctx context.Context, e *gcal.Event) (*gcal.Event, error)
	DeleteEvent(ctx context.Context, id string) error
}

// Model is the root bubbletea model.
type Model struct {
	client eventAPI
	ctx    context.Context

	day      time.Time
	events   []gcal.Event
	cursor   int
	loading  bool
	loaded   bool
	loadErr  error
	lastSync time.Time

	screen   screen
	form     *form
	confirm  *gcal.Event // event awaiting delete confirmation
	help     bool
	width    int
	height   int
	quitting bool
}

func New(client eventAPI) (*Model, error) {
	m := &Model{
		client: client,
		ctx:    context.Background(),
		day:    today(),
		form:   newForm(),
	}
	m.reload()
	return m, nil
}

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
	day := m.day
	events, err := m.client.ListEvents(m.ctx, day)
	if err != nil {
		return msgError{err: err}
	}
	return msgEventsLoaded{events: events, day: day}
}

// ---- updates ----

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height

	case tea.KeyMsg:
		if m.screen == screenForm {
			return m.updateForm(msg)
		}
		if m.screen == screenConfirm {
			return m.updateConfirm(msg)
		}
		return m.updateList(msg)

	case msgEventsLoaded:
		m.loading = false
		if msg.day.Equal(m.day) {
			m.events = msg.events
			m.loaded = true
			m.loadErr = nil
			m.lastSync = time.Now()
			if m.cursor >= len(m.events) {
				m.cursor = 0
			}
		}

	case msgError:
		m.loading = false
		if m.screen == screenList {
			m.loadErr = msg.err
		}

	case msgSaved:
		m.screen = screenList
		m.form.reset()
		return m, m.reload()

	case msgDeleted:
		m.confirm = nil
		m.screen = screenList
		return m, m.reload()
	}
	return m, nil
}

func (m *Model) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		m.quitting = true
		return m, tea.Quit

	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.events)-1 {
			m.cursor++
		}
	case "g", "home":
		m.cursor = 0
	case "G", "end":
		m.cursor = len(m.events) - 1

	case "left", "h":
		m.day = m.day.AddDate(0, 0, -1)
		m.resetDay()
		return m, m.reload()
	case "right", "l":
		m.day = m.day.AddDate(0, 0, 1)
		m.resetDay()
		return m, m.reload()
	case "t":
		m.day = today()
		m.resetDay()
		return m, m.reload()

	case "n":
		m.form.reset()
		m.screen = screenForm
	case "e", "enter":
		if len(m.events) > 0 {
			ev := m.events[m.cursor]
			m.form.setEvent(&ev)
			m.screen = screenForm
		}
	case "d":
		if len(m.events) > 0 {
			ev := m.events[m.cursor]
			m.confirm = &ev
			m.screen = screenConfirm
		}
	case "r":
		return m, m.reload()
	case "?":
		m.help = !m.help
	}
	return m, nil
}

func (m *Model) resetDay() {
	m.events = nil
	m.loaded = false
	m.cursor = 0
	m.loading = true
}

func (m *Model) updateConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "y", "enter":
		m.ctx = context.Background()
		id := m.confirm.Id
		m.confirm = nil
		m.screen = screenList
		return m, func() tea.Msg {
			if err := m.client.DeleteEvent(m.ctx, id); err != nil {
				return msgError{err: err}
			}
			return msgDeleted{}
		}
	case "n", "esc", "q":
		m.confirm = nil
		m.screen = screenList
	}
	return m, nil
}

func (m *Model) updateForm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "ctrl+c":
		m.screen = screenList
		m.form.reset()
		return m, nil

	case "enter":
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

	case "tab", "down", "j":
		m.form.next()
	case "shift+tab", "up", "k":
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

func (m *Model) saveEvent(ev *gcal.Event, create bool) tea.Cmd {
	ctx := m.ctx
	return func() tea.Msg {
		var err error
		if create {
			_, err = m.client.CreateEvent(m.ctx, ev)
		} else {
			_, err = m.client.UpdateEvent(ctx, ev)
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
	b += formHint.Render("y: delete · n: cancel")
	boxed := formBox.Render(b)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
		confirmBox.Render(boxed))
}

func (m *Model) renderHelp() string {
	lines := []struct{ key, desc string }{
		{"j/k, ↑/↓", "move between events"},
		{"h/l, ←/→", "previous / next day"},
		{"t", "jump to today"},
		{"enter, e", "edit event"},
		{"n", "new event"},
		{"d", "delete event"},
		{"r", "refresh"},
		{"g/G", "top / bottom"},
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

// eventTimeLine returns a human label for event start.
func eventTimeLine(e *gcal.Event) string {
	if e.AllDay() {
		return "all day"
	}
	t, err := e.StartTime()
	if err != nil {
		return "??:??"
	}
	return t.Format("15:04")
}
