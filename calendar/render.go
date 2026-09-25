package calendar

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/cgund98/gopi"
)

var _ gopi.ToolRenderer = (*CalendarTool)(nil)

// Headline names the operation and its subject on the tool line.
func (t *CalendarTool) Headline(raw json.RawMessage) string {
	var args calendarArgs
	if json.Unmarshal(raw, &args) != nil {
		return ""
	}
	switch args.Operation {
	case "list_events":
		if day := formatDay(args.StartTime); day != "" {
			return "calendar list " + day
		}
		return "calendar list"
	case "get_event":
		return strings.TrimSpace("calendar get " + args.EventID)
	case "create_event":
		return strings.TrimSpace("calendar create " + args.Summary)
	case "update_event":
		return strings.TrimSpace("calendar update " + firstNonEmpty(args.Summary, args.EventID))
	case "delete_event":
		return strings.TrimSpace("calendar delete " + args.EventID)
	default:
		return ""
	}
}

// RenderApproval shows the event a write will create, change, or delete.
func (t *CalendarTool) RenderApproval(raw json.RawMessage) gopi.ToolView {
	var args calendarArgs
	if json.Unmarshal(raw, &args) != nil {
		return gopi.ToolView{}
	}
	var fields []gopi.ToolField
	add := func(label, value string) {
		if value != "" {
			fields = append(fields, gopi.ToolField{Label: label, Value: value})
		}
	}
	if event, ok := t.currentEvent(args.EventID); ok {
		switch args.Operation {
		case "delete_event":
			add("Summary", event.Summary)
			add("When", formatSpan(event.StartTime, event.EndTime))
			add("Location", event.Location)
			return gopi.ToolView{Fields: fields}
		case "update_event":
			start := firstNonEmpty(args.StartTime, event.StartTime)
			end := firstNonEmpty(args.EndTime, event.EndTime)
			add("Summary", change(event.Summary, args.Summary))
			add("When", change(formatSpan(event.StartTime, event.EndTime), formatSpan(start, end)))
			add("Location", change(event.Location, args.Location))
			add("Description", change(event.Description, args.Description))
			return gopi.ToolView{Fields: fields}
		}
	}
	if args.Operation != "create_event" {
		add("Event", args.EventID)
	}
	add("Summary", args.Summary)
	add("Start", formatTime(args.StartTime))
	add("End", formatTime(args.EndTime))
	add("Location", args.Location)
	add("Description", args.Description)
	return gopi.ToolView{Fields: fields}
}

// RenderResult shows events as lines, one event as fields, and a delete as one line.
func (t *CalendarTool) RenderResult(_, raw json.RawMessage) gopi.ToolView {
	var result struct {
		Operation string         `json:"operation"`
		Events    []eventSummary `json:"events"`
		Event     *eventDetail   `json:"event"`
		Success   bool           `json:"success"`
	}
	if json.Unmarshal(raw, &result) != nil {
		return gopi.ToolView{}
	}
	switch result.Operation {
	case "list_events":
		if len(result.Events) == 0 {
			return gopi.ToolView{Lines: []string{"No events"}}
		}
		lines := make([]string, 0, len(result.Events))
		for _, event := range result.Events {
			lines = append(lines, formatSpan(event.StartTime, event.EndTime)+"  "+event.Summary)
		}
		return gopi.ToolView{Lines: lines}
	case "get_event", "create_event", "update_event":
		if result.Event == nil {
			return gopi.ToolView{}
		}
		event := result.Event
		var fields []gopi.ToolField
		for _, field := range []gopi.ToolField{
			{Label: "Summary", Value: event.Summary},
			{Label: "When", Value: formatSpan(event.StartTime, event.EndTime)},
			{Label: "Location", Value: event.Location},
			{Label: "Link", Value: event.HTMLLink},
		} {
			if strings.TrimSpace(field.Value) != "" {
				fields = append(fields, field)
			}
		}
		return gopi.ToolView{Fields: fields}
	case "delete_event":
		if result.Success {
			return gopi.ToolView{Lines: []string{"Deleted"}}
		}
		return gopi.ToolView{}
	default:
		return gopi.ToolView{}
	}
}

func parseTime(value string) (time.Time, bool) {
	for _, layout := range []string{time.RFC3339, "2006-01-02"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed.Local(), true
		}
	}
	return time.Time{}, false
}

func formatDay(value string) string {
	if parsed, ok := parseTime(value); ok {
		return parsed.Format("2006-01-02")
	}
	return value
}

func formatTime(value string) string {
	if parsed, ok := parseTime(value); ok {
		return parsed.Format("Mon Jan 2 15:04")
	}
	return value
}

// formatSpan writes "Mon Jan 2 09:00–09:30", repeating the date only when the end is another day.
func formatSpan(start, end string) string {
	from, okFrom := parseTime(start)
	to, okTo := parseTime(end)
	switch {
	case okFrom && okTo && from.Format("2006-01-02") == to.Format("2006-01-02"):
		return from.Format("Mon Jan 2 15:04") + "–" + to.Format("15:04")
	case okFrom && okTo:
		return from.Format("Mon Jan 2 15:04") + " – " + to.Format("Mon Jan 2 15:04")
	case start != "" && end != "":
		return fmt.Sprintf("%s – %s", start, end)
	default:
		return firstNonEmpty(formatTime(start), formatTime(end))
	}
}

// change shows "old → new" when an update sets a different value. An empty next
// means the update leaves the field alone.
func change(current, next string) string {
	if next == "" || next == current {
		return current
	}
	return firstNonEmpty(current, "(none)") + " → " + next
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
