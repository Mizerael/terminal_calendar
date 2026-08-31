# terminal_calendar

A bubbletea TUI for working with Google Calendar events from the terminal.

![bubbletea](https://github.com/charmbracelet/bubbletea)

## Features

- Day-by-day navigation (`←`/`→` or `h`/`l`)
- Event list with a detail pane
- Create, edit and delete events
- Full-day and timed events (auto-detected via `Start date`/`Start time`)
- OAuth2 login with the browser flow and token caching
- **Multi-calendar**: events from all enabled calendars shown merged and color-coded;
  a calendar picker (`c`) toggles which calendars are shown and picks the
  create-target; the selection persists between runs

## Setup

1. Create a Google Cloud project and enable the **Google Calendar API**.
2. Create an **OAuth client ID** of type *Desktop app*; note the client id and secret.
3. Export them along with your calendar id — or copy the template and fill it in:

```sh
cp .env.example .env   # then edit .env with your values
```

The app automatically loads `.env` from the current directory (real env vars
take precedence). `.env` is git-ignored.

4. Build and run:

```sh
go build -o terminal_calendar .
./terminal_calendar
```

On first run your browser opens with a Google consent screen; the refresh token
is stored in `token.json` (git-ignored) so you only authenticate once.

## Usage

The main view is a **Google-Calendar-style week grid**: seven day columns
(Monday → Sunday) with an hour gutter on the left. Events occupy their hour
spans in each column; all-day events sit in a banner above the grid; overlapping
events show a `Nx` clash marker. The grid scrolls vertically through the day's
hours (starting at 06:00) and the popup shows the full event details.

| Key        | Action                    |
|------------|---------------------------|
| `j`/`k`, `↑`/`↓` | move between events |
| `h`/`l`, `←`/`→` | move between days (lands near the same time-of-day) |
| `ctrl+u` / `ctrl+d` | scroll the hour rows up / down |
| `[` / `]`  | previous / next week      |
| `t`        | jump to the current week  |
| `enter`    | open event detail (modal popup; `esc`/`enter` closes, `e` edit, `d` delete) |
| `e`        | edit the focused event    |
| `n`        | new event (start date pre-filled with the focused day) |
| `d`        | delete event (confirm)    |
| `r`        | refresh                   |
| `g`/`G`    | first / last event of the week |
| `c`        | calendar picker: `enter` toggles a calendar on/off, `t` marks the new-event target |
| `?`        | help                      |
| `q`/`ctrl+c` | quit                    |

In the event form, leave both *Start time* and *End time* empty to create an
all-day event. New events are created in the calendar marked `→` in the picker
(by default your primary calendar, or `GOOGLE_CALENDAR_ID`); editing/deleting an
event always acts on the calendar that owns it.

## Environment variables

| Variable               | Purpose                      | Default                 |
|------------------------|------------------------------|-------------------------|
| `GOOGLE_CLIENT_ID`     | OAuth client id              | *(required)*            |
| `GOOGLE_CLIENT_SECRET` | OAuth client secret          | *(required)*            |
| `GOOGLE_CALENDAR_ID`   | default new-event target calendar id | `primary`          |
| `GOOGLE_REDIRECT_URI`  | OAuth redirect URI           | `http://localhost:8765` |
| `GOOGLE_ACCOUNT`       | pre-select Google account (email); otherwise you are prompted in the terminal | — |
| `GOOGLE_TOKEN`         | token cache path             | `token.json`            |
| `PORT`                 | OAuth loopback port          | `8765`                  |

## Flags

```
-calendar <id>    override GOOGLE_CALENDAR_ID
-account <email>  override GOOGLE_ACCOUNT (pre-selects the account to authorize)
-token <path>     override GOOGLE_TOKEN
-port <n>         override PORT
```

## Account selection

When authorization is needed and `GOOGLE_ACCOUNT` is not set, a prompt appears
in the terminal asking which Google account to authorize with. The chosen email
is passed to Google as a login hint and remembered for later runs (sidecar
`token.json.account`). Press `esc` to skip and use the browser's account
chooser instead.

## Layout

- `main.go` — entry point and flags
- `internal/gcal` — Google Calendar API client, OAuth flow, token caching
- `internal/tui` — the bubbletea model, list/detail views and the event form

## Tests

```sh
go test ./...
```