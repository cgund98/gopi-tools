package linear

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cgund98/gogent"
	"github.com/cgund98/gopi"
)

// issuesOps are the operations linear_issues exposes.
var issuesOps = ops{
	{name: "list_issues"},
	{name: "get_issue"},
	{name: "create_issue", write: true},
	{name: "update_issue", write: true},
	{name: "archive_issue", write: true},
	{name: "create_relation", write: true},
}

// issuesArgs holds the arguments linear_issues accepts. The jsonschema tag
// splits on commas, so descriptions go in jsonschema_description.
type issuesArgs struct {
	Operation string `json:"operation" jsonschema_description:"The operation to perform."`

	Issue            string   `json:"issue,omitempty" jsonschema_description:"Issue identifier such as ENG-123, or a UUID."`
	Team             string   `json:"team,omitempty" jsonschema_description:"Team UUID, from list_teams. Create operations fall back to the configured default team."`
	Project          string   `json:"project,omitempty" jsonschema_description:"Project UUID, from list_projects."`
	ProjectMilestone string   `json:"project_milestone,omitempty" jsonschema_description:"Project milestone name or UUID, to filter issues by."`
	State            string   `json:"state,omitempty" jsonschema_description:"Workflow state name or type: backlog, unstarted, started, completed, or canceled."`
	Assignee         string   `json:"assignee,omitempty" jsonschema_description:"Assignee: me, an email, a name, or a user ID."`
	ParentIssue      string   `json:"parent_issue,omitempty" jsonschema_description:"Parent issue identifier or UUID."`
	RelatedIssue     string   `json:"related_issue,omitempty" jsonschema_description:"The other issue in a relation, for create_relation."`
	Labels           []string `json:"labels,omitempty" jsonschema_description:"Label names or IDs. For create_issue and update_issue this replaces the issue's labels; pass an empty list to clear them."`

	Title       string `json:"title,omitempty" jsonschema_description:"Issue title. Required for create_issue."`
	Description string `json:"description,omitempty" jsonschema_description:"Issue description, in Markdown."`
	Priority    *int   `json:"priority,omitempty" jsonschema_description:"Issue priority: 0 None, 1 Urgent, 2 High, 3 Medium, 4 Low."`
	Estimate    *int   `json:"estimate,omitempty" jsonschema_description:"Issue estimate, in the team's estimation scale."`
	DueDate     string `json:"due_date,omitempty" jsonschema_description:"Issue due date, as YYYY-MM-DD."`

	Relation string `json:"relation,omitempty" jsonschema:"enum=blocks,enum=blocked_by,enum=related,enum=duplicate" jsonschema_description:"Relation type for create_relation. blocked_by relates the other issue as blocking this one."`

	listArgs
}

// issuesTool implements gogent.Tool for Linear issues.
type issuesTool struct {
	*shared
	readonly bool
}

func newIssuesTool(s *shared, readonly bool) gogent.Tool {
	return &issuesTool{shared: s, readonly: readonly}
}

var (
	_ gogent.Tool       = (*issuesTool)(nil)
	_ gopi.ToolRenderer = (*issuesTool)(nil)
)

// Name returns the tool name.
func (t *issuesTool) Name() string { return "linear_issues" }

// Description returns the tool description for the model.
func (t *issuesTool) Description() string {
	return "Read and write Linear issues. Reads: list and get issues. Writes: create, update, and archive issues, and relate two issues. " +
		"Read operations run without approval. Write operations require user approval and show a before/after diff. " +
		"Issues accept the shorthand identifier such as ENG-123. Team and project take a UUID from list_teams and list_projects."
}

// Parameters returns the JSON Schema for the tool arguments.
func (t *issuesTool) Parameters() json.RawMessage {
	return parameters(new(issuesArgs), issuesOps, t.readonly)
}

// RequiresApproval returns whether the operation needs human approval. Reads
// never do. Updates and archives look the current issue up so the approval
// prompt can show what will change.
func (t *issuesTool) RequiresApproval(ctx context.Context, raw json.RawMessage) (gogent.ApprovalDecision, error) {
	var args issuesArgs
	if err := parseArgs(raw, &args); err != nil {
		return gogent.ApprovalDecision{}, err
	}
	return approvalFor(issuesOps, t.readonly, args.Operation, func() string {
		return t.approvalReason(ctx, args)
	}), nil
}

// Execute runs the requested Linear operation.
func (t *issuesTool) Execute(ctx context.Context, raw json.RawMessage) (json.RawMessage, error) {
	var args issuesArgs
	if err := parseArgs(raw, &args); err != nil {
		return nil, err
	}
	return t.execute(ctx, issuesOps, t.readonly, args.Operation,
		func(ctx context.Context, client linearClient, r *resolver) (json.RawMessage, error) {
			return t.dispatch(ctx, client, r, args)
		})
}

// approvalReason names the change the user is approving, using the current
// issue when it can be fetched.
func (t *issuesTool) approvalReason(ctx context.Context, args issuesArgs) string {
	switch args.Operation {
	case "update_issue", "archive_issue":
		verb := "Update"
		if args.Operation == "archive_issue" {
			verb = "Archive"
		}
		if issue, ok := t.lookupIssue(ctx, args.Issue); ok {
			return fmt.Sprintf("%s issue %s %q", verb, issue.Identifier, issue.Title)
		}
		return fmt.Sprintf("%s issue %s", verb, args.Issue)
	case "create_issue":
		return strings.TrimSpace(fmt.Sprintf("Create issue %q in team %s", args.Title, t.teamName(firstNonEmpty(args.Team, t.cfg.DefaultTeam))))
	case "create_relation":
		return fmt.Sprintf("Relate issue %s to %s as %s", args.Issue, args.RelatedIssue, args.Relation)
	}
	return "Linear write operation: " + args.Operation
}

func (t *issuesTool) dispatch(ctx context.Context, client linearClient, r *resolver, args issuesArgs) (json.RawMessage, error) {
	switch args.Operation {
	case "list_issues":
		return t.listIssues(ctx, client, r, args)
	case "get_issue":
		return t.getIssue(ctx, client, args)
	case "create_issue":
		return t.createIssue(ctx, client, r, args)
	case "update_issue":
		return t.updateIssue(ctx, client, r, args)
	case "archive_issue":
		return t.archiveIssue(ctx, client, args)
	case "create_relation":
		return t.createRelation(ctx, client, args)
	}
	return errorResult("invalid_operation", fmt.Sprintf("unknown operation %q", args.Operation))
}

func (t *issuesTool) listIssues(ctx context.Context, client linearClient, r *resolver, args issuesArgs) (json.RawMessage, error) {
	if err := requireUUID("team", args.Team); err != nil {
		return nil, err
	}
	if err := requireUUID("project", args.Project); err != nil {
		return nil, err
	}
	opts, err := args.options()
	if err != nil {
		return nil, err
	}
	filter, err := t.issueFilter(ctx, r, args)
	if err != nil {
		return nil, err
	}
	found, err := client.issues(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	return listOperationResult("list_issues", "issues", found.Nodes, found.PageInfo)
}

// issueFilter builds an IssueFilter from the list_issues arguments. The team is
// only applied when the model names one, so a query can span teams.
func (t *issuesTool) issueFilter(ctx context.Context, r *resolver, args issuesArgs) (map[string]any, error) {
	filter := map[string]any{}
	if args.Team != "" {
		team, err := r.team(ctx, args.Team)
		if err != nil {
			return nil, err
		}
		filter["team"] = map[string]any{"id": map[string]any{"eq": team.ID}}
	}
	if args.State != "" {
		branches := []map[string]any{{"name": map[string]any{"eqIgnoreCase": args.State}}}
		branches = append(branches, stateTypeBranches(args.State)...)
		branches = append(branches, idBranches(args.State)...)
		filter["state"] = map[string]any{"or": branches}
	}
	if args.Assignee != "" {
		user, err := r.user(ctx, args.Assignee)
		if err != nil {
			return nil, err
		}
		filter["assignee"] = map[string]any{"id": map[string]any{"eq": user.ID}}
	}
	if args.Project != "" {
		project, err := r.project(ctx, args.Project)
		if err != nil {
			return nil, err
		}
		filter["project"] = map[string]any{"id": map[string]any{"eq": project.ID}}
	}
	if args.ProjectMilestone != "" {
		if args.Project == "" {
			return nil, invalid("project is required to filter by project_milestone")
		}
		project, err := r.project(ctx, args.Project)
		if err != nil {
			return nil, err
		}
		milestone, err := r.milestone(ctx, project.ID, args.ProjectMilestone)
		if err != nil {
			return nil, err
		}
		filter["projectMilestone"] = map[string]any{"id": map[string]any{"eq": milestone.ID}}
	}
	if len(args.Labels) > 0 {
		ids, err := t.labelIDs(ctx, r, "", args.Labels)
		if err != nil {
			return nil, err
		}
		filter["labels"] = map[string]any{"id": map[string]any{"in": ids}}
	}
	if args.Priority != nil {
		filter["priority"] = map[string]any{"eq": float64(*args.Priority)}
	}
	if args.Query != "" {
		filter["or"] = []map[string]any{
			{"title": map[string]any{"contains": args.Query}},
			{"description": map[string]any{"contains": args.Query}},
		}
	}
	if args.UpdatedSince != "" {
		filter["updatedAt"] = map[string]any{"gte": args.UpdatedSince}
	}
	if args.CreatedSince != "" {
		filter["createdAt"] = map[string]any{"gte": args.CreatedSince}
	}
	return filter, nil
}

func (t *issuesTool) getIssue(ctx context.Context, client linearClient, args issuesArgs) (json.RawMessage, error) {
	if args.Issue == "" {
		return nil, invalid("issue is required for get_issue")
	}
	issue, err := client.issue(ctx, args.Issue)
	if err != nil {
		return nil, err
	}
	return operationResult("get_issue", map[string]any{"issue": issue})
}

func (t *issuesTool) createIssue(ctx context.Context, client linearClient, r *resolver, args issuesArgs) (json.RawMessage, error) {
	if err := requireUUID("team", args.Team); err != nil {
		return nil, err
	}
	if err := requireUUID("project", args.Project); err != nil {
		return nil, err
	}
	if args.Title == "" {
		return nil, invalid("title is required for create_issue")
	}
	team, err := r.team(ctx, args.Team)
	if err != nil {
		return nil, err
	}
	in := issueInput{
		TeamID:      team.ID,
		Title:       args.Title,
		Description: args.Description,
		Priority:    args.Priority,
		Estimate:    args.Estimate,
		DueDate:     args.DueDate,
	}
	if args.Assignee != "" {
		user, err := r.user(ctx, args.Assignee)
		if err != nil {
			return nil, err
		}
		in.AssigneeID = user.ID
	}
	if args.State != "" {
		state, err := r.state(ctx, team.ID, args.State)
		if err != nil {
			return nil, err
		}
		in.StateID = state.ID
	}
	if args.Project != "" {
		project, err := r.project(ctx, args.Project)
		if err != nil {
			return nil, err
		}
		in.ProjectID = project.ID
	}
	if args.ParentIssue != "" {
		parentID, err := t.issueID(ctx, client, args.ParentIssue)
		if err != nil {
			return nil, err
		}
		in.ParentID = parentID
	}
	if args.Labels != nil {
		ids, err := t.labelIDs(ctx, r, team.ID, args.Labels)
		if err != nil {
			return nil, err
		}
		in.LabelIDs = ids
	}
	created, err := client.createIssue(ctx, in)
	if err != nil {
		return nil, err
	}
	return operationResult("create_issue", map[string]any{"issue": created})
}

func (t *issuesTool) updateIssue(ctx context.Context, client linearClient, r *resolver, args issuesArgs) (json.RawMessage, error) {
	if err := requireUUID("team", args.Team); err != nil {
		return nil, err
	}
	if err := requireUUID("project", args.Project); err != nil {
		return nil, err
	}
	if args.Issue == "" {
		return nil, invalid("issue is required for update_issue")
	}
	// The current issue supplies the team used to scope state and label names.
	current, ok := t.lookupIssue(ctx, args.Issue)
	if !ok {
		fetched, err := client.issue(ctx, args.Issue)
		if err != nil {
			return nil, err
		}
		current = fetched
	}
	teamID := ""
	if current.Team != nil {
		teamID = current.Team.ID
	}
	if args.Team != "" {
		team, err := r.team(ctx, args.Team)
		if err != nil {
			return nil, err
		}
		teamID = team.ID
	}

	in := issueInput{
		TeamID:      teamIDIfExplicit(args.Team, teamID),
		Title:       args.Title,
		Description: args.Description,
		Priority:    args.Priority,
		Estimate:    args.Estimate,
		DueDate:     args.DueDate,
	}
	if args.Assignee != "" {
		user, err := r.user(ctx, args.Assignee)
		if err != nil {
			return nil, err
		}
		in.AssigneeID = user.ID
	}
	if args.State != "" {
		if teamID == "" {
			return nil, invalid("state requires a team; pass team or set default_team")
		}
		state, err := r.state(ctx, teamID, args.State)
		if err != nil {
			return nil, err
		}
		in.StateID = state.ID
	}
	if args.Project != "" {
		project, err := r.project(ctx, args.Project)
		if err != nil {
			return nil, err
		}
		in.ProjectID = project.ID
	}
	if args.ParentIssue != "" {
		parentID, err := t.issueID(ctx, client, args.ParentIssue)
		if err != nil {
			return nil, err
		}
		in.ParentID = parentID
	}
	if args.Labels != nil {
		ids, err := t.labelIDs(ctx, r, teamID, args.Labels)
		if err != nil {
			return nil, err
		}
		in.LabelIDs = ids
	}
	updated, err := client.updateIssue(ctx, args.Issue, in)
	if err != nil {
		return nil, err
	}
	t.forgetCurrentIssue(args.Issue)
	return operationResult("update_issue", map[string]any{"issue": updated})
}

func (t *issuesTool) archiveIssue(ctx context.Context, client linearClient, args issuesArgs) (json.RawMessage, error) {
	if args.Issue == "" {
		return nil, invalid("issue is required for archive_issue")
	}
	ref, err := client.archiveIssue(ctx, args.Issue)
	if err != nil {
		return nil, err
	}
	t.forgetCurrentIssue(args.Issue)
	return operationResult("archive_issue", map[string]any{"issue": ref})
}

func (t *issuesTool) createRelation(ctx context.Context, client linearClient, args issuesArgs) (json.RawMessage, error) {
	if args.Issue == "" || args.RelatedIssue == "" {
		return nil, invalid("issue and related_issue are required for create_relation")
	}
	issueID, err := t.issueID(ctx, client, args.Issue)
	if err != nil {
		return nil, err
	}
	relatedID, err := t.issueID(ctx, client, args.RelatedIssue)
	if err != nil {
		return nil, err
	}
	var in relationInput
	switch args.Relation {
	case "blocks":
		in = relationInput{IssueID: issueID, RelatedIssueID: relatedID, Type: "blocks"}
	case "blocked_by":
		// Linear has no blocked_by type; the other issue blocks this one.
		in = relationInput{IssueID: relatedID, RelatedIssueID: issueID, Type: "blocks"}
	case "related":
		in = relationInput{IssueID: issueID, RelatedIssueID: relatedID, Type: "related"}
	case "duplicate":
		in = relationInput{IssueID: issueID, RelatedIssueID: relatedID, Type: "duplicate"}
	default:
		return nil, invalid("unknown relation %q; use blocks, blocked_by, related, or duplicate", args.Relation)
	}
	relation, err := client.createRelation(ctx, in)
	if err != nil {
		return nil, err
	}
	return operationResult("create_relation", map[string]any{"relation": relation})
}

// Headline names the operation and its subject on the tool line.
func (t *issuesTool) Headline(raw json.RawMessage) string {
	var args issuesArgs
	if json.Unmarshal(raw, &args) != nil {
		return ""
	}
	switch args.Operation {
	case "list_issues":
		return join("linear issues", t.issueListSubject(args))
	case "get_issue":
		return join("linear get", args.Issue)
	case "create_issue":
		return join("linear create", args.Title)
	case "update_issue":
		return join("linear update", args.Issue)
	case "archive_issue":
		return join("linear archive", args.Issue)
	case "create_relation":
		return join("linear relate", args.Issue) + " -> " + args.RelatedIssue
	}
	return ""
}

// issueListSubject summarizes the filters on a list_issues call, showing a
// cached name where the model passed a team or project ID.
func (t *issuesTool) issueListSubject(args issuesArgs) string {
	switch {
	case args.Assignee != "":
		return args.Assignee
	case args.Query != "":
		return args.Query
	case args.Team != "":
		return t.teamName(args.Team)
	case args.Project != "":
		return t.projectName(args.Project)
	}
	return ""
}

// RenderApproval shows the issue a write will create, change, or archive.
func (t *issuesTool) RenderApproval(raw json.RawMessage) gopi.ToolView {
	var args issuesArgs
	if json.Unmarshal(raw, &args) != nil {
		return gopi.ToolView{}
	}
	var out approvalFields
	switch args.Operation {
	case "update_issue":
		t.approvalUpdateIssue(args, &out)
	case "archive_issue":
		t.approvalArchiveIssue(args, &out)
	case "create_issue":
		out.add("Title", args.Title)
		out.add("Team", t.teamName(firstNonEmpty(args.Team, t.cfg.DefaultTeam)))
		out.add("Assignee", t.userName(args.Assignee))
		out.add("State", t.stateName(args.State))
		out.add("Priority", priorityName(args.Priority))
		out.add("Estimate", estimateName(args.Estimate))
		out.add("Due", args.DueDate)
		out.add("Project", t.projectName(args.Project))
		out.add("Parent", args.ParentIssue)
		out.add("Labels", t.labelList(args.Labels))
		out.add("Description", truncate(args.Description, 200))
	case "create_relation":
		out.add("Issue", args.Issue)
		out.add("Relation", args.Relation)
		out.add("Related issue", args.RelatedIssue)
	}
	return out.view()
}

func (t *issuesTool) approvalUpdateIssue(args issuesArgs, out *approvalFields) {
	current, ok := t.currentIssue(args.Issue)
	if !ok {
		out.add("Issue", args.Issue)
		out.add("Title", args.Title)
		out.add("State", t.stateName(args.State))
		out.add("Assignee", t.userName(args.Assignee))
		out.add("Project", t.projectName(args.Project))
		out.add("Priority", priorityName(args.Priority))
		out.add("Due", args.DueDate)
		out.add("Labels", t.labelList(args.Labels))
		out.add("Description", truncate(args.Description, 200))
		return
	}
	out.add("Issue", fmt.Sprintf("%s  %s", current.Identifier, current.Title))
	out.add("Title", change(current.Title, args.Title))
	out.add("Description", bodyChange(args.Description))
	out.add("State", change(stateName(current.State), t.stateName(args.State)))
	out.add("Assignee", change(userName(current.Assignee), t.userName(args.Assignee)))
	out.add("Project", change(projectName(current.Project), t.projectName(args.Project)))
	out.add("Priority", changePriority(current.Priority, args.Priority))
	out.add("Estimate", changeEstimate(current.Estimate, args.Estimate))
	out.add("Due", change(current.DueDate, args.DueDate))
	if args.Labels != nil {
		out.add("Labels", change(labelNames(current.Labels), t.labelList(args.Labels)))
	}
}

func (t *issuesTool) approvalArchiveIssue(args issuesArgs, out *approvalFields) {
	if current, ok := t.currentIssue(args.Issue); ok {
		out.add("Issue", fmt.Sprintf("%s  %s", current.Identifier, current.Title))
		out.add("State", stateName(current.State))
		out.add("Assignee", userName(current.Assignee))
		out.add("Project", projectName(current.Project))
		return
	}
	out.add("Issue", args.Issue)
}

// RenderResult draws a completed issues result.
func (t *issuesTool) RenderResult(_, raw json.RawMessage) gopi.ToolView {
	var result linearResult
	if json.Unmarshal(raw, &result) != nil {
		return gopi.ToolView{}
	}
	switch result.Operation {
	case "list_issues":
		lines := make([]string, 0, len(result.Issues))
		for _, issue := range result.Issues {
			lines = append(lines, issueLine(issue))
		}
		return lineView(lines, "No issues")
	case "get_issue":
		return issueFields(result.Issue)
	case "create_issue", "update_issue":
		if result.Issue == nil {
			return gopi.ToolView{}
		}
		return confirmation(issueVerb(result.Operation), result.Issue.Identifier, result.Issue.Title, result.Issue.URL)
	case "archive_issue":
		if result.Issue == nil {
			return gopi.ToolView{Lines: []string{"Archived"}}
		}
		return confirmation("Archived", result.Issue.Identifier, result.Issue.Title, "")
	case "create_relation":
		if result.Relation == nil {
			return gopi.ToolView{}
		}
		return confirmation("Related", relationTitle(result.Relation.Issue), relationTitle(result.Relation.RelatedIssue), "")
	}
	return gopi.ToolView{}
}

func issueFields(issue *issueView) gopi.ToolView {
	if issue == nil {
		return gopi.ToolView{}
	}
	return gopi.ToolView{Fields: compactFields([]gopi.ToolField{
		{Label: "Issue", Value: fmt.Sprintf("%s  %s", issue.Identifier, issue.Title)},
		{Label: "State", Value: stateName(issue.State)},
		{Label: "Assignee", Value: userName(issue.Assignee)},
		{Label: "Team", Value: teamLabel(issue.Team)},
		{Label: "Project", Value: projectName(issue.Project)},
		{Label: "Priority", Value: priorityLabel(issue.Priority)},
		{Label: "Due", Value: issue.DueDate},
		{Label: "Labels", Value: labelNames(issue.Labels)},
		{Label: "Description", Value: truncate(issue.Description, 400)},
		{Label: "Updated", Value: issue.UpdatedAt},
		{Label: "URL", Value: issue.URL},
	})}
}

// issueLine renders one issue as a compact list row.
func issueLine(issue issueSummary) string {
	parts := []string{issue.Identifier}
	if issue.State != nil && issue.State.Name != "" {
		parts = append(parts, issue.State.Name)
	}
	parts = append(parts, issue.Title)
	line := strings.Join(parts, "  ")
	if issue.Assignee != nil && issue.Assignee.Name != "" {
		line += "  @" + issue.Assignee.Name
	}
	return line
}

func issueVerb(operation string) string {
	if operation == "update_issue" {
		return "Updated"
	}
	return "Created"
}
