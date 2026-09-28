package linear

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/cgund98/gogent"
	"github.com/invopop/jsonschema"
)

const (
	// defaultPageSize is the page size for a list operation that omits limit.
	defaultPageSize = 25
	// maxPageSize is the largest page size Linear's pagination is asked for.
	maxPageSize = 50
)

// op is one operation a tool exposes.
type op struct {
	name  string
	write bool
}

// ops is a tool's operation table. It is the single source of truth for the
// operation enum in the schema, the write classification that drives approval,
// and what a read-only tool is allowed to run.
type ops []op

// names returns the operation names, dropping writes when readonly is set.
func (o ops) names(readonly bool) []string {
	out := make([]string, 0, len(o))
	for _, entry := range o {
		if readonly && entry.write {
			continue
		}
		out = append(out, entry.name)
	}
	return out
}

// has reports whether name is one of the operations.
func (o ops) has(name string) bool {
	for _, entry := range o {
		if entry.name == name {
			return true
		}
	}
	return false
}

// isWrite reports whether name is a write operation.
func (o ops) isWrite(name string) bool {
	for _, entry := range o {
		if entry.name == name {
			return entry.write
		}
	}
	return false
}

// listArgs holds the pagination and filter fields the list operations share. It
// is embedded without a json tag, so jsonschema inlines it at the top level the
// same way encoding/json treats an anonymous struct.
type listArgs struct {
	Query           string `json:"query,omitempty" jsonschema_description:"Free-text filter matched against issue titles and descriptions, or project and label names."`
	IncludeArchived bool   `json:"include_archived,omitempty" jsonschema_description:"Include archived entities in list results."`
	UpdatedSince    string `json:"updated_since,omitempty" jsonschema_description:"Only return issues or projects updated at or after this value, as an ISO 8601 date or duration such as -P2W."`
	CreatedSince    string `json:"created_since,omitempty" jsonschema_description:"Only return issues created at or after this value, as an ISO 8601 date or duration such as -P2W."`
	Limit           int64  `json:"limit,omitempty" jsonschema_description:"Maximum results for a list operation. Default 25, maximum 50."`
	Cursor          string `json:"cursor,omitempty" jsonschema_description:"Pagination cursor from a previous page's page_info.end_cursor."`
	OrderBy         string `json:"order_by,omitempty" jsonschema:"enum=createdAt,enum=updatedAt" jsonschema_description:"Order list results by creation or last update time. Defaults to updatedAt."`
}

// options turns the pagination arguments into list options.
func (a listArgs) options() (listOptions, error) {
	limit := int(a.Limit)
	if limit <= 0 {
		limit = defaultPageSize
	}
	if limit > maxPageSize {
		return listOptions{}, invalid("limit %d exceeds the maximum of %d", limit, maxPageSize)
	}
	orderBy := a.OrderBy
	if orderBy == "" {
		orderBy = "updatedAt"
	}
	return listOptions{
		First:           limit,
		After:           a.Cursor,
		IncludeArchived: a.IncludeArchived,
		OrderBy:         orderBy,
	}, nil
}

// shared is the state every Linear tool holds a pointer to. Sharing it is what
// lets a name one tool cached appear in another tool's headline or approval
// prompt: renderers run in the terminal UI's paint loop with no context and
// cannot call the API.
type shared struct {
	cfg Config

	// names caches entity IDs to display names. The zero value is an empty
	// cache, and it is never reassigned.
	names nameCache

	mu       sync.Mutex
	client   linearClient
	resolved *resolver

	// currentIssues, currentProjects, and currentComments hold entities fetched
	// for an approval prompt, by ID and identifier, so RenderApproval can show
	// what will change.
	currentIssues   map[string]issueDetail
	currentProjects map[string]projectDetail
	currentComments map[string]commentInfo
}

// newShared builds the state a set of tools shares.
func newShared(cfg Config) *shared {
	return &shared{cfg: cfg}
}

// ensure builds the client and resolver on first use.
func (s *shared) ensure() (linearClient, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.client == nil {
		s.client = newHTTPClient(s.cfg)
	}
	if s.resolved == nil {
		s.resolved = newResolver(s.client, &s.names, s.cfg.DefaultTeam)
	}
	return s.client, nil
}

// resolverRef returns the resolver that ensure has already built.
func (s *shared) resolverRef() *resolver {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.resolved
}

// execute is the Execute flow every tool shares: validate the operation against
// its table, refuse a write in a read-only tool, ensure the client, run the
// operation, and record the names in the result.
func (s *shared) execute(ctx context.Context, table ops, readonly bool, operation string,
	run func(ctx context.Context, client linearClient, r *resolver) (json.RawMessage, error)) (json.RawMessage, error) {
	if !table.has(operation) {
		return errorResult("invalid_operation", fmt.Sprintf("unknown operation %q", operation))
	}
	if readonly && table.isWrite(operation) {
		return errorResult("permission_denied", fmt.Sprintf("%s is a write operation, which is disabled in this mode", operation))
	}
	client, err := s.ensure()
	if err != nil {
		return errorResult("client_setup_failed", err.Error())
	}
	result, err := run(ctx, client, s.resolverRef())
	if err != nil {
		return errorPayload(operation, err)
	}
	s.rememberResult(result)
	return result, nil
}

// rememberResult records the names in a completed payload, so a later render can
// show them for the IDs it carries.
func (s *shared) rememberResult(raw json.RawMessage) {
	var result linearResult
	if json.Unmarshal(raw, &result) != nil {
		return
	}
	s.names.rememberResult(result)
}

// parameters reflects args into a JSON Schema and constrains operation to the
// operations the tool exposes. A read-only tool drops its writes from the enum.
func parameters(args any, table ops, readonly bool) json.RawMessage {
	reflector := jsonschema.Reflector{DoNotReference: true}
	schema := reflector.Reflect(args)
	if schema.Properties != nil {
		if prop, ok := schema.Properties.Get("operation"); ok {
			enum := make([]any, 0, len(table))
			for _, name := range table.names(readonly) {
				enum = append(enum, name)
			}
			prop.Enum = enum
		}
	}
	body, err := json.Marshal(schema)
	if err != nil {
		panic(err)
	}
	return body
}

// labelIDs resolves label names or IDs, preferring a label owned by the team
// and falling back to a workspace-wide one when teamID is set.
func (s *shared) labelIDs(ctx context.Context, r *resolver, teamID string, refs []string) ([]string, error) {
	ids := make([]string, 0, len(refs))
	for _, ref := range refs {
		label, err := r.label(ctx, teamID, ref)
		if err != nil {
			return nil, err
		}
		ids = append(ids, label.ID)
	}
	return ids, nil
}

// issueID resolves an issue identifier or UUID to a UUID, for mutations whose
// inputs require one.
func (s *shared) issueID(ctx context.Context, client linearClient, ref string) (string, error) {
	if ref == "" {
		return "", invalid("issue is required")
	}
	if isUUID(ref) {
		return ref, nil
	}
	found, err := client.issue(ctx, ref)
	if err != nil {
		return "", err
	}
	return found.ID, nil
}

// lookupIssue fetches an issue for an approval prompt and caches it by
// identifier and ID. A failed lookup leaves the prompt showing less.
func (s *shared) lookupIssue(ctx context.Context, ref string) (issueDetail, bool) {
	if ref == "" {
		return issueDetail{}, false
	}
	if issue, ok := s.currentIssue(ref); ok {
		return issue, true
	}
	client, err := s.ensure()
	if err != nil {
		return issueDetail{}, false
	}
	issue, err := client.issue(ctx, ref)
	if err != nil {
		return issueDetail{}, false
	}
	s.names.rememberIssueDetail(issue)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.currentIssues == nil {
		s.currentIssues = map[string]issueDetail{}
	}
	for _, key := range []string{ref, issue.ID, issue.Identifier} {
		if key != "" {
			s.currentIssues[key] = issue
		}
	}
	return issue, true
}

// lookupProject fetches a project for an approval prompt.
func (s *shared) lookupProject(ctx context.Context, ref string) (projectDetail, bool) {
	if ref == "" {
		return projectDetail{}, false
	}
	if project, ok := s.currentProject(ref); ok {
		return project, true
	}
	client, err := s.ensure()
	if err != nil {
		return projectDetail{}, false
	}
	r := s.resolverRef()
	if r == nil {
		return projectDetail{}, false
	}
	resolved, err := r.project(ctx, ref)
	if err != nil {
		return projectDetail{}, false
	}
	project, err := client.project(ctx, resolved.ID)
	if err != nil {
		return projectDetail{}, false
	}
	s.names.rememberProjectDetail(project)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.currentProjects == nil {
		s.currentProjects = map[string]projectDetail{}
	}
	for _, key := range []string{ref, project.ID, project.Name} {
		if key != "" {
			s.currentProjects[key] = project
		}
	}
	return project, true
}

// lookupComment fetches a comment for an approval prompt.
func (s *shared) lookupComment(ctx context.Context, id string) (commentInfo, bool) {
	if id == "" {
		return commentInfo{}, false
	}
	if comment, ok := s.currentComment(id); ok {
		return comment, true
	}
	client, err := s.ensure()
	if err != nil {
		return commentInfo{}, false
	}
	comment, err := client.comment(ctx, id)
	if err != nil {
		return commentInfo{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.currentComments == nil {
		s.currentComments = map[string]commentInfo{}
	}
	s.currentComments[id] = comment
	return comment, true
}

func (s *shared) currentIssue(ref string) (issueDetail, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	issue, ok := s.currentIssues[ref]
	return issue, ok
}

func (s *shared) currentProject(ref string) (projectDetail, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	project, ok := s.currentProjects[ref]
	return project, ok
}

func (s *shared) currentComment(id string) (commentInfo, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	comment, ok := s.currentComments[id]
	return comment, ok
}

func (s *shared) forgetCurrentIssue(ref string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.currentIssues, ref)
	for key, issue := range s.currentIssues {
		if issue.ID == ref || issue.Identifier == ref {
			delete(s.currentIssues, key)
		}
	}
}

func (s *shared) forgetCurrentProject(ref string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.currentProjects, ref)
	for key, project := range s.currentProjects {
		if project.ID == ref || project.Name == ref {
			delete(s.currentProjects, key)
		}
	}
}

func (s *shared) forgetCurrentComment(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.currentComments, id)
}

// errorPayload turns an internal error into the JSON shape the model sees.
func errorPayload(operation string, err error) (json.RawMessage, error) {
	var apiErr *apiError
	if errors.As(err, &apiErr) {
		message := apiErr.Message
		if apiErr.Kind == "rate_limited" && !apiErr.Reset.IsZero() {
			message = fmt.Sprintf("%s; the limit resets at %s", message, apiErr.Reset.Format(time.RFC3339))
		}
		return errorResult(apiErr.Kind, message)
	}
	var invalidErr *invalidArgumentError
	if errors.As(err, &invalidErr) {
		return errorResult("invalid_argument", invalidErr.message)
	}
	return errorResult(operation+"_failed", err.Error())
}

// invalidArgumentError marks a request the tool rejects before calling Linear.
type invalidArgumentError struct{ message string }

func (e *invalidArgumentError) Error() string { return e.message }

func invalid(format string, args ...any) error {
	return &invalidArgumentError{message: fmt.Sprintf(format, args...)}
}

// requireUUID rejects a model-supplied team or project reference that is not a
// UUID. Those two fields take an ID from a list call rather than a name, which
// is ambiguous, so the error names the call that returns one.
func requireUUID(field, ref string) error {
	if ref == "" || isUUID(ref) {
		return nil
	}
	return invalid("%s must be a UUID, not %q; call %s to list them", field, ref, listOpFor(field))
}

// listOpFor names the list operation that returns a field's IDs.
func listOpFor(field string) string {
	switch field {
	case "team":
		return "list_teams"
	case "project":
		return "list_projects"
	}
	return "the matching list operation"
}

// errorResult builds the structured error payload the model reads.
func errorResult(code, message string) (json.RawMessage, error) {
	return json.Marshal(map[string]any{"error": code, "message": message})
}

func operationResult(operation string, payload map[string]any) (json.RawMessage, error) {
	payload["operation"] = operation
	return json.Marshal(payload)
}

func listOperationResult(operation, key string, nodes any, page *pageInfo) (json.RawMessage, error) {
	payload := map[string]any{key: nodes}
	if page != nil {
		payload["page_info"] = page
	}
	return operationResult(operation, payload)
}

// teamIDIfExplicit returns the team ID only when the model named a team, so an
// update does not move an issue on its own.
func teamIDIfExplicit(requested, teamID string) string {
	if requested == "" {
		return ""
	}
	return teamID
}

// filterUsersByName narrows team members by a free-text query.
func filterUsersByName(users []userInfo, query string) []userInfo {
	if query == "" {
		return users
	}
	lower := strings.ToLower(query)
	out := make([]userInfo, 0, len(users))
	for _, user := range users {
		for _, field := range []string{user.Name, user.DisplayName, user.Email} {
			if strings.Contains(strings.ToLower(field), lower) {
				out = append(out, user)
				break
			}
		}
	}
	return out
}

// isUUID reports whether ref is a 36-character Linear UUID.
func isUUID(ref string) bool {
	if len(ref) != 36 {
		return false
	}
	for i, r := range ref {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if r != '-' {
				return false
			}
			continue
		}
		if !isHexDigit(r) {
			return false
		}
	}
	return true
}

func isHexDigit(r rune) bool {
	return (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

// parseArgs unmarshals the model's arguments into v.
func parseArgs(raw json.RawMessage, v any) error {
	if err := json.Unmarshal(raw, v); err != nil {
		return fmt.Errorf("parse arguments: %w", err)
	}
	return nil
}

// approvalFor returns the approval decision for a parsed operation, using
// reason only when the write needs approval.
func approvalFor(table ops, readonly bool, operation string, reason func() string) gogent.ApprovalDecision {
	if readonly || !table.isWrite(operation) {
		return gogent.ApprovalDecision{}
	}
	return gogent.ApprovalDecision{Required: true, Reason: reason()}
}
