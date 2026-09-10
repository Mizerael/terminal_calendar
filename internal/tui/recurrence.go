package tui

import (
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type recurrenceFreq int

const (
	recurNone recurrenceFreq = iota
	recurDaily
	recurWeekly
	recurMonthly
	recurYearly
)

type recurrenceOption struct {
	label string
	freq  recurrenceFreq
}

var recurrenceOptions = []recurrenceOption{
	{label: "None (no repeat)", freq: recurNone},
	{label: "Daily", freq: recurDaily},
	{label: "Weekly", freq: recurWeekly},
	{label: "Monthly", freq: recurMonthly},
	{label: "Yearly", freq: recurYearly},
}

func (m *Model) updateRecurrencePicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.recurrencePicker = false
	case "j", "down":
		if m.recurrencePickerIndex < len(recurrenceOptions)-1 {
			m.recurrencePickerIndex++
		} else {
			m.recurrencePickerIndex = 0
		}
	case "k", "up":
		if m.recurrencePickerIndex > 0 {
			m.recurrencePickerIndex--
		} else {
			m.recurrencePickerIndex = len(recurrenceOptions) - 1
		}
	case "l", "right":
		if m.recurrencePickerCount < 99 {
			m.recurrencePickerCount++
		} else {
			m.recurrencePickerCount = 0
		}
	case "h", "left":
		if m.recurrencePickerCount > 1 {
			m.recurrencePickerCount--
		}
	case "enter":
		m.form.recurFreq = recurrenceOptions[m.recurrencePickerIndex].freq
		m.form.recurCount = m.recurrencePickerCount
		m.form.next()
		m.recurrencePicker = false
	}
	return m, nil
}

func (m *Model) renderRecurrencePicker() string {
	var b strings.Builder
	b.WriteString(pickerTitle.Render("Repeat") + "\n\n")
	for i, opt := range recurrenceOptions {
		line := fmt.Sprintf("%d. %s", i+1, opt.label)
		if m.recurrencePickerIndex == i {
			line = pickerCursor.Render(line)
		} else {
			line = pickerRow.Render(line)
		}
		b.WriteString(line + "\n")
	}
	countLabel := "forever"
	if m.recurrencePickerCount > 0 {
		countLabel = fmt.Sprintf("%d times", m.recurrencePickerCount)
	}
	b.WriteString("\n" + formLabel.Render("repeat count:") + " " + subtle.Render(countLabel) + "\n")
	b.WriteString((pickerHint.Render("j/k: frequency · h/l: count · enter: apply · esc: cancel")))
	boxed := formBox.Render(b.String())
	return overlay(m.form.render(m.width, m.height), boxed, m.width, m.height)
}

func buildRecurrenceRRule(freq recurrenceFreq, count int) string {
	if freq == recurNone {
		return ""
	}

	var f string
	switch freq {
	case recurDaily:
		f = "DAILY"
	case recurWeekly:
		f = "WEEKLY"
	case recurMonthly:
		f = "MONTHLY"
	case recurYearly:
		f = "YEARLY"
	}
	rrule := "RRULE:FREQ=" + f
	if count > 0 {
		rrule += ";COUNT=" + strconv.Itoa(count)
	}
	return rrule
}

func parseRecurrenceRRule(rrule string) (recurrenceFreq, int) {
	rrules := strings.Split(strings.TrimPrefix(rrule, "RRULE:"), ";")
	freq := recurNone
	count := 0
	for _, rule := range rrules {
		type_value := strings.SplitN(rule, "=", 2)
		switch type_value[0] {
		case "FREQ":
			switch type_value[1] {
			case "DAILY":
				freq = recurDaily
			case "WEEKLY":
				freq = recurWeekly
			case "MONTHLY":
				freq = recurMonthly
			case "YEARLY":
				freq = recurYearly
			}
		case "COUNT":
			n, err := strconv.Atoi(type_value[1])
			if err != nil {
				n = 0
			}
			count = n
		}
	}
	return freq, count
}
