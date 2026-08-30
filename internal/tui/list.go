package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"gitnub.com/Mizerael/terminal_calendar/internal/gcal"
)

// renderList builds the main day view: a header with the date, the list of
// events and a detail pane for the selected event.
func (m *Model) renderList() string {
	header := m.renderHeader()
	var body string

	switch {
	case m.loading && !m.loaded:
		body = spinnerBox.Render(m.renderSpinner())
	case m.loadErr != nil:
		body = errorBox.Render(m.renderError())
	case len(m.events) == 0:
		body = emptyBox.Render(m.renderEmpty())
	default:
		body = lipgloss.JoinHorizontal(lipgloss.Top, m.renderEvents(), m.renderDetail())
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		statusLine.Render(m.renderStatus()),
		body,
	)
}

func (m *Model) renderHeader() string {
	weekday := m.day.Format("Mon")
	date := m.day.Format("02 Jan 2006")
	sel := m.day.Format("Monday 02.01.2006")

	var chevron string
	switch {
	case m.day.Equal(today()):
		chevron = ""
	case m.day.After(today()):
		chevron = " ▲"
	default:
		chevron = " ▼"
	}

	title := fmt.Sprintf("%s %s%s", weekday, date, chevron)
	right := ""
	if m.lastSync.IsZero() {
		right = "…"
	} else {
		right = "synced " + m.lastSync.Format("15:04:05")
	}
	_ = sel

	titleStr := titleStyle.SetString(title).Render()
	rightStr := subtle.Render(right)
	row := lipgloss.JoinHorizontal(lipgloss.Center, titleStr, lipgloss.NewStyle().Width(m.width-80).Render(""))
	return lipgloss.JoinVertical(lipgloss.Left, row, lipgloss.PlaceHorizontal(m.width, lipgloss.Left, rightStr))
}

func (m *Model) renderStatus() string {
	if m.loading {
		return "loading…"
	}
	if m.loadErr != nil {
		return "error: " + m.loadErr.Error()
	}
	n := len(m.events)
	lab := "events"
	if n == 1 {
		lab = "event"
	}
	return fmt.Sprintf("%d %s on %s   %s", n, lab, m.day.Format("Mon 02 Jan"), keysHint())
}

func keysHint() string {
	return "[j/k move] [n new] [e edit] [d delete] [←/→ day] [t today] [? help] [q quit]"
}

// renderEvents renders the scrollable list of event rows.
func (m *Model) renderEvents() string {
	rows := make([]string, 0, len(m.events))
	for i, e := range m.events {
		rows = append(rows, m.renderEventRow(i, &e))
	}
	list := lipgloss.JoinVertical(lipgloss.Left, rows...)
	w := min(m.width-2, 56)
	h := m.height - 7
	if w < 8 {
		w = 8
	}
	if h < 3 {
		h = 3
	}
	return listBox.Width(w).Height(h).Render(list)
}

func (m *Model) renderEventRow(i int, e *gcal.Event) string {
	sel := i == m.cursor

	timeTag := eventTimeLine(e)
	tag := timebox.Render(timeTag)

	title := e.Summary
	if title == "" {
		title = "(untitled)"
	}
	titleStr := rowTitle.Render(title)

	line := lipgloss.JoinVertical(lipgloss.Left, titleStr, tag)

	if sel {
		pre := listCursor.Render("▸")
		return lipgloss.JoinHorizontal(lipgloss.Left, pre, selectedRow.Inline(true).Render(line))
	}

	return lipgloss.JoinHorizontal(lipgloss.Left, listCursor.Render(" "), line)
}

// renderDetail shows the selected event's details.
func (m *Model) renderDetail() string {
	if len(m.events) == 0 {
		return detailBox.Render(subtle.Render("no events selected"))
	}
	e := m.events[m.cursor]

	var b strings.Builder
	b.WriteString(detailTitle.Render(e.Summary) + "\n\n")

	start, _ := e.StartTime()
	end, _ := e.EndTime()
	if !start.IsZero() {
		var when string
		if e.AllDay() {
			when = start.Format("Mon 02 Jan") + " — all day"
			if !end.IsZero() && !start.Equal(end) {
				when += " through " + end.Format("Mon 02 Jan")
			}
		} else {
			when = start.Format("Mon 02 Jan 15:04") + " — " + end.Format("15:04")
		}
		fmt.Fprintf(&b, "%s %s\n", detailLabel.Render("when:"), when)
	}
	if e.Location != "" {
		fmt.Fprintf(&b, "%s %s\n", detailLabel.Render("where:"), detailValue.Render(e.Location))
	}
	if e.Description != "" {
		fmt.Fprintf(&b, "\n%s\n", detailValue.Render(e.Description))
	}
	if e.RecurringEventId != "" {
		fmt.Fprintf(&b, "\n%s\n", subtle.Render("part of a recurring series"))
	}

	w := min(m.width-60, 60)
	h := m.height - 7
	if w < 8 {
		w = 8
	}
	if h < 3 {
		h = 3
	}
	return detailBox.Width(w).Height(h).Render(b.String())
}

func (m *Model) renderEmpty() string {
	sel := ""
	d := m.day.Format("Mon 02 Jan")
	if m.day.After(today()) {
		sel = " (future)"
	} else if m.day.Before(today()) {
		sel = " (past)"
	}
	return "No events on " + d + sel + ".\n\nPress n to create one."
}

func (m *Model) renderError() string {
	return "Failed to load events:\n\n" + m.loadErr.Error() + "\n\nPress r to retry."
}

func (m *Model) renderSpinner() string {
	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	idx := int(time.Now().UnixMilli()/100) % len(frames)
	return frames[idx] + " loading events…"
}
