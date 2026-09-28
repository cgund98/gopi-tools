package linear

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/cgund98/gogent"
)

// fakeClient implements linearClient for testing. Each method calls its
// configured func, or reports that it was not implemented.
type fakeClient struct {
	viewerFunc            func(ctx context.Context) (viewerInfo, error)
	teamsFunc             func(ctx context.Context, filter map[string]any, opts listOptions) (connection[teamInfo], error)
	teamFunc              func(ctx context.Context, id string) (teamDetail, error)
	workflowStatesFunc    func(ctx context.Context, filter map[string]any, opts listOptions) (connection[workflowStateInfo], error)
	usersFunc             func(ctx context.Context, filter map[string]any, opts listOptions) (connection[userInfo], error)
	labelsFunc            func(ctx context.Context, filter map[string]any, opts listOptions) (connection[labelInfo], error)
	projectStatusesFunc   func(ctx context.Context, opts listOptions) (connection[projectStatusInfo], error)
	projectMilestonesFunc func(ctx context.Context, filter map[string]any, opts listOptions) (connection[milestoneInfo], error)
	issuesFunc            func(ctx context.Context, filter map[string]any, opts listOptions) (connection[issueSummary], error)
	issueFunc             func(ctx context.Context, id string) (issueDetail, error)
	projectsFunc          func(ctx context.Context, filter map[string]any, opts listOptions) (connection[projectSummary], error)
	projectFunc           func(ctx context.Context, id string) (projectDetail, error)
	commentFunc           func(ctx context.Context, id string) (commentInfo, error)
	createIssueFunc       func(ctx context.Context, in issueInput) (issueSummary, error)
	updateIssueFunc       func(ctx context.Context, id string, in issueInput) (issueSummary, error)
	archiveIssueFunc      func(ctx context.Context, id string) (issueRef, error)
	createCommentFunc     func(ctx context.Context, in commentInput) (commentInfo, error)
	updateCommentFunc     func(ctx context.Context, id, body string) (commentInfo, error)
	createProjectFunc     func(ctx context.Context, in projectInput) (projectSummary, error)
	updateProjectFunc     func(ctx context.Context, id string, in projectInput) (projectSummary, error)
	createMilestoneFunc   func(ctx context.Context, in milestoneInput) (milestoneInfo, error)
	updateMilestoneFunc   func(ctx context.Context, id string, in milestoneInput) (milestoneInfo, error)
	createLabelFunc       func(ctx context.Context, in labelInput) (labelInfo, error)
	createRelationFunc    func(ctx context.Context, in relationInput) (relationInfo, error)
}

var errNotImplemented = errors.New("not implemented")

func (f *fakeClient) viewer(ctx context.Context) (viewerInfo, error) {
	if f.viewerFunc != nil {
		return f.viewerFunc(ctx)
	}
	return viewerInfo{}, errNotImplemented
}

func (f *fakeClient) teams(ctx context.Context, filter map[string]any, opts listOptions) (connection[teamInfo], error) {
	if f.teamsFunc != nil {
		return f.teamsFunc(ctx, filter, opts)
	}
	return connection[teamInfo]{}, errNotImplemented
}

func (f *fakeClient) team(ctx context.Context, id string) (teamDetail, error) {
	if f.teamFunc != nil {
		return f.teamFunc(ctx, id)
	}
	return teamDetail{}, errNotImplemented
}

func (f *fakeClient) workflowStates(ctx context.Context, filter map[string]any, opts listOptions) (connection[workflowStateInfo], error) {
	if f.workflowStatesFunc != nil {
		return f.workflowStatesFunc(ctx, filter, opts)
	}
	return connection[workflowStateInfo]{}, errNotImplemented
}

func (f *fakeClient) users(ctx context.Context, filter map[string]any, opts listOptions) (connection[userInfo], error) {
	if f.usersFunc != nil {
		return f.usersFunc(ctx, filter, opts)
	}
	return connection[userInfo]{}, errNotImplemented
}

func (f *fakeClient) labels(ctx context.Context, filter map[string]any, opts listOptions) (connection[labelInfo], error) {
	if f.labelsFunc != nil {
		return f.labelsFunc(ctx, filter, opts)
	}
	return connection[labelInfo]{}, errNotImplemented
}

func (f *fakeClient) projectStatuses(ctx context.Context, opts listOptions) (connection[projectStatusInfo], error) {
	if f.projectStatusesFunc != nil {
		return f.projectStatusesFunc(ctx, opts)
	}
	return connection[projectStatusInfo]{}, errNotImplemented
}

func (f *fakeClient) projectMilestones(ctx context.Context, filter map[string]any, opts listOptions) (connection[milestoneInfo], error) {
	if f.projectMilestonesFunc != nil {
		return f.projectMilestonesFunc(ctx, filter, opts)
	}
	return connection[milestoneInfo]{}, errNotImplemented
}

func (f *fakeClient) issues(ctx context.Context, filter map[string]any, opts listOptions) (connection[issueSummary], error) {
	if f.issuesFunc != nil {
		return f.issuesFunc(ctx, filter, opts)
	}
	return connection[issueSummary]{}, errNotImplemented
}

func (f *fakeClient) issue(ctx context.Context, id string) (issueDetail, error) {
	if f.issueFunc != nil {
		return f.issueFunc(ctx, id)
	}
	return issueDetail{}, errNotImplemented
}

func (f *fakeClient) projects(ctx context.Context, filter map[string]any, opts listOptions) (connection[projectSummary], error) {
	if f.projectsFunc != nil {
		return f.projectsFunc(ctx, filter, opts)
	}
	return connection[projectSummary]{}, errNotImplemented
}

func (f *fakeClient) project(ctx context.Context, id string) (projectDetail, error) {
	if f.projectFunc != nil {
		return f.projectFunc(ctx, id)
	}
	return projectDetail{}, errNotImplemented
}

func (f *fakeClient) comment(ctx context.Context, id string) (commentInfo, error) {
	if f.commentFunc != nil {
		return f.commentFunc(ctx, id)
	}
	return commentInfo{}, errNotImplemented
}

func (f *fakeClient) createIssue(ctx context.Context, in issueInput) (issueSummary, error) {
	if f.createIssueFunc != nil {
		return f.createIssueFunc(ctx, in)
	}
	return issueSummary{}, errNotImplemented
}

func (f *fakeClient) updateIssue(ctx context.Context, id string, in issueInput) (issueSummary, error) {
	if f.updateIssueFunc != nil {
		return f.updateIssueFunc(ctx, id, in)
	}
	return issueSummary{}, errNotImplemented
}

func (f *fakeClient) archiveIssue(ctx context.Context, id string) (issueRef, error) {
	if f.archiveIssueFunc != nil {
		return f.archiveIssueFunc(ctx, id)
	}
	return issueRef{}, errNotImplemented
}

func (f *fakeClient) createComment(ctx context.Context, in commentInput) (commentInfo, error) {
	if f.createCommentFunc != nil {
		return f.createCommentFunc(ctx, in)
	}
	return commentInfo{}, errNotImplemented
}

func (f *fakeClient) updateComment(ctx context.Context, id, body string) (commentInfo, error) {
	if f.updateCommentFunc != nil {
		return f.updateCommentFunc(ctx, id, body)
	}
	return commentInfo{}, errNotImplemented
}

func (f *fakeClient) createProject(ctx context.Context, in projectInput) (projectSummary, error) {
	if f.createProjectFunc != nil {
		return f.createProjectFunc(ctx, in)
	}
	return projectSummary{}, errNotImplemented
}

func (f *fakeClient) updateProject(ctx context.Context, id string, in projectInput) (projectSummary, error) {
	if f.updateProjectFunc != nil {
		return f.updateProjectFunc(ctx, id, in)
	}
	return projectSummary{}, errNotImplemented
}

func (f *fakeClient) createMilestone(ctx context.Context, in milestoneInput) (milestoneInfo, error) {
	if f.createMilestoneFunc != nil {
		return f.createMilestoneFunc(ctx, in)
	}
	return milestoneInfo{}, errNotImplemented
}

func (f *fakeClient) updateMilestone(ctx context.Context, id string, in milestoneInput) (milestoneInfo, error) {
	if f.updateMilestoneFunc != nil {
		return f.updateMilestoneFunc(ctx, id, in)
	}
	return milestoneInfo{}, errNotImplemented
}

func (f *fakeClient) createLabel(ctx context.Context, in labelInput) (labelInfo, error) {
	if f.createLabelFunc != nil {
		return f.createLabelFunc(ctx, in)
	}
	return labelInfo{}, errNotImplemented
}

func (f *fakeClient) createRelation(ctx context.Context, in relationInput) (relationInfo, error) {
	if f.createRelationFunc != nil {
		return f.createRelationFunc(ctx, in)
	}
	return relationInfo{}, errNotImplemented
}

// newTestShared builds the state a tool reads, around a fake client. Passing nil
// leaves the client unset, so client setup is attempted against the real
// endpoint.
func newTestShared(client linearClient) *shared {
	s := newShared(Config{DefaultTeam: "ENG"})
	if client != nil {
		s.client = client
		s.resolved = newResolver(client, &s.names, "ENG")
	}
	return s
}

// The per-tool constructors keep the tests reading like the old single-tool
// ones, while each test now exercises the tool that owns the operation.

func newIssues(client linearClient) *issuesTool {
	return &issuesTool{shared: newTestShared(client)}
}

func newProjects(client linearClient) *projectsTool {
	return &projectsTool{shared: newTestShared(client)}
}

func newTeams(client linearClient) *teamsTool {
	return &teamsTool{shared: newTestShared(client)}
}

func newUsers(client linearClient) *usersTool {
	return &usersTool{shared: newTestShared(client)}
}

func newLabels(client linearClient) *labelsTool {
	return &labelsTool{shared: newTestShared(client)}
}

func newComments(client linearClient) *commentsTool {
	return &commentsTool{shared: newTestShared(client)}
}

// decodeResult unmarshals a tool result into a generic map.
func decodeResult(t *testing.T, raw json.RawMessage) map[string]any {
	t.Helper()
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}
	return payload
}

// parameterSchema is the part of a JSON Schema the tests inspect.
type parameterSchema struct {
	Type       string                     `json:"type"`
	Properties map[string]json.RawMessage `json:"properties"`
}

func schemaOf(t *testing.T, tool gogent.Tool) parameterSchema {
	t.Helper()
	var schema parameterSchema
	if err := json.Unmarshal(tool.Parameters(), &schema); err != nil {
		t.Fatalf("Parameters() is not valid JSON: %v", err)
	}
	return schema
}

func operationEnum(t *testing.T, schema parameterSchema) []string {
	t.Helper()
	raw, ok := schema.Properties["operation"]
	if !ok {
		t.Fatal("schema has no operation property")
	}
	var prop struct {
		Enum []string `json:"enum"`
	}
	if err := json.Unmarshal(raw, &prop); err != nil {
		t.Fatalf("operation property is not valid JSON: %v", err)
	}
	return prop.Enum
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func TestTools_Names(t *testing.T) {
	want := []string{
		"linear_issues",
		"linear_projects",
		"linear_comments",
		"linear_teams",
		"linear_users",
		"linear_labels",
	}
	tools := Tools(Config{})
	if len(tools) != len(want) {
		t.Fatalf("Tools() returned %d tools, want %d", len(tools), len(want))
	}
	for i, tool := range tools {
		if got := tool.Name(); got != want[i] {
			t.Errorf("tool %d name = %q, want %q", i, got, want[i])
		}
	}
}

// TestTools_ShareState is the invariant the whole split rests on: separate
// tools, one client, one resolver, one name cache.
func TestTools_ShareState(t *testing.T) {
	tools := Tools(Config{DefaultTeam: "ENG"})
	issues, ok := tools[0].(*issuesTool)
	if !ok {
		t.Fatalf("tools[0] is %T, want *issuesTool", tools[0])
	}
	labels, ok := tools[len(tools)-1].(*labelsTool)
	if !ok {
		t.Fatalf("tools[last] is %T, want *labelsTool", tools[len(tools)-1])
	}
	if issues.shared != labels.shared {
		t.Fatal("every tool from Tools() must share one state")
	}
}

func TestOptions_Count(t *testing.T) {
	// Six tools in agent mode, plus the five with a read operation in plan and
	// ask mode. The comments tool has no read operation, so it is agent-only.
	const want = 6 + 5*2
	if got := len(Options()); got != want {
		t.Fatalf("Options() returned %d options, want %d", got, want)
	}
}

func TestTools_Parameters(t *testing.T) {
	tests := []struct {
		name       string
		tool       gogent.Tool
		want       []string
		absent     []string
		listFields bool
	}{
		{
			name:       "issues",
			tool:       newIssues(nil),
			want:       []string{"list_issues", "get_issue", "create_issue", "update_issue", "archive_issue", "create_relation"},
			absent:     []string{"create_project", "get_viewer", "create_label"},
			listFields: true,
		},
		{
			name:       "projects",
			tool:       newProjects(nil),
			want:       []string{"list_projects", "get_project", "create_project", "update_project", "create_milestone", "update_milestone"},
			absent:     []string{"list_issues", "create_issue", "get_viewer"},
			listFields: true,
		},
		{
			name:   "comments",
			tool:   newComments(nil),
			want:   []string{"create_comment", "update_comment"},
			absent: []string{"list_issues", "get_viewer"},
		},
		{
			name:       "teams",
			tool:       newTeams(nil),
			want:       []string{"list_teams", "get_team"},
			absent:     []string{"create_issue", "get_viewer"},
			listFields: true,
		},
		{
			name:       "users",
			tool:       newUsers(nil),
			want:       []string{"get_viewer", "list_users"},
			absent:     []string{"create_issue", "list_issues"},
			listFields: true,
		},
		{
			name:       "labels",
			tool:       newLabels(nil),
			want:       []string{"list_labels", "create_label"},
			absent:     []string{"create_issue", "get_viewer"},
			listFields: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema := schemaOf(t, tt.tool)
			if schema.Type != "object" {
				t.Fatalf("schema type = %q, want object", schema.Type)
			}
			enum := operationEnum(t, schema)
			for _, want := range tt.want {
				if !contains(enum, want) {
					t.Errorf("operation enum is missing %q: %v", want, enum)
				}
			}
			for _, absent := range tt.absent {
				if contains(enum, absent) {
					t.Errorf("operation enum must not carry %q: %v", absent, enum)
				}
			}
			if !tt.listFields {
				return
			}
			// The embedded listArgs must inline at the top level of the schema.
			for _, field := range []string{"query", "include_archived", "updated_since", "created_since", "limit", "cursor", "order_by"} {
				if _, ok := schema.Properties[field]; !ok {
					t.Errorf("schema is missing the shared list field %q", field)
				}
			}
		})
	}
}

func TestRequiresApproval(t *testing.T) {
	withIssue := func() *fakeClient {
		return &fakeClient{
			issueFunc: func(_ context.Context, _ string) (issueDetail, error) {
				return issueDetail{ID: "uuid-1", Identifier: "ENG-1", Title: "Fix login redirect"}, nil
			},
		}
	}
	tests := []struct {
		name         string
		tool         gogent.Tool
		args         string
		wantRequired bool
		wantReason   string
	}{
		{name: "get_viewer auto-approves", tool: newUsers(nil), args: `{"operation":"get_viewer"}`},
		{name: "list_issues auto-approves", tool: newIssues(nil), args: `{"operation":"list_issues"}`},
		{name: "get_issue auto-approves", tool: newIssues(nil), args: `{"operation":"get_issue","issue":"ENG-1"}`},
		{
			name:         "create_issue requires approval",
			tool:         newIssues(nil),
			args:         `{"operation":"create_issue","title":"Fix bug"}`,
			wantRequired: true,
			wantReason:   `Create issue "Fix bug" in team ENG`,
		},
		{
			name:         "update_issue requires approval",
			tool:         newIssues(withIssue()),
			args:         `{"operation":"update_issue","issue":"ENG-1"}`,
			wantRequired: true,
			wantReason:   `Update issue ENG-1 "Fix login redirect"`,
		},
		{
			name:         "archive_issue requires approval",
			tool:         newIssues(withIssue()),
			args:         `{"operation":"archive_issue","issue":"ENG-1"}`,
			wantRequired: true,
			wantReason:   `Archive issue ENG-1 "Fix login redirect"`,
		},
		{
			name:         "create_project requires approval",
			tool:         newProjects(nil),
			args:         `{"operation":"create_project","name":"Q3"}`,
			wantRequired: true,
			wantReason:   `Create project "Q3"`,
		},
		{
			name:         "create_comment requires approval",
			tool:         newComments(nil),
			args:         `{"operation":"create_comment","issue":"ENG-1","body":"hi"}`,
			wantRequired: true,
			wantReason:   `Comment on issue ENG-1`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decision, err := tt.tool.RequiresApproval(context.Background(), json.RawMessage(tt.args))
			if err != nil {
				t.Fatalf("RequiresApproval error: %v", err)
			}
			if decision.Required != tt.wantRequired {
				t.Fatalf("Required = %v, want %v", decision.Required, tt.wantRequired)
			}
			if tt.wantReason != "" && decision.Reason != tt.wantReason {
				t.Fatalf("Reason = %q, want %q", decision.Reason, tt.wantReason)
			}
		})
	}
}

func TestUsers_Execute_GetViewer(t *testing.T) {
	tool := newUsers(&fakeClient{
		viewerFunc: func(context.Context) (viewerInfo, error) {
			return viewerInfo{ID: "u1", Name: "Ada", Email: "ada@example.com"}, nil
		},
	})
	result, err := tool.Execute(context.Background(), json.RawMessage(`{"operation":"get_viewer"}`))
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	payload := decodeResult(t, result)
	if payload["error"] != nil {
		t.Fatalf("unexpected error payload: %v", payload)
	}
	viewer, ok := payload["viewer"].(map[string]any)
	if !ok || viewer["name"] != "Ada" {
		t.Fatalf("viewer = %v, want name Ada", payload["viewer"])
	}
}

func TestIssues_Execute_ListIssues(t *testing.T) {
	var gotFilter map[string]any
	var gotOpts listOptions
	tool := newIssues(&fakeClient{
		viewerFunc: func(context.Context) (viewerInfo, error) {
			return viewerInfo{ID: "u1", Name: "Ada"}, nil
		},
		issuesFunc: func(_ context.Context, filter map[string]any, opts listOptions) (connection[issueSummary], error) {
			gotFilter = filter
			gotOpts = opts
			return connection[issueSummary]{
				Nodes:    []issueSummary{{Identifier: "ENG-1", Title: "Fix bug"}},
				PageInfo: &pageInfo{HasNextPage: true, EndCursor: "cur"},
			}, nil
		},
	})
	args := `{"operation":"list_issues","assignee":"me","state":"started","query":"login","limit":10}`
	result, err := tool.Execute(context.Background(), json.RawMessage(args))
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	payload := decodeResult(t, result)
	if payload["error"] != nil {
		t.Fatalf("unexpected error payload: %v", payload)
	}
	issues, ok := payload["issues"].([]any)
	if !ok || len(issues) != 1 {
		t.Fatalf("issues = %v, want 1 issue", payload["issues"])
	}
	page, ok := payload["page_info"].(map[string]any)
	if !ok || page["end_cursor"] != "cur" {
		t.Fatalf("page_info = %v, want end_cursor cur", payload["page_info"])
	}
	if gotOpts.First != 10 {
		t.Fatalf("limit = %d, want 10", gotOpts.First)
	}
	if _, ok := gotFilter["assignee"]; !ok {
		t.Fatalf("filter is missing assignee: %v", gotFilter)
	}
	if _, ok := gotFilter["state"]; !ok {
		t.Fatalf("filter is missing state: %v", gotFilter)
	}
	if _, ok := gotFilter["or"]; !ok {
		t.Fatalf("filter is missing the query or-clause: %v", gotFilter)
	}
}

func TestIssues_Execute_ListIssues_DefaultLimit(t *testing.T) {
	var gotOpts listOptions
	tool := newIssues(&fakeClient{
		issuesFunc: func(_ context.Context, _ map[string]any, opts listOptions) (connection[issueSummary], error) {
			gotOpts = opts
			return connection[issueSummary]{}, nil
		},
	})
	if _, err := tool.Execute(context.Background(), json.RawMessage(`{"operation":"list_issues"}`)); err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if gotOpts.First != defaultPageSize {
		t.Fatalf("first = %d, want %d", gotOpts.First, defaultPageSize)
	}
	if gotOpts.OrderBy != "updatedAt" {
		t.Fatalf("orderBy = %q, want updatedAt", gotOpts.OrderBy)
	}
}

func TestIssues_Execute_ListIssues_LimitTooLarge(t *testing.T) {
	tool := newIssues(&fakeClient{})
	result, err := tool.Execute(context.Background(), json.RawMessage(`{"operation":"list_issues","limit":500}`))
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	payload := decodeResult(t, result)
	if payload["error"] != "invalid_argument" {
		t.Fatalf("error = %v, want invalid_argument", payload["error"])
	}
}

func TestIssues_Execute_CreateIssue(t *testing.T) {
	var got issueInput
	tool := newIssues(&fakeClient{
		teamsFunc: func(_ context.Context, _ map[string]any, _ listOptions) (connection[teamInfo], error) {
			return connection[teamInfo]{Nodes: []teamInfo{{ID: "team-1", Key: "ENG", Name: "Engineering"}}}, nil
		},
		workflowStatesFunc: func(context.Context, map[string]any, listOptions) (connection[workflowStateInfo], error) {
			return connection[workflowStateInfo]{Nodes: []workflowStateInfo{{ID: "state-1", Name: "In Progress", Type: "started"}}}, nil
		},
		labelsFunc: func(context.Context, map[string]any, listOptions) (connection[labelInfo], error) {
			return connection[labelInfo]{Nodes: []labelInfo{{ID: "label-1", Name: "Bug"}}}, nil
		},
		createIssueFunc: func(_ context.Context, in issueInput) (issueSummary, error) {
			got = in
			return issueSummary{ID: "issue-1", Identifier: "ENG-7", Title: in.Title}, nil
		},
	})
	args := `{"operation":"create_issue","title":"New bug","state":"started","labels":["Bug"],"priority":2}`
	result, err := tool.Execute(context.Background(), json.RawMessage(args))
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	payload := decodeResult(t, result)
	if payload["error"] != nil {
		t.Fatalf("unexpected error payload: %v", payload)
	}
	if got.TeamID != "team-1" {
		t.Fatalf("teamId = %q, want team-1 (from default team)", got.TeamID)
	}
	if got.StateID != "state-1" {
		t.Fatalf("stateId = %q, want state-1", got.StateID)
	}
	if len(got.LabelIDs) != 1 || got.LabelIDs[0] != "label-1" {
		t.Fatalf("labelIds = %v, want [label-1]", got.LabelIDs)
	}
	if got.Priority == nil || *got.Priority != 2 {
		t.Fatalf("priority = %v, want 2", got.Priority)
	}
}

// TestIssues_Execute_CreateIssue_LabelScope checks that a workspace-wide label,
// which Linear reports with an empty team, resolves for an issue in a team. A
// team-scoped lookup used to drop it, so a workspace label was unassignable.
func TestIssues_Execute_CreateIssue_LabelScope(t *testing.T) {
	tests := []struct {
		name   string
		labels []labelInfo
		want   string
	}{
		{
			name:   "workspace label when the team owns no label of that name",
			labels: []labelInfo{{ID: "label-feature", Name: "Feature"}},
			want:   "label-feature",
		},
		{
			name: "team label wins over the workspace label",
			labels: []labelInfo{
				{ID: "label-global", Name: "Feature"},
				{ID: "label-team", Name: "Feature", Team: &labelTeam{ID: "team-1"}},
			},
			want: "label-team",
		},
		{
			name:   "another team's label does not match",
			labels: []labelInfo{{ID: "label-other", Name: "Feature", Team: &labelTeam{ID: "team-2"}}},
			want:   "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got issueInput
			var filter map[string]any
			tool := newIssues(&fakeClient{
				teamsFunc: func(_ context.Context, _ map[string]any, _ listOptions) (connection[teamInfo], error) {
					return connection[teamInfo]{Nodes: []teamInfo{{ID: "team-1", Key: "ENG", Name: "Engineering"}}}, nil
				},
				labelsFunc: func(_ context.Context, f map[string]any, _ listOptions) (connection[labelInfo], error) {
					filter = f
					return connection[labelInfo]{Nodes: tt.labels}, nil
				},
				createIssueFunc: func(_ context.Context, in issueInput) (issueSummary, error) {
					got = in
					return issueSummary{ID: "issue-1", Identifier: "ENG-7", Title: in.Title}, nil
				},
			})
			args := `{"operation":"create_issue","title":"New feature","labels":["Feature"]}`
			result, err := tool.Execute(context.Background(), json.RawMessage(args))
			if err != nil {
				t.Fatalf("Execute error: %v", err)
			}
			if _, ok := filter["team"]; ok {
				t.Fatalf("label lookup filter = %v, want no team filter", filter)
			}
			payload := decodeResult(t, result)
			if tt.want == "" {
				if payload["error"] != "create_issue_failed" {
					t.Fatalf("error = %v, want create_issue_failed", payload["error"])
				}
				return
			}
			if payload["error"] != nil {
				t.Fatalf("unexpected error payload: %v", payload)
			}
			if len(got.LabelIDs) != 1 || got.LabelIDs[0] != tt.want {
				t.Fatalf("labelIds = %v, want [%s]", got.LabelIDs, tt.want)
			}
		})
	}
}

func TestIssues_Execute_CreateIssue_MissingTitle(t *testing.T) {
	tool := newIssues(&fakeClient{})
	result, err := tool.Execute(context.Background(), json.RawMessage(`{"operation":"create_issue"}`))
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	payload := decodeResult(t, result)
	if payload["error"] != "invalid_argument" {
		t.Fatalf("error = %v, want invalid_argument", payload["error"])
	}
}

func TestIssues_Execute_TeamMustBeUUID(t *testing.T) {
	tool := newIssues(&fakeClient{})
	result, err := tool.Execute(context.Background(), json.RawMessage(`{"operation":"create_issue","title":"New bug","team":"ENG"}`))
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	payload := decodeResult(t, result)
	if payload["error"] != "invalid_argument" {
		t.Fatalf("error = %v, want invalid_argument", payload["error"])
	}
	message, _ := payload["message"].(string)
	if !strings.Contains(message, "list_teams") {
		t.Fatalf("message = %q, want it to name list_teams", message)
	}
}

func TestIssues_Execute_ProjectMustBeUUID(t *testing.T) {
	tool := newIssues(&fakeClient{})
	args := `{"operation":"create_issue","title":"New bug","project":"NeuralScale Desktop App"}`
	result, err := tool.Execute(context.Background(), json.RawMessage(args))
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	payload := decodeResult(t, result)
	if payload["error"] != "invalid_argument" {
		t.Fatalf("error = %v, want invalid_argument", payload["error"])
	}
	message, _ := payload["message"].(string)
	if !strings.Contains(message, "list_projects") {
		t.Fatalf("message = %q, want it to name list_projects", message)
	}
}

// TestIssues_Execute_RecordsResolvedTeamName covers the resolver as a source for
// the display cache: the default team is resolved during the write, and its name
// is recorded for the next render.
func TestIssues_Execute_RecordsResolvedTeamName(t *testing.T) {
	const teamUUID = "254e39f7-6915-4b53-8371-ec981a2c1e06"
	tool := newIssues(&fakeClient{
		teamsFunc: func(context.Context, map[string]any, listOptions) (connection[teamInfo], error) {
			return connection[teamInfo]{Nodes: []teamInfo{{ID: teamUUID, Key: "ENG", Name: "Engineering"}}}, nil
		},
		createIssueFunc: func(_ context.Context, in issueInput) (issueSummary, error) {
			if in.TeamID != teamUUID {
				t.Fatalf("teamId = %q, want %q from the default team", in.TeamID, teamUUID)
			}
			return issueSummary{ID: "issue-1", Identifier: "ENG-7", Title: in.Title}, nil
		},
	})
	if _, err := tool.Execute(context.Background(), json.RawMessage(`{"operation":"create_issue","title":"New bug"}`)); err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	got, ok := tool.names.team(teamUUID)
	if !ok || got != "ENG  Engineering" {
		t.Fatalf("cached team = %q, %v; want the resolved name", got, ok)
	}
}

// TestResolver_OmitsIDBranchForNonUUID covers the bug where every resolver added
// an id filter for a plain name, which Linear rejects outright instead of
// matching nothing.
func TestResolver_OmitsIDBranchForNonUUID(t *testing.T) {
	const teamUUID = "254e39f7-6915-4b53-8371-ec981a2c1e06"
	tests := []struct {
		name         string
		ref          string
		wantIDBranch bool
	}{
		{name: "team key", ref: "ENG"},
		{name: "team name", ref: "Engineering"},
		{name: "team uuid", ref: teamUUID, wantIDBranch: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var filter map[string]any
			client := &fakeClient{
				teamsFunc: func(_ context.Context, f map[string]any, _ listOptions) (connection[teamInfo], error) {
					filter = f
					return connection[teamInfo]{Nodes: []teamInfo{{ID: teamUUID, Key: "ENG", Name: "Engineering"}}}, nil
				},
			}
			r := newResolver(client, nil, "")
			if _, err := r.team(context.Background(), tt.ref); err != nil {
				t.Fatalf("team resolve: %v", err)
			}
			if got := hasIDBranch(filter, tt.ref); got != tt.wantIDBranch {
				t.Fatalf("id branch for %q = %v, want %v (filter %v)", tt.ref, got, tt.wantIDBranch, filter)
			}
		})
	}
}

func TestResolver_OmitsIDBranchForAssigneeEmail(t *testing.T) {
	var filter map[string]any
	client := &fakeClient{
		usersFunc: func(_ context.Context, f map[string]any, _ listOptions) (connection[userInfo], error) {
			filter = f
			return connection[userInfo]{Nodes: []userInfo{{ID: "user-1", Name: "Callum"}}}, nil
		},
	}
	r := newResolver(client, nil, "")
	if _, err := r.user(context.Background(), "callum@synchroni.co"); err != nil {
		t.Fatalf("user resolve: %v", err)
	}
	if hasIDBranch(filter, "callum@synchroni.co") {
		t.Fatalf("filter carries an id branch for an email: %v", filter)
	}
}

// TestResolver_OmitsTypeBranchForStateName covers the enum half of the same bug:
// a state name is not a valid workflow state type.
func TestResolver_OmitsTypeBranchForStateName(t *testing.T) {
	var filter map[string]any
	client := &fakeClient{
		workflowStatesFunc: func(_ context.Context, f map[string]any, _ listOptions) (connection[workflowStateInfo], error) {
			filter = f
			return connection[workflowStateInfo]{Nodes: []workflowStateInfo{{ID: "state-1", Name: "In Review", Type: "started"}}}, nil
		},
	}
	r := newResolver(client, nil, "")
	if _, err := r.state(context.Background(), "team-1", "In Review"); err != nil {
		t.Fatalf("state resolve: %v", err)
	}
	if hasTypeBranch(filter) {
		t.Fatalf("filter carries a type branch for a state name: %v", filter)
	}
}

// hasIDBranch reports whether an "or" filter carries an id equality branch.
func hasIDBranch(filter map[string]any, ref string) bool {
	branches, _ := filter["or"].([]map[string]any)
	for _, branch := range branches {
		id, ok := branch["id"].(map[string]any)
		if !ok {
			continue
		}
		if id["eq"] == ref {
			return true
		}
	}
	return false
}

// hasTypeBranch reports whether an "or" filter carries a type branch.
func hasTypeBranch(filter map[string]any) bool {
	branches, _ := filter["or"].([]map[string]any)
	for _, branch := range branches {
		if _, ok := branch["type"]; ok {
			return true
		}
	}
	return false
}

func TestIssues_Execute_UpdateIssue_ResolvesStateFromIssueTeam(t *testing.T) {
	var gotID string
	var got issueInput
	tool := newIssues(&fakeClient{
		issueFunc: func(_ context.Context, _ string) (issueDetail, error) {
			return issueDetail{
				ID:         "issue-1",
				Identifier: "ENG-1",
				Title:      "Fix bug",
				Team:       &teamInfo{ID: "team-9", Key: "ENG", Name: "Engineering"},
			}, nil
		},
		workflowStatesFunc: func(_ context.Context, _ map[string]any, _ listOptions) (connection[workflowStateInfo], error) {
			return connection[workflowStateInfo]{Nodes: []workflowStateInfo{{ID: "state-2", Name: "Done", Type: "completed"}}}, nil
		},
		updateIssueFunc: func(_ context.Context, id string, in issueInput) (issueSummary, error) {
			gotID = id
			got = in
			return issueSummary{Identifier: "ENG-1", Title: "Fix bug"}, nil
		},
	})
	// The state lookup is scoped to the issue's own team when none is given.
	tool.resolved = newResolver(&capturingTeamResolver{linearClient: tool.client, wantTeam: "team-9"}, &tool.names, "ENG")
	result, err := tool.Execute(context.Background(), json.RawMessage(`{"operation":"update_issue","issue":"ENG-1","state":"Done"}`))
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if payload := decodeResult(t, result); payload["error"] != nil {
		t.Fatalf("unexpected error payload: %v", payload)
	}
	if gotID != "ENG-1" {
		t.Fatalf("issue id = %q, want ENG-1", gotID)
	}
	if got.StateID != "state-2" {
		t.Fatalf("stateId = %q, want state-2", got.StateID)
	}
	if got.TeamID != "" {
		t.Fatalf("teamId = %q, want empty so the update does not move the issue", got.TeamID)
	}
}

// capturingTeamResolver wraps a client and asserts that a workflow state lookup
// is scoped to the expected team.
type capturingTeamResolver struct {
	linearClient
	wantTeam string
}

func (c *capturingTeamResolver) workflowStates(ctx context.Context, filter map[string]any, opts listOptions) (connection[workflowStateInfo], error) {
	team, ok := filter["team"].(map[string]any)
	if !ok {
		return connection[workflowStateInfo]{}, errors.New("state lookup did not scope to a team")
	}
	id, _ := team["id"].(map[string]any)["eq"].(string)
	if id != c.wantTeam {
		return connection[workflowStateInfo]{}, errors.New("state lookup scoped to the wrong team")
	}
	return c.linearClient.workflowStates(ctx, filter, opts)
}

func TestIssues_Execute_ArchiveIssue(t *testing.T) {
	tool := newIssues(&fakeClient{
		archiveIssueFunc: func(_ context.Context, id string) (issueRef, error) {
			if id != "ENG-1" {
				t.Fatalf("archive id = %q, want ENG-1", id)
			}
			return issueRef{ID: "issue-1", Identifier: "ENG-1", Title: "Fix bug"}, nil
		},
	})
	result, err := tool.Execute(context.Background(), json.RawMessage(`{"operation":"archive_issue","issue":"ENG-1"}`))
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if payload := decodeResult(t, result); payload["error"] != nil {
		t.Fatalf("unexpected error payload: %v", payload)
	}
}

func TestIssues_Execute_CreateRelation_BlockedBy(t *testing.T) {
	var got relationInput
	tool := newIssues(&fakeClient{
		issueFunc: func(_ context.Context, identifier string) (issueDetail, error) {
			return issueDetail{ID: "uuid-" + identifier, Identifier: identifier}, nil
		},
		createRelationFunc: func(_ context.Context, in relationInput) (relationInfo, error) {
			got = in
			return relationInfo{Type: in.Type}, nil
		},
	})
	args := `{"operation":"create_relation","issue":"ENG-1","related_issue":"ENG-2","relation":"blocked_by"}`
	result, err := tool.Execute(context.Background(), json.RawMessage(args))
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if payload := decodeResult(t, result); payload["error"] != nil {
		t.Fatalf("unexpected error payload: %v", payload)
	}
	// blocked_by means the related issue blocks this one, so the roles swap.
	if got.IssueID != "uuid-ENG-2" || got.RelatedIssueID != "uuid-ENG-1" || got.Type != "blocks" {
		t.Fatalf("relation input = %+v, want ENG-2 blocks ENG-1", got)
	}
}

// TestIssues_Execute_UnknownOperationIsRejectedAlsoByOtherTools covers that each
// tool refuses an operation that belongs to a different tool.
func TestIssues_Execute_UnknownOperationIsRejectedAlsoByOtherTools(t *testing.T) {
	tests := []struct {
		name string
		tool gogent.Tool
		args string
	}{
		{name: "issues rejects an unknown name", tool: newIssues(&fakeClient{}), args: `{"operation":"magic"}`},
		{name: "issues reject another tool's operation", tool: newIssues(&fakeClient{}), args: `{"operation":"create_project"}`},
		{name: "projects reject another tool's operation", tool: newProjects(&fakeClient{}), args: `{"operation":"list_issues"}`},
		{name: "teams reject another tool's operation", tool: newTeams(&fakeClient{}), args: `{"operation":"get_viewer"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.tool.Execute(context.Background(), json.RawMessage(tt.args))
			if err != nil {
				t.Fatalf("Execute error: %v", err)
			}
			payload := decodeResult(t, result)
			if payload["error"] != "invalid_operation" {
				t.Fatalf("error = %v, want invalid_operation", payload["error"])
			}
		})
	}
}

func TestIssues_Execute_ErrorMapping(t *testing.T) {
	tests := []struct {
		name     string
		client   *fakeClient
		wantCode string
	}{
		{
			name: "rate limited",
			client: &fakeClient{
				issuesFunc: func(context.Context, map[string]any, listOptions) (connection[issueSummary], error) {
					return connection[issueSummary]{}, &apiError{Kind: "rate_limited", Message: "slow down"}
				},
			},
			wantCode: "rate_limited",
		},
		{
			name: "permission denied",
			client: &fakeClient{
				issuesFunc: func(context.Context, map[string]any, listOptions) (connection[issueSummary], error) {
					return connection[issueSummary]{}, &apiError{Kind: "permission_denied", Message: "no access"}
				},
			},
			wantCode: "permission_denied",
		},
		{
			name: "generic api error",
			client: &fakeClient{
				issuesFunc: func(context.Context, map[string]any, listOptions) (connection[issueSummary], error) {
					return connection[issueSummary]{}, errors.New("boom")
				},
			},
			wantCode: "list_issues_failed",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tool := newIssues(tt.client)
			result, err := tool.Execute(context.Background(), json.RawMessage(`{"operation":"list_issues"}`))
			if err != nil {
				t.Fatalf("Execute error: %v", err)
			}
			payload := decodeResult(t, result)
			if payload["error"] != tt.wantCode {
				t.Fatalf("error = %v, want %v", payload["error"], tt.wantCode)
			}
		})
	}
}

func TestIssues_ReadOnly_RejectsWrites(t *testing.T) {
	tool := newIssues(&fakeClient{
		createIssueFunc: func(context.Context, issueInput) (issueSummary, error) {
			t.Fatal("create_issue must not reach the client in read-only mode")
			return issueSummary{}, nil
		},
	})
	tool.readonly = true

	decision, err := tool.RequiresApproval(context.Background(), json.RawMessage(`{"operation":"create_issue","title":"x"}`))
	if err != nil {
		t.Fatalf("RequiresApproval error: %v", err)
	}
	if decision.Required {
		t.Fatal("read-only mode must not request approval")
	}

	result, err := tool.Execute(context.Background(), json.RawMessage(`{"operation":"create_issue","title":"x"}`))
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	payload := decodeResult(t, result)
	if payload["error"] != "permission_denied" {
		t.Fatalf("error = %v, want permission_denied", payload["error"])
	}
}

// TestIssues_ReadOnly_EnumOmitsWrites covers that a read-only tool advertises
// only what it can run, so the model is never offered a write it cannot make.
func TestIssues_ReadOnly_EnumOmitsWrites(t *testing.T) {
	tool := &issuesTool{shared: newTestShared(nil), readonly: true}
	enum := operationEnum(t, schemaOf(t, tool))
	if contains(enum, "create_issue") || contains(enum, "update_issue") || contains(enum, "create_relation") {
		t.Fatalf("read-only enum still carries writes: %v", enum)
	}
	for _, want := range []string{"list_issues", "get_issue"} {
		if !contains(enum, want) {
			t.Errorf("read-only enum is missing the read %q: %v", want, enum)
		}
	}
}

func TestProjectInput_Variables(t *testing.T) {
	in := projectInput{Name: "Q3 cleanup", Summary: "Short note", TeamIDs: []string{"team-1"}}
	vars := in.variables()
	// Linear's project summary field is named content.
	if vars["content"] != "Short note" {
		t.Fatalf("content = %v, want %q", vars["content"], "Short note")
	}
	if _, ok := vars["summary"]; ok {
		t.Fatal("summary is not a Linear project input field; it must map to content")
	}
	if vars["name"] != "Q3 cleanup" {
		t.Fatalf("name = %v, want Q3 cleanup", vars["name"])
	}
	teams, ok := vars["teamIds"].([]string)
	if !ok || len(teams) != 1 || teams[0] != "team-1" {
		t.Fatalf("teamIds = %v, want [team-1]", vars["teamIds"])
	}
}

func TestIssueInput_Variables(t *testing.T) {
	in := issueInput{TeamID: "team-1", Title: "Fix bug", LabelIDs: []string{}}
	vars := in.variables()
	if vars["teamId"] != "team-1" || vars["title"] != "Fix bug" {
		t.Fatalf("vars = %v", vars)
	}
	if _, ok := vars["description"]; ok {
		t.Fatal("an empty description must be dropped so an update leaves it alone")
	}
	if _, ok := vars["priority"]; ok {
		t.Fatal("a nil priority must be dropped so an update leaves it alone")
	}
	labels, ok := vars["labelIds"].([]string)
	if !ok || len(labels) != 0 {
		t.Fatalf("labelIds = %v, want an empty list that clears labels", vars["labelIds"])
	}
}
