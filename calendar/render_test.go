package calendar

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/cgund98/gopi"
)

func TestHeadlineNamesOperation(t *testing.T) {
	tool := &Tool{}
	cases := map[string]string{
		`{"operation":"list_events","start_time":"2026-09-25T00:00:00Z"}`: "calendar list " + time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC).Local().Format("2006-01-02"),
		`{"operation":"list_events"}`:                                     "calendar list",
		`{"operation":"create_event","summary":"Standup"}`:                "calendar create Standup",
		`{"operation":"update_event","event_id":"abc"}`:                   "calendar update abc",
		`{"operation":"delete_event","event_id":"abc"}`:                   "calendar delete abc",
		`{"operation":"unknown"}`:                                         "",
	}
	for args, want := range cases {
		if got := tool.Headline(json.RawMessage(args)); got != want {
			t.Errorf("Headline(%s) = %q, want %q", args, got, want)
		}
	}
}

func TestRenderApprovalShowsEventFields(t *testing.T) {
	view := (&Tool{}).RenderApproval(json.RawMessage(`{"operation":"create_event","summary":"Standup","start_time":"2026-09-25T16:00:00Z","end_time":"2026-09-25T16:30:00Z","location":"Room 1"}`))
	labels := fieldLabels(view)
	if labels != "Summary Start End Location" {
		t.Fatalf("labels = %q", labels)
	}
	if view.Fields[0].Value != "Standup" || view.Fields[3].Value != "Room 1" {
		t.Fatalf("fields = %+v", view.Fields)
	}
}

func TestDeleteApprovalNamesTheEvent(t *testing.T) {
	tool := newTestTool(&mockCalendarClient{
		getEventFunc: func(_ context.Context, _, eventID string) (*eventDetail, error) {
			if eventID != "evt1" {
				return nil, errors.New("not found")
			}
			return &eventDetail{ID: "evt1", Summary: "Gopi Test", StartTime: "2026-09-29T13:30:00-07:00", EndTime: "2026-09-29T14:00:00-07:00"}, nil
		},
	})
	args := json.RawMessage(`{"operation":"delete_event","event_id":"evt1"}`)
	decision, err := tool.RequiresApproval(context.Background(), args)
	if err != nil {
		t.Fatal(err)
	}
	when := formatSpan("2026-09-29T13:30:00-07:00", "2026-09-29T14:00:00-07:00")
	if !decision.Required || decision.Reason != `Delete "Gopi Test" on `+when {
		t.Fatalf("decision = %+v", decision)
	}
	view := tool.RenderApproval(args)
	if fieldLabels(view) != "Summary When" || view.Fields[0].Value != "Gopi Test" {
		t.Fatalf("approval = %+v", view)
	}

	missing := json.RawMessage(`{"operation":"delete_event","event_id":"gone"}`)
	decision, err = tool.RequiresApproval(context.Background(), missing)
	if err != nil {
		t.Fatal(err)
	}
	if decision.Reason != "Calendar write operation: delete_event" || fieldLabels(tool.RenderApproval(missing)) != "Event" {
		t.Fatalf("fallback decision = %+v", decision)
	}
}

func TestUpdateApprovalShowsChanges(t *testing.T) {
	tool := newTestTool(&mockCalendarClient{
		getEventFunc: func(context.Context, string, string) (*eventDetail, error) {
			return &eventDetail{ID: "evt1", Summary: "Gopi Test", Location: "Desk", StartTime: "2026-09-29T13:30:00-07:00", EndTime: "2026-09-29T14:00:00-07:00"}, nil
		},
	})
	args := json.RawMessage(`{"operation":"update_event","event_id":"evt1","summary":"Gopi Demo","start_time":"2026-09-29T15:00:00-07:00","end_time":"2026-09-29T15:30:00-07:00"}`)
	decision, err := tool.RequiresApproval(context.Background(), args)
	if err != nil {
		t.Fatal(err)
	}
	before := formatSpan("2026-09-29T13:30:00-07:00", "2026-09-29T14:00:00-07:00")
	after := formatSpan("2026-09-29T15:00:00-07:00", "2026-09-29T15:30:00-07:00")
	if !decision.Required || decision.Reason != `Update "Gopi Test" on `+before {
		t.Fatalf("decision = %+v", decision)
	}
	view := tool.RenderApproval(args)
	want := []gopi.ToolField{
		{Label: "Summary", Value: "Gopi Test → Gopi Demo"},
		{Label: "When", Value: before + " → " + after},
		{Label: "Location", Value: "Desk"},
	}
	if len(view.Fields) != len(want) {
		t.Fatalf("fields = %+v", view.Fields)
	}
	for i := range want {
		if view.Fields[i] != want[i] {
			t.Fatalf("field %d = %+v, want %+v", i, view.Fields[i], want[i])
		}
	}
}

func TestPatchLeavesEmptyFieldsUnset(t *testing.T) {
	event := mapToGEvent(eventDetail{Summary: "Renamed"})
	if event.Start != nil || event.End != nil || event.Location != "" {
		t.Fatalf("event = %+v", event)
	}
}

func TestRenderResultByOperation(t *testing.T) {
	tool := &Tool{}

	list := tool.RenderResult(nil, json.RawMessage(`{"operation":"list_events","events":[{"id":"1","summary":"Standup","start_time":"2026-09-25T16:00:00Z","end_time":"2026-09-25T16:30:00Z"}]}`))
	if len(list.Lines) != 1 || !strings.HasSuffix(list.Lines[0], "  Standup") || !strings.Contains(list.Lines[0], "–") {
		t.Fatalf("list = %+v", list)
	}
	if empty := tool.RenderResult(nil, json.RawMessage(`{"operation":"list_events","events":[]}`)); len(empty.Lines) != 1 || empty.Lines[0] != "No events" {
		t.Fatalf("empty list = %+v", empty)
	}

	created := tool.RenderResult(nil, json.RawMessage(`{"operation":"create_event","event":{"summary":"Standup","start_time":"2026-09-25T16:00:00Z","end_time":"2026-09-25T16:30:00Z","html_link":"https://calendar.google.com/x"}}`))
	if fieldLabels(created) != "Summary When Link" {
		t.Fatalf("created = %+v", created)
	}

	deleted := tool.RenderResult(nil, json.RawMessage(`{"operation":"delete_event","success":true}`))
	if len(deleted.Lines) != 1 || deleted.Lines[0] != "Deleted" {
		t.Fatalf("deleted = %+v", deleted)
	}

	if !tool.RenderResult(nil, json.RawMessage(`not json`)).IsZero() {
		t.Fatal("bad result did not fall back to the default")
	}
}

func TestParametersOfferEveryOperation(t *testing.T) {
	var schema struct {
		Properties map[string]struct {
			Enum        []string `json:"enum"`
			Description string   `json:"description"`
		} `json:"properties"`
		Required []string `json:"required"`
	}
	if err := json.Unmarshal((&Tool{}).Parameters(), &schema); err != nil {
		t.Fatal(err)
	}
	operation := schema.Properties["operation"]
	if got := strings.Join(operation.Enum, " "); got != "list_events get_event create_event update_event delete_event" {
		t.Fatalf("operation enum = %q", got)
	}
	if !strings.HasSuffix(operation.Description, "or delete_event.") {
		t.Fatalf("operation description = %q", operation.Description)
	}
	if got := schema.Properties["event_id"].Description; !strings.HasSuffix(got, "and delete_event.") {
		t.Fatalf("event_id description = %q", got)
	}
	if strings.Join(schema.Required, " ") != "operation" {
		t.Fatalf("required = %v", schema.Required)
	}
}

func fieldLabels(view gopi.ToolView) string {
	labels := make([]string, 0, len(view.Fields))
	for _, field := range view.Fields {
		labels = append(labels, field.Label)
	}
	return strings.Join(labels, " ")
}
