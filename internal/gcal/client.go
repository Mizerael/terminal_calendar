// Package gcal is the Clean Architecture gateway: it adapts the Google
// Calendar API (a framework/driver) to the use-case port (usecase.CalendarGateway)
// and translates between the repo's domain types (internal/domain) and the
// google.golang.org/api/calendar/v3 types. All Google-specific and OAuth
// concerns live here and nowhere else.
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
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"

	"github.com/Mizerael/terminal_calendar/internal/domain"
)

// Client talks to the Google Calendar API and satisfies the use-case gateway
// contract for calendar queries and writes.
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
	// CalendarID is the default calendar to operate on. Empty means "primary".
	CalendarID string
	// Port is the local port for the OAuth loopback callback.
	Port int
	// Account is a fixed Google login hint (email). Overrides PromptAccount.
	Account string
	// PromptAccount, when set, lets the user pick the Google account in the
	// terminal before the OAuth consent page opens. Called only when
	// authorization is actually needed.
	PromptAccount func(current string) (string, error)
}

// Env variable names. Exported so callers can document them.
const (
	EnvClientID     = "GOOGLE_CLIENT_ID"
	EnvClientSecret = "GOOGLE_CLIENT_SECRET"
	EnvRedirectURI  = "GOOGLE_REDIRECT_URI"
	EnvToken        = "GOOGLE_TOKEN"
	EnvAccount      = "GOOGLE_ACCOUNT"
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

// resolveAccount decides which Google account (login hint) to use for the
// OAuth flow: a fixed Account, or the user's choice via PromptAccount. The
// previously used account is offered as the default choice.
func (o Options) resolveAccount() (string, error) {
	if o.Account != "" {
		return strings.TrimSpace(o.Account), nil
	}
	if o.PromptAccount == nil {
		return "", nil
	}
	account, err := o.PromptAccount(loadAccountHint(o.Token))
	if err != nil {
		return "", fmt.Errorf("no account selected: %w", err)
	}
	return strings.TrimSpace(account), nil
}

// validateClientCredentials catches the most common causes of Google's
// "invalid_client" error before the consent page is even reached.
func validateClientCredentials(clientID, clientSecret string) error {
	if strings.HasPrefix(clientID, "your_client") || strings.HasPrefix(clientSecret, "your_client") {
		return fmt.Errorf("OAuth credentials look like the .env.example placeholders.\n" +
			"Edit .env and replace GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET with the real values\n" +
			"from your Desktop app at https://console.cloud.google.com/apis/credentials")
	}
	if !strings.HasSuffix(clientID, ".apps.googleusercontent.com") {
		return fmt.Errorf("GOOGLE_CLIENT_ID=%q does not look like a Google OAuth client id\n"+
			"(it should end with .apps.googleusercontent.com). A GitHub/other id will not work.", clientID)
	}
	if len(clientSecret) < 8 {
		return fmt.Errorf("GOOGLE_CLIENT_SECRET is suspiciously short (%d chars); double check it\n"+
			"was copied from the same OAuth client as the client id", len(clientSecret))
	}
	return nil
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
	if err := validateClientCredentials(opts.ClientID, opts.ClientSecret); err != nil {
		return nil, err
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
	switch {
	case err != nil:
		// No cached token yet: run the interactive account flow.
		tok, err = opts.doAuthFlow(ctx, cfg)
		if err != nil {
			return nil, err
		}
	default:
		// Cached refresh token: refresh silently. Only fall back to the
		// interactive flow when the refresh is rejected (revoked credentials).
		ts := cfg.TokenSource(ctx, tok)
		if fresh, rerr := ts.Token(); rerr == nil {
			tok = fresh
		} else {
			tok, err = opts.doAuthFlow(ctx, cfg)
			if err != nil {
				return nil, err
			}
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

// doAuthFlow runs the interactive account selection and OAuth loopback flow,
// then persists the new token and the chosen account.
func (o Options) doAuthFlow(ctx context.Context, cfg *oauth2.Config) (*oauth2.Token, error) {
	loginHint, err := o.resolveAccount()
	if err != nil {
		return nil, err
	}
	tok, err := authFlow(ctx, cfg, o.Port, loginHint)
	if err != nil {
		return nil, fmt.Errorf("authentication failed: %w", err)
	}
	if err := saveToken(o.Token, tok); err != nil {
		return nil, err
	}
	if err := saveAccountHint(o.Token, loginHint); err != nil {
		return nil, err
	}
	return tok, nil
}

// accountHintPath returns the sidecar file that remembers the email used for
// the OAuth flow, derived from the token path.
func accountHintPath(tokenPath string) string {
	return tokenPath + ".account"
}

func saveAccountHint(tokenPath, email string) error {
	if email == "" {
		_ = os.Remove(accountHintPath(tokenPath))
		return nil
	}
	return os.WriteFile(accountHintPath(tokenPath), []byte(email), 0o600)
}

func loadAccountHint(tokenPath string) string {
	b, err := os.ReadFile(accountHintPath(tokenPath))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// ListEvents returns events overlapping the given day, ordered by start time,
// from the client's configured calendar.
func (c *Client) ListEvents(ctx context.Context, day time.Time) ([]domain.Event, error) {
	start := domain.StartOfDay(day)
	return c.ListEventsRange(ctx, start, start.Add(24*time.Hour))
}

// ListCalendars returns the calendars the user can see, in the API's default
// (typically summary) order.
func (c *Client) ListCalendars(ctx context.Context) ([]domain.Calendar, error) {
	res, err := c.svc.CalendarList.List().Context(ctx).Do()
	if err != nil {
		return nil, err
	}
	out := make([]domain.Calendar, 0, len(res.Items))
	for _, item := range res.Items {
		if item == nil {
			continue
		}
		out = append(out, domain.Calendar{
			ID:      item.Id,
			Summary: item.Summary,
			Primary: item.Primary,
		})
	}
	return out, nil
}

// ListEventsRange returns all events overlapping [start, end) from the
// client's configured calendar, ordered by start time.
func (c *Client) ListEventsRange(ctx context.Context, start, end time.Time) ([]domain.Event, error) {
	return c.ListEventsRangeIn(ctx, c.calID, start, end)
}

// ListEventsRangeIn returns events overlapping [start, end) from the given
// calendar, tagging each returned event with that calendar's id and summary.
func (c *Client) ListEventsRangeIn(ctx context.Context, calID string, start, end time.Time) ([]domain.Event, error) {
	return c.listEventsIn(ctx, calID, calID, start, end)
}

// listEventsIn is the shared implementation: it queries events and attaches
// the owning calendar's id to each result.
func (c *Client) listEventsIn(ctx context.Context, calID, summary string, start, end time.Time) ([]domain.Event, error) {
	evs, err := c.svc.Events.List(calID).
		TimeMin(start.Format(time.RFC3339)).
		TimeMax(end.Format(time.RFC3339)).
		SingleEvents(true).
		OrderBy("startTime").
		Context(ctx).
		Do()
	if err != nil {
		return nil, err
	}
	out := make([]domain.Event, 0, len(evs.Items))
	for _, e := range evs.Items {
		ev := fromAPIEvent(e)
		if ev == nil {
			continue
		}
		ev.CalendarID = calID
		ev.CalendarSummary = summary
		out = append(out, *ev)
	}
	return out, nil
}

// ListUpcoming returns events from now until `days` days ahead.
func (c *Client) ListUpcoming(ctx context.Context, days int) ([]domain.Event, error) {
	now := time.Now()
	end := domain.StartOfDay(now).AddDate(0, 0, days+1)

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
	out := make([]domain.Event, 0, len(evs.Items))
	for _, e := range evs.Items {
		ev := fromAPIEvent(e)
		if ev != nil {
			out = append(out, *ev)
		}
	}
	return out, nil
}

// CreateEvent inserts a new event into the client's configured calendar.
func (c *Client) CreateEvent(ctx context.Context, e *domain.Event) (*domain.Event, error) {
	return c.CreateEventIn(ctx, c.calID, e)
}

// CreateEventIn inserts a new event into the given calendar.
func (c *Client) CreateEventIn(ctx context.Context, calID string, e *domain.Event) (*domain.Event, error) {
	api := toAPIEvent(e)
	res, err := c.svc.Events.Insert(calID, api).Context(ctx).Do()
	if err != nil {
		return nil, err
	}
	return fromAPIEvent(res), nil
}

// UpdateEvent replaces an existing event identified by e.Id in the client's
// configured calendar.
func (c *Client) UpdateEvent(ctx context.Context, e *domain.Event) (*domain.Event, error) {
	return c.UpdateEventIn(ctx, c.calID, e)
}

// UpdateEventIn replaces an existing event identified by e.Id in the given
// calendar.
func (c *Client) UpdateEventIn(ctx context.Context, calID string, e *domain.Event) (*domain.Event, error) {
	if e.ID == "" {
		return nil, errors.New("event has no id")
	}
	api := toAPIEvent(e)
	res, err := c.svc.Events.Update(calID, e.ID, api).Context(ctx).Do()
	if err != nil {
		return nil, err
	}
	return fromAPIEvent(res), nil
}

// DeleteEvent removes an event by id from the client's configured calendar.
func (c *Client) DeleteEvent(ctx context.Context, id string) error {
	return c.DeleteEventIn(ctx, c.calID, id)
}

// DeleteEventIn removes an event by id from the given calendar.
func (c *Client) DeleteEventIn(ctx context.Context, calID, id string) error {
	return c.svc.Events.Delete(calID, id).Context(ctx).Do()
}

// ---- domain <-> API mapping ----

// toAPIEvent converts a domain event to the Google Calendar API shape. All-day
// events carry date-only fields; timed events prefer a naive dateTime plus an
// explicit IANA timeZone (which Google accepts) and fall back to RFC3339 (with
// an embedded offset) when no timezone is known.
func toAPIEvent(e *domain.Event) *calendar.Event {
	if e == nil {
		return nil
	}
	out := &calendar.Event{
		Id:               e.ID,
		Summary:          e.Summary,
		Location:         e.Location,
		Description:      e.Description,
		RecurringEventId: e.RecurringEventID,
	}
	if e.AllDay {
		out.Start = &calendar.EventDateTime{Date: e.Start.Format("2006-01-02"), TimeZone: e.TimeZone}
		out.End = &calendar.EventDateTime{Date: e.EndTime().Format("2006-01-02"), TimeZone: e.TimeZone}
		return out
	}
	layout := "2006-01-02T15:04:05" // timezone carried by the timeZone field
	if e.TimeZone == "" {
		layout = time.RFC3339 // no timeZone: embed the offset in dateTime
	}
	out.Start = &calendar.EventDateTime{DateTime: e.Start.Format(layout), TimeZone: e.TimeZone}
	out.End = &calendar.EventDateTime{DateTime: e.EndTime().Format(layout), TimeZone: e.TimeZone}
	return out
}

// fromAPIEvent converts a Google Calendar API event to a domain event, parsing
// times into concrete time.Time values.
func fromAPIEvent(e *calendar.Event) *domain.Event {
	if e == nil {
		return nil
	}
	out := &domain.Event{
		ID:               e.Id,
		Summary:          e.Summary,
		Location:         e.Location,
		Description:      e.Description,
		RecurringEventID: e.RecurringEventId,
	}
	if e.Start != nil {
		out.TimeZone = e.Start.TimeZone
		if e.Start.Date != "" {
			if t, err := time.Parse("2006-01-02", e.Start.Date); err == nil {
				out.Start = t
				out.AllDay = true
			}
		} else if e.Start.DateTime != "" {
			if t, ok := parseAPIDateTime(e.Start.DateTime, e.Start.TimeZone); ok {
				out.Start = t
			}
		}
	}
	if e.End != nil {
		if e.End.Date != "" {
			if t, err := time.Parse("2006-01-02", e.End.Date); err == nil {
				out.End = t
			}
		} else if e.End.DateTime != "" {
			if t, ok := parseAPIDateTime(e.End.DateTime, e.End.TimeZone); ok {
				out.End = t
			}
		}
	}
	if out.End.IsZero() {
		// All-day events from Google often omit an explicit end.
		out.End = out.Start.AddDate(0, 0, 1)
		if !out.AllDay {
			out.End = out.Start
		}
	}
	return out
}

// parseAPIDateTime parses a Google EventDateTime value. When a timeZone is
// given the dateTime is treated as wall-clock time in that zone (Google omits
// the offset and sends it as a separate field); otherwise the dateTime must
// carry its own offset (RFC3339).
func parseAPIDateTime(v, zone string) (time.Time, bool) {
	if zone != "" {
		if loc, err := time.LoadLocation(zone); err == nil {
			if t, err := time.ParseInLocation("2006-01-02T15:04:05", v, loc); err == nil {
				return t, true
			}
		}
	}
	if t, err := time.Parse(time.RFC3339, v); err == nil {
		return t, true
	}
	return time.Time{}, false
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
// authorization code. loginHint pre-selects a Google account when given.
func authFlow(ctx context.Context, cfg *oauth2.Config, port int, loginHint string) (*oauth2.Token, error) {
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

	authOptions := []oauth2.AuthCodeOption{oauth2.AccessTypeOffline}
	if loginHint != "" {
		// Pre-select the account chosen in the terminal.
		authOptions = append(authOptions, oauth2.SetAuthURLParam("login_hint", loginHint))
	} else {
		// No account chosen: force the account chooser so the user can pick
		// the browser profile that owns the calendar instead of whatever
		// account happens to be logged in.
		authOptions = append(authOptions, oauth2.SetAuthURLParam("prompt", "select_account"))
	}
	authURL := cfg.AuthCodeURL("state-token", authOptions...)

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
		// Google redirects here with error=... when consent fails
		// (e.g. the account is not a test user of the OAuth app).
		if e := r.URL.Query().Get("error"); e != "" {
			desc := r.URL.Query().Get("error_description")
			msg := fmt.Sprintf("authorization denied: %s", e)
			if desc != "" {
				msg += ": " + desc
			}
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprintf(w, "<html><body style='font-family:sans-serif;display:flex;flex-direction:column;align-items:center;justify-content:center;height:90vh'><h2 style='color:#d33'>Authorization failed</h2><p>%s</p><p style='color:#888'>You can go back to the terminal now.</p></body></html>", htmlEscape(msg))
			errCh <- errors.New(msg)
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
		fmt.Fprint(w, "<html><body style='font-family:sans-serif;display:flex;align-items:center;justify-content:center;height:90vh'>"+
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

func htmlEscape(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '<':
			b.WriteString("&lt;")
		case '>':
			b.WriteString("&gt;")
		case '&':
			b.WriteString("&amp;")
		case '"':
			b.WriteString("&quot;")
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}
