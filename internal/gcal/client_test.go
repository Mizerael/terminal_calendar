package gcal

import (
	"context"
	"strings"
	"testing"
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
