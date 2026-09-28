package linear

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cgund98/gogent"
	"github.com/cgund98/gopi"
)

// labelsOps are the operations linear_labels exposes.
var labelsOps = ops{
	{name: "list_labels"},
	{name: "create_label", write: true},
}

// labelsArgs holds the arguments linear_labels accepts.
type labelsArgs struct {
	Operation string `json:"operation" jsonschema_description:"The operation to perform."`

	Team        string `json:"team,omitempty" jsonschema_description:"Team UUID, to scope list_labels or a created label to one team."`
	Name        string `json:"name,omitempty" jsonschema_description:"Label name. Required for create_label."`
	Color       string `json:"color,omitempty" jsonschema_description:"Label color as a hex code, for create_label."`
	Description string `json:"description,omitempty" jsonschema_description:"Label description, for create_label."`

	listArgs
}

// labelsTool implements gogent.Tool for Linear issue labels.
type labelsTool struct {
	*shared
	readonly bool
}

func newLabelsTool(s *shared, readonly bool) gogent.Tool {
	return &labelsTool{shared: s, readonly: readonly}
}

var (
	_ gogent.Tool       = (*labelsTool)(nil)
	_ gopi.ToolRenderer = (*labelsTool)(nil)
)

// Name returns the tool name.
func (t *labelsTool) Name() string { return "linear_labels" }

// Description returns the tool description for the model.
func (t *labelsTool) Description() string {
	return "Read and create Linear issue labels. list_labels runs without approval; " +
		"create_label requires user approval. A label is workspace-wide unless a team is given."
}

// Parameters returns the JSON Schema for the tool arguments.
func (t *labelsTool) Parameters() json.RawMessage {
	return parameters(new(labelsArgs), labelsOps, t.readonly)
}

// RequiresApproval returns whether the operation needs human approval.
func (t *labelsTool) RequiresApproval(_ context.Context, raw json.RawMessage) (gogent.ApprovalDecision, error) {
	var args labelsArgs
	if err := parseArgs(raw, &args); err != nil {
		return gogent.ApprovalDecision{}, err
	}
	return approvalFor(labelsOps, t.readonly, args.Operation, func() string {
		return fmt.Sprintf("Create label %q", args.Name)
	}), nil
}

// Execute runs the requested Linear operation.
func (t *labelsTool) Execute(ctx context.Context, raw json.RawMessage) (json.RawMessage, error) {
	var args labelsArgs
	if err := parseArgs(raw, &args); err != nil {
		return nil, err
	}
	return t.execute(ctx, labelsOps, t.readonly, args.Operation,
		func(ctx context.Context, client linearClient, r *resolver) (json.RawMessage, error) {
			return t.dispatch(ctx, client, r, args)
		})
}

func (t *labelsTool) dispatch(ctx context.Context, client linearClient, r *resolver, args labelsArgs) (json.RawMessage, error) {
	switch args.Operation {
	case "list_labels":
		return t.listLabels(ctx, client, r, args)
	case "create_label":
		return t.createLabel(ctx, client, r, args)
	}
	return errorResult("invalid_operation", fmt.Sprintf("unknown operation %q", args.Operation))
}

func (t *labelsTool) listLabels(ctx context.Context, client linearClient, r *resolver, args labelsArgs) (json.RawMessage, error) {
	opts, err := args.options()
	if err != nil {
		return nil, err
	}
	filter := map[string]any{}
	if args.Team != "" {
		team, err := r.team(ctx, args.Team)
		if err != nil {
			return nil, err
		}
		filter["team"] = map[string]any{"id": map[string]any{"eq": team.ID}}
	}
	if args.Query != "" {
		filter["name"] = map[string]any{"contains": args.Query}
	}
	found, err := client.labels(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	return listOperationResult("list_labels", "labels", found.Nodes, found.PageInfo)
}

func (t *labelsTool) createLabel(ctx context.Context, client linearClient, r *resolver, args labelsArgs) (json.RawMessage, error) {
	if args.Name == "" {
		return nil, invalid("name is required for create_label")
	}
	in := labelInput{Name: args.Name, Color: args.Color, Description: args.Description}
	if args.Team != "" {
		team, err := r.team(ctx, args.Team)
		if err != nil {
			return nil, err
		}
		in.TeamID = team.ID
	}
	label, err := client.createLabel(ctx, in)
	if err != nil {
		return nil, err
	}
	r.invalidate("label/")
	return operationResult("create_label", map[string]any{"label": label})
}

// Headline names the operation and its subject on the tool line.
func (t *labelsTool) Headline(raw json.RawMessage) string {
	var args labelsArgs
	if json.Unmarshal(raw, &args) != nil {
		return ""
	}
	switch args.Operation {
	case "list_labels":
		return "linear labels"
	case "create_label":
		return join("linear label", args.Name)
	}
	return ""
}

// RenderApproval shows the label a write will create.
func (t *labelsTool) RenderApproval(raw json.RawMessage) gopi.ToolView {
	var args labelsArgs
	if json.Unmarshal(raw, &args) != nil {
		return gopi.ToolView{}
	}
	if args.Operation != "create_label" {
		return gopi.ToolView{}
	}
	var out approvalFields
	out.add("Name", args.Name)
	out.add("Team", t.teamName(args.Team))
	out.add("Color", args.Color)
	out.add("Description", truncate(args.Description, 200))
	return out.view()
}

// RenderResult draws a completed labels result.
func (t *labelsTool) RenderResult(_, raw json.RawMessage) gopi.ToolView {
	var result linearResult
	if json.Unmarshal(raw, &result) != nil {
		return gopi.ToolView{}
	}
	switch result.Operation {
	case "list_labels":
		lines := make([]string, 0, len(result.Labels))
		for _, label := range result.Labels {
			lines = append(lines, label.Name)
		}
		return lineView(lines, "No labels")
	case "create_label":
		if result.Label == nil {
			return gopi.ToolView{}
		}
		return confirmation("Label created", "", result.Label.Name, "")
	}
	return gopi.ToolView{}
}
