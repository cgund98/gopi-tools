package linear

import "sync"

// nameCache maps entity IDs to display names so the renderer can show a name
// where the model passed a UUID.
//
// Renderers run inside the terminal UI's paint loop with no context and cannot
// call the API, so everything they can show has to have been recorded by an
// earlier call. The cache is populated from two places: resolver lookups, and
// the payload of every completed tool call.
//
// The zero value is an empty cache, ready to use. It is never reassigned, so a
// caller may read it without holding the tool's own lock.
type nameCache struct {
	mu         sync.RWMutex
	teams      map[string]string
	projects   map[string]string
	users      map[string]string
	labels     map[string]string
	states     map[string]string
	milestones map[string]string
}

// store records one ID to name pair, ignoring an empty ID or name and
// allocating the map on first use.
func (c *nameCache) store(index *map[string]string, id, name string) {
	if id == "" || name == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if *index == nil {
		*index = map[string]string{}
	}
	(*index)[id] = name
}

// lookup reads one ID from a map. A nil map simply has no entry.
func (c *nameCache) lookup(index *map[string]string, id string) (string, bool) {
	if id == "" {
		return "", false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	name, ok := (*index)[id]
	return name, ok
}

func (c *nameCache) rememberTeam(team teamInfo) {
	c.store(&c.teams, team.ID, teamLabel(&team))
}

func (c *nameCache) rememberProject(project projectRef) {
	c.store(&c.projects, project.ID, project.Name)
}

func (c *nameCache) rememberUser(user userInfo) {
	c.store(&c.users, user.ID, userName(&user))
}

func (c *nameCache) rememberLabel(label labelInfo) {
	c.store(&c.labels, label.ID, label.Name)
}

func (c *nameCache) rememberState(state workflowStateInfo) {
	c.store(&c.states, state.ID, state.Name)
}

func (c *nameCache) rememberMilestone(milestone milestoneInfo) {
	c.store(&c.milestones, milestone.ID, milestone.Name)
}

// team returns the cached "KEY  Name" label for a team ID.
func (c *nameCache) team(id string) (string, bool) { return c.lookup(&c.teams, id) }

// project returns the cached project name for a project ID.
func (c *nameCache) project(id string) (string, bool) { return c.lookup(&c.projects, id) }

// user returns the cached member name for a user ID.
func (c *nameCache) user(id string) (string, bool) { return c.lookup(&c.users, id) }

// label returns the cached label name for a label ID.
func (c *nameCache) label(id string) (string, bool) { return c.lookup(&c.labels, id) }

// state returns the cached workflow state name for a state ID.
func (c *nameCache) state(id string) (string, bool) { return c.lookup(&c.states, id) }

// milestone returns the cached milestone name for a milestone ID.
func (c *nameCache) milestone(id string) (string, bool) {
	return c.lookup(&c.milestones, id)
}

// rememberResult records every named entity in a decoded tool payload, so the
// next render can show names for the IDs it carries.
func (c *nameCache) rememberResult(result linearResult) {
	for _, team := range result.Teams {
		c.rememberTeam(team)
	}
	if result.Team != nil {
		c.rememberTeam(teamInfo{ID: result.Team.ID, Key: result.Team.Key, Name: result.Team.Name})
		if result.Team.States != nil {
			for _, state := range result.Team.States.Nodes {
				c.rememberState(state)
			}
		}
		if result.Team.Labels != nil {
			for _, label := range result.Team.Labels.Nodes {
				c.rememberLabel(label)
			}
		}
		if result.Team.Members != nil {
			for _, user := range result.Team.Members.Nodes {
				c.rememberUser(user)
			}
		}
	}
	for _, issue := range result.Issues {
		c.rememberIssue(issue)
	}
	if result.Issue != nil {
		c.rememberIssueView(*result.Issue)
	}
	for _, project := range result.Projects {
		c.rememberProject(projectRef{ID: project.ID, Name: project.Name})
		if project.Lead != nil {
			c.rememberUser(*project.Lead)
		}
	}
	if result.Project != nil {
		c.rememberProject(projectRef{ID: result.Project.ID, Name: result.Project.Name})
		if result.Project.Lead != nil {
			c.rememberUser(*result.Project.Lead)
		}
	}
	for _, user := range result.Users {
		c.rememberUser(user)
	}
	for _, label := range result.Labels {
		c.rememberLabel(label)
	}
	if result.Comment != nil && result.Comment.User != nil {
		c.rememberUser(*result.Comment.User)
	}
	if result.Milestone != nil {
		c.rememberMilestone(*result.Milestone)
	}
	if result.Label != nil {
		c.rememberLabel(*result.Label)
	}
}

func (c *nameCache) rememberIssue(issue issueSummary) {
	if issue.State != nil {
		c.rememberState(*issue.State)
	}
	if issue.Assignee != nil {
		c.rememberUser(*issue.Assignee)
	}
	if issue.Project != nil {
		c.rememberProject(*issue.Project)
	}
}

func (c *nameCache) rememberIssueView(issue issueView) {
	if issue.State != nil {
		c.rememberState(*issue.State)
	}
	if issue.Assignee != nil {
		c.rememberUser(*issue.Assignee)
	}
	if issue.Team != nil {
		c.rememberTeam(*issue.Team)
	}
	if issue.Project != nil {
		c.rememberProject(*issue.Project)
	}
	if issue.Labels != nil {
		for _, label := range issue.Labels.Nodes {
			c.rememberLabel(label)
		}
	}
}

// rememberIssueDetail records a full issue fetched for an approval diff.
func (c *nameCache) rememberIssueDetail(issue issueDetail) {
	if issue.State != nil {
		c.rememberState(*issue.State)
	}
	if issue.Assignee != nil {
		c.rememberUser(*issue.Assignee)
	}
	if issue.Team != nil {
		c.rememberTeam(*issue.Team)
	}
	if issue.Project != nil {
		c.rememberProject(*issue.Project)
	}
	if issue.Labels != nil {
		for _, label := range issue.Labels.Nodes {
			c.rememberLabel(label)
		}
	}
}

// rememberProjectDetail records a full project fetched for an approval diff.
func (c *nameCache) rememberProjectDetail(project projectDetail) {
	c.rememberProject(projectRef{ID: project.ID, Name: project.Name})
	if project.Lead != nil {
		c.rememberUser(*project.Lead)
	}
	if project.Teams != nil {
		for _, team := range project.Teams.Nodes {
			c.rememberTeam(team)
		}
	}
	if project.Milestones != nil {
		for _, milestone := range project.Milestones.Nodes {
			c.rememberMilestone(milestone)
		}
	}
}
