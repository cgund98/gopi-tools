package linear

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cgund98/gogent"
	"github.com/cgund98/gopi"
)

// teamsOps are the operations linear_teams exposes. Both are reads, so the tool
// is identical in agent, plan, and ask modes.
var teamsOps = ops{
	{name: "list_teams"},
	{name: "get_team"},
}

// teamsArgs holds the arguments linear_teams accepts.
type teamsArgs struct {
	Operation string `json:"operation" jsonschema_description:"The operation to perform."`
	Team      string `json:"team,omitempty" jsonschema_description:"Team UUID, from list_teams. Defaults to the configured default team."`

	listArgs
}

// teamsTool implements gogent.Tool for Linear teams.
type teamsTool struct {
	*shared
	readonly bool
}

func newTeamsTool(s *shared, readonly bool) gogent.Tool {
	return &teamsTool{shared: s, readonly: readonly}
}

var (
	_ gogent.Tool       = (*teamsTool)(nil)
	_ gopi.ToolRenderer = (*teamsTool)(nil)
)

// Name returns the tool name.
func (t *teamsTool) Name() string { return "linear_teams" }

// Description returns the tool description for the model.
func (t *teamsTool) Description() string {
	return "Read Linear teams. list_teams returns the team keys and IDs that every other Linear tool takes as team; " +
		"get_team also returns one team's workflow states, labels, and members. " +
		"Both operations are reads and run without approval."
}

// Parameters returns the JSON Schema for the tool arguments.
func (t *teamsTool) Parameters() json.RawMessage {
	return parameters(new(teamsArgs), teamsOps, t.readonly)
}

// RequiresApproval returns whether the operation needs human approval. Teams
// only reads, so it never does.
func (t *teamsTool) RequiresApproval(_ context.Context, raw json.RawMessage) (gogent.ApprovalDecision, error) {
	var args teamsArgs
	if err := parseArgs(raw, &args); err != nil {
		return gogent.ApprovalDecision{}, err
	}
	return approvalFor(teamsOps, t.readonly, args.Operation, func() string { return "" }), nil
}

// Execute runs the requested Linear operation.
func (t *teamsTool) Execute(ctx context.Context, raw json.RawMessage) (json.RawMessage, error) {
	var args teamsArgs
	if err := parseArgs(raw, &args); err != nil {
		return nil, err
	}
	return t.execute(ctx, teamsOps, t.readonly, args.Operation,
		func(ctx context.Context, client linearClient, r *resolver) (json.RawMessage, error) {
			return t.dispatch(ctx, client, r, args)
		})
}

func (t *teamsTool) dispatch(ctx context.Context, client linearClient, r *resolver, args teamsArgs) (json.RawMessage, error) {
	switch args.Operation {
	case "list_teams":
		return t.listTeams(ctx, client, args)
	case "get_team":
		return t.getTeam(ctx, client, r, args)
	}
	return errorResult("invalid_operation", fmt.Sprintf("unknown operation %q", args.Operation))
}

func (t *teamsTool) listTeams(ctx context.Context, client linearClient, args teamsArgs) (json.RawMessage, error) {
	opts, err := args.options()
	if err != nil {
		return nil, err
	}
	found, err := client.teams(ctx, nil, opts)
	if err != nil {
		return nil, err
	}
	return listOperationResult("list_teams", "teams", found.Nodes, found.PageInfo)
}

func (t *teamsTool) getTeam(ctx context.Context, client linearClient, r *resolver, args teamsArgs) (json.RawMessage, error) {
	team, err := r.team(ctx, args.Team)
	if err != nil {
		return nil, err
	}
	detail, err := client.team(ctx, team.ID)
	if err != nil {
		return nil, err
	}
	return operationResult("get_team", map[string]any{"team": detail})
}

// Headline names the operation and its subject on the tool line.
func (t *teamsTool) Headline(raw json.RawMessage) string {
	var args teamsArgs
	if json.Unmarshal(raw, &args) != nil {
		return ""
	}
	switch args.Operation {
	case "list_teams":
		return "linear teams"
	case "get_team":
		return join("linear team", t.teamName(firstNonEmpty(args.Team, t.cfg.DefaultTeam)))
	}
	return ""
}

// RenderApproval draws nothing: teams has no write operations.
func (t *teamsTool) RenderApproval(json.RawMessage) gopi.ToolView { return gopi.ToolView{} }

// RenderResult draws a completed teams result.
func (t *teamsTool) RenderResult(_, raw json.RawMessage) gopi.ToolView {
	var result linearResult
	if json.Unmarshal(raw, &result) != nil {
		return gopi.ToolView{}
	}
	switch result.Operation {
	case "list_teams":
		lines := make([]string, 0, len(result.Teams))
		for _, team := range result.Teams {
			lines = append(lines, fmt.Sprintf("%s  %s", team.Key, team.Name))
		}
		return lineView(lines, "No teams")
	case "get_team":
		if result.Team == nil {
			return gopi.ToolView{}
		}
		return gopi.ToolView{Fields: compactFields([]gopi.ToolField{
			{Label: "Team", Value: fmt.Sprintf("%s  %s", result.Team.Key, result.Team.Name)},
			{Label: "Description", Value: result.Team.Description},
			{Label: "States", Value: stateNames(result.Team.States)},
			{Label: "Labels", Value: labelNames(result.Team.Labels)},
			{Label: "Members", Value: userNames(result.Team.Members)},
		})}
	}
	return gopi.ToolView{}
}
