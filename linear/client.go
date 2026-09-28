package linear

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// defaultEndpoint is Linear's public GraphQL endpoint.
const defaultEndpoint = "https://api.linear.app/graphql"

// maxResponseBytes caps how much of a response body is read.
const maxResponseBytes = 8 << 20

// apiError is a typed error from the Linear API. Kind is the error code the
// tool reports to the model: "rate_limited", "permission_denied",
// "invalid_argument", or "api_error".
type apiError struct {
	Kind    string
	Message string
	// Reset is when the rate limit window resets, for a rate_limited error.
	Reset time.Time
}

func (e *apiError) Error() string { return e.Message }

// listOptions carries pagination and ordering for a connection query.
type listOptions struct {
	First           int
	After           string
	IncludeArchived bool
	OrderBy         string
}

// linearClient is the interface the tool uses to talk to Linear. It is
// implemented by *httpLinearClient and can be mocked in tests.
type linearClient interface {
	viewer(ctx context.Context) (viewerInfo, error)
	teams(ctx context.Context, filter map[string]any, opts listOptions) (connection[teamInfo], error)
	team(ctx context.Context, id string) (teamDetail, error)
	workflowStates(ctx context.Context, filter map[string]any, opts listOptions) (connection[workflowStateInfo], error)
	users(ctx context.Context, filter map[string]any, opts listOptions) (connection[userInfo], error)
	labels(ctx context.Context, filter map[string]any, opts listOptions) (connection[labelInfo], error)
	projectStatuses(ctx context.Context, opts listOptions) (connection[projectStatusInfo], error)
	projectMilestones(ctx context.Context, filter map[string]any, opts listOptions) (connection[milestoneInfo], error)
	issues(ctx context.Context, filter map[string]any, opts listOptions) (connection[issueSummary], error)
	issue(ctx context.Context, id string) (issueDetail, error)
	projects(ctx context.Context, filter map[string]any, opts listOptions) (connection[projectSummary], error)
	project(ctx context.Context, id string) (projectDetail, error)
	comment(ctx context.Context, id string) (commentInfo, error)
	createIssue(ctx context.Context, in issueInput) (issueSummary, error)
	updateIssue(ctx context.Context, id string, in issueInput) (issueSummary, error)
	archiveIssue(ctx context.Context, id string) (issueRef, error)
	createComment(ctx context.Context, in commentInput) (commentInfo, error)
	updateComment(ctx context.Context, id, body string) (commentInfo, error)
	createProject(ctx context.Context, in projectInput) (projectSummary, error)
	updateProject(ctx context.Context, id string, in projectInput) (projectSummary, error)
	createMilestone(ctx context.Context, in milestoneInput) (milestoneInfo, error)
	updateMilestone(ctx context.Context, id string, in milestoneInput) (milestoneInfo, error)
	createLabel(ctx context.Context, in labelInput) (labelInfo, error)
	createRelation(ctx context.Context, in relationInput) (relationInfo, error)
}

// issueInput is an IssueCreateInput or IssueUpdateInput payload. Nil pointers and
// empty strings are dropped, so an update leaves unset fields alone.
type issueInput struct {
	TeamID      string
	Title       string
	Description string
	AssigneeID  string
	StateID     string
	DueDate     string
	ProjectID   string
	ParentID    string
	Priority    *int
	Estimate    *int
	LabelIDs    []string
}

func (in issueInput) variables() map[string]any {
	out := map[string]any{}
	setString(out, "teamId", in.TeamID)
	setString(out, "title", in.Title)
	setString(out, "description", in.Description)
	setString(out, "assigneeId", in.AssigneeID)
	setString(out, "stateId", in.StateID)
	setString(out, "dueDate", in.DueDate)
	setString(out, "projectId", in.ProjectID)
	setString(out, "parentId", in.ParentID)
	setInt(out, "priority", in.Priority)
	setInt(out, "estimate", in.Estimate)
	setStrings(out, "labelIds", in.LabelIDs)
	return out
}

// commentInput is a CommentCreateInput payload.
type commentInput struct {
	IssueID string
	Body    string
}

// projectInput is a ProjectCreateInput or ProjectUpdateInput payload.
type projectInput struct {
	Name        string
	Description string
	Summary     string
	LeadID      string
	StatusID    string
	StartDate   string
	TargetDate  string
	TeamIDs     []string
}

func (in projectInput) variables() map[string]any {
	out := map[string]any{}
	setString(out, "name", in.Name)
	setString(out, "description", in.Description)
	// Linear's project "summary" is the content field.
	setString(out, "content", in.Summary)
	setString(out, "leadId", in.LeadID)
	setString(out, "statusId", in.StatusID)
	setString(out, "startDate", in.StartDate)
	setString(out, "targetDate", in.TargetDate)
	setStrings(out, "teamIds", in.TeamIDs)
	return out
}

// milestoneInput is a ProjectMilestoneCreateInput or ProjectMilestoneUpdateInput
// payload.
type milestoneInput struct {
	Name        string
	ProjectID   string
	Description string
	TargetDate  string
}

func (in milestoneInput) variables() map[string]any {
	out := map[string]any{}
	setString(out, "name", in.Name)
	setString(out, "projectId", in.ProjectID)
	setString(out, "description", in.Description)
	setString(out, "targetDate", in.TargetDate)
	return out
}

// labelInput is an IssueLabelCreateInput payload.
type labelInput struct {
	Name        string
	TeamID      string
	Color       string
	Description string
}

func (in labelInput) variables() map[string]any {
	out := map[string]any{}
	setString(out, "name", in.Name)
	setString(out, "teamId", in.TeamID)
	setString(out, "color", in.Color)
	setString(out, "description", in.Description)
	return out
}

// relationInput is an IssueRelationCreateInput payload.
type relationInput struct {
	IssueID        string
	RelatedIssueID string
	Type           string
}

func (in relationInput) variables() map[string]any {
	return map[string]any{
		"issueId":        in.IssueID,
		"relatedIssueId": in.RelatedIssueID,
		"type":           in.Type,
	}
}

// httpLinearClient talks to Linear's GraphQL API over HTTP.
type httpLinearClient struct {
	endpoint string
	apiKey   string
	http     *http.Client
}

// newHTTPClient builds the default client from cfg. It never fails: a missing
// API key is reported when a query runs.
func newHTTPClient(cfg Config) *httpLinearClient {
	endpoint := cfg.Endpoint
	if endpoint == "" {
		endpoint = defaultEndpoint
	}
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}
	return &httpLinearClient{endpoint: endpoint, apiKey: cfg.APIKey, http: client}
}

// gqlError is one entry in a GraphQL response's errors array.
type gqlError struct {
	Message    string `json:"message"`
	Extensions struct {
		Code string `json:"code"`
	} `json:"extensions"`
}

// do posts a GraphQL request and decodes data into out.
func (c *httpLinearClient) do(ctx context.Context, query string, variables map[string]any, out any) error {
	if strings.TrimSpace(c.apiKey) == "" {
		return &apiError{Kind: "api_error", Message: "Linear API key is not configured"}
	}
	payload, err := json.Marshal(map[string]any{"query": query, "variables": variables})
	if err != nil {
		return &apiError{Kind: "api_error", Message: fmt.Sprintf("encode request: %v", err)}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(payload))
	if err != nil {
		return &apiError{Kind: "api_error", Message: fmt.Sprintf("build request: %v", err)}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	// Linear API keys are sent raw, without a "Bearer" prefix.
	req.Header.Set("Authorization", c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return &apiError{Kind: "api_error", Message: fmt.Sprintf("request failed: %v", err)}
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return &apiError{Kind: "api_error", Message: fmt.Sprintf("read response: %v", err)}
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return &apiError{
			Kind:    "rate_limited",
			Message: fmt.Sprintf("Linear rate limit exceeded (HTTP 429): %s", truncate(string(body), 200)),
			Reset:   rateLimitReset(resp.Header),
		}
	}

	var envelope struct {
		Data   json.RawMessage `json:"data"`
		Errors []gqlError      `json:"errors"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return &apiError{
			Kind:    "api_error",
			Message: fmt.Sprintf("Linear returned HTTP %d: %s", resp.StatusCode, truncate(string(body), 300)),
		}
	}
	if len(envelope.Errors) > 0 {
		return classifyErrors(envelope.Errors, resp.Header)
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return &apiError{Kind: "api_error", Message: fmt.Sprintf("Linear returned HTTP %d", resp.StatusCode)}
	}
	if len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return &apiError{Kind: "api_error", Message: "Linear returned no data"}
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(envelope.Data, out); err != nil {
		return &apiError{Kind: "api_error", Message: fmt.Sprintf("decode response: %v", err)}
	}
	return nil
}

// classifyErrors maps a GraphQL errors array onto one typed error. Rate limits
// win over permission errors, which win over invalid arguments.
func classifyErrors(errs []gqlError, header http.Header) error {
	kind := "api_error"
	messages := make([]string, 0, len(errs))
	for _, e := range errs {
		if e.Message != "" {
			messages = append(messages, e.Message)
		}
		code := e.Extensions.Code
		switch {
		case strings.EqualFold(code, "RATELIMITED"):
			kind = "rate_limited"
		case isPermissionError(code, e.Message):
			if kind != "rate_limited" {
				kind = "permission_denied"
			}
		case isInvalidError(code, e.Message):
			if kind != "rate_limited" && kind != "permission_denied" {
				kind = "invalid_argument"
			}
		}
	}
	if len(messages) == 0 {
		messages = []string{"Linear returned an unspecified error"}
	}
	out := &apiError{Kind: kind, Message: strings.Join(messages, "; ")}
	if kind == "rate_limited" {
		out.Reset = rateLimitReset(header)
	}
	return out
}

var permissionCodes = map[string]bool{
	"AUTHENTICATION_ERROR": true,
	"FORBIDDEN":            true,
	"UNAUTHORIZED":         true,
	"PERMISSION_DENIED":    true,
	"ACCESS_DENIED":        true,
}

var invalidCodes = map[string]bool{
	"INVALID_INPUT":             true,
	"BAD_USER_INPUT":            true,
	"VALIDATION_ERROR":          true,
	"GRAPHQL_VALIDATION_FAILED": true,
	"NOT_FOUND":                 true,
}

func isPermissionError(code, message string) bool {
	if permissionCodes[strings.ToUpper(code)] {
		return true
	}
	lower := strings.ToLower(message)
	for _, needle := range []string{"access denied", "not authorized", "you do not have", "permission"} {
		if strings.Contains(lower, needle) {
			return true
		}
	}
	return false
}

func isInvalidError(code, message string) bool {
	if invalidCodes[strings.ToUpper(code)] {
		return true
	}
	lower := strings.ToLower(message)
	return strings.Contains(lower, "not found") || strings.Contains(lower, "invalid")
}

// rateLimitReset reads the rate limit reset time from Linear's response headers.
func rateLimitReset(header http.Header) time.Time {
	for _, name := range []string{"X-RateLimit-Requests-Reset", "X-RateLimit-Endpoint-Requests-Reset"} {
		value := header.Get(name)
		if value == "" {
			continue
		}
		if millis, err := strconv.ParseInt(value, 10, 64); err == nil {
			return time.UnixMilli(millis)
		}
	}
	return time.Time{}
}

func (c *httpLinearClient) viewer(ctx context.Context) (viewerInfo, error) {
	var resp struct {
		Viewer viewerInfo `json:"viewer"`
	}
	if err := c.do(ctx, viewerQuery, nil, &resp); err != nil {
		return viewerInfo{}, err
	}
	return resp.Viewer, nil
}

func (c *httpLinearClient) teams(ctx context.Context, filter map[string]any, opts listOptions) (connection[teamInfo], error) {
	var resp struct {
		Teams connection[teamInfo] `json:"teams"`
	}
	if err := c.do(ctx, teamsQuery, queryVariables(filter, opts), &resp); err != nil {
		return connection[teamInfo]{}, err
	}
	return resp.Teams, nil
}

func (c *httpLinearClient) team(ctx context.Context, id string) (teamDetail, error) {
	var resp struct {
		Team teamDetail `json:"team"`
	}
	if err := c.do(ctx, teamQuery, map[string]any{"id": id}, &resp); err != nil {
		return teamDetail{}, err
	}
	return resp.Team, nil
}

func (c *httpLinearClient) workflowStates(ctx context.Context, filter map[string]any, opts listOptions) (connection[workflowStateInfo], error) {
	var resp struct {
		WorkflowStates connection[workflowStateInfo] `json:"workflowStates"`
	}
	if err := c.do(ctx, workflowStatesQuery, queryVariables(filter, opts), &resp); err != nil {
		return connection[workflowStateInfo]{}, err
	}
	return resp.WorkflowStates, nil
}

func (c *httpLinearClient) users(ctx context.Context, filter map[string]any, opts listOptions) (connection[userInfo], error) {
	var resp struct {
		Users connection[userInfo] `json:"users"`
	}
	if err := c.do(ctx, usersQuery, queryVariables(filter, opts), &resp); err != nil {
		return connection[userInfo]{}, err
	}
	return resp.Users, nil
}

func (c *httpLinearClient) labels(ctx context.Context, filter map[string]any, opts listOptions) (connection[labelInfo], error) {
	var resp struct {
		IssueLabels connection[labelInfo] `json:"issueLabels"`
	}
	if err := c.do(ctx, labelsQuery, queryVariables(filter, opts), &resp); err != nil {
		return connection[labelInfo]{}, err
	}
	return resp.IssueLabels, nil
}

func (c *httpLinearClient) projectStatuses(ctx context.Context, opts listOptions) (connection[projectStatusInfo], error) {
	var resp struct {
		ProjectStatuses connection[projectStatusInfo] `json:"projectStatuses"`
	}
	if err := c.do(ctx, projectStatusesQuery, opts.variables(), &resp); err != nil {
		return connection[projectStatusInfo]{}, err
	}
	return resp.ProjectStatuses, nil
}

func (c *httpLinearClient) projectMilestones(ctx context.Context, filter map[string]any, opts listOptions) (connection[milestoneInfo], error) {
	var resp struct {
		ProjectMilestones connection[milestoneInfo] `json:"projectMilestones"`
	}
	if err := c.do(ctx, projectMilestonesQuery, queryVariables(filter, opts), &resp); err != nil {
		return connection[milestoneInfo]{}, err
	}
	return resp.ProjectMilestones, nil
}

func (c *httpLinearClient) issues(ctx context.Context, filter map[string]any, opts listOptions) (connection[issueSummary], error) {
	var resp struct {
		Issues connection[issueSummary] `json:"issues"`
	}
	if err := c.do(ctx, issuesQuery, queryVariables(filter, opts), &resp); err != nil {
		return connection[issueSummary]{}, err
	}
	return resp.Issues, nil
}

func (c *httpLinearClient) issue(ctx context.Context, id string) (issueDetail, error) {
	var resp struct {
		Issue issueDetail `json:"issue"`
	}
	if err := c.do(ctx, issueQuery, map[string]any{"id": id}, &resp); err != nil {
		return issueDetail{}, err
	}
	return resp.Issue, nil
}

func (c *httpLinearClient) projects(ctx context.Context, filter map[string]any, opts listOptions) (connection[projectSummary], error) {
	var resp struct {
		Projects connection[projectSummary] `json:"projects"`
	}
	if err := c.do(ctx, projectsQuery, queryVariables(filter, opts), &resp); err != nil {
		return connection[projectSummary]{}, err
	}
	return resp.Projects, nil
}

func (c *httpLinearClient) project(ctx context.Context, id string) (projectDetail, error) {
	var resp struct {
		Project projectDetail `json:"project"`
	}
	if err := c.do(ctx, projectQuery, map[string]any{"id": id}, &resp); err != nil {
		return projectDetail{}, err
	}
	return resp.Project, nil
}

func (c *httpLinearClient) comment(ctx context.Context, id string) (commentInfo, error) {
	var resp struct {
		Comment commentInfo `json:"comment"`
	}
	if err := c.do(ctx, commentQuery, map[string]any{"id": id}, &resp); err != nil {
		return commentInfo{}, err
	}
	return resp.Comment, nil
}

func (c *httpLinearClient) createIssue(ctx context.Context, in issueInput) (issueSummary, error) {
	var resp struct {
		IssueCreate struct {
			Success bool         `json:"success"`
			Issue   issueSummary `json:"issue"`
		} `json:"issueCreate"`
	}
	if err := c.do(ctx, issueCreateMutation, map[string]any{"input": in.variables()}, &resp); err != nil {
		return issueSummary{}, err
	}
	if !resp.IssueCreate.Success {
		return issueSummary{}, &apiError{Kind: "invalid_argument", Message: "Linear rejected issueCreate"}
	}
	return resp.IssueCreate.Issue, nil
}

func (c *httpLinearClient) updateIssue(ctx context.Context, id string, in issueInput) (issueSummary, error) {
	var resp struct {
		IssueUpdate struct {
			Success bool         `json:"success"`
			Issue   issueSummary `json:"issue"`
		} `json:"issueUpdate"`
	}
	vars := map[string]any{"id": id, "input": in.variables()}
	if err := c.do(ctx, issueUpdateMutation, vars, &resp); err != nil {
		return issueSummary{}, err
	}
	if !resp.IssueUpdate.Success {
		return issueSummary{}, &apiError{Kind: "invalid_argument", Message: "Linear rejected issueUpdate"}
	}
	return resp.IssueUpdate.Issue, nil
}

func (c *httpLinearClient) archiveIssue(ctx context.Context, id string) (issueRef, error) {
	var resp struct {
		IssueArchive struct {
			Success bool     `json:"success"`
			Entity  issueRef `json:"entity"`
		} `json:"issueArchive"`
	}
	if err := c.do(ctx, issueArchiveMutation, map[string]any{"id": id}, &resp); err != nil {
		return issueRef{}, err
	}
	if !resp.IssueArchive.Success {
		return issueRef{}, &apiError{Kind: "invalid_argument", Message: "Linear rejected issueArchive"}
	}
	return resp.IssueArchive.Entity, nil
}

func (c *httpLinearClient) createComment(ctx context.Context, in commentInput) (commentInfo, error) {
	var resp struct {
		CommentCreate struct {
			Success bool        `json:"success"`
			Comment commentInfo `json:"comment"`
		} `json:"commentCreate"`
	}
	vars := map[string]any{"input": map[string]any{"issueId": in.IssueID, "body": in.Body}}
	if err := c.do(ctx, commentCreateMutation, vars, &resp); err != nil {
		return commentInfo{}, err
	}
	if !resp.CommentCreate.Success {
		return commentInfo{}, &apiError{Kind: "invalid_argument", Message: "Linear rejected commentCreate"}
	}
	return resp.CommentCreate.Comment, nil
}

func (c *httpLinearClient) updateComment(ctx context.Context, id, body string) (commentInfo, error) {
	var resp struct {
		CommentUpdate struct {
			Success bool        `json:"success"`
			Comment commentInfo `json:"comment"`
		} `json:"commentUpdate"`
	}
	vars := map[string]any{"id": id, "input": map[string]any{"body": body}}
	if err := c.do(ctx, commentUpdateMutation, vars, &resp); err != nil {
		return commentInfo{}, err
	}
	if !resp.CommentUpdate.Success {
		return commentInfo{}, &apiError{Kind: "invalid_argument", Message: "Linear rejected commentUpdate"}
	}
	return resp.CommentUpdate.Comment, nil
}

func (c *httpLinearClient) createProject(ctx context.Context, in projectInput) (projectSummary, error) {
	var resp struct {
		ProjectCreate struct {
			Success bool           `json:"success"`
			Project projectSummary `json:"project"`
		} `json:"projectCreate"`
	}
	if err := c.do(ctx, projectCreateMutation, map[string]any{"input": in.variables()}, &resp); err != nil {
		return projectSummary{}, err
	}
	if !resp.ProjectCreate.Success {
		return projectSummary{}, &apiError{Kind: "invalid_argument", Message: "Linear rejected projectCreate"}
	}
	return resp.ProjectCreate.Project, nil
}

func (c *httpLinearClient) updateProject(ctx context.Context, id string, in projectInput) (projectSummary, error) {
	var resp struct {
		ProjectUpdate struct {
			Success bool           `json:"success"`
			Project projectSummary `json:"project"`
		} `json:"projectUpdate"`
	}
	vars := map[string]any{"id": id, "input": in.variables()}
	if err := c.do(ctx, projectUpdateMutation, vars, &resp); err != nil {
		return projectSummary{}, err
	}
	if !resp.ProjectUpdate.Success {
		return projectSummary{}, &apiError{Kind: "invalid_argument", Message: "Linear rejected projectUpdate"}
	}
	return resp.ProjectUpdate.Project, nil
}

func (c *httpLinearClient) createMilestone(ctx context.Context, in milestoneInput) (milestoneInfo, error) {
	var resp struct {
		ProjectMilestoneCreate struct {
			Success          bool          `json:"success"`
			ProjectMilestone milestoneInfo `json:"projectMilestone"`
		} `json:"projectMilestoneCreate"`
	}
	if err := c.do(ctx, milestoneCreateMutation, map[string]any{"input": in.variables()}, &resp); err != nil {
		return milestoneInfo{}, err
	}
	if !resp.ProjectMilestoneCreate.Success {
		return milestoneInfo{}, &apiError{Kind: "invalid_argument", Message: "Linear rejected projectMilestoneCreate"}
	}
	return resp.ProjectMilestoneCreate.ProjectMilestone, nil
}

func (c *httpLinearClient) updateMilestone(ctx context.Context, id string, in milestoneInput) (milestoneInfo, error) {
	var resp struct {
		ProjectMilestoneUpdate struct {
			Success          bool          `json:"success"`
			ProjectMilestone milestoneInfo `json:"projectMilestone"`
		} `json:"projectMilestoneUpdate"`
	}
	vars := map[string]any{"id": id, "input": in.variables()}
	if err := c.do(ctx, milestoneUpdateMutation, vars, &resp); err != nil {
		return milestoneInfo{}, err
	}
	if !resp.ProjectMilestoneUpdate.Success {
		return milestoneInfo{}, &apiError{Kind: "invalid_argument", Message: "Linear rejected projectMilestoneUpdate"}
	}
	return resp.ProjectMilestoneUpdate.ProjectMilestone, nil
}

func (c *httpLinearClient) createLabel(ctx context.Context, in labelInput) (labelInfo, error) {
	var resp struct {
		IssueLabelCreate struct {
			Success    bool      `json:"success"`
			IssueLabel labelInfo `json:"issueLabel"`
		} `json:"issueLabelCreate"`
	}
	if err := c.do(ctx, labelCreateMutation, map[string]any{"input": in.variables()}, &resp); err != nil {
		return labelInfo{}, err
	}
	if !resp.IssueLabelCreate.Success {
		return labelInfo{}, &apiError{Kind: "invalid_argument", Message: "Linear rejected issueLabelCreate"}
	}
	return resp.IssueLabelCreate.IssueLabel, nil
}

func (c *httpLinearClient) createRelation(ctx context.Context, in relationInput) (relationInfo, error) {
	var resp struct {
		IssueRelationCreate struct {
			Success       bool         `json:"success"`
			IssueRelation relationInfo `json:"issueRelation"`
		} `json:"issueRelationCreate"`
	}
	if err := c.do(ctx, relationCreateMutation, map[string]any{"input": in.variables()}, &resp); err != nil {
		return relationInfo{}, err
	}
	if !resp.IssueRelationCreate.Success {
		return relationInfo{}, &apiError{Kind: "invalid_argument", Message: "Linear rejected issueRelationCreate"}
	}
	return resp.IssueRelationCreate.IssueRelation, nil
}

// queryVariables merges a filter into pagination variables, dropping an empty
// filter so queries that accept none stay valid.
func queryVariables(filter map[string]any, opts listOptions) map[string]any {
	vars := opts.variables()
	if len(filter) > 0 {
		vars["filter"] = filter
	}
	return vars
}

// variables turns pagination options into GraphQL variables.
func (o listOptions) variables() map[string]any {
	vars := map[string]any{}
	if o.First > 0 {
		vars["first"] = o.First
	}
	if o.After != "" {
		vars["after"] = o.After
	}
	if o.IncludeArchived {
		vars["includeArchived"] = true
	}
	if o.OrderBy != "" {
		vars["orderBy"] = o.OrderBy
	}
	return vars
}

func setString(out map[string]any, key, value string) {
	if value != "" {
		out[key] = value
	}
}

func setInt(out map[string]any, key string, value *int) {
	if value != nil {
		out[key] = *value
	}
}

// setStrings sets a list field, keeping an explicitly empty list so an update
// can clear it.
func setStrings(out map[string]any, key string, values []string) {
	if values != nil {
		out[key] = values
	}
}

func truncate(value string, limit int) string {
	value = strings.TrimSpace(value)
	if len(value) <= limit {
		return value
	}
	return value[:limit] + "..."
}
