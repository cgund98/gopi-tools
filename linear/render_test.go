package linear

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/cgund98/gopi"
)

// fieldValue returns the value of a labeled field, or "" when it is absent.
func fieldValue(t *testing.T, view gopi.ToolView, label string) string {
	t.Helper()
	for _, field := range view.Fields {
		if field.Label == label {
			return field.Value
		}
	}
	return ""
}

func TestHeadline_Issues(t *testing.T) {
	tool := newIssues(nil)
	tests := []struct {
		args string
		want string
	}{
		{`{"operation":"get_issue","issue":"ENG-1"}`, "linear get ENG-1"},
		{`{"operation":"create_issue","title":"Fix bug"}`, "linear create Fix bug"},
		{`{"operation":"archive_issue","issue":"ENG-1"}`, "linear archive ENG-1"},
		{`{"operation":"list_issues","assignee":"me"}`, "linear issues me"},
		{`{"operation":"magic"}`, ""},
	}
	for _, tt := range tests {
		if got := tool.Headline(json.RawMessage(tt.args)); got != tt.want {
			t.Errorf("Headline(%s) = %q, want %q", tt.args, got, tt.want)
		}
	}
}

func TestHeadline_OtherTools(t *testing.T) {
	tests := []struct {
		name string
		tool gopi.ToolRenderer
		args string
		want string
	}{
		{name: "users whoami", tool: newUsers(nil), args: `{"operation":"get_viewer"}`, want: "linear whoami"},
		{name: "users list", tool: newUsers(nil), args: `{"operation":"list_users"}`, want: "linear users"},
		{name: "teams list", tool: newTeams(nil), args: `{"operation":"list_teams"}`, want: "linear teams"},
		{name: "team from default", tool: newTeams(nil), args: `{"operation":"get_team"}`, want: "linear team ENG"},
		{name: "project create", tool: newProjects(nil), args: `{"operation":"create_project","name":"Q3"}`, want: "linear create project Q3"},
		{name: "comment", tool: newComments(nil), args: `{"operation":"create_comment","issue":"ENG-1"}`, want: "linear comment on ENG-1"},
		{name: "label", tool: newLabels(nil), args: `{"operation":"create_label","name":"Bug"}`, want: "linear label Bug"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.tool.Headline(json.RawMessage(tt.args)); got != tt.want {
				t.Errorf("Headline(%s) = %q, want %q", tt.args, got, tt.want)
			}
		})
	}
}

func TestRenderApproval_UpdateIssueShowsDiff(t *testing.T) {
	tool := newIssues(nil)
	tool.currentIssues = map[string]issueDetail{
		"ENG-1": {
			ID:         "issue-1",
			Identifier: "ENG-1",
			Title:      "Fix login redirect",
			State:      &workflowStateInfo{ID: "s1", Name: "In Progress"},
			Assignee:   &userInfo{Name: "Ada"},
			Project:    &projectRef{Name: "Q3 cleanup"},
		},
	}
	args := `{"operation":"update_issue","issue":"ENG-1","state":"Done","title":"Fix login redirect"}`
	view := tool.RenderApproval(json.RawMessage(args))

	if got := fieldValue(t, view, "Issue"); got != "ENG-1  Fix login redirect" {
		t.Fatalf("Issue field = %q", got)
	}
	state := fieldValue(t, view, "State")
	if !strings.Contains(state, "→") || !strings.Contains(state, "Done") {
		t.Fatalf("State field = %q, want a diff ending in Done", state)
	}
	// An unchanged title shows the current value, not a diff.
	if got := fieldValue(t, view, "Title"); got != "Fix login redirect" {
		t.Fatalf("Title field = %q, want the current title", got)
	}
}

func TestRenderApproval_CreateIssueFields(t *testing.T) {
	tool := newIssues(nil)
	args := `{"operation":"create_issue","title":"New bug","state":"started","priority":1,"labels":["Bug"]}`
	view := tool.RenderApproval(json.RawMessage(args))

	if got := fieldValue(t, view, "Title"); got != "New bug" {
		t.Fatalf("Title field = %q", got)
	}
	if got := fieldValue(t, view, "Team"); got != "ENG" {
		t.Fatalf("Team field = %q, want the default team", got)
	}
	if got := fieldValue(t, view, "Priority"); got != "Urgent" {
		t.Fatalf("Priority field = %q, want Urgent", got)
	}
	if got := fieldValue(t, view, "Labels"); got != "Bug" {
		t.Fatalf("Labels field = %q, want Bug", got)
	}
}

func TestRenderApproval_CreateLabel(t *testing.T) {
	tool := newLabels(nil)
	view := tool.RenderApproval(json.RawMessage(`{"operation":"create_label","name":"Bug","color":"#ff0000"}`))
	if got := fieldValue(t, view, "Name"); got != "Bug" {
		t.Fatalf("Name field = %q", got)
	}
	if got := fieldValue(t, view, "Color"); got != "#ff0000" {
		t.Fatalf("Color field = %q", got)
	}
}

func TestRenderApproval_UpdateCommentShowsBody(t *testing.T) {
	tool := newComments(nil)
	tool.currentComments = map[string]commentInfo{"c1": {ID: "c1", Body: "old body"}}
	view := tool.RenderApproval(json.RawMessage(`{"operation":"update_comment","comment_id":"c1","body":"new body"}`))
	body := fieldValue(t, view, "Body")
	if !strings.Contains(body, "→") || !strings.Contains(body, "new body") {
		t.Fatalf("Body field = %q, want a diff ending in the new body", body)
	}
}

func TestRenderResult_ListIssues(t *testing.T) {
	tool := newIssues(nil)
	result := `{"operation":"list_issues","issues":[` +
		`{"identifier":"ENG-1","title":"Fix bug","state":{"name":"In Progress"},"assignee":{"name":"Ada"}},` +
		`{"identifier":"ENG-2","title":"Ship it"}]}`
	view := tool.RenderResult(nil, json.RawMessage(result))

	if len(view.Lines) != 2 {
		t.Fatalf("lines = %v, want 2", view.Lines)
	}
	if view.Lines[0] != "ENG-1  In Progress  Fix bug  @Ada" {
		t.Fatalf("line 0 = %q", view.Lines[0])
	}
	if view.Lines[1] != "ENG-2  Ship it" {
		t.Fatalf("line 1 = %q", view.Lines[1])
	}
}

func TestRenderResult_ListIssues_Empty(t *testing.T) {
	tool := newIssues(nil)
	view := tool.RenderResult(nil, json.RawMessage(`{"operation":"list_issues","issues":[]}`))
	if len(view.Lines) != 1 || view.Lines[0] != "No issues" {
		t.Fatalf("lines = %v, want [No issues]", view.Lines)
	}
}

func TestRenderResult_GetIssue(t *testing.T) {
	tool := newIssues(nil)
	result := `{"operation":"get_issue","issue":{"identifier":"ENG-1","title":"Fix bug",` +
		`"state":{"name":"In Progress"},"assignee":{"name":"Ada"},"priority":2,` +
		`"due_date":"2026-01-15","url":"https://linear.app/acme/issue/ENG-1","labels":{"nodes":[{"name":"Bug"}]}}}`
	view := tool.RenderResult(nil, json.RawMessage(result))

	if got := fieldValue(t, view, "Issue"); got != "ENG-1  Fix bug" {
		t.Fatalf("Issue field = %q", got)
	}
	if got := fieldValue(t, view, "Priority"); got != "High" {
		t.Fatalf("Priority field = %q, want High", got)
	}
	if got := fieldValue(t, view, "Labels"); got != "Bug" {
		t.Fatalf("Labels field = %q, want Bug", got)
	}
	if got := fieldValue(t, view, "URL"); got != "https://linear.app/acme/issue/ENG-1" {
		t.Fatalf("URL field = %q", got)
	}
}

func TestRenderResult_CreateIssueConfirmation(t *testing.T) {
	tool := newIssues(nil)
	result := `{"operation":"create_issue","issue":{"identifier":"ENG-7","title":"New bug","url":"https://linear.app/acme/issue/ENG-7"}}`
	view := tool.RenderResult(nil, json.RawMessage(result))

	if len(view.Lines) != 2 {
		t.Fatalf("lines = %v, want 2", view.Lines)
	}
	if view.Lines[0] != "Created  ENG-7  New bug" {
		t.Fatalf("line 0 = %q", view.Lines[0])
	}
	if view.Lines[1] != "https://linear.app/acme/issue/ENG-7" {
		t.Fatalf("line 1 = %q", view.Lines[1])
	}
}

func TestRenderResult_Archive(t *testing.T) {
	tool := newIssues(nil)
	result := `{"operation":"archive_issue","issue":{"identifier":"ENG-1","title":"Fix bug"}}`
	view := tool.RenderResult(nil, json.RawMessage(result))
	if len(view.Lines) == 0 || view.Lines[0] != "Archived  ENG-1  Fix bug" {
		t.Fatalf("lines = %v", view.Lines)
	}
}

func TestRenderResult_ListProjects(t *testing.T) {
	tool := newProjects(nil)
	result := `{"operation":"list_projects","projects":[{"name":"Q3 cleanup","status":{"name":"Started"},"target_date":"2026-09-30"}]}`
	view := tool.RenderResult(nil, json.RawMessage(result))
	if len(view.Lines) != 1 || view.Lines[0] != "Q3 cleanup  Started  target 2026-09-30" {
		t.Fatalf("lines = %v", view.Lines)
	}
}

func TestRenderResult_ListTeams(t *testing.T) {
	tool := newTeams(nil)
	result := `{"operation":"list_teams","teams":[{"key":"ENG","name":"Engineering"}]}`
	view := tool.RenderResult(nil, json.RawMessage(result))
	if len(view.Lines) != 1 || view.Lines[0] != "ENG  Engineering" {
		t.Fatalf("lines = %v", view.Lines)
	}
}

func TestRenderResult_GetViewer(t *testing.T) {
	tool := newUsers(nil)
	result := `{"operation":"get_viewer","viewer":{"name":"Ada","email":"ada@example.com","id":"u1"}}`
	view := tool.RenderResult(nil, json.RawMessage(result))
	if got := fieldValue(t, view, "Name"); got != "Ada" {
		t.Fatalf("Name field = %q", got)
	}
}

// TestRenderApproval_ShowsCachedTeamName is the cross-tool case: the name is
// recorded by the teams tool, then drawn by the issues tool's approval prompt.
func TestRenderApproval_ShowsCachedTeamName(t *testing.T) {
	const teamUUID = "254e39f7-6915-4b53-8371-ec981a2c1e06"
	s := newTestShared(&fakeClient{
		teamsFunc: func(context.Context, map[string]any, listOptions) (connection[teamInfo], error) {
			return connection[teamInfo]{Nodes: []teamInfo{{ID: teamUUID, Key: "ENG", Name: "Engineering"}}}, nil
		},
	})
	teams := &teamsTool{shared: s}
	issues := &issuesTool{shared: s}
	// A prior call on a different tool records the ID and its name.
	if _, err := teams.Execute(context.Background(), json.RawMessage(`{"operation":"list_teams"}`)); err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	args := `{"operation":"create_issue","title":"New bug","team":"` + teamUUID + `"}`
	view := issues.RenderApproval(json.RawMessage(args))
	if got := fieldValue(t, view, "Team"); got != "ENG  Engineering" {
		t.Fatalf("Team field = %q, want the name recorded by the teams tool", got)
	}
}

// TestTools_ShareNameCache drives the same path through the real registration
// helper, proving Tools wires every tool to one cache.
func TestTools_ShareNameCache(t *testing.T) {
	const projectUUID = "49f0b86d-a45b-4481-bb65-f49834ef8abe"
	tools := Tools(Config{DefaultTeam: "ENG"})
	issues, ok := tools[0].(*issuesTool)
	if !ok {
		t.Fatalf("tools[0] is %T, want *issuesTool", tools[0])
	}
	projects, ok := tools[1].(*projectsTool)
	if !ok {
		t.Fatalf("tools[1] is %T, want *projectsTool", tools[1])
	}
	if issues.shared != projects.shared {
		t.Fatal("Tools must wire every tool to one shared state")
	}
	client := &fakeClient{
		projectsFunc: func(context.Context, map[string]any, listOptions) (connection[projectSummary], error) {
			return connection[projectSummary]{Nodes: []projectSummary{{ID: projectUUID, Name: "NeuralScale Desktop App"}}}, nil
		},
	}
	projects.client = client
	projects.resolved = newResolver(client, &projects.names, "ENG")

	if _, err := projects.Execute(context.Background(), json.RawMessage(`{"operation":"list_projects"}`)); err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	args := `{"operation":"create_issue","title":"New bug","project":"` + projectUUID + `"}`
	view := issues.RenderApproval(json.RawMessage(args))
	if got := fieldValue(t, view, "Project"); got != "NeuralScale Desktop App" {
		t.Fatalf("Project field = %q, want the name recorded by the projects tool", got)
	}
}

func TestRenderApproval_UnknownTeamFallsBackToID(t *testing.T) {
	const teamUUID = "254e39f7-6915-4b53-8371-ec981a2c1e06"
	tool := newIssues(nil)
	args := `{"operation":"create_issue","title":"New bug","team":"` + teamUUID + `"}`
	view := tool.RenderApproval(json.RawMessage(args))
	if got := fieldValue(t, view, "Team"); got != teamUUID {
		t.Fatalf("Team field = %q, want the raw UUID", got)
	}
}

func TestHeadline_ShowsCachedProjectName(t *testing.T) {
	const projectUUID = "49f0b86d-a45b-4481-bb65-f49834ef8abe"
	tool := newProjects(&fakeClient{
		projectsFunc: func(context.Context, map[string]any, listOptions) (connection[projectSummary], error) {
			return connection[projectSummary]{Nodes: []projectSummary{{ID: projectUUID, Name: "NeuralScale Desktop App"}}}, nil
		},
	})
	if _, err := tool.Execute(context.Background(), json.RawMessage(`{"operation":"list_projects"}`)); err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	args := `{"operation":"get_project","project":"` + projectUUID + `"}`
	if got := tool.Headline(json.RawMessage(args)); got != "linear project NeuralScale Desktop App" {
		t.Fatalf("Headline = %q, want the cached project name", got)
	}
}

func TestHeadline_UnknownProjectFallsBackToID(t *testing.T) {
	const projectUUID = "49f0b86d-a45b-4481-bb65-f49834ef8abe"
	tool := newProjects(nil)
	args := `{"operation":"get_project","project":"` + projectUUID + `"}`
	if got := tool.Headline(json.RawMessage(args)); got != "linear project "+projectUUID {
		t.Fatalf("Headline = %q, want the raw UUID", got)
	}
}

func TestRenderApproval_ArchiveIssue(t *testing.T) {
	tool := newIssues(nil)
	tool.currentIssues = map[string]issueDetail{
		"ENG-1": {
			Identifier: "ENG-1",
			Title:      "Fix bug",
			State:      &workflowStateInfo{Name: "In Progress"},
		},
	}
	view := tool.RenderApproval(json.RawMessage(`{"operation":"archive_issue","issue":"ENG-1"}`))
	if got := fieldValue(t, view, "Issue"); got != "ENG-1  Fix bug" {
		t.Fatalf("Issue field = %q", got)
	}
	if got := fieldValue(t, view, "State"); got != "In Progress" {
		t.Fatalf("State field = %q", got)
	}
}
