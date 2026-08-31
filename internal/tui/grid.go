package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/Mizerael/terminal_calendar/internal/domain"
)

const (
	// defaultScrollHour is the first hour row shown at the top of the grid
	// when the user has not scrolled yet (the "workday" start).
	defaultScrollHour = 6

	// gutterW is the width reserved for the leading hour-label column.
	gutterW = 6

	// rowBorderW is the width added by the vertical borders of the day cells
	// across a single hour row (shared borders, not per column).
	rowBorderW = 2

	// colWMin is the smallest width each day column may have on a narrow
	// terminal. There is no upper bound: columns stretch with the terminal so
	// event titles get more room on wide screens.
	colWMin = 8
)

// colW returns the width of each day column based on the terminal width. It
// leaves room for the hour gutter and the cells' shared borders so the grid
// never overflows the terminal.
func (m *Model) colW() int {
	cw := (m.width - gutterW - rowBorderW) / 7
	if cw < colWMin {
		cw = colWMin
	}
	return cw
}

// hourRange delegates the pure hour-span of a timed event to the domain.
func hourRange(e *domain.Event) (start, end int, ok bool) {
	return domain.HourRange(e)
}

// renderGrid draws the week as a Google-Calendar-style grid: a day-header row,
// an optional all-day banner, an hour gutter on the left, and 7 day columns
// where events occupy their hour spans. Only the visible hour window
// [scrollHour, scrollHour+rows) is rendered.
func (m *Model) renderGrid() string {
	rows := m.effectiveRows()
	cw := m.colW()

	// Header row: blank gutter + one cell per day.
	var header []string
	header = append(header, gridGutter.Render("     "))
	for d := 0; d < 7; d++ {
		header = append(header, m.renderDayHeaderCell(d, cw))
	}
	gridHeader := lipgloss.JoinHorizontal(lipgloss.Top, header...)

	// All-day banner row (above the hour grid).
	allDay := m.renderAllDayRow(cw)
	if allDay != "" {
		allDay += "\n"
	}

	// Build each day column into a slice of per-hour rendered strings.
	var cols [7][]string
	for d := 0; d < 7; d++ {
		cols[d] = m.buildDayColumn(d, rows, m.scrollHour, cw)
	}

	var body []string
	for r := 0; r < rows; r++ {
		hour := m.scrollHour + r
		var line []string
		line = append(line, gridGutter.Render(fmt.Sprintf("%02d:00", hour)))
		for d := 0; d < 7; d++ {
			line = append(line, cols[d][r])
		}
		body = append(body, lipgloss.JoinHorizontal(lipgloss.Top, line...))
	}

	return strings.Join(append([]string{gridHeader, allDay}, body...), "\n")
}

// gridRows computes how many hour rows fit given the terminal height. The
// grid stretches to fill the available space, but never more than the 24 hours
// in a day.
func (m *Model) gridRows() int {
	avail := m.height - m.gridOverhead()
	if avail < 6 {
		avail = 6
	}
	if avail > 24 {
		avail = 24
	}
	return avail
}

// gridOverhead is the number of fixed layout lines around the hour grid:
// header, status line, and the all-day banner when present.
func (m *Model) gridOverhead() int {
	overhead := 2 // week header + status line
	for d := 0; d < 7; d++ {
		for i := range m.weekEvents[d] {
			if m.weekEvents[d][i].AllDay {
				return overhead + 1
			}
		}
	}
	return overhead
}

// effectiveRows clamps the available rows to the remaining hours of the day so
// the window never renders past 23:00.
func (m *Model) effectiveRows() int {
	rows := m.gridRows()
	if remain := 24 - m.scrollHour; rows > remain {
		rows = remain
	}
	if rows < 1 {
		rows = 1
	}
	return rows
}

// renderDayHeaderCell builds one day-column header (including an hour marker
// and today/focused highlighting).
func (m *Model) renderDayHeaderCell(d, colW int) string {
	day := m.weekStart.AddDate(0, 0, d)
	label := fmt.Sprintf("%s %02d", weekDayLabel(d), day.Day())
	if day.Equal(today()) {
		label += "●"
	}

	var s lipgloss.Style
	switch {
	case d == m.dayIndex:
		s = gridHeaderDaySelected
	case day.Equal(today()):
		s = gridHeaderDayToday
	case d >= 5:
		s = gridHeaderDayWeekend
	default:
		s = gridHeaderDay
	}
	return s.Copy().Width(colW).Render(label)
}

// renderAllDayRow builds a single row of all-day events, one cell per day.
func (m *Model) renderAllDayRow(colW int) string {
	var cells []string
	any := false
	for d := 0; d < 7; d++ {
		var names []string
		for i := range m.weekEvents[d] {
			e := &m.weekEvents[d][i]
			if e.AllDay {
				names = append(names, e.Summary)
			}
		}
		if len(names) == 0 {
			cells = append(cells, gridCell.Copy().Width(colW).Render(""))
			continue
		}
		any = true
		s := gridCellWeekend
		if d == m.dayIndex {
			s = gridCellFocused
		}
		cells = append(cells, s.Copy().Width(colW).Render(strings.Join(names, ",")))
	}
	if !any {
		return ""
	}
	return lipgloss.JoinHorizontal(lipgloss.Top,
		append([]string{gridGutter.Render("all-day")}, cells...)...)
}

// buildDayColumn returns one rendered string per visible hour row for a single
// day column, laying out events as time-block spans with clash markers.
func (m *Model) buildDayColumn(d, rows, startHour, colW int) []string {
	out := make([]string, rows)
	var blocks []*span
	for i := range m.weekEvents[d] {
		e := &m.weekEvents[d][i]
		if e.AllDay {
			continue
		}
		sh, eh, ok := hourRange(e)
		if !ok {
			continue
		}
		blocks = append(blocks, &span{start: sh, end: eh, idx: i, ev: e})
	}

	// covering[h] = spans active at hour h (0..23).
	covering := make(map[int][]*span)
	for _, b := range blocks {
		for h := b.start; h < b.end && h < 24; h++ {
			covering[h] = append(covering[h], b)
		}
	}

	now := timeNow()
	for r := 0; r < rows; r++ {
		hour := startHour + r
		out[r] = m.renderHourCell(d, hour, covering, now, colW)
	}
	return out
}

type span struct {
	start, end int
	idx        int
	ev         *domain.Event
}

// renderHourCell renders the single cell at the given hour for a day.
func (m *Model) renderHourCell(d, hour int, covering map[int][]*span, now time.Time, colW int) string {
	isNow := d == m.dayIndex && now.Hour() == hour && today().Equal(m.weekStart.AddDate(0, 0, d))
	isWeekend := d >= 5

	active := covering[hour]
	if len(active) == 0 {
		// An empty cell is only highlighted when a real event is focused at
		// this hour; otherwise (no selection) it stays plain, per the user.
		return m.renderEmptyCell(m.emptyCellCursor(d, hour), isWeekend, isNow, colW)
	}

	// The first span active here; it is a block "top" only at its own start
	// hour (where the title is shown), otherwise a continuation cell.
	top := active[0]
	cont := top.start < hour
	isSel := d == m.dayIndex && top.idx == m.eventIndex

	if isSel {
		// The focused event: highlight the whole span distinctly so the
		// selection is obvious regardless of which calendar it belongs to.
		if cont {
			return gridEventCont.Copy().Background(lipgloss.Color("237")).Width(colW).Render("")
		}
		full := focusedEventLabel(top.ev)
		if len(active) > 1 {
			full += " " + lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15")).Render(itoa(len(active))+"x")
		}
		full = truncate(full, colW)
		return gridEventCursor.Copy().Width(colW).Render(full)
	}

	if cont {
		return gridEventCont.Copy().Background(lipgloss.Color(calendarColor(top.ev.CalendarID))).Width(colW).Render("")
	}

	text := top.ev.Summary
	if text == "" {
		text = "(no title)"
	}
	full := fmt.Sprintf("%s", text)
	if len(active) > 1 {
		full += " " + lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("196")).Render(itoa(len(active))+"x")
	}
	full = truncate(full, colW)
	return gridEventTop.Copy().Background(lipgloss.Color(calendarColor(top.ev.CalendarID))).Width(colW).Render(full)
}

// focusedEventLabel prefixes the focused event's title with a pointer so the
// selected row is unmistakable in a merged, color-coded grid.
func focusedEventLabel(e *domain.Event) string {
	text := e.Summary
	if text == "" {
		text = "(no title)"
	}
	return "▸ " + text
}

// emptyCellCursor reports whether the free-hour cell at (d, hour) should be
// painted with the focused highlight. This only happens when an actual event is
// selected, so that an empty day/free hour is never falsely highlighted.
func (m *Model) emptyCellCursor(d, hour int) bool {
	return d == m.dayIndex && hour == m.focusedHour() && m.eventIndex >= 0
}

func (m *Model) renderEmptyCell(isCursor, isWeekend, isNow bool, colW int) string {
	var content string
	if isNow {
		content = "•"
	}
	switch {
	case isCursor:
		return gridCellFocused.Copy().Width(colW).Render(content)
	case isWeekend:
		return gridCellWeekend.Copy().Width(colW).Render(content)
	default:
		return gridCell.Copy().Width(colW).Render(content)
	}
}

func itoa(n int) string {
	return fmt.Sprintf("%d", n)
}

// calendarPalette is a set of pleasant ANSI-256 background colors, one per
// calendar. Events are colored by their owning calendar so a merged view
// remains visually distinguishable.
var calendarPalette = []string{
	"24", "28", "52", "58", "90", "94", "23", "30", "25", "29",
}

// calendarColor returns a stable ANSI-256 color for a calendar id.
func calendarColor(id string) string {
	if id == "" {
		return "58"
	}
	var h uint32 = 2166136261
	for _, b := range []byte(id) {
		h ^= uint32(b)
		h *= 16777619
	}
	return calendarPalette[int(h)%len(calendarPalette)]
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n <= 1 {
		return "…"
	}
	return string(r[:n-1]) + "…"
}

func timeNow() time.Time { return time.Now() }
