package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"gitnub.com/Mizerael/terminal_calendar/internal/gcal"
)

// renderList builds the week view: a header, a status line and the week grid
// (Google-calendar style). When a popup is active it is layered on top by
// View().
func (m *Model) renderList() string {
	var body string
	switch {
	case m.loading && !m.loaded:
		body = spinnerBox.Render(m.renderSpinner())
	case m.loadErr != nil:
		body = errorBox.Render(m.renderError())
	default:
		body = m.renderGrid()
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
	sel := fmt.Sprintf("%s %02d:00", weekDayLabel(m.dayIndex), m.focusedHour())
	return fmt.Sprintf("%d events · %s · %s   %s", n, day.Format("Mon 02 Jan"), sel, weekKeysHint())
}

func weekKeysHint() string {
	return "[j/k event] [h/l day] [ctrl+u/ctrl+d scroll] [</> week] [enter detail] [n new] [e edit] [d delete] [t today] [?] [q quit]"
}

func weekDayLabel(idx int) string {
	return time.Weekday((idx + 1) % 7).String()[:3]
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

func (m *Model) renderError() string {
	return "Failed to load events:\n\n" + m.loadErr.Error() + "\n\nPress r to retry."
}

func (m *Model) renderSpinner() string {
	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	idx := int(time.Now().UnixMilli()/100) % len(frames)
	return frames[idx] + " loading week…"
}
