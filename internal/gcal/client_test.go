package gcal

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestParseTimeVariants(t *testing.T) {
	cases := []struct {
		name  string
		start *EDT
		end   *EDT
		want  string // expected local layout "2006-01-02 15:04"
	}{
		{
			name:  "dateTime",
			start: &EDT{DateTime: "2026-08-30T09:15:00+02:00"},
			want:  "2026-08-30 09:15",
		},
		{
			name:  "all-day",
			start: &EDT{Date: "2026-08-30"},
			want:  "2026-08-30 00:00",
		},
		{
			name:  "empty start falls back to end",
			start: &EDT{},
			end:   &EDT{DateTime: "2026-08-30T10:00:00Z"},
			want:  "2026-08-30 10:00",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ev := &Event{Start: c.start, End: c.end}
			got, err := ev.StartTime()
			if err != nil {
				t.Fatalf("StartTime: %v", err)
			}
			if got.Format("2006-01-02 15:04") != c.want {
				t.Errorf("got %v, want %s", got, c.want)
			}
		})
	}
}

func TestAllDayFlag(t *testing.T) {
	timed := &Event{Start: &EDT{DateTime: "2026-08-30T09:00:00Z"}}
	if timed.AllDay() {
		t.Fatal("timed event must not be all-day")
	}
	aday := &Event{Start: &EDT{Date: "2026-08-30"}}
	if !aday.AllDay() {
		t.Fatal("date-only event must be all-day")
	}
	none := &Event{}
	if none.AllDay() {
		t.Fatal("event without start must not be all-day")
	}
}

func TestTimezoneDefaultLocal(t *testing.T) {
	ev := &Event{Start: &EDT{DateTime: "2026-08-30T09:00:00Z"}}
	if ev.Timezone() != time.Local.String() {
		t.Errorf("default tz = %q, want %q", ev.Timezone(), time.Local.String())
	}
	ev = &Event{Start: &EDT{DateTime: "2026-08-30T09:00:00Z", TimeZone: "UTC"}}
	if ev.Timezone() != "UTC" {
		t.Errorf("tz = %q, want UTC", ev.Timezone())
	}
}

func TestNewRequiresCredentials(t *testing.T) {
	t.Setenv(EnvClientID, "")
	t.Setenv(EnvClientSecret, "")
	_, err := New(context.Background(), Options{})
	if err == nil {
		t.Fatal("expected an error when OAuth credentials are missing")
	}
	if !strings.Contains(err.Error(), EnvClientID) || !strings.Contains(err.Error(), "GOOGLE_CLIENT_SECRET") {
		t.Errorf("error should mention the env vars, got: %v", err)
	}
}
