package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"

	"gitnub.com/Mizerael/terminal_calendar/internal/gcal"
)

type fieldID int

const (
	fieldTitle fieldID = iota
	fieldLocation
	fieldStartDate
	fieldStartTime
	fieldEndDate
	fieldEndTime
	fieldDescription
	fieldCount
)

type form struct {
	inputs []textinput.Model
	labels map[fieldID]string
	// focus is the index into inputs.
	focus int
	// editing is non-nil when we edit an existing event.
	editing *gcal.Event
	// error line shown when validation fails.
	err string
}

func newForm() *form {
	labels := map[fieldID]string{
		fieldTitle:       "Title",
		fieldLocation:    "Location",
		fieldStartDate:   "Start date",
		fieldStartTime:   "Start time",
		fieldEndDate:     "End date",
		fieldEndTime:     "End time",
		fieldDescription: "Description",
	}
	f := &form{labels: labels}
	for i := 0; i < int(fieldCount); i++ {
		ti := textinput.New()
		ti.Width = 34
		ti.Prompt = ""
		ti.Placeholder = placeholder(fieldID(i))
		f.inputs = append(f.inputs, ti)
	}
	f.inputs[fieldTitle].Focus()
	return f
}

func placeholder(id fieldID) string {
	switch id {
	case fieldTitle:
		return "Team standup"
	case fieldLocation:
		return "Conference room"
	case fieldStartDate:
		return "YYYY-MM-DD"
	case fieldStartTime:
		return "HH:MM (empty = all day)"
	case fieldEndDate:
		return "YYYY-MM-DD"
	case fieldEndTime:
		return "HH:MM"
	case fieldDescription:
		return "Meeting notes…"
	}
	return ""
}

func (f *form) reset() {
	for i := range f.inputs {
		f.inputs[i].SetValue("")
		f.inputs[i].Blur()
	}
	f.focus = 0
	f.editing = nil
	f.err = ""
	f.inputs[fieldTitle].Focus()
}

func (f *form) setEvent(e *gcal.Event) {
	f.reset()
	f.editing = e
	f.inputs[fieldTitle].SetValue(e.Summary)
	f.inputs[fieldLocation].SetValue(e.Location)

	start, err := e.StartTime()
	if err != nil {
		return
	}
	end, err := e.EndTime()
	if err != nil {
		end = start
	}
	f.inputs[fieldStartDate].SetValue(formatDate(start))
	if e.AllDay() {
		f.inputs[fieldStartTime].SetValue("")
		f.inputs[fieldEndTime].SetValue("")
	} else {
		f.inputs[fieldStartTime].SetValue(start.Format("15:04"))
		f.inputs[fieldEndTime].SetValue(end.Format("15:04"))
	}
	f.inputs[fieldEndDate].SetValue(formatDate(end))
	if e.Description != "" {
		f.inputs[fieldDescription].SetValue(e.Description)
	}
}

func formatDate(t time.Time) string { return t.Format("2006-01-02") }

func (f *form) focused() fieldID { return fieldID(f.focus) }

func (f *form) prev() {
	if f.focus == 0 {
		f.focus = int(fieldCount) - 1
	} else {
		f.focus--
	}
	f.syncFocus()
}

func (f *form) next() {
	f.focus = (f.focus + 1) % int(fieldCount)
	f.syncFocus()
}

func (f *form) syncFocus() {
	for i := range f.inputs {
		if i == f.focus {
			f.inputs[i].Focus()
		} else {
			f.inputs[i].Blur()
		}
	}
}

// value returns the trimmed value of the given field.
func (f *form) value(id fieldID) string { return strings.TrimSpace(f.inputs[id].Value()) }

// buildEvent validates the form and produces a gcal.Event ready to insert or
// update. On validation error it returns an error and stores a message.
func (f *form) buildEvent() (*gcal.Event, error) {
	title := f.value(fieldTitle)
	if title == "" {
		return nil, f.fail("title is required")
	}
	startDate := f.value(fieldStartDate)
	if startDate == "" {
		startDate = time.Now().Format("2006-01-02")
	}
	sd, err := time.ParseInLocation("2006-01-02", startDate, time.Local)
	if err != nil {
		return nil, f.fail("start date must be YYYY-MM-DD")
	}

	endDate := f.value(fieldEndDate)
	if endDate == "" {
		endDate = startDate
	}
	ed, err := time.ParseInLocation("2006-01-02", endDate, time.Local)
	if err != nil {
		return nil, f.fail("end date must be YYYY-MM-DD")
	}

	startTime := f.value(fieldStartTime)
	endTime := f.value(fieldEndTime)
	allDay := startTime == "" && endTime == ""
	if !allDay && startTime == "" {
		return nil, f.fail("start time missing (or leave both empty for all-day)")
	}

	ev := &gcal.Event{}
	if f.editing != nil {
		ev.Id = f.editing.Id
		ev.Etag = f.editing.Etag
	}
	ev.Summary = title
	if loc := f.value(fieldLocation); loc != "" {
		ev.Location = loc
	}
	if desc := f.value(fieldDescription); desc != "" {
		ev.Description = desc
	}

	tz := "Local"
	if f.editing != nil {
		tz = f.editing.Timezone()
	}

	if allDay {
		ev.Start = &gcal.EDT{Date: startDate, TimeZone: tz}
		ev.End = &gcal.EDT{Date: endDate, TimeZone: tz}
		return ev, nil
	}

	if endTime == "" {
		endTime = startTime
	}
	st, err := time.ParseInLocation("15:04", startTime, time.Local)
	if err != nil {
		return nil, f.fail("start time must be HH:MM")
	}
	et, err := time.ParseInLocation("15:04", endTime, time.Local)
	if err != nil {
		return nil, f.fail("end time must be HH:MM")
	}

	start := merge(sd, st)
	end := merge(ed, et)
	if !end.After(start) {
		return nil, f.fail("end must be after start")
	}

	ev.Start = &gcal.EDT{
		DateTime: start.Format(time.RFC3339),
		TimeZone: tz,
	}
	ev.End = &gcal.EDT{
		DateTime: end.Format(time.RFC3339),
		TimeZone: tz,
	}
	return ev, nil
}

func merge(day, clock time.Time) time.Time {
	return time.Date(day.Year(), day.Month(), day.Day(), clock.Hour(), clock.Minute(), 0, 0, day.Location())
}

func (f *form) fail(msg string) error {
	f.err = msg
	return errValidation
}

var errValidation = fmt.Errorf("validation failed")

func (f *form) title() string {
	if f.editing != nil {
		return "Edit event"
	}
	return "New event"
}

// render draws the form centered on the screen.
func (f *form) render(w, h int) string {
	var b strings.Builder
	b.WriteString(titleStyle.SetString(f.title()).Render() + "\n\n")

	rows := make([]string, 0, int(fieldCount)+2)
	for i := 0; i < int(fieldCount); i++ {
		id := fieldID(i)
		value := f.inputs[i].View()
		var row string
		if i == f.focus {
			row = formFocusLabel.Render(f.labels[id]) + " " + value
		} else {
			row = formLabel.Render(f.labels[id]) + " " + value
		}
		rows = append(rows, row)
	}
	b.WriteString(lipgloss.JoinVertical(lipgloss.Left, rows...))
	b.WriteString("\n\n")
	b.WriteString(formHint.Render("tab/↑↓: move · enter: save · esc: cancel") + "\n")
	if f.err != "" {
		b.WriteString(formError.Render("✗ " + f.err + "\n"))
	}

	inner := b.String()
	boxed := formBox.Width(min(w-4, 64)).Render(inner)
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, boxed)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
