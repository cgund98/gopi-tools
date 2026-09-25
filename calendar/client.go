package calendar

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

// googleCalendarClient wraps the official Google Calendar API client.
type googleCalendarClient struct {
	srv *calendar.Service
}

// newGoogleClient builds a Google Calendar client using OAuth2 with auto-refresh.
// It loads the stored token (which includes a refresh_token) and uses an
// oauth2.TokenSource to automatically obtain new access tokens when they expire.
func newGoogleClient(cfg CalendarConfig) (*googleCalendarClient, error) {
	if len(cfg.CredentialsJSON) == 0 && (len(cfg.ClientID) == 0 || len(cfg.ClientSecret) == 0) {
		return nil, fmt.Errorf("Either CredentialsJSON or both ClientID and ClientSecret must be provided")
	}
	if len(cfg.TokenJSON) == 0 {
		return nil, fmt.Errorf("TokenJSON is required; run 'go run ./cmd/gcal-auth' to generate it")
	}

	ctx := context.Background()

	// Build OAuth2 config from credentials JSON or explicit ClientID/ClientSecret.
	var oauthCfg *oauth2.Config
	if len(cfg.CredentialsJSON) > 0 {
		var err error
		oauthCfg, err = google.ConfigFromJSON(cfg.CredentialsJSON, calendar.CalendarScope)
		if err != nil {
			return nil, fmt.Errorf("parse credentials JSON: %w", err)
		}
	} else {
		oauthCfg = &oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			Scopes:       []string{calendar.CalendarScope},
			Endpoint:     google.Endpoint,
		}
	}

	// Load the stored token (contains refresh_token).
	var token oauth2.Token
	if err := json.Unmarshal(cfg.TokenJSON, &token); err != nil {
		return nil, fmt.Errorf("parse token JSON: %w", err)
	}

	// Create a token source that auto-refreshes using the refresh_token.
	ts := oauthCfg.TokenSource(ctx, &token)
	client := oauth2.NewClient(ctx, ts)

	srv, err := calendar.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("create calendar service: %w", err)
	}
	return &googleCalendarClient{srv: srv}, nil
}

func (c *googleCalendarClient) listEvents(ctx context.Context, calendarID string, minTime, maxTime time.Time, maxResults int64) ([]eventSummary, error) {
	call := c.srv.Events.List(calendarID).Context(ctx).MaxResults(maxResults).OrderBy("startTime").SingleEvents(true)
	if !minTime.IsZero() {
		call = call.TimeMin(minTime.Format(time.RFC3339))
	}
	if !maxTime.IsZero() {
		call = call.TimeMax(maxTime.Format(time.RFC3339))
	}
	resp, err := call.Do()
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}
	var out []eventSummary
	for _, item := range resp.Items {
		out = append(out, eventSummary{
			ID:        item.Id,
			Summary:   item.Summary,
			StartTime: item.Start.DateTime,
			EndTime:   item.End.DateTime,
		})
	}
	return out, nil
}

func (c *googleCalendarClient) getEvent(ctx context.Context, calendarID, eventID string) (*eventDetail, error) {
	evt, err := c.srv.Events.Get(calendarID, eventID).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("get event: %w", err)
	}
	return mapEvent(evt), nil
}

func (c *googleCalendarClient) createEvent(ctx context.Context, calendarID string, evt eventDetail) (*eventDetail, error) {
	created, err := c.srv.Events.Insert(calendarID, mapToGEvent(evt)).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("create event: %w", err)
	}
	return mapEvent(created), nil
}

func (c *googleCalendarClient) updateEvent(ctx context.Context, calendarID, eventID string, evt eventDetail) (*eventDetail, error) {
	updated, err := c.srv.Events.Patch(calendarID, eventID, mapToGEvent(evt)).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("update event: %w", err)
	}
	return mapEvent(updated), nil
}

func (c *googleCalendarClient) deleteEvent(ctx context.Context, calendarID, eventID string) error {
	if err := c.srv.Events.Delete(calendarID, eventID).Context(ctx).Do(); err != nil {
		return fmt.Errorf("delete event: %w", err)
	}
	return nil
}

func mapEvent(evt *calendar.Event) *eventDetail {
	return &eventDetail{
		ID:          evt.Id,
		Summary:     evt.Summary,
		Description: evt.Description,
		Location:    evt.Location,
		StartTime:   evt.Start.DateTime,
		EndTime:     evt.End.DateTime,
		HTMLLink:    evt.HtmlLink,
	}
}

// mapToGEvent leaves empty fields unset so a patch keeps their current values.
func mapToGEvent(evt eventDetail) *calendar.Event {
	out := &calendar.Event{
		Summary:     evt.Summary,
		Description: evt.Description,
		Location:    evt.Location,
	}
	if evt.StartTime != "" {
		out.Start = &calendar.EventDateTime{DateTime: evt.StartTime}
	}
	if evt.EndTime != "" {
		out.End = &calendar.EventDateTime{DateTime: evt.EndTime}
	}
	return out
}
