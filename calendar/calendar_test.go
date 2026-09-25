package calendar

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/cgund98/gogent"
)

// mockCalendarClient implements calendarClient for testing.
type mockCalendarClient struct {
	listEventsFunc  func(ctx context.Context, calendarID string, minTime, maxTime time.Time, maxResults int64) ([]eventSummary, error)
	getEventFunc    func(ctx context.Context, calendarID, eventID string) (*eventDetail, error)
	createEventFunc func(ctx context.Context, calendarID string, evt eventDetail) (*eventDetail, error)
	updateEventFunc func(ctx context.Context, calendarID, eventID string, evt eventDetail) (*eventDetail, error)
	deleteEventFunc func(ctx context.Context, calendarID, eventID string) error
}

func (m *mockCalendarClient) listEvents(ctx context.Context, calendarID string, minTime, maxTime time.Time, maxResults int64) ([]eventSummary, error) {
	if m.listEventsFunc != nil {
		return m.listEventsFunc(ctx, calendarID, minTime, maxTime, maxResults)
	}
	return nil, errors.New("listEvents not implemented")
}

func (m *mockCalendarClient) getEvent(ctx context.Context, calendarID, eventID string) (*eventDetail, error) {
	if m.getEventFunc != nil {
		return m.getEventFunc(ctx, calendarID, eventID)
	}
	return nil, errors.New("getEvent not implemented")
}

func (m *mockCalendarClient) createEvent(ctx context.Context, calendarID string, evt eventDetail) (*eventDetail, error) {
	if m.createEventFunc != nil {
		return m.createEventFunc(ctx, calendarID, evt)
	}
	return nil, errors.New("createEvent not implemented")
}

func (m *mockCalendarClient) updateEvent(ctx context.Context, calendarID, eventID string, evt eventDetail) (*eventDetail, error) {
	if m.updateEventFunc != nil {
		return m.updateEventFunc(ctx, calendarID, eventID, evt)
	}
	return nil, errors.New("updateEvent not implemented")
}

func (m *mockCalendarClient) deleteEvent(ctx context.Context, calendarID, eventID string) error {
	if m.deleteEventFunc != nil {
		return m.deleteEventFunc(ctx, calendarID, eventID)
	}
	return errors.New("deleteEvent not implemented")
}

func newTestTool(client calendarClient) *CalendarTool {
	t := &CalendarTool{
		cfg: CalendarConfig{CalendarID: "primary"},
	}
	if client != nil {
		t.client = client
	}
	return t
}

func TestCalendarTool_Name(t *testing.T) {
	tool := newTestTool(nil)
	if got := tool.Name(); got != "google_calendar" {
		t.Fatalf("Name() = %q, want %q", got, "google_calendar")
	}
}

func TestCalendarTool_Parameters(t *testing.T) {
	tool := newTestTool(nil)
	params := tool.Parameters()
	if len(params) == 0 {
		t.Fatal("Parameters() returned empty schema")
	}
	var schema map[string]any
	if err := json.Unmarshal(params, &schema); err != nil {
		t.Fatalf("Parameters() is not valid JSON: %v", err)
	}
	if schema["type"] != "object" {
		t.Fatalf("schema type = %q, want object", schema["type"])
	}
}

func TestCalendarTool_RequiresApproval(t *testing.T) {
	tests := []struct {
		name         string
		args         string
		wantRequired bool
		wantReason   string
	}{
		{
			name:         "list_events auto-approves",
			args:         `{"operation":"list_events"}`,
			wantRequired: false,
		},
		{
			name:         "get_event auto-approves",
			args:         `{"operation":"get_event","event_id":"123"}`,
			wantRequired: false,
		},
		{
			name:         "create_event requires approval",
			args:         `{"operation":"create_event","summary":"Meeting","start_time":"2024-01-15T09:00:00Z","end_time":"2024-01-15T10:00:00Z"}`,
			wantRequired: true,
			wantReason:   "Calendar write operation: create_event",
		},
		{
			name:         "update_event requires approval",
			args:         `{"operation":"update_event","event_id":"123"}`,
			wantRequired: true,
			wantReason:   "Calendar write operation: update_event",
		},
		{
			name:         "delete_event requires approval",
			args:         `{"operation":"delete_event","event_id":"123"}`,
			wantRequired: true,
			wantReason:   "Calendar write operation: delete_event",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tool := newTestTool(nil)
			decision, err := tool.RequiresApproval(context.Background(), json.RawMessage(tt.args))
			if err != nil {
				t.Fatalf("RequiresApproval error: %v", err)
			}
			if decision.Required != tt.wantRequired {
				t.Fatalf("Required = %v, want %v", decision.Required, tt.wantRequired)
			}
			if decision.Required && decision.Reason != tt.wantReason {
				t.Fatalf("Reason = %q, want %q", decision.Reason, tt.wantReason)
			}
		})
	}
}

func TestCalendarTool_Execute_ListEvents(t *testing.T) {
	mock := &mockCalendarClient{
		listEventsFunc: func(ctx context.Context, calendarID string, minTime, maxTime time.Time, maxResults int64) ([]eventSummary, error) {
			if calendarID != "primary" {
				t.Fatalf("calendarID = %q, want primary", calendarID)
			}
			return []eventSummary{
				{ID: "evt1", Summary: "Standup", StartTime: "2024-01-15T09:00:00Z", EndTime: "2024-01-15T09:30:00Z"},
			}, nil
		},
	}
	tool := newTestTool(mock)
	result, err := tool.Execute(context.Background(), json.RawMessage(`{"operation":"list_events"}`))
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(result, &payload); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}
	if payload["error"] != nil {
		t.Fatalf("unexpected error payload: %v", payload)
	}
	events, ok := payload["events"].([]any)
	if !ok || len(events) != 1 {
		t.Fatalf("events = %v, want 1 event", payload["events"])
	}
}

func TestCalendarTool_Execute_ListEvents_ClientError(t *testing.T) {
	mock := &mockCalendarClient{
		listEventsFunc: func(ctx context.Context, calendarID string, minTime, maxTime time.Time, maxResults int64) ([]eventSummary, error) {
			return nil, errors.New("api error")
		},
	}
	tool := newTestTool(mock)
	result, err := tool.Execute(context.Background(), json.RawMessage(`{"operation":"list_events"}`))
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(result, &payload); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}
	if payload["error"] != "list_failed" {
		t.Fatalf("error = %q, want list_failed", payload["error"])
	}
}

func TestCalendarTool_Execute_CreateEvent(t *testing.T) {
	mock := &mockCalendarClient{
		createEventFunc: func(ctx context.Context, calendarID string, evt eventDetail) (*eventDetail, error) {
			return &eventDetail{ID: "new123", Summary: evt.Summary, StartTime: evt.StartTime, EndTime: evt.EndTime}, nil
		},
	}
	tool := newTestTool(mock)
	args := `{"operation":"create_event","summary":"Team Meeting","start_time":"2024-01-15T09:00:00Z","end_time":"2024-01-15T10:00:00Z"}`
	result, err := tool.Execute(context.Background(), json.RawMessage(args))
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(result, &payload); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}
	if payload["error"] != nil {
		t.Fatalf("unexpected error payload: %v", payload)
	}
	evt, ok := payload["event"].(map[string]any)
	if !ok || evt["id"] != "new123" {
		t.Fatalf("event = %v, want id=new123", payload["event"])
	}
}

func TestCalendarTool_Execute_CreateEvent_MissingArgs(t *testing.T) {
	// Use a mock client so validation runs before client setup is attempted.
	mock := &mockCalendarClient{}
	tool := newTestTool(mock)
	result, err := tool.Execute(context.Background(), json.RawMessage(`{"operation":"create_event","summary":"Only Title"}`))
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(result, &payload); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}
	if payload["error"] != "missing_argument" {
		t.Fatalf("error = %q, want missing_argument", payload["error"])
	}
}

func TestCalendarTool_Execute_InvalidOperation(t *testing.T) {
	tool := newTestTool(nil)
	result, err := tool.Execute(context.Background(), json.RawMessage(`{"operation":"magic"}`))
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(result, &payload); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}
	if payload["error"] != "invalid_operation" {
		t.Fatalf("error = %q, want invalid_operation", payload["error"])
	}
}

func TestCalendarTool_Execute_DeleteEvent(t *testing.T) {
	mock := &mockCalendarClient{
		deleteEventFunc: func(ctx context.Context, calendarID, eventID string) error {
			return nil
		},
	}
	tool := newTestTool(mock)
	result, err := tool.Execute(context.Background(), json.RawMessage(`{"operation":"delete_event","event_id":"del123"}`))
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(result, &payload); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}
	if payload["error"] != nil {
		t.Fatalf("unexpected error payload: %v", payload)
	}
	if payload["success"] != true {
		t.Fatalf("success = %v, want true", payload["success"])
	}
}

func TestCalendarTool_Interface(t *testing.T) {
	var _ gogent.Tool = (*CalendarTool)(nil)
}
