package tui

import (
	"fmt"
	"os"
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

	// Only send a timeZone the API accepts (a real IANA name). Never invent
	// one from time.Local: strings like "Local" or "+04:00" are rejected by
	// Google with a 400 error, which silently broke create/update.
	tzName := ""
	if f.editing != nil && f.editing.Start != nil {
		tzName = f.editing.Start.TimeZone // provided by Google, always valid
	}
	if tzName == "" {
		tzName = localIANATimezone() // best effort from the environment
	}

	if allDay {
		ev.Start = &gcal.EDT{Date: startDate, TimeZone: tzName}
		ev.End = &gcal.EDT{Date: endDate, TimeZone: tzName}
		return ev, nil
	}

	if endTime == "" {
		endTime = startTime
	}
	clock, err := parseClock("start", startTime, f)
	if err != nil {
		return nil, err
	}
	endClock, err := parseClock("end", endTime, f)
	if err != nil {
		return nil, err
	}

	// Interpret the typed wall-clock time directly in the target zone so the
	// stored instant matches what the user intended.
	loc := time.Local
	if tzName != "" {
		if l, err := time.LoadLocation(tzName); err == nil {
			loc = l
		}
	}
	start := time.Date(sd.Year(), sd.Month(), sd.Day(), clock.hour, clock.minute, 0, 0, loc)
	end := time.Date(ed.Year(), ed.Month(), ed.Day(), endClock.hour, endClock.minute, 0, 0, loc)
	if !end.After(start) {
		return nil, f.fail("end must be after start")
	}

	layout := "2006-01-02T15:04:05" // timezone carried by the timeZone field
	if tzName == "" {
		layout = time.RFC3339 // no timeZone: embed the offset in dateTime
	}
	ev.Start = &gcal.EDT{DateTime: start.Format(layout), TimeZone: tzName}
	ev.End = &gcal.EDT{DateTime: end.Format(layout), TimeZone: tzName}
	return ev, nil
}

type clock struct{ hour, minute int }

func parseClock(label, v string, f *form) (clock, error) {
	t, err := time.ParseInLocation("15:04", v, time.Local)
	if err != nil {
		return clock{}, f.fail(label + " time must be HH:MM")
	}
	return clock{hour: t.Hour(), minute: t.Minute()}, nil
}

// localIANATimezone returns the user's IANA timezone name when it can be
// determined, otherwise "" (the API then uses the calendar's default zone).
func localIANATimezone() string {
	if tz := os.Getenv("TZ"); tz != "" {
		if _, err := time.LoadLocation(tz); err == nil {
			return tz
		}
	}
	if link, err := os.Readlink("/etc/localtime"); err == nil {
		if i := strings.Index(link, "zoneinfo/"); i >= 0 {
			name := link[i+len("zoneinfo/"):]
			if _, err := time.LoadLocation(name); err == nil {
				return name
			}
		}
	}
	return ""
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
