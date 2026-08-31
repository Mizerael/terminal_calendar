package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// updatePicker handles the calendar visibility/target overlay.
func (m *Model) updatePicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.picker = false
		m.pickerErr = nil
		return m, m.reload()
	case "j", "down":
		if m.pickerIndex < len(m.calendars)-1 {
			m.pickerIndex++
		}
	case "k", "up":
		if m.pickerIndex > 0 {
			m.pickerIndex--
		}
	case "enter", " ":
		m.togglePickerCalendar()
	case "t":
		if i := m.pickerIndex; i >= 0 && i < len(m.calendars) {
			m.targetCalID = m.calendars[i].ID
			m.saveState()
		}
	case "r":
		return m, m.reload()
	}
	return m, nil
}

// togglePickerCalendar flips visibility of the calendar at the picker cursor.
func (m *Model) togglePickerCalendar() {
	if m.pickerIndex < 0 || m.pickerIndex >= len(m.calendars) {
		return
	}
	id := m.calendars[m.pickerIndex].ID
	m.isEnabled[id] = !m.isEnabled[id]
	m.saveState()
}

// renderPicker draws the calendar list overlay.
func (m *Model) renderPicker() string {
	var b strings.Builder
	b.WriteString(pickerTitle.Render("Calendars") + "\n\n")
	for i, cal := range m.calendars {
		mark := "  "
		check := "[ ]"
		if m.isEnabled[cal.ID] {
			check = "[x]"
		}
		if cal.ID == m.targetCalID {
			mark = "→ "
		}
		line := fmt.Sprintf("%s%s %s", mark, check, cal.Summary)
		if cal.Primary {
			line += "  (primary)"
		}
		if m.pickerIndex == i {
			line = pickerCursor.Render(line)
		} else {
			line = pickerRow.Render(line)
		}
		b.WriteString(line + "\n")
	}
	b.WriteString("\n" + pickerHint.Render("j/k: move · enter: show/hide · t: new-event target · esc/q: close"))
	boxed := formBox.Render(b.String())
	return overlay(m.renderList(), boxed, m.width, m.height)
}
