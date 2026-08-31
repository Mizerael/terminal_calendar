# AGENTS.md

Guide for AI agents working in this repository.

## Project

A bubbletea (Go) terminal UI for Google Calendar: a week grid, event details,
form-based create/edit, delete confirmation, and a multi-calendar picker.

## Architecture: Clean Architecture

The code follows Robert Martin's Clean Architecture, layered from the inside out:

- **`internal/domain`** — framework-independent entities and pure business
  rules. No imports beyond the stdlib.
  - `event.go`: `Calendar`, `Event` (with concrete `time.Time` `Start`/`End`,
    `AllDay bool` field, `TimeZone string`), methods `StartTime()`,
    `EndTime()`, `Timezone()`.
  - `week.go`: pure helpers `MondayOf`, `StartOfDay`, `ClampHour`, `DayIndex`,
    `HourRange`.
- **`internal/usecase`** — application-specific rules; depends only on
  `domain`. Defines the **port** `CalendarGateway` and `CalendarService`
  (load/merge/CUD) plus `Selection` (`Enabled` map + `Target`) and
  `Reconcile`/`PrimaryCalID` policy.
- **`internal/gcal`** — the **gateway adapter** that implements
  `usecase.CalendarGateway` against the Google Calendar API. Owns OAuth,
  account/token handling, and the `toAPIEvent`/`fromAPIEvent` mapping between
  `calendar.Event` and `domain.Event`. Keeps timezone fidelity on round-trip.
- **`internal/tui`** — the controller/presenter (bubbletea `Model`). Holds
  `svc *usecase.CalendarService`, `sel usecase.Selection`, UI state, and
  rendering. Never talks to the Google API directly.

Layer dependencies must point inward: `tui` → `usecase` → `domain`, and
`gcal` → `usecase` → `domain`. `domain` must stay leaf-empty of everything.

## Important invariants

- **Module path**: `github.com/Mizerael/terminal_calendar` (it was temporarily
  `gitnub.com/...` due to a typo). Verify every `go` import uses `github.com`;
  typos like `gitnub.com` or `nginx.com/placeholder/...` keep resurfacing — an
  LSP/build error mentioning a wrong host is the signal.
- Week starts **Monday**; `weekStart` = Monday 00:00 local; week spans
  `[weekStart, weekStart+7d)`.
- Cursor is composite `(dayIndex, eventIndex)`; `eventIndex == -1` when the
  focused day has no events.
- `domain.Event.ID` (not `Id`); `AllDay` is a **field**, not a method; no
  `Etag`; no JSON-tagged `EDT` (that old API is gone).
- Timezone round-trip (`gcal`): keep `Start.TimeZone`; when set, emit a naive
  `"2006-01-02T15:04:05"` dateTime + `timeZone` field; else RFC3339. Parse via
  `parseAPIDateTime` (naive-in-zone or RFC3339). Never invent a zone from
  `time.Local`.
- OAuth env vars: `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`,
  `GOOGLE_REDIRECT_URI`, `GOOGLE_TOKEN`, `GOOGLE_ACCOUNT`,
  `GOOGLE_CALENDAR_ID`; `.env` auto-loaded; token in `token.json`.
- Persisted selection lives in `calendar_state.json`; `m.sel.Enabled` /
  `m.sel.Target` and `usecase.Selection.Reconcile` drive reconciliation.
- Compile-time port assertion in `main.go`:
  `var _ usecase.CalendarGateway = (*gcal.Client)(nil)`.

## Commands

- `make fmt` — gofmt -w .
- `make vet` — go vet ./...
- `make test` — go test ./...
- `make build` — go build -o build/terminal_calendar .

Run all four after any change. Keep code gofmt-clean (no trailing spaces/tabs).

## Testing conventions

- Pure `domain` logic is tested in `internal/domain/week_test.go`.
- `usecase` selection policy in `internal/usecase/selection_test.go`.
- UI tests use a `fakeAPI` stub implementing `usecase.CalendarGateway` with
  `domain.Event`/`domain.Calendar`, wrapped via
  `usecase.NewCalendarService(fake)`. Helpers: `newTestModel`, `load`,
  `run`, `upd`, `makeEvent`, `eventAt`, `alldayOn` (in `model_test.go`).
- Grid/render tests must strip ANSI/terminal color before asserting layout
  (lipgloss injects escape codes; compare via `lipgloss.Width`, not raw len).

## Workflow / Git

- Do not create new features or changes directly on `main`. `main` only
  receives merged pull requests.
- Work is done on feature branches and merged into `main` via pull requests.
  Feature branch names start with `feature/` (e.g. `feature/quick-add`).

## Commit messages

Follow the repo convention of an `[ai]` tag prefix, e.g.
`fix[ai]:`, `feat[ai]:`, `refactor[ai]:`.
