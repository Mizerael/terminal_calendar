package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"gitnub.com/Mizerael/terminal_calendar/internal/gcal"
)

// renderList builds the week view: a header, the agenda (7 day sections) and
// a status line. When a popup is active it is layered on top by View().
func (m *Model) renderList() string {
	var body string
	switch {
	case m.loading && !m.loaded:
		body = spinnerBox.Render(m.renderSpinner())
	case m.loadErr != nil:
		body = errorBox.Render(m.renderError())
	case m.totalEvents() == 0:
		body = emptyBox.Render(m.renderEmpty())
	default:
		body = m.renderAgenda()
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		m.renderHeader(),
		statusLine.Render(m.renderStatus()),
		body,
	)
}

func (m *Model) renderHeader() string {
	weekEnd := m.weekStart.AddDate(0, 0, 6)
	title := fmt.Sprintf("Week: %s — %s", m.weekStart.Format("02 Jan"), weekEnd.Format("02 Jan 2006"))
	right := "◄ [ week ] ►"
	if !m.lastSync.IsZero() {
		right = "synced " + m.lastSync.Format("15:04:05") + "  ◄ [ week ] ►"
	}
	titleStr := titleStyle.SetString(title).Render()
	row := lipgloss.JoinHorizontal(lipgloss.Center,
		titleStr,
		lipgloss.NewStyle().Width(m.width-80).Render(""),
		subtle.Render(right),
	)
	return lipgloss.JoinVertical(lipgloss.Left, row)
}

func (m *Model) renderStatus() string {
	if m.loading {
		return "loading week…"
	}
	if m.loadErr != nil {
		return "error: " + m.loadErr.Error()
	}
	n := m.totalEvents()
	day := m.focusedDay()
	return fmt.Sprintf("%d events this week · %s   %s", n, day.Format("Mon 02 Jan"), weekKeysHint())
}

func weekKeysHint() string {
	return "[j/k move] [h/l day] [</> week] [enter detail] [n new] [e edit] [d delete] [t today] [?] [q quit]"
}

// renderAgenda lays out the 7 days vertically with their events, auto-scrolling
// so the focused day/event stays within the terminal height.
func (m *Model) renderAgenda() string {
	var parts []string
	for d := 0; d < 7; d++ {
		parts = append(parts, m.renderDaySection(d))
	}
	joined := lipgloss.JoinVertical(lipgloss.Left, parts...)
	lines := strings.Split(joined, "\n")
	avail := m.height - 5
	if avail < 3 {
		avail = 3
	}
	focusRow := m.focusedRow()
	top := 0
	if focusRow >= avail {
		top = focusRow - avail + 3
	}
	if top < 0 {
		top = 0
	}
	end := top + avail
	if end > len(lines) {
		end = len(lines)
	}
	return lipgloss.JoinVertical(lipgloss.Left, lines[top:end]...)
}

// focusedRow computes the line number (within the agenda) of the focused
// event, or of the focused day header when that day has no events.
func (m *Model) focusedRow() int {
	row := 0
	for d := 0; d < m.dayIndex; d++ {
		row += 1 + len(m.weekEvents[d]) // day header + its events
	}
	row += 1 // focused day's header
	if m.eventIndex >= 0 {
		row += m.eventIndex
	}
	return row
}

func weekDayLabel(idx int) string {
	return time.Weekday((idx + 1) % 7).String()[:3]
}

func (m *Model) renderDayHeader(idx int, day time.Time) string {
	label := fmt.Sprintf("%s %02d", weekDayLabel(idx), day.Day())

	isToday := day.Equal(today())
	isFocused := idx == m.dayIndex

	var s lipgloss.Style
	switch {
	case isFocused:
		s = dayHeaderSelected
	default:
		s = dayHeader
	}
	txt := s.Render(label)
	if isToday {
		txt += subtle.Render("  ◆ today")
	}
	if m.weekEvents[idx] == nil || len(m.weekEvents[idx]) == 0 {
		txt += subtle.Render("  (no events)")
	}
	return txt
}

func (m *Model) renderDaySection(idx int) string {
	var b strings.Builder
	b.WriteString(m.renderDayHeader(idx, m.weekStart.AddDate(0, 0, idx)))
	b.WriteString("\n")
	for i := range m.weekEvents[idx] {
		b.WriteString(m.renderEventRow(idx, i, &m.weekEvents[idx][i]))
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func (m *Model) renderEventRow(dayIdx, evtIdx int, e *gcal.Event) string {
	sel := dayIdx == m.dayIndex && evtIdx == m.eventIndex

	timeTag := timebox.Render(eventTimeLine(e))
	title := e.Summary
	if title == "" {
		title = "(untitled)"
	}
	titleStr := rowTitle.Render(title)
	line := lipgloss.JoinHorizontal(lipgloss.Left, timeTag, " ", titleStr)

	if sel {
		return lipgloss.JoinHorizontal(lipgloss.Left, listCursor.Render("▸ "), selectedRow.Inline(true).Render(line))
	}
	return lipgloss.JoinHorizontal(lipgloss.Left, "  ", line)
}

// renderPopup draws the modal detail overlay content (no frame; the caller
// places it and dims the background).
func (m *Model) renderPopup(e *gcal.Event) string {
	var b strings.Builder
	b.WriteString(popupTitle.Render(detailTitleText(e)) + "\n\n")

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
	b.WriteString("\n")
	b.WriteString(formHint.Render("esc: close · e: edit · d: delete"))

	return popupBox.Width(m.width - 6).Render(b.String())
}

func detailTitleText(e *gcal.Event) string {
	if e.Summary != "" {
		return e.Summary
	}
	return "(untitled event)"
}

func (m *Model) renderEmpty() string {
	sel := weeklyRange(m.weekStart)
	return "No events scheduled this week (" + sel + ").\n\nPress n to create one."
}

func weeklyRange(start time.Time) string {
	return start.Format("02 Jan") + " — " + start.AddDate(0, 0, 6).Format("02 Jan 06")
}

func (m *Model) renderError() string {
	return "Failed to load events:\n\n" + m.loadErr.Error() + "\n\nPress r to retry."
}

func (m *Model) renderSpinner() string {
	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	idx := int(time.Now().UnixMilli()/100) % len(frames)
	return frames[idx] + " loading week…"
}
