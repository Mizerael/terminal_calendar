// Package usecase contains the application-specific business rules of the
// calendar TUI: loading/merging events across calendars and reconciling the
// calendar selection. It depends only on domain, never on the UI or the Google
// API transport, following Clean Architecture's use-case layer.
package usecase

import (
	"context"
	"time"

	"github.com/Mizerael/terminal_calendar/internal/domain"
)

// CalendarGateway is the port the use cases need from an external calendar
// store. The gateway implementation (internal/gcal) adapts a Google Calendar
// API client to this interface. It is the boundary across which data flows
// into and out of this layer.
type CalendarGateway interface {
	ListCalendars(ctx context.Context) ([]domain.Calendar, error)
	ListEventsRangeIn(ctx context.Context, calID string, start, end time.Time) ([]domain.Event, error)
	CreateEventIn(ctx context.Context, calID string, e *domain.Event) (*domain.Event, error)
	UpdateEventIn(ctx context.Context, calID string, e *domain.Event) (*domain.Event, error)
	DeleteEventIn(ctx context.Context, calID, id string) error
}

// CalendarService implements the calendar use cases the UI drives: loading
// the calendar list, merging a week of events from enabled calendars, and
// routing create/update/delete to the calendar that owns each event.
type CalendarService struct {
	gw CalendarGateway
}

// NewCalendarService wires the service to a gateway.
func NewCalendarService(gw CalendarGateway) *CalendarService {
	return &CalendarService{gw: gw}
}

// LoadCalendars returns the calendars the user can see.
func (s *CalendarService) LoadCalendars(ctx context.Context) ([]domain.Calendar, error) {
	return s.gw.ListCalendars(ctx)
}

// LoadWeek fetches events for every enabled calendar overlapping
// [weekStart, weekStart+7d) and merges them into a single flat list. It stops
// at the first calendar that errors.
func (s *CalendarService) LoadWeek(ctx context.Context, weekStart time.Time, enabled map[string]bool) ([]domain.Event, error) {
	end := weekStart.Add(7 * 24 * time.Hour)
	var all []domain.Event
	for calID := range enabled {
		if !enabled[calID] {
			continue
		}
		items, err := s.gw.ListEventsRangeIn(ctx, calID, weekStart, end)
		if err != nil {
			return nil, err
		}
		all = append(all, items...)
	}
	return all, nil
}

// Create creates a new event in the given calendar.
func (s *CalendarService) Create(ctx context.Context, calID string, e *domain.Event) (*domain.Event, error) {
	return s.gw.CreateEventIn(ctx, calID, e)
}

// Update updates an existing event, routing to the calendar that owns it
// (falling back to fallback when the event has no owning calendar).
func (s *CalendarService) Update(ctx context.Context, fallback string, e *domain.Event) (*domain.Event, error) {
	calID := e.CalendarID
	if calID == "" {
		calID = fallback
	}
	return s.gw.UpdateEventIn(ctx, calID, e)
}

// Delete removes an event, routing to the calendar that owns it (falling back
// to fallback when the event has no owning calendar).
func (s *CalendarService) Delete(ctx context.Context, fallback string, e *domain.Event) error {
	calID := e.CalendarID
	if calID == "" {
		calID = fallback
	}
	return s.gw.DeleteEventIn(ctx, calID, e.ID)
}
