package linear

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cgund98/gogent"
	"github.com/cgund98/gopi"
)

// projectsOps are the operations linear_projects exposes.
var projectsOps = ops{
	{name: "list_projects"},
	{name: "get_project"},
	{name: "create_project", write: true},
	{name: "update_project", write: true},
	{name: "create_milestone", write: true},
	{name: "update_milestone", write: true},
}

// projectsArgs holds the arguments linear_projects accepts.
type projectsArgs struct {
	Operation string `json:"operation" jsonschema_description:"The operation to perform."`

	Team             string `json:"team,omitempty" jsonschema_description:"Team UUID, from list_teams. Create operations fall back to the configured default team."`
	Project          string `json:"project,omitempty" jsonschema_description:"Project UUID, from list_projects. Required for the milestone operations."`
	ProjectMilestone string `json:"project_milestone,omitempty" jsonschema_description:"Project milestone name or UUID, for update_milestone."`
	Lead             string `json:"lead,omitempty" jsonschema_description:"Project lead: me, an email, a name, or a user ID."`

	Name          string `json:"name,omitempty" jsonschema_description:"Project or milestone name."`
	Description   string `json:"description,omitempty" jsonschema_description:"Project or milestone description, in Markdown."`
	Summary       string `json:"summary,omitempty" jsonschema_description:"Short project summary."`
	ProjectStatus string `json:"project_status,omitempty" jsonschema_description:"Project status name or type: backlog, planned, started, paused, completed, or canceled."`
	TargetDate    string `json:"target_date,omitempty" jsonschema_description:"Project or milestone target date, as YYYY-MM-DD."`
	StartDate     string `json:"start_date,omitempty" jsonschema_description:"Project start date, as YYYY-MM-DD."`

	listArgs
}

// projectsTool implements gogent.Tool for Linear projects and milestones.
type projectsTool struct {
	*shared
	readonly bool
}

func newProjectsTool(s *shared, readonly bool) gogent.Tool {
	return &projectsTool{shared: s, readonly: readonly}
}

var (
	_ gogent.Tool       = (*projectsTool)(nil)
	_ gopi.ToolRenderer = (*projectsTool)(nil)
)

// Name returns the tool name.
func (t *projectsTool) Name() string { return "linear_projects" }

// Description returns the tool description for the model.
func (t *projectsTool) Description() string {
	return "Read and write Linear projects and their milestones. Reads: list and get projects. " +
		"Writes: create and update projects, and create or update milestones. " +
		"Read operations run without approval. Write operations require user approval and show a before/after diff. " +
		"Team and project take a UUID from list_teams and list_projects."
}

// Parameters returns the JSON Schema for the tool arguments.
func (t *projectsTool) Parameters() json.RawMessage {
	return parameters(new(projectsArgs), projectsOps, t.readonly)
}

// RequiresApproval returns whether the operation needs human approval.
func (t *projectsTool) RequiresApproval(ctx context.Context, raw json.RawMessage) (gogent.ApprovalDecision, error) {
	var args projectsArgs
	if err := parseArgs(raw, &args); err != nil {
		return gogent.ApprovalDecision{}, err
	}
	return approvalFor(projectsOps, t.readonly, args.Operation, func() string {
		return t.approvalReason(ctx, args)
	}), nil
}

// Execute runs the requested Linear operation.
func (t *projectsTool) Execute(ctx context.Context, raw json.RawMessage) (json.RawMessage, error) {
	var args projectsArgs
	if err := parseArgs(raw, &args); err != nil {
		return nil, err
	}
	return t.execute(ctx, projectsOps, t.readonly, args.Operation,
		func(ctx context.Context, client linearClient, r *resolver) (json.RawMessage, error) {
			return t.dispatch(ctx, client, r, args)
		})
}

// approvalReason names the change the user is approving.
func (t *projectsTool) approvalReason(ctx context.Context, args projectsArgs) string {
	switch args.Operation {
	case "update_project":
		if project, ok := t.lookupProject(ctx, args.Project); ok {
			return fmt.Sprintf("Update project %q", project.Name)
		}
		return fmt.Sprintf("Update project %s", t.projectName(args.Project))
	case "create_project":
		return fmt.Sprintf("Create project %q", args.Name)
	case "create_milestone":
		return fmt.Sprintf("Create milestone %q", args.Name)
	case "update_milestone":
		return fmt.Sprintf("Update milestone %s", t.milestoneName(args.ProjectMilestone))
	}
	return "Linear write operation: " + args.Operation
}

func (t *projectsTool) dispatch(ctx context.Context, client linearClient, r *resolver, args projectsArgs) (json.RawMessage, error) {
	switch args.Operation {
	case "list_projects":
		return t.listProjects(ctx, client, r, args)
	case "get_project":
		return t.getProject(ctx, client, r, args)
	case "create_project":
		return t.createProject(ctx, client, r, args)
	case "update_project":
		return t.updateProject(ctx, client, r, args)
	case "create_milestone":
		return t.createMilestone(ctx, client, r, args)
	case "update_milestone":
		return t.updateMilestone(ctx, client, r, args)
	}
	return errorResult("invalid_operation", fmt.Sprintf("unknown operation %q", args.Operation))
}

func (t *projectsTool) listProjects(ctx context.Context, client linearClient, r *resolver, args projectsArgs) (json.RawMessage, error) {
	if err := requireUUID("team", args.Team); err != nil {
		return nil, err
	}
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
		filter["accessibleTeams"] = map[string]any{"some": map[string]any{"id": map[string]any{"eq": team.ID}}}
	}
	if args.Lead != "" {
		lead, err := r.user(ctx, args.Lead)
		if err != nil {
			return nil, err
		}
		filter["lead"] = map[string]any{"id": map[string]any{"eq": lead.ID}}
	}
	if args.ProjectStatus != "" {
		teamID := ""
		if args.Team != "" {
			team, err := r.team(ctx, args.Team)
			if err != nil {
				return nil, err
			}
			teamID = team.ID
		}
		status, err := r.projectStatus(ctx, teamID, args.ProjectStatus)
		if err != nil {
			return nil, err
		}
		filter["status"] = map[string]any{"id": map[string]any{"eq": status.ID}}
	}
	if args.Query != "" {
		filter["name"] = map[string]any{"contains": args.Query}
	}
	if args.UpdatedSince != "" {
		filter["updatedAt"] = map[string]any{"gte": args.UpdatedSince}
	}
	if args.CreatedSince != "" {
		filter["createdAt"] = map[string]any{"gte": args.CreatedSince}
	}
	found, err := client.projects(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	return listOperationResult("list_projects", "projects", found.Nodes, found.PageInfo)
}

func (t *projectsTool) getProject(ctx context.Context, client linearClient, r *resolver, args projectsArgs) (json.RawMessage, error) {
	ref, err := r.project(ctx, args.Project)
	if err != nil {
		return nil, err
	}
	project, err := client.project(ctx, ref.ID)
	if err != nil {
		return nil, err
	}
	return operationResult("get_project", map[string]any{"project": project})
}

func (t *projectsTool) createProject(ctx context.Context, client linearClient, r *resolver, args projectsArgs) (json.RawMessage, error) {
	if err := requireUUID("team", args.Team); err != nil {
		return nil, err
	}
	if args.Name == "" {
		return nil, invalid("name is required for create_project")
	}
	team, err := r.team(ctx, args.Team)
	if err != nil {
		return nil, err
	}
	in := projectInput{
		Name:        args.Name,
		Description: args.Description,
		Summary:     args.Summary,
		StartDate:   args.StartDate,
		TargetDate:  args.TargetDate,
		TeamIDs:     []string{team.ID},
	}
	if args.Lead != "" {
		lead, err := r.user(ctx, args.Lead)
		if err != nil {
			return nil, err
		}
		in.LeadID = lead.ID
	}
	if args.ProjectStatus != "" {
		status, err := r.projectStatus(ctx, team.ID, args.ProjectStatus)
		if err != nil {
			return nil, err
		}
		in.StatusID = status.ID
	}
	created, err := client.createProject(ctx, in)
	if err != nil {
		return nil, err
	}
	return operationResult("create_project", map[string]any{"project": created})
}

func (t *projectsTool) updateProject(ctx context.Context, client linearClient, r *resolver, args projectsArgs) (json.RawMessage, error) {
	if err := requireUUID("team", args.Team); err != nil {
		return nil, err
	}
	if err := requireUUID("project", args.Project); err != nil {
		return nil, err
	}
	ref, err := r.project(ctx, args.Project)
	if err != nil {
		return nil, err
	}
	in := projectInput{
		Name:        args.Name,
		Description: args.Description,
		Summary:     args.Summary,
		StartDate:   args.StartDate,
		TargetDate:  args.TargetDate,
	}
	teamID := ""
	if args.Team != "" {
		team, err := r.team(ctx, args.Team)
		if err != nil {
			return nil, err
		}
		teamID = team.ID
		in.TeamIDs = []string{team.ID}
	}
	if args.Lead != "" {
		lead, err := r.user(ctx, args.Lead)
		if err != nil {
			return nil, err
		}
		in.LeadID = lead.ID
	}
	if args.ProjectStatus != "" {
		status, err := r.projectStatus(ctx, teamID, args.ProjectStatus)
		if err != nil {
			return nil, err
		}
		in.StatusID = status.ID
	}
	updated, err := client.updateProject(ctx, ref.ID, in)
	if err != nil {
		return nil, err
	}
	t.forgetCurrentProject(args.Project)
	return operationResult("update_project", map[string]any{"project": updated})
}

func (t *projectsTool) createMilestone(ctx context.Context, client linearClient, r *resolver, args projectsArgs) (json.RawMessage, error) {
	if args.Name == "" {
		return nil, invalid("name is required for create_milestone")
	}
	if args.Project == "" {
		return nil, invalid("project is required for create_milestone")
	}
	if err := requireUUID("project", args.Project); err != nil {
		return nil, err
	}
	project, err := r.project(ctx, args.Project)
	if err != nil {
		return nil, err
	}
	milestone, err := client.createMilestone(ctx, milestoneInput{
		Name:        args.Name,
		ProjectID:   project.ID,
		Description: args.Description,
		TargetDate:  args.TargetDate,
	})
	if err != nil {
		return nil, err
	}
	r.invalidate("milestone/" + project.ID + "/")
	return operationResult("create_milestone", map[string]any{"milestone": milestone})
}

func (t *projectsTool) updateMilestone(ctx context.Context, client linearClient, r *resolver, args projectsArgs) (json.RawMessage, error) {
	if args.ProjectMilestone == "" {
		return nil, invalid("project_milestone is required for update_milestone")
	}
	if args.Project == "" {
		return nil, invalid("project is required for update_milestone")
	}
	if err := requireUUID("project", args.Project); err != nil {
		return nil, err
	}
	project, err := r.project(ctx, args.Project)
	if err != nil {
		return nil, err
	}
	milestone, err := r.milestone(ctx, project.ID, args.ProjectMilestone)
	if err != nil {
		return nil, err
	}
	in := milestoneInput{
		Name:        args.Name,
		Description: args.Description,
		TargetDate:  args.TargetDate,
	}
	updated, err := client.updateMilestone(ctx, milestone.ID, in)
	if err != nil {
		return nil, err
	}
	r.invalidate("milestone/" + project.ID + "/")
	return operationResult("update_milestone", map[string]any{"milestone": updated})
}

// Headline names the operation and its subject on the tool line.
func (t *projectsTool) Headline(raw json.RawMessage) string {
	var args projectsArgs
	if json.Unmarshal(raw, &args) != nil {
		return ""
	}
	switch args.Operation {
	case "list_projects":
		return join("linear projects", args.Query)
	case "get_project":
		return join("linear project", t.projectName(args.Project))
	case "create_project":
		return join("linear create project", args.Name)
	case "update_project":
		return join("linear update project", t.projectName(args.Project))
	case "create_milestone":
		return join("linear milestone", args.Name)
	case "update_milestone":
		return join("linear update milestone", t.milestoneName(args.ProjectMilestone))
	}
	return ""
}

// RenderApproval shows the entity a write will create or change.
func (t *projectsTool) RenderApproval(raw json.RawMessage) gopi.ToolView {
	var args projectsArgs
	if json.Unmarshal(raw, &args) != nil {
		return gopi.ToolView{}
	}
	var out approvalFields
	switch args.Operation {
	case "update_project":
		t.approvalUpdateProject(args, &out)
	case "create_project":
		out.add("Name", args.Name)
		out.add("Team", t.teamName(firstNonEmpty(args.Team, t.cfg.DefaultTeam)))
		out.add("Lead", args.Lead)
		out.add("Status", args.ProjectStatus)
		out.add("Start", args.StartDate)
		out.add("Target", args.TargetDate)
		out.add("Summary", truncate(args.Summary, 160))
		out.add("Description", truncate(args.Description, 200))
	case "create_milestone":
		out.add("Name", args.Name)
		out.add("Project", t.projectName(args.Project))
		out.add("Target", args.TargetDate)
		out.add("Description", truncate(args.Description, 200))
	case "update_milestone":
		out.add("Milestone", t.milestoneName(args.ProjectMilestone))
		out.add("Project", t.projectName(args.Project))
		out.add("Name", args.Name)
		out.add("Target", args.TargetDate)
		out.add("Description", truncate(args.Description, 200))
	}
	return out.view()
}

func (t *projectsTool) approvalUpdateProject(args projectsArgs, out *approvalFields) {
	current, ok := t.currentProject(args.Project)
	if !ok {
		out.add("Project", t.projectName(args.Project))
		out.add("Name", args.Name)
		out.add("Status", args.ProjectStatus)
		out.add("Lead", args.Lead)
		out.add("Start", args.StartDate)
		out.add("Target", args.TargetDate)
		out.add("Summary", truncate(args.Summary, 160))
		out.add("Description", truncate(args.Description, 200))
		return
	}
	out.add("Project", current.Name)
	out.add("Name", change(current.Name, args.Name))
	out.add("Status", change(projectStatusName(current.Status), args.ProjectStatus))
	out.add("Lead", change(userName(current.Lead), args.Lead))
	out.add("Start", change(current.StartDate, args.StartDate))
	out.add("Target", change(current.TargetDate, args.TargetDate))
	out.add("Summary", change(truncate(current.Content, 120), truncate(args.Summary, 120)))
	out.add("Description", bodyChange(args.Description))
}

// RenderResult draws a completed projects result.
func (t *projectsTool) RenderResult(_, raw json.RawMessage) gopi.ToolView {
	var result linearResult
	if json.Unmarshal(raw, &result) != nil {
		return gopi.ToolView{}
	}
	switch result.Operation {
	case "list_projects":
		lines := make([]string, 0, len(result.Projects))
		for _, project := range result.Projects {
			lines = append(lines, projectLine(project))
		}
		return lineView(lines, "No projects")
	case "get_project":
		return projectFields(result.Project)
	case "create_project", "update_project":
		if result.Project == nil {
			return gopi.ToolView{}
		}
		verb := "Project created"
		if result.Operation == "update_project" {
			verb = "Project updated"
		}
		return confirmation(verb, "", result.Project.Name, "")
	case "create_milestone", "update_milestone":
		if result.Milestone == nil {
			return gopi.ToolView{}
		}
		return confirmation("Milestone saved", "", result.Milestone.Name, "")
	}
	return gopi.ToolView{}
}

func projectFields(project *projectView) gopi.ToolView {
	if project == nil {
		return gopi.ToolView{}
	}
	return gopi.ToolView{Fields: compactFields([]gopi.ToolField{
		{Label: "Project", Value: project.Name},
		{Label: "Status", Value: projectStatusName(project.Status)},
		{Label: "Lead", Value: userName(project.Lead)},
		{Label: "Progress", Value: progressLabel(project.Progress)},
		{Label: "Start", Value: project.StartDate},
		{Label: "Target", Value: project.TargetDate},
		{Label: "Summary", Value: truncate(project.Content, 160)},
	})}
}

// projectLine renders one project as a compact list row.
func projectLine(project projectSummary) string {
	parts := []string{project.Name}
	if project.Status != nil && project.Status.Name != "" {
		parts = append(parts, project.Status.Name)
	}
	if project.TargetDate != "" {
		parts = append(parts, "target "+project.TargetDate)
	}
	return strings.Join(parts, "  ")
}
