# terminal_calendar

A bubbletea TUI for working with Google Calendar events from the terminal.

![bubbletea](https://github.com/charmbracelet/bubbletea)

## Features

- Day-by-day navigation (`←`/`→` or `h`/`l`)
- Event list with a detail pane
- Create, edit and delete events
- Full-day and timed events (auto-detected via `Start date`/`Start time`)
- OAuth2 login with the browser flow and token caching

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

| Key        | Action                    |
|------------|---------------------------|
| `j`/`k`, `↑`/`↓` | move between events |
| `h`/`l`, `←`/`→` | previous / next day  |
| `t`        | jump to today             |
| `enter`/`e` | edit event               |
| `n`        | new event                 |
| `d`        | delete event (confirm)    |
| `r`        | refresh                   |
| `g`/`G`    | top / bottom              |
| `?`        | help                      |
| `q`/`ctrl+c` | quit                    |

In the event form, leave both *Start time* and *End time* empty to create an
all-day event.

## Environment variables

| Variable               | Purpose                      | Default                 |
|------------------------|------------------------------|-------------------------|
| `GOOGLE_CLIENT_ID`     | OAuth client id              | *(required)*            |
| `GOOGLE_CLIENT_SECRET` | OAuth client secret          | *(required)*            |
| `GOOGLE_CALENDAR_ID`   | calendar id to use           | `primary`               |
| `GOOGLE_REDIRECT_URI`  | OAuth redirect URI           | `http://localhost:8765` |
| `GOOGLE_TOKEN`         | token cache path             | `token.json`            |
| `PORT`                 | OAuth loopback port          | `8765`                  |

## Flags

```
-calendar <id>    override GOOGLE_CALENDAR_ID
-token <path>     override GOOGLE_TOKEN
-port <n>         override PORT
```

## Layout

- `main.go` — entry point and flags
- `internal/gcal` — Google Calendar API client, OAuth flow, token caching
- `internal/tui` — the bubbletea model, list/detail views and the event form

## Tests

```sh
go test ./...
```