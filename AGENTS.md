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

- Исполнение задач из `README.md` TODO:
  - Координатор ведёт список по порядку и выдаёт каждую задачу субагенту на
    более слабой модели, если задача связана с написанием или изменением кода.
  - В запросе субагенту фиксируются цель, допустимые файлы/сценарии, ограничения
    архитектуры и ожидаемые проверки. Субагент не коммитит и не меняет файлы
    вне согласованного scope.
  - Кодовые задачи выполняются в отдельном git worktree от ветки координатора.
    Если worktree технически недоступен, субагент работает в копии проекта вне
    общей рабочей директории и передаёт diff; координатор применяет его после
    проверки. Не разрешается параллельно писать в одну рабочую директорию.
  - Координатор самостоятельно проверяет diff и результаты доступных проверок,
    учитывает существующие незакоммиченные изменения и не выдаёт результат за
    полностью проверенный, если сборка или тесты заблокированы.
  - Для завершённой задачи с изменениями кода координатор формирует локальную
    HTML-страницу через skill `change-review-page` и передаёт её вместе с
    кратким итогом и ограничениями проверки. Для задачи, меняющей только Markdown,
    HTML-страница не требуется.
  - Следующий пункт TODO берётся в работу после подготовки результата текущего
    пункта к проверке пользователем; пункты и их статус обновляются в README.

- Параллелизм задач и ожидание:
  - Задачи считаются независимыми, если выполнены три условия:
    1. они не изменяют одни и те же доменные сущности, use cases и
       контракты между ними (каждый субагент получает явный список
       сущностей/сценариев своей задачи);
    2. задача №+1 не опирается на контракт, который пишет занятый
       субагент, — используются уже зафиксированные интерфейсы;
    3. слияние ветки координатора не конфликтует с работающими ветками.
  - Независимые задачи запускаются параллельно; каждый субагент работает
    в своём git worktree, созданном от ветки координатора.
  - Следующий, зависимый этап запускается только после ревью и
    squash-merge готовых веток, от которых он зависит.
  - Общие точки композиции (app.py, pyproject.toml, uv.lock, conftest.py)
    субагенты не меняют — их заранее готовит координатор. Если общий
    файл избежать нельзя — задачи сериализуются.
  - Ревью и merge готовой ветки можно проводить параллельно с работой
    других субагентов, если выполняется условие 3; иначе merge ставится
    в очередь.
  - Финальная валидация (ruff/mypy/pytest по всему репозиторию, MR в
    `main`) выполняется только когда слиты все ветки этапов.
- Ветки и merge-requests:
  - `main` — только через MR. Прямые коммиты в `main` запрещены.
  - Иерархия веток:
    - `feature/<название фичи>` — интеграционная ветка фичи, которую курирует
      координирующий агент; ведёт её только координатор.
    - `feature/stage-<название этапа>` — ветка этапа, которую реализует
      субагент; создаётся от текущей `feature/<фича>`.
  - Перед подготовкой MR субагент обязан убедиться, что ветка координатора
    (`feature/<фича>`) актуальна относительно его ветки: сделать rebase
    (или мерж ветки координатора в свою) и перегнать тесты; при конфликтах —
    решить их в своей ветке, а не в MR после ревью.
  - MR субагентов (`feature/stage-*` → `feature/<фича>`) ревьюит и сливает
    координатор; итоговый MR (`feature/<фича>` → `main`) ревьюит человек.
  - Слияние — squash: один коммит на MR, с префиксом из раздела
    Commit messages.

## Commit messages

Follow the repo convention of an `[ai]` tag prefix, e.g.
`fix[ai]:`, `feat[ai]:`, `refactor[ai]:`.
