package linear

// viewerInfo is the authenticated user, from the viewer query.
type viewerInfo struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// userInfo is a workspace member.
type userInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	DisplayName string `json:"display_name,omitempty"`
	Email       string `json:"email,omitempty"`
}

// teamInfo identifies a Linear team.
type teamInfo struct {
	ID   string `json:"id"`
	Key  string `json:"key"`
	Name string `json:"name"`
}

// workflowStateInfo is one state in a team's issue pipeline.
type workflowStateInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// labelInfo is an issue label. Linear exposes the owning team as a relation and
// reports a workspace-wide label with a null team; Team is nil for those, and
// the label stays assignable to issues in every team.
type labelInfo struct {
	ID    string     `json:"id"`
	Name  string     `json:"name"`
	Color string     `json:"color,omitempty"`
	Team  *labelTeam `json:"team,omitempty"`
}

// labelTeam is the team a label belongs to.
type labelTeam struct {
	ID string `json:"id"`
}

// teamID is the owning team's ID, or empty for a workspace-wide label.
func (l labelInfo) teamID() string {
	if l.Team == nil {
		return ""
	}
	return l.Team.ID
}

// projectStatusInfo is one project status option.
type projectStatusInfo struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Type   string `json:"type"`
	TeamID string `json:"team_id,omitempty"`
}

// issueRef is a minimal issue reference, used for parents, children, and
// relations.
type issueRef struct {
	ID         string `json:"id"`
	Identifier string `json:"identifier"`
	Title      string `json:"title"`
}

// projectRef is a minimal project reference.
type projectRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// commentInfo is an issue comment.
type commentInfo struct {
	ID        string    `json:"id"`
	Body      string    `json:"body"`
	User      *userInfo `json:"user,omitempty"`
	CreatedAt string    `json:"created_at,omitempty"`
	URL       string    `json:"url,omitempty"`
}

// issueSummary is a compact issue, returned by list_issues and the write
// operations.
type issueSummary struct {
	ID         string             `json:"id"`
	Identifier string             `json:"identifier"`
	Title      string             `json:"title"`
	State      *workflowStateInfo `json:"state,omitempty"`
	Assignee   *userInfo          `json:"assignee,omitempty"`
	Project    *projectRef        `json:"project,omitempty"`
	Priority   float64            `json:"priority"`
	Estimate   float64            `json:"estimate,omitempty"`
	DueDate    string             `json:"due_date,omitempty"`
	UpdatedAt  string             `json:"updated_at,omitempty"`
	ArchivedAt string             `json:"archived_at,omitempty"`
	URL        string             `json:"url"`
}

// issueDetail is the full issue, returned by get_issue.
type issueDetail struct {
	ID          string                   `json:"id"`
	Identifier  string                   `json:"identifier"`
	Title       string                   `json:"title"`
	Description string                   `json:"description,omitempty"`
	State       *workflowStateInfo       `json:"state,omitempty"`
	Assignee    *userInfo                `json:"assignee,omitempty"`
	Team        *teamInfo                `json:"team,omitempty"`
	Project     *projectRef              `json:"project,omitempty"`
	Parent      *issueRef                `json:"parent,omitempty"`
	Children    *connection[issueRef]    `json:"children,omitempty"`
	Labels      *connection[labelInfo]   `json:"labels,omitempty"`
	Comments    *connection[commentInfo] `json:"comments,omitempty"`
	Priority    float64                  `json:"priority"`
	Estimate    float64                  `json:"estimate,omitempty"`
	DueDate     string                   `json:"due_date,omitempty"`
	CreatedAt   string                   `json:"created_at,omitempty"`
	UpdatedAt   string                   `json:"updated_at,omitempty"`
	ArchivedAt  string                   `json:"archived_at,omitempty"`
	URL         string                   `json:"url"`
}

// projectSummary is a compact project.
type projectSummary struct {
	ID         string             `json:"id"`
	Name       string             `json:"name"`
	Status     *projectStatusInfo `json:"status,omitempty"`
	Lead       *userInfo          `json:"lead,omitempty"`
	Progress   float64            `json:"progress"`
	StartDate  string             `json:"start_date,omitempty"`
	TargetDate string             `json:"target_date,omitempty"`
	CreatedAt  string             `json:"created_at,omitempty"`
	UpdatedAt  string             `json:"updated_at,omitempty"`
	ArchivedAt string             `json:"archived_at,omitempty"`
}

// projectDetail is the full project.
type projectDetail struct {
	ID          string                     `json:"id"`
	Name        string                     `json:"name"`
	Description string                     `json:"description,omitempty"`
	Content     string                     `json:"content,omitempty"`
	Status      *projectStatusInfo         `json:"status,omitempty"`
	Lead        *userInfo                  `json:"lead,omitempty"`
	Progress    float64                    `json:"progress"`
	StartDate   string                     `json:"start_date,omitempty"`
	TargetDate  string                     `json:"target_date,omitempty"`
	Teams       *connection[teamInfo]      `json:"teams,omitempty"`
	Milestones  *connection[milestoneInfo] `json:"milestones,omitempty"`
	CreatedAt   string                     `json:"created_at,omitempty"`
	UpdatedAt   string                     `json:"updated_at,omitempty"`
	ArchivedAt  string                     `json:"archived_at,omitempty"`
}

// milestoneInfo is a project milestone.
type milestoneInfo struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	TargetDate string `json:"target_date,omitempty"`
}

// relationInfo is an issue relation.
type relationInfo struct {
	ID           string    `json:"id"`
	Type         string    `json:"type"`
	Issue        *issueRef `json:"issue,omitempty"`
	RelatedIssue *issueRef `json:"related_issue,omitempty"`
}

// teamDetail is a team with its workflow states, labels, and members.
type teamDetail struct {
	ID          string                         `json:"id"`
	Key         string                         `json:"key"`
	Name        string                         `json:"name"`
	Description string                         `json:"description,omitempty"`
	ArchivedAt  string                         `json:"archived_at,omitempty"`
	States      *connection[workflowStateInfo] `json:"states,omitempty"`
	Labels      *connection[labelInfo]         `json:"labels,omitempty"`
	Members     *connection[userInfo]          `json:"members,omitempty"`
}

// pageInfo is a Relay page cursor.
type pageInfo struct {
	HasNextPage bool   `json:"has_next_page"`
	EndCursor   string `json:"end_cursor,omitempty"`
}

// connection is a Relay connection of nodes plus, on the top-level list
// queries, a page cursor.
type connection[T any] struct {
	Nodes    []T       `json:"nodes"`
	PageInfo *pageInfo `json:"page_info,omitempty"`
}
