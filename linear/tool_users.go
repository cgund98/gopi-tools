package linear

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cgund98/gogent"
	"github.com/cgund98/gopi"
)

// usersOps are the operations linear_users exposes. Both are reads, so the tool
// is identical in agent, plan, and ask modes.
var usersOps = ops{
	{name: "get_viewer"},
	{name: "list_users"},
}

// usersArgs holds the arguments linear_users accepts.
type usersArgs struct {
	Operation string `json:"operation" jsonschema_description:"The operation to perform."`
	Team      string `json:"team,omitempty" jsonschema_description:"Team UUID, to list one team's members instead of the whole workspace."`

	listArgs
}

// usersTool implements gogent.Tool for Linear users.
type usersTool struct {
	*shared
	readonly bool
}

func newUsersTool(s *shared, readonly bool) gogent.Tool {
	return &usersTool{shared: s, readonly: readonly}
}

var (
	_ gogent.Tool       = (*usersTool)(nil)
	_ gopi.ToolRenderer = (*usersTool)(nil)
)

// Name returns the tool name.
func (t *usersTool) Name() string { return "linear_users" }

// Description returns the tool description for the model.
func (t *usersTool) Description() string {
	return "Read Linear users. get_viewer returns the authenticated user (whoami); " +
		"list_users returns workspace members, or one team's members when team is set. " +
		"Both operations are reads and run without approval."
}

// Parameters returns the JSON Schema for the tool arguments.
func (t *usersTool) Parameters() json.RawMessage {
	return parameters(new(usersArgs), usersOps, t.readonly)
}

// RequiresApproval returns whether the operation needs human approval. Users
// only reads, so it never does.
func (t *usersTool) RequiresApproval(_ context.Context, raw json.RawMessage) (gogent.ApprovalDecision, error) {
	var args usersArgs
	if err := parseArgs(raw, &args); err != nil {
		return gogent.ApprovalDecision{}, err
	}
	return approvalFor(usersOps, t.readonly, args.Operation, func() string { return "" }), nil
}

// Execute runs the requested Linear operation.
func (t *usersTool) Execute(ctx context.Context, raw json.RawMessage) (json.RawMessage, error) {
	var args usersArgs
	if err := parseArgs(raw, &args); err != nil {
		return nil, err
	}
	return t.execute(ctx, usersOps, t.readonly, args.Operation,
		func(ctx context.Context, client linearClient, r *resolver) (json.RawMessage, error) {
			return t.dispatch(ctx, client, r, args)
		})
}

func (t *usersTool) dispatch(ctx context.Context, client linearClient, r *resolver, args usersArgs) (json.RawMessage, error) {
	switch args.Operation {
	case "get_viewer":
		return t.getViewer(ctx, client)
	case "list_users":
		return t.listUsers(ctx, client, r, args)
	}
	return errorResult("invalid_operation", fmt.Sprintf("unknown operation %q", args.Operation))
}

func (t *usersTool) getViewer(ctx context.Context, client linearClient) (json.RawMessage, error) {
	me, err := client.viewer(ctx)
	if err != nil {
		return nil, err
	}
	return operationResult("get_viewer", map[string]any{"viewer": me})
}

func (t *usersTool) listUsers(ctx context.Context, client linearClient, r *resolver, args usersArgs) (json.RawMessage, error) {
	opts, err := args.options()
	if err != nil {
		return nil, err
	}
	if args.Team != "" {
		team, err := r.team(ctx, args.Team)
		if err != nil {
			return nil, err
		}
		detail, err := client.team(ctx, team.ID)
		if err != nil {
			return nil, err
		}
		users := []userInfo{}
		if detail.Members != nil {
			users = detail.Members.Nodes
		}
		return listOperationResult("list_users", "users", filterUsersByName(users, args.Query), nil)
	}
	filter := map[string]any{}
	if args.Query != "" {
		filter["or"] = []map[string]any{
			{"name": map[string]any{"contains": args.Query}},
			{"displayName": map[string]any{"contains": args.Query}},
			{"email": map[string]any{"contains": args.Query}},
		}
	}
	found, err := client.users(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	return listOperationResult("list_users", "users", found.Nodes, found.PageInfo)
}

// Headline names the operation and its subject on the tool line.
func (t *usersTool) Headline(raw json.RawMessage) string {
	var args usersArgs
	if json.Unmarshal(raw, &args) != nil {
		return ""
	}
	switch args.Operation {
	case "get_viewer":
		return "linear whoami"
	case "list_users":
		return join("linear users", args.Query)
	}
	return ""
}

// RenderApproval draws nothing: users has no write operations.
func (t *usersTool) RenderApproval(json.RawMessage) gopi.ToolView { return gopi.ToolView{} }

// RenderResult draws a completed users result.
func (t *usersTool) RenderResult(_, raw json.RawMessage) gopi.ToolView {
	var result linearResult
	if json.Unmarshal(raw, &result) != nil {
		return gopi.ToolView{}
	}
	switch result.Operation {
	case "get_viewer":
		if result.Viewer == nil {
			return gopi.ToolView{}
		}
		return gopi.ToolView{Fields: compactFields([]gopi.ToolField{
			{Label: "Name", Value: result.Viewer.Name},
			{Label: "Email", Value: result.Viewer.Email},
			{Label: "ID", Value: result.Viewer.ID},
		})}
	case "list_users":
		lines := make([]string, 0, len(result.Users))
		for _, user := range result.Users {
			lines = append(lines, fmt.Sprintf("%s  %s", user.Name, user.Email))
		}
		return lineView(lines, "No users")
	}
	return gopi.ToolView{}
}
