package tui

import (
	"testing"

	"github.com/Mizerael/terminal_calendar/internal/domain"
	tea "github.com/charmbracelet/bubbletea"
)

func createRecurringFromForm(t *testing.T, frequency recurrenceFreq, count int) *domain.Event {
	t.Helper()
	f := &fakeAPI{}
	m, err := newTestModel(f)
	if err != nil {
		t.Fatal(err)
	}
	m = upd(t, m, tea.WindowSizeMsg{Width: 100, Height: 40})
	m = upd(t, m, msgKey("n"))
	m.form.inputs[fieldTitle].SetValue("Recurring event")
	m.form.inputs[fieldStartDate].SetValue("2026-09-01")
	m.form.inputs[fieldStartTime].SetValue("10:30")
	m.form.inputs[fieldEndDate].SetValue("2026-09-01")
	m.form.inputs[fieldEndTime].SetValue("11:00")

	m.form.focus = int(fieldRepeat)
	m = upd(t, m, msgKey("enter")) // Open the recurrence picker.
	for i := 0; i < int(frequency); i++ {
		m = upd(t, m, msgKey("j"))
	}
	for i := 0; i < count; i++ {
		m = upd(t, m, msgKey("l"))
	}
	m = run(t, m, msgKey("enter")) // Apply the choice.
	m = run(t, m, msgKey("enter")) // Submit the form through the create command.
	if len(f.created) != 1 {
		t.Fatalf("created events = %d, want 1; form error=%q", len(f.created), m.form.err)
	}
	return f.created[0]
}

func TestRecurrencePickerCreateCommandFlow(t *testing.T) {
	t.Setenv("TZ", "Europe/Saratov")
	cases := []struct {
		name  string
		freq  recurrenceFreq
		count int
		want  string
	}{
		{name: "no repeat", freq: recurNone, want: ""},
		{name: "daily count", freq: recurDaily, count: 3, want: "RRULE:FREQ=DAILY;COUNT=3"},
		{name: "weekly forever", freq: recurWeekly, want: "RRULE:FREQ=WEEKLY"},
		{name: "monthly count", freq: recurMonthly, count: 2, want: "RRULE:FREQ=MONTHLY;COUNT=2"},
		{name: "yearly forever", freq: recurYearly, want: "RRULE:FREQ=YEARLY"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ev := createRecurringFromForm(t, tc.freq, tc.count)
			if tc.want == "" {
				if len(ev.Recurrence) != 0 {
					t.Errorf("recurrence = %v, want no recurrence", ev.Recurrence)
				}
				return
			}
			if len(ev.Recurrence) != 1 || ev.Recurrence[0] != tc.want {
				t.Errorf("recurrence = %v, want [%s]", ev.Recurrence, tc.want)
			}
			if ev.TimeZone != "Europe/Saratov" {
				t.Errorf("timeZone = %q, want Europe/Saratov", ev.TimeZone)
			}
		})
	}
}

func TestRecurrenceCreateValidationStopsCommand(t *testing.T) {
	cases := []struct {
		name string
		set  func(*Model)
	}{
		{name: "required title", set: func(*Model) {}},
		{name: "invalid date", set: func(m *Model) {
			m.form.inputs[fieldTitle].SetValue("Recurring event")
			m.form.inputs[fieldStartDate].SetValue("2026-02-31")
		}},
		{name: "invalid time", set: func(m *Model) {
			m.form.inputs[fieldTitle].SetValue("Recurring event")
			m.form.inputs[fieldStartDate].SetValue("2026-09-01")
			m.form.inputs[fieldStartTime].SetValue("25:00")
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := &fakeAPI{}
			m, err := newTestModel(f)
			if err != nil {
				t.Fatal(err)
			}
			m = upd(t, m, msgKey("n"))
			m.form.inputs[fieldStartDate].SetValue("2026-09-01")
			m.form.inputs[fieldStartTime].SetValue("10:30")
			m.form.inputs[fieldEndDate].SetValue("2026-09-01")
			m.form.inputs[fieldEndTime].SetValue("11:00")
			tc.set(m)
			m.form.focus = int(fieldRepeat)
			m = upd(t, m, msgKey("enter")) // Open the recurrence picker.
			m = upd(t, m, msgKey("j"))     // Select Daily.
			m = run(t, m, msgKey("enter")) // Apply the recurrence.
			m = run(t, m, msgKey("enter"))
			if len(f.created) != 0 {
				t.Fatalf("created %d events despite validation failure", len(f.created))
			}
			if m.form.err == "" {
				t.Fatal("form did not show a validation error")
			}
		})
	}
}

func TestAllDayRecurringFormKeepsDateOnlyEvent(t *testing.T) {
	f := &fakeAPI{}
	m, err := newTestModel(f)
	if err != nil {
		t.Fatal(err)
	}
	m.form.timezoneResolver = func() string { return "" }
	m = upd(t, m, msgKey("n"))
	m.form.inputs[fieldTitle].SetValue("Holiday")
	m.form.inputs[fieldStartDate].SetValue("2026-12-24")
	m.form.focus = int(fieldRepeat)
	m = upd(t, m, msgKey("enter"))
	for i := 0; i < int(recurYearly); i++ {
		m = upd(t, m, msgKey("j"))
	}
	m = run(t, m, msgKey("enter"))
	m = run(t, m, msgKey("enter"))
	if len(f.created) != 1 {
		t.Fatalf("created events = %d, want 1; error=%q", len(f.created), m.form.err)
	}
	ev := f.created[0]
	if !ev.AllDay || ev.Start.Format("2006-01-02") != "2026-12-24" || ev.End.Format("2006-01-02") != "2026-12-25" {
		t.Errorf("all-day recurrence dates = %v .. %v, allDay=%v", ev.Start, ev.End, ev.AllDay)
	}
	if len(ev.Recurrence) != 1 || ev.Recurrence[0] != "RRULE:FREQ=YEARLY" {
		t.Errorf("recurrence = %v, want yearly forever", ev.Recurrence)
	}
	if ev.TimeZone != "" {
		t.Errorf("all-day event timeZone = %q, want no explicit timezone", ev.TimeZone)
	}
}

func TestAllDayRecurringFormRejectsNonExclusiveEndDate(t *testing.T) {
	for _, endDate := range []string{"2026-12-24", "2026-12-23"} {
		t.Run(endDate, func(t *testing.T) {
			f := &fakeAPI{}
			m, err := newTestModel(f)
			if err != nil {
				t.Fatal(err)
			}
			m.form.timezoneResolver = func() string { return "" }
			m = upd(t, m, msgKey("n"))
			m.form.inputs[fieldTitle].SetValue("Holiday")
			m.form.inputs[fieldStartDate].SetValue("2026-12-24")
			m.form.inputs[fieldEndDate].SetValue(endDate)
			m.form.focus = int(fieldRepeat)
			m = upd(t, m, msgKey("enter"))
			for i := 0; i < int(recurYearly); i++ {
				m = upd(t, m, msgKey("j"))
			}
			m = run(t, m, msgKey("enter"))
			m = run(t, m, msgKey("enter"))
			if len(f.created) != 0 {
				t.Fatalf("created %d all-day events with non-exclusive end date", len(f.created))
			}
			if m.form.err != "all-day end date must be after start date" {
				t.Errorf("form error = %q, want all-day exclusive-end validation", m.form.err)
			}
		})
	}
}

func TestTimedRecurringFormStillRequiresTimezone(t *testing.T) {
	f := &fakeAPI{}
	m, err := newTestModel(f)
	if err != nil {
		t.Fatal(err)
	}
	m.form.timezoneResolver = func() string { return "" }
	m = upd(t, m, msgKey("n"))
	m.form.inputs[fieldTitle].SetValue("Daily sync")
	m.form.inputs[fieldStartDate].SetValue("2026-09-01")
	m.form.inputs[fieldStartTime].SetValue("10:30")
	m.form.inputs[fieldEndDate].SetValue("2026-09-01")
	m.form.inputs[fieldEndTime].SetValue("11:00")
	m.form.focus = int(fieldRepeat)
	m = upd(t, m, msgKey("enter"))
	m = upd(t, m, msgKey("j"))
	m = run(t, m, msgKey("enter"))
	m = run(t, m, msgKey("enter"))
	if len(f.created) != 0 {
		t.Fatal("created timed recurring event without an IANA timezone")
	}
	if m.form.err != "repeats require timezone" {
		t.Errorf("form error = %q, want recurring timezone validation", m.form.err)
	}
}
