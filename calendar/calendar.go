package calendar

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/cgund98/gogent"
	"github.com/invopop/jsonschema"
)

// calendarArgs defines the parameters the model passes to the tool.
// The jsonschema tag splits on commas, so descriptions go in jsonschema_description.
type calendarArgs struct {
	Operation   string `json:"operation" jsonschema:"enum=list_events,enum=get_event,enum=create_event,enum=update_event,enum=delete_event" jsonschema_description:"The operation to perform: list_events, get_event, create_event, update_event, or delete_event."`
	EventID     string `json:"event_id,omitempty" jsonschema_description:"The Google Calendar event ID. Required for get_event, update_event, and delete_event."`
	Summary     string `json:"summary,omitempty" jsonschema_description:"Event title. Required for create_event."`
	Description string `json:"description,omitempty" jsonschema_description:"Event description. Optional."`
	Location    string `json:"location,omitempty" jsonschema_description:"Event location. Optional."`
	StartTime   string `json:"start_time,omitempty" jsonschema_description:"Start time in RFC3339 format with an offset (e.g. 2024-01-15T09:00:00-08:00). For list_events it is the earliest time; required for create_event."`
	EndTime     string `json:"end_time,omitempty" jsonschema_description:"End time in RFC3339 format. Required for create_event. Optional for list_events (the latest time)."`
	MaxResults  int64  `json:"max_results,omitempty" jsonschema_description:"Maximum number of events to return for list_events. Default 10."`
}

// Tool implements gogent.Tool for Google Calendar operations.
type Tool struct {
	cfg    Config
	client calendarClient

	mu sync.Mutex
	// current holds events looked up for an update or delete approval, by event ID,
	// so the approval prompt can show what will change.
	current map[string]eventDetail
}

// calendarClient is the interface the tool uses to talk to Google Calendar.
// It is implemented by *googleCalendarClient and can be mocked in tests.
type calendarClient interface {
	listEvents(ctx context.Context, calendarID string, minTime, maxTime time.Time, maxResults int64) ([]eventSummary, error)
	getEvent(ctx context.Context, calendarID, eventID string) (*eventDetail, error)
	createEvent(ctx context.Context, calendarID string, evt eventDetail) (*eventDetail, error)
	updateEvent(ctx context.Context, calendarID, eventID string, evt eventDetail) (*eventDetail, error)
	deleteEvent(ctx context.Context, calendarID, eventID string) error
}

// eventSummary is a lightweight representation returned by list_events.
type eventSummary struct {
	ID        string `json:"id"`
	Summary   string `json:"summary"`
	StartTime string `json:"start_time"`
	EndTime   string `json:"end_time"`
}

// eventDetail is the full representation of a calendar event.
type eventDetail struct {
	ID          string `json:"id,omitempty"`
	Summary     string `json:"summary"`
	Description string `json:"description,omitempty"`
	Location    string `json:"location,omitempty"`
	StartTime   string `json:"start_time"`
	EndTime     string `json:"end_time"`
	HTMLLink    string `json:"html_link,omitempty"`
}

// New creates a new Google Calendar tool.
// If cfg is incomplete or the client cannot be built, the tool will return
// errors at execution time rather than failing construction.
func New(cfg Config) gogent.Tool {
	return &Tool{cfg: cfg}
}

// Name returns the tool name.
func (t *Tool) Name() string { return "google_calendar" }

// Description returns the tool description for the model.
func (t *Tool) Description() string {
	return "Interact with Google Calendar. Supports listing events, reading an event, creating, updating, and deleting events. " +
		"Time values must be in RFC3339 format. Read operations (list_events, get_event) do not require approval. " +
		"Write operations (create_event, update_event, delete_event) require user approval."
}

// Parameters returns the JSON Schema for the tool arguments.
func (t *Tool) Parameters() json.RawMessage {
	reflector := jsonschema.Reflector{DoNotReference: true}
	body, err := json.Marshal(reflector.Reflect(new(calendarArgs)))
	if err != nil {
		panic(err)
	}
	return body
}

// RequiresApproval returns whether the operation needs human approval.
func (t *Tool) RequiresApproval(ctx context.Context, raw json.RawMessage) (gogent.ApprovalDecision, error) {
	var args calendarArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return gogent.ApprovalDecision{}, fmt.Errorf("parse arguments: %w", err)
	}
	switch args.Operation {
	case "delete_event", "update_event":
		reason := "Calendar write operation: " + args.Operation
		if event, ok := t.lookupCurrent(ctx, args.EventID); ok {
			verb := "Delete"
			if args.Operation == "update_event" {
				verb = "Update"
			}
			reason = fmt.Sprintf("%s %q on %s", verb, event.Summary, formatSpan(event.StartTime, event.EndTime))
		}
		return gogent.ApprovalDecision{Required: true, Reason: reason}, nil
	case "create_event":
		return gogent.ApprovalDecision{
			Required: true,
			Reason:   fmt.Sprintf("Calendar write operation: %s", args.Operation),
		}, nil
	default:
		return gogent.ApprovalDecision{}, nil
	}
}

// lookupCurrent fetches the event an update or delete names. A failed lookup
// leaves the prompt showing only the event ID and the requested values.
func (t *Tool) lookupCurrent(ctx context.Context, eventID string) (eventDetail, bool) {
	if eventID == "" {
		return eventDetail{}, false
	}
	client, err := t.ensureClient()
	if err != nil {
		return eventDetail{}, false
	}
	event, err := client.getEvent(ctx, t.cfg.CalendarID, eventID)
	if err != nil || event == nil {
		return eventDetail{}, false
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.current == nil {
		t.current = map[string]eventDetail{}
	}
	t.current[eventID] = *event
	return *event, true
}

func (t *Tool) currentEvent(eventID string) (eventDetail, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	event, ok := t.current[eventID]
	return event, ok
}

func (t *Tool) forgetCurrent(eventID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.current, eventID)
}

func (t *Tool) ensureClient() (calendarClient, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.client == nil {
		c, err := newGoogleClient(t.cfg)
		if err != nil {
			return nil, err
		}
		t.client = c
	}
	return t.client, nil
}

// Execute runs the requested calendar operation.
func (t *Tool) Execute(ctx context.Context, raw json.RawMessage) (json.RawMessage, error) {
	var args calendarArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, fmt.Errorf("parse arguments: %w", err)
	}

	switch args.Operation {
	case "list_events", "get_event", "create_event", "update_event", "delete_event":
		// valid
	default:
		return json.Marshal(map[string]any{
			"error":   "invalid_operation",
			"message": fmt.Sprintf("unknown operation %q", args.Operation),
		})
	}

	if _, err := t.ensureClient(); err != nil {
		return json.Marshal(map[string]any{
			"error":   "client_setup_failed",
			"message": err.Error(),
		})
	}

	switch args.Operation {
	case "list_events":
		return t.executeListEvents(ctx, args)
	case "get_event":
		return t.executeGetEvent(ctx, args)
	case "create_event":
		return t.executeCreateEvent(ctx, args)
	case "update_event":
		return t.executeUpdateEvent(ctx, args)
	case "delete_event":
		return t.executeDeleteEvent(ctx, args)
	}
	return nil, nil // unreachable
}

func (t *Tool) executeListEvents(ctx context.Context, args calendarArgs) (json.RawMessage, error) {
	var minTime, maxTime time.Time
	if args.StartTime != "" {
		var err error
		minTime, err = time.Parse(time.RFC3339, args.StartTime)
		if err != nil {
			return json.Marshal(map[string]any{
				"error":   "invalid_time",
				"message": fmt.Sprintf("start_time: %v", err),
			})
		}
	}
	if args.EndTime != "" {
		var err error
		maxTime, err = time.Parse(time.RFC3339, args.EndTime)
		if err != nil {
			return json.Marshal(map[string]any{
				"error":   "invalid_time",
				"message": fmt.Sprintf("end_time: %v", err),
			})
		}
	}
	if args.MaxResults <= 0 {
		args.MaxResults = 10
	}
	events, err := t.client.listEvents(ctx, t.cfg.CalendarID, minTime, maxTime, args.MaxResults)
	if err != nil {
		return json.Marshal(map[string]any{
			"error":   "list_failed",
			"message": err.Error(),
		})
	}
	return json.Marshal(map[string]any{
		"operation": "list_events",
		"events":    events,
	})
}

func (t *Tool) executeGetEvent(ctx context.Context, args calendarArgs) (json.RawMessage, error) {
	if args.EventID == "" {
		return json.Marshal(map[string]any{
			"error":   "missing_argument",
			"message": "event_id is required for get_event",
		})
	}
	evt, err := t.client.getEvent(ctx, t.cfg.CalendarID, args.EventID)
	if err != nil {
		return json.Marshal(map[string]any{
			"error":   "get_failed",
			"message": err.Error(),
		})
	}
	return json.Marshal(map[string]any{
		"operation": "get_event",
		"event":     evt,
	})
}

func (t *Tool) executeCreateEvent(ctx context.Context, args calendarArgs) (json.RawMessage, error) {
	if args.Summary == "" || args.StartTime == "" || args.EndTime == "" {
		return json.Marshal(map[string]any{
			"error":   "missing_argument",
			"message": "summary, start_time, and end_time are required for create_event",
		})
	}
	evt := eventDetail{
		Summary:     args.Summary,
		Description: args.Description,
		Location:    args.Location,
		StartTime:   args.StartTime,
		EndTime:     args.EndTime,
	}
	created, err := t.client.createEvent(ctx, t.cfg.CalendarID, evt)
	if err != nil {
		return json.Marshal(map[string]any{
			"error":   "create_failed",
			"message": err.Error(),
		})
	}
	return json.Marshal(map[string]any{
		"operation": "create_event",
		"event":     created,
	})
}

func (t *Tool) executeUpdateEvent(ctx context.Context, args calendarArgs) (json.RawMessage, error) {
	if args.EventID == "" {
		return json.Marshal(map[string]any{
			"error":   "missing_argument",
			"message": "event_id is required for update_event",
		})
	}
	evt := eventDetail{
		Summary:     args.Summary,
		Description: args.Description,
		Location:    args.Location,
	}
	if args.StartTime != "" {
		evt.StartTime = args.StartTime
	}
	if args.EndTime != "" {
		evt.EndTime = args.EndTime
	}
	updated, err := t.client.updateEvent(ctx, t.cfg.CalendarID, args.EventID, evt)
	if err != nil {
		return json.Marshal(map[string]any{
			"error":   "update_failed",
			"message": err.Error(),
		})
	}
	t.forgetCurrent(args.EventID)
	return json.Marshal(map[string]any{
		"operation": "update_event",
		"event":     updated,
	})
}

func (t *Tool) executeDeleteEvent(ctx context.Context, args calendarArgs) (json.RawMessage, error) {
	if args.EventID == "" {
		return json.Marshal(map[string]any{
			"error":   "missing_argument",
			"message": "event_id is required for delete_event",
		})
	}
	if err := t.client.deleteEvent(ctx, t.cfg.CalendarID, args.EventID); err != nil {
		return json.Marshal(map[string]any{
			"error":   "delete_failed",
			"message": err.Error(),
		})
	}
	t.forgetCurrent(args.EventID)
	return json.Marshal(map[string]any{
		"operation": "delete_event",
		"success":   true,
	})
}
