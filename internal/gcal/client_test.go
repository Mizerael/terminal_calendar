package gcal

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
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

func TestTimezone(t *testing.T) {
	ev := &Event{Start: &EDT{DateTime: "2026-08-30T09:00:00Z"}}
	if got := ev.Timezone(); got != "" {
		t.Errorf("tz without explicit zone = %q, want empty", got)
	}
	ev = &Event{Start: &EDT{DateTime: "2026-08-30T09:00:00Z", TimeZone: "Europe/Berlin"}}
	if ev.Timezone() != "Europe/Berlin" {
		t.Errorf("tz = %q, want Europe/Berlin", ev.Timezone())
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

func TestValidateClientCredentials(t *testing.T) {
	good := "1234567890-abc123.apps.googleusercontent.com"
	cases := []struct {
		name   string
		id     string
		secret string
		wantOK bool
	}{
		{"valid", good, strings.Repeat("G", 24), true},
		{"placeholder id", "your_client_id.apps.googleusercontent.com", strings.Repeat("G", 24), false},
		{"placeholder secret", good, "your_client_secret", false},
		{"non-google id", "ghp_abc123", strings.Repeat("G", 24), false},
		{"short secret", good, "short", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := validateClientCredentials(c.id, c.secret)
			if (err == nil) != c.wantOK {
				t.Errorf("valid = %v (%v), want %v", err == nil, err, c.wantOK)
			}
		})
	}
}

func TestAccountHintPersistence(t *testing.T) {
	tokenPath := t.TempDir() + "/token.json"
	if got := loadAccountHint(tokenPath); got != "" {
		t.Fatalf("hint before save = %q, want empty", got)
	}
	if err := saveAccountHint(tokenPath, "me@gmail.com"); err != nil {
		t.Fatal(err)
	}
	if got := loadAccountHint(tokenPath); got != "me@gmail.com" {
		t.Errorf("hint after save = %q", got)
	}
	// clearing the hint removes the file
	if err := saveAccountHint(tokenPath, ""); err != nil {
		t.Fatal(err)
	}
	if got := loadAccountHint(tokenPath); got != "" {
		t.Errorf("hint after clear = %q, want empty", got)
	}
}

func TestResolveAccountFixed(t *testing.T) {
	account, err := (Options{Account: "  fixed@gmail.com "}).resolveAccount()
	if err != nil {
		t.Fatal(err)
	}
	if account != "fixed@gmail.com" {
		t.Errorf("account = %q", account)
	}
}

func TestResolveAccountPrompts(t *testing.T) {
	prompted := 0
	opts := Options{
		PromptAccount: func(current string) (string, error) {
			prompted++
			return "picked@gmail.com", nil
		},
	}
	account, err := opts.resolveAccount()
	if err != nil {
		t.Fatal(err)
	}
	if prompted != 1 {
		t.Errorf("prompt called %d times, want 1", prompted)
	}
	if account != "picked@gmail.com" {
		t.Errorf("account = %q", account)
	}
}

func TestResolveAccountPassesPrefill(t *testing.T) {
	tokenPath := t.TempDir() + "/token.json"
	if err := saveAccountHint(tokenPath, "prev@gmail.com"); err != nil {
		t.Fatal(err)
	}
	var current string
	opts := Options{
		Token: tokenPath,
		PromptAccount: func(prev string) (string, error) {
			current = prev
			return "", nil
		},
	}
	if _, err := opts.resolveAccount(); err != nil {
		t.Fatal(err)
	}
	if current != "prev@gmail.com" {
		t.Errorf("prefill = %q, want prev@gmail.com", current)
	}
}

func TestListEventsRange(t *testing.T) {
	var gotMin, gotMax string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMin = r.URL.Query().Get("timeMin")
		gotMax = r.URL.Query().Get("timeMax")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"items":[]}`)
	}))
	defer srv.Close()

	svc, err := calendar.NewService(context.Background(), option.WithoutAuthentication(), option.WithEndpoint(srv.URL))
	if err != nil {
		t.Fatal(err)
	}
	c := &Client{svc: svc, calID: "primary"}

	start := time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC)
	evs, err := c.ListEventsRange(context.Background(), start, start.Add(7*24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(evs) != 0 {
		t.Fatalf("unexpected events: %d", len(evs))
	}
	if gotMin != "2026-08-31T00:00:00Z" {
		t.Errorf("timeMin = %q", gotMin)
	}
	if gotMax != "2026-09-07T00:00:00Z" {
		t.Errorf("timeMax = %q", gotMax)
	}
}

func TestListCalendars(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"items":[
			{"id":"c1","summary":"Work","primary":true},
			{"id":"c2","summary":"Personal"}
		]}`)
	}))
	defer srv.Close()

	svc, err := calendar.NewService(context.Background(), option.WithoutAuthentication(), option.WithEndpoint(srv.URL))
	if err != nil {
		t.Fatal(err)
	}
	c := &Client{svc: svc, calID: "primary"}

	cals, err := c.ListCalendars(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(cals) != 2 {
		t.Fatalf("got %d calendars, want 2", len(cals))
	}
	if cals[0].ID != "c1" || !cals[0].Primary || cals[0].Summary != "Work" {
		t.Errorf("unexpected first calendar: %+v", cals[0])
	}
	if cals[1].ID != "c2" || cals[1].Primary {
		t.Errorf("unexpected second calendar: %+v", cals[1])
	}
}

func TestListEventsRangeInTagsCalendar(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"items":[{"id":"e1","summary":"Standup","start":{"dateTime":"2026-08-31T09:00:00Z"},"end":{"dateTime":"2026-08-31T09:30:00Z"}}]}`)
	}))
	defer srv.Close()

	svc, _ := calendar.NewService(context.Background(), option.WithoutAuthentication(), option.WithEndpoint(srv.URL))
	c := &Client{svc: svc, calID: "primary"}

	start := time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC)
	evs, err := c.ListEventsRangeIn(context.Background(), "personal", start, start.Add(7*24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(evs) != 1 {
		t.Fatalf("got %d events, want 1", len(evs))
	}
	if evs[0].CalendarID != "personal" || evs[0].CalendarSummary != "personal" {
		t.Errorf("event not tagged with calendar: %+v", evs[0])
	}
}
