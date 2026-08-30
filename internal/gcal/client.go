// Package gcal wraps the Google Calendar API for the TUI.
package gcal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

// Event is the subset of a calendar event the UI needs.
type Event struct {
	Id               string `json:"id,omitempty"`
	Etag             string `json:"etag,omitempty"`
	Summary          string `json:"summary,omitempty"`
	Location         string `json:"location,omitempty"`
	Description      string `json:"description,omitempty"`
	RecurringEventId string `json:"recurringEventId,omitempty"`
	Start            *EDT   `json:"start,omitempty"`
	End              *EDT   `json:"end,omitempty"`
}

// EDT mirrors the calendar API's EventDateTime.
type EDT struct {
	Date     string `json:"date,omitempty"`
	DateTime string `json:"dateTime,omitempty"`
	TimeZone string `json:"timeZone,omitempty"`
}

func toAPIEvent(e *Event) *calendar.Event {
	if e == nil {
		return nil
	}
	out := &calendar.Event{
		Id:               e.Id,
		Etag:             e.Etag,
		Summary:          e.Summary,
		Location:         e.Location,
		Description:      e.Description,
		RecurringEventId: e.RecurringEventId,
	}
	if e.Start != nil {
		out.Start = &calendar.EventDateTime{Date: e.Start.Date, DateTime: e.Start.DateTime, TimeZone: e.Start.TimeZone}
	}
	if e.End != nil {
		out.End = &calendar.EventDateTime{Date: e.End.Date, DateTime: e.End.DateTime, TimeZone: e.End.TimeZone}
	}
	return out
}

func fromAPIEvent(e *calendar.Event) *Event {
	if e == nil {
		return nil
	}
	out := &Event{
		Id:               e.Id,
		Etag:             e.Etag,
		Summary:          e.Summary,
		Location:         e.Location,
		Description:      e.Description,
		RecurringEventId: e.RecurringEventId,
	}
	if e.Start != nil {
		out.Start = &EDT{Date: e.Start.Date, DateTime: e.Start.DateTime, TimeZone: e.Start.TimeZone}
	}
	if e.End != nil {
		out.End = &EDT{Date: e.End.Date, DateTime: e.End.DateTime, TimeZone: e.End.TimeZone}
	}
	return out
}

// Client talks to the Google Calendar API.
type Client struct {
	svc   *calendar.Service
	calID string
}

// Options configures how the client authenticates. Empty fields fall back to
// environment variables: GOOGLE_CLIENT_ID, GOOGLE_CLIENT_SECRET,
// GOOGLE_REDIRECT_URI and GOOGLE_TOKEN.
type Options struct {
	// ClientID is the OAuth client id. Defaults to $GOOGLE_CLIENT_ID.
	ClientID string
	// ClientSecret is the OAuth client secret. Defaults to $GOOGLE_CLIENT_SECRET.
	ClientSecret string
	// RedirectURL is the OAuth redirect URI. Defaults to $GOOGLE_REDIRECT_URI
	// or http://localhost:<port>.
	RedirectURL string
	// Token is the path where the refresh token is stored. Defaults to
	// $GOOGLE_TOKEN or "token.json".
	Token string
	// CalendarID is the calendar to operate on. Empty means "primary".
	CalendarID string
	// Port is the local port for the OAuth loopback callback.
	Port int
}

// Env variable names. Exported so callers can document them.
const (
	EnvClientID     = "GOOGLE_CLIENT_ID"
	EnvClientSecret = "GOOGLE_CLIENT_SECRET"
	EnvRedirectURI  = "GOOGLE_REDIRECT_URI"
	EnvToken        = "GOOGLE_TOKEN"
)

func orEnv(v, key string) string {
	if v != "" {
		return v
	}
	return os.Getenv(key)
}

func defaultPort() int {
	if v := os.Getenv("PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil && p > 0 {
			return p
		}
	}
	return 8765
}

// New authenticates (if needed) and returns a ready client.
func New(ctx context.Context, opts Options) (*Client, error) {
	opts.ClientID = orEnv(opts.ClientID, EnvClientID)
	opts.ClientSecret = orEnv(opts.ClientSecret, EnvClientSecret)
	opts.RedirectURL = orEnv(opts.RedirectURL, EnvRedirectURI)

	if opts.ClientID == "" || opts.ClientSecret == "" {
		return nil, fmt.Errorf("missing OAuth client credentials: set %s and %s\n"+
			"Create an OAuth client id here: https://console.cloud.google.com/apis/credentials",
			EnvClientID, EnvClientSecret)
	}
	if opts.RedirectURL == "" {
		if opts.Port == 0 {
			opts.Port = defaultPort()
		}
		opts.RedirectURL = "http://localhost:" + strconv.Itoa(opts.Port)
	} else if opts.Port == 0 {
		// Derive the loopback port from the redirect URI when possible.
		if u, err := url.Parse(opts.RedirectURL); err == nil && u.Port() != "" {
			if p, err := strconv.Atoi(u.Port()); err == nil {
				opts.Port = p
			}
		}
		if opts.Port == 0 {
			opts.Port = defaultPort()
		}
	}
	if opts.Token == "" {
		opts.Token = orEnv("", EnvToken)
	}
	if opts.Token == "" {
		opts.Token = "token.json"
	}

	cfg := &oauth2.Config{
		ClientID:     opts.ClientID,
		ClientSecret: opts.ClientSecret,
		Endpoint:     google.Endpoint,
		RedirectURL:  opts.RedirectURL,
		Scopes:       []string{calendar.CalendarScope},
	}

	tok, err := loadToken(opts.Token)
	if err != nil || !tok.Valid() {
		tok, err = authFlow(ctx, cfg, opts.Port)
		if err != nil {
			return nil, fmt.Errorf("authentication failed: %w", err)
		}
		if err := saveToken(opts.Token, tok); err != nil {
			return nil, err
		}
	}

	svc, err := calendar.NewService(ctx, option.WithTokenSource(cfg.TokenSource(ctx, tok)))
	if err != nil {
		return nil, err
	}

	calID := opts.CalendarID
	if calID == "" {
		calID = "primary"
	}

	return &Client{svc: svc, calID: calID}, nil
}

// ListEvents returns events overlapping the given day, ordered by start time.
func (c *Client) ListEvents(ctx context.Context, day time.Time) ([]Event, error) {
	start := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, day.Location())
	end := start.Add(24 * time.Hour)

	evs, err := c.svc.Events.List(c.calID).
		TimeMin(start.Format(time.RFC3339)).
		TimeMax(end.Format(time.RFC3339)).
		SingleEvents(true).
		OrderBy("startTime").
		Context(ctx).
		Do()
	if err != nil {
		return nil, err
	}
	out := make([]Event, 0, len(evs.Items))
	for _, e := range evs.Items {
		ev := fromAPIEvent(e)
		if ev != nil {
			out = append(out, *ev)
		}
	}
	return out, nil
}

// ListUpcoming returns events from now until `days` days ahead.
func (c *Client) ListUpcoming(ctx context.Context, days int) ([]Event, error) {
	now := time.Now()
	end := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, days+1)

	evs, err := c.svc.Events.List(c.calID).
		TimeMin(now.Format(time.RFC3339)).
		TimeMax(end.Format(time.RFC3339)).
		SingleEvents(true).
		OrderBy("startTime").
		Context(ctx).
		Do()
	if err != nil {
		return nil, err
	}
	out := make([]Event, 0, len(evs.Items))
	for _, e := range evs.Items {
		ev := fromAPIEvent(e)
		if ev != nil {
			out = append(out, *ev)
		}
	}
	return out, nil
}

// CreateEvent inserts a new event into the calendar.
func (c *Client) CreateEvent(ctx context.Context, e *Event) (*Event, error) {
	api := toAPIEvent(e)
	res, err := c.svc.Events.Insert(c.calID, api).Context(ctx).Do()
	if err != nil {
		return nil, err
	}
	return fromAPIEvent(res), nil
}

// UpdateEvent replaces an existing event identified by e.Id.
func (c *Client) UpdateEvent(ctx context.Context, e *Event) (*Event, error) {
	if e.Id == "" {
		return nil, errors.New("event has no id")
	}
	api := toAPIEvent(e)
	res, err := c.svc.Events.Update(c.calID, e.Id, api).Context(ctx).Do()
	if err != nil {
		return nil, err
	}
	return fromAPIEvent(res), nil
}

// DeleteEvent removes an event by id.
func (c *Client) DeleteEvent(ctx context.Context, id string) error {
	return c.svc.Events.Delete(c.calID, id).Context(ctx).Do()
}

// StartTime returns the event start as a time.Time, falling back to the
// event end if it is a full-day event without explicit time.
func (e *Event) StartTime() (time.Time, error) {
	return parseTime(e.Start, e.End)
}

// EndTime returns the event end as a time.Time.
func (e *Event) EndTime() (time.Time, error) {
	end := e.End
	if end == nil {
		end = e.Start
	}
	return parseTime(end, end)
}

// AllDay reports whether the event is a full-day event.
func (e *Event) AllDay() bool { return e.Start != nil && e.Start.Date != "" }

func parseTime(start, end *EDT) (time.Time, error) {
	if start == nil {
		return time.Time{}, errors.New("event has no start time")
	}
	if start.Date != "" {
		return time.ParseInLocation("2006-01-02", start.Date, time.Local)
	}
	if start.DateTime != "" {
		return time.Parse(time.RFC3339, start.DateTime)
	}
	// All-day events may carry both empty fields and a date only on End.
	// Fall back to End (e.g. Microsoft-imported calendars).
	if end != nil && end.DateTime != "" {
		return time.Parse(time.RFC3339, end.DateTime)
	}
	return time.Time{}, errors.New("could not determine event time")
}

// Timezone returns the event timezone (or the system local zone).
func (e *Event) Timezone() string {
	if e.Start != nil && e.Start.TimeZone != "" {
		return e.Start.TimeZone
	}
	return time.Local.String()
}

func saveToken(path string, tok *oauth2.Token) error {
	dir := filepath.Dir(path)
	if dir != "." {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return err
		}
	}
	b, err := json.Marshal(tok)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o600)
}

func loadToken(path string) (*oauth2.Token, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var tok oauth2.Token
	if err := json.Unmarshal(b, &tok); err != nil {
		return nil, err
	}
	if tok.RefreshToken == "" {
		return nil, errors.New("token has no refresh token")
	}
	ts := &oauth2.Token{AccessToken: tok.AccessToken, TokenType: tok.TokenType, RefreshToken: tok.RefreshToken, Expiry: tok.Expiry}
	return ts, nil
}

// authFlow performs the OAuth loopback flow: opens the consent page in the
// browser (or prints the URL), then binds a local HTTP server to receive the
// authorization code.
func authFlow(ctx context.Context, cfg *oauth2.Config, port int) (*oauth2.Token, error) {
	u, err := url.Parse(cfg.RedirectURL)
	if err != nil {
		return nil, err
	}
	portStr := u.Port()
	if portStr == "" {
		portStr = strconv.Itoa(port)
	}

	redirect := "http://localhost:" + portStr
	cfg.RedirectURL = redirect

	authURL := cfg.AuthCodeURL("state-token", oauth2.AccessTypeOffline)

	mux := http.NewServeMux()
	var (
		codeCh = make(chan string, 1)
		errCh  = make(chan error, 1)
	)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		c := r.URL.Query().Get("code")
		if c == "" {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, "Missing authorization code: %s", r.URL.RawQuery)
			errCh <- errors.New("missing code in callback")
			return
		}
		if r.URL.Query().Get("state") != "state-token" {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, "state mismatch")
			errCh <- errors.New("state mismatch")
			return
		}
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, "<html><body style='font-family:sans-serif;display:flex;align-items:center;justify-content:center'>"+
			"<h2>Authenticated! You may close this tab now.</h2></body></html>")
		codeCh <- c
	})

	srv := &http.Server{Addr: "127.0.0.1:" + portStr, Handler: mux}
	go func() {
		<-ctx.Done()
		_ = srv.Shutdown(ctx)
	}()
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	fmt.Println("\nTo authorize terminal_calendar, open the following URL in your browser:")
	fmt.Println(authURL)
	fmt.Println("\nWaiting for the authorization callback on " + redirect + " ...")
	_ = openBrowser(authURL)

	select {
	case code := <-codeCh:
		_ = srv.Close()
		tok, err := cfg.Exchange(ctx, code)
		if err != nil {
			return nil, err
		}
		return tok, nil
	case err := <-errCh:
		return nil, err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
