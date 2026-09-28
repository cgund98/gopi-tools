package linear

import (
	"fmt"
	"strings"

	"github.com/cgund98/gopi"
)

// This file holds the rendering pieces every tool shares: the result decode
// types, the approval field collector, the formatting helpers, and the name
// lookups that turn a cached ID into a display name. Each tool's own Headline,
// RenderApproval, and RenderResult live beside the tool.

// issueView is the result shape shared by the issue operations. It covers the
// summary returned by the writes, the full issue returned by get_issue, and the
// short reference returned by archive_issue.
type issueView struct {
	ID          string                 `json:"id"`
	Identifier  string                 `json:"identifier"`
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	State       *workflowStateInfo     `json:"state"`
	Assignee    *userInfo              `json:"assignee"`
	Team        *teamInfo              `json:"team"`
	Project     *projectRef            `json:"project"`
	Labels      *connection[labelInfo] `json:"labels"`
	Priority    float64                `json:"priority"`
	Estimate    float64                `json:"estimate"`
	DueDate     string                 `json:"due_date"`
	UpdatedAt   string                 `json:"updated_at"`
	ArchivedAt  string                 `json:"archived_at"`
	URL         string                 `json:"url"`
}

// projectView is the result shape for the project operations.
type projectView struct {
	ID         string             `json:"id"`
	Name       string             `json:"name"`
	Status     *projectStatusInfo `json:"status"`
	Lead       *userInfo          `json:"lead"`
	Progress   float64            `json:"progress"`
	StartDate  string             `json:"start_date"`
	TargetDate string             `json:"target_date"`
	Content    string             `json:"content"`
	ArchivedAt string             `json:"archived_at"`
}

// linearResult decodes whichever payload an operation returned. It is a superset
// of the list and get shapes, and is also what feeds the name cache.
type linearResult struct {
	Operation string           `json:"operation"`
	Viewer    *viewerInfo      `json:"viewer"`
	Teams     []teamInfo       `json:"teams"`
	Team      *teamDetail      `json:"team"`
	Issues    []issueSummary   `json:"issues"`
	Issue     *issueView       `json:"issue"`
	Projects  []projectSummary `json:"projects"`
	Project   *projectView     `json:"project"`
	Users     []userInfo       `json:"users"`
	Labels    []labelInfo      `json:"labels"`
	Comment   *commentInfo     `json:"comment"`
	Milestone *milestoneInfo   `json:"milestone"`
	Label     *labelInfo       `json:"label"`
	Relation  *relationInfo    `json:"relation"`
}

// approvalFields collects the labeled rows of an approval prompt.
type approvalFields struct {
	fields []gopi.ToolField
}

// add records a row, skipping empty values.
func (f *approvalFields) add(label, value string) {
	if strings.TrimSpace(value) != "" {
		f.fields = append(f.fields, gopi.ToolField{Label: label, Value: value})
	}
}

func (f *approvalFields) view() gopi.ToolView {
	return gopi.ToolView{Fields: f.fields}
}

// confirmation shows what a write produced, with a link when there is one.
func confirmation(verb, identifier, subject, url string) gopi.ToolView {
	line := strings.TrimSpace(strings.Join([]string{verb, identifier, subject}, "  "))
	lines := []string{line}
	if url != "" {
		lines = append(lines, url)
	}
	return gopi.ToolView{Lines: lines}
}

func lineView(lines []string, empty string) gopi.ToolView {
	if len(lines) == 0 {
		return gopi.ToolView{Lines: []string{empty}}
	}
	return gopi.ToolView{Lines: lines}
}

// compactFields keeps only the rows that carry a value.
func compactFields(fields []gopi.ToolField) []gopi.ToolField {
	out := make([]gopi.ToolField, 0, len(fields))
	for _, field := range fields {
		if strings.TrimSpace(field.Value) != "" {
			out = append(out, field)
		}
	}
	return out
}

// change shows "old → new" when an update sets a different value. An empty next
// means the update leaves the field alone.
func change(current, next string) string {
	if next == "" || next == current {
		return current
	}
	return firstNonEmpty(current, "(none)") + " → " + next
}

// bodyChange reports that a long body changed without printing both versions.
func bodyChange(next string) string {
	if next == "" {
		return ""
	}
	return "(updated)"
}

func changePriority(current float64, next *int) string {
	if next == nil {
		return priorityLabel(current)
	}
	return change(priorityLabel(current), priorityName(next))
}

func changeEstimate(current float64, next *int) string {
	if next == nil {
		return estimateLabel(current)
	}
	return change(estimateLabel(current), estimateName(next))
}

// priorityLabel formats an issue's current priority for display. Zero means no
// priority, which draws nothing.
func priorityLabel(p float64) string {
	if p == 0 {
		return ""
	}
	return priorityValue(int(p))
}

// priorityName formats a priority the model asked for, including "None".
func priorityName(p *int) string {
	if p == nil {
		return ""
	}
	return priorityValue(*p)
}

func priorityValue(p int) string {
	switch p {
	case 1:
		return "Urgent"
	case 2:
		return "High"
	case 3:
		return "Medium"
	case 4:
		return "Low"
	case 0:
		return "None"
	default:
		return ""
	}
}

func estimateLabel(e float64) string {
	if e == 0 {
		return ""
	}
	return fmt.Sprintf("%g", e)
}

func estimateName(e *int) string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("%d", *e)
}

func stateName(state *workflowStateInfo) string {
	if state == nil {
		return ""
	}
	return state.Name
}

func userName(user *userInfo) string {
	if user == nil {
		return ""
	}
	return firstNonEmpty(user.Name, user.DisplayName, user.Email)
}

func projectName(project *projectRef) string {
	if project == nil {
		return ""
	}
	return project.Name
}

func projectStatusName(status *projectStatusInfo) string {
	if status == nil {
		return ""
	}
	return status.Name
}

func teamLabel(team *teamInfo) string {
	if team == nil {
		return ""
	}
	return strings.TrimSpace(team.Key + "  " + team.Name)
}

func relationTitle(ref *issueRef) string {
	if ref == nil {
		return ""
	}
	return strings.TrimSpace(ref.Identifier + "  " + ref.Title)
}

func progressLabel(progress float64) string {
	if progress == 0 {
		return ""
	}
	return fmt.Sprintf("%.0f%%", progress*100)
}

func stateNames(states *connection[workflowStateInfo]) string {
	if states == nil {
		return ""
	}
	names := make([]string, 0, len(states.Nodes))
	for _, state := range states.Nodes {
		names = append(names, state.Name)
	}
	return strings.Join(names, ", ")
}

func labelNames(labels *connection[labelInfo]) string {
	if labels == nil {
		return ""
	}
	names := make([]string, 0, len(labels.Nodes))
	for _, label := range labels.Nodes {
		names = append(names, label.Name)
	}
	return strings.Join(names, ", ")
}

func userNames(users *connection[userInfo]) string {
	if users == nil {
		return ""
	}
	names := make([]string, 0, len(users.Nodes))
	for _, user := range users.Nodes {
		names = append(names, user.Name)
	}
	return strings.Join(names, ", ")
}

// join appends a subject to a headline when the subject is not empty.
func join(headline, subject string) string {
	if subject == "" {
		return headline
	}
	return headline + " " + subject
}

// teamName shows the cached "KEY  Name" label for a team ID, falling back to the
// raw ID when this session has not seen the team. A reference that is not a UUID
// is not in the cache and comes back unchanged.
func (s *shared) teamName(id string) string {
	if name, ok := s.names.team(id); ok {
		return name
	}
	return id
}

// projectName shows the cached name for a project ID, falling back to the raw
// ID.
func (s *shared) projectName(id string) string {
	if name, ok := s.names.project(id); ok {
		return name
	}
	return id
}

// milestoneName shows the cached name for a milestone ID, falling back to the
// raw ID.
func (s *shared) milestoneName(id string) string {
	if name, ok := s.names.milestone(id); ok {
		return name
	}
	return id
}

// stateName shows the cached workflow state name for a state ID, falling back to
// the raw ID. A reference that is a state name or a type comes back unchanged.
func (s *shared) stateName(id string) string {
	if name, ok := s.names.state(id); ok {
		return name
	}
	return id
}

// userName shows the cached member name for a user ID, falling back to the raw
// ID. An assignee given as "me", an email, or a name comes back unchanged.
func (s *shared) userName(id string) string {
	if name, ok := s.names.user(id); ok {
		return name
	}
	return id
}

// labelList renders label references, showing a cached name for each ID.
func (s *shared) labelList(refs []string) string {
	names := make([]string, 0, len(refs))
	for _, ref := range refs {
		if name, ok := s.names.label(ref); ok {
			names = append(names, name)
			continue
		}
		names = append(names, ref)
	}
	return strings.Join(names, ", ")
}
