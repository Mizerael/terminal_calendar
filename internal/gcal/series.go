package gcal

import (
	"context"
	"errors"

	"github.com/Mizerael/terminal_calendar/internal/domain"
	"google.golang.org/api/calendar/v3"
)

func (c *Client) CreateSeries(ctx context.Context, calID string, e *domain.Event) (*domain.Event, error) {
	api := toAPIEvent(e)
	res, err := c.svc.Events.Insert(calID, api).Context(ctx).Do()
	if err != nil {
		return nil, err
	}
	return fromAPIEvent(res), nil
}

func (c *Client) UpdateSeries(ctx context.Context, calID string, e *domain.Event) (*domain.Event, error) {
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

func (c *Client) DeleteSeries(ctx context.Context, calID, instanceID string) error {
	return c.svc.Events.Delete(calID, instanceID).Context(ctx).Do()
}

func (c *Client) UpsertInstance(ctx context.Context, calID string, e *domain.Event) (*domain.Event, error) {
	api := toAPIEvent(e)
	res, err := c.svc.Events.Insert(calID, api).Context(ctx).Do()
	if err != nil {
		return nil, err
	}
	return fromAPIEvent(res), nil
}

func (c *Client) CancelInstance(ctx context.Context, calID, instanceID string) error {
	_, err := c.svc.Events.Patch(calID, instanceID, &calendar.Event{Status: "cancelled"}).Context(ctx).Do()
	if err != nil {
		return err
	}
	return nil
}
