package linear

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

const (
	// defaultCacheTTL is how long a resolved name stays cached.
	defaultCacheTTL = 5 * time.Minute
	// resolveMaxResults caps how many candidates a lookup fetches.
	resolveMaxResults = 250
)

// resolver turns the friendly names the model passes (team keys, state names,
// assignee emails) into Linear IDs, caching each answer for a short time so a
// burst of operations does not repeat the same lookup.
type resolver struct {
	client      linearClient
	defaultTeam string
	// names records every successful resolution so the renderer can show a name
	// for an ID the model passed. A nil cache is a no-op.
	names *nameCache
	ttl   time.Duration

	mu    sync.Mutex
	cache map[string]cacheEntry
}

type cacheEntry struct {
	value   any
	expires time.Time
}

// newResolver builds a name resolver. A nil names cache disables recording.
func newResolver(client linearClient, names *nameCache, defaultTeam string) *resolver {
	return &resolver{
		client:      client,
		defaultTeam: defaultTeam,
		names:       names,
		ttl:         defaultCacheTTL,
		cache:       map[string]cacheEntry{},
	}
}

// get returns a cached value, or calls load and caches the result.
func (r *resolver) get(ctx context.Context, key string, load func(context.Context) (any, error)) (any, error) {
	r.mu.Lock()
	if entry, ok := r.cache[key]; ok && time.Now().Before(entry.expires) {
		value := entry.value
		r.mu.Unlock()
		return value, nil
	}
	r.mu.Unlock()

	value, err := load(ctx)
	if err != nil {
		return nil, err
	}
	r.mu.Lock()
	r.cache[key] = cacheEntry{value: value, expires: time.Now().Add(r.ttl)}
	r.mu.Unlock()
	return value, nil
}

// invalidate drops cached entries whose key starts with prefix.
func (r *resolver) invalidate(prefix string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for key := range r.cache {
		if strings.HasPrefix(key, prefix) {
			delete(r.cache, key)
		}
	}
}

// cached is the typed form of resolver.get.
func cached[T any](ctx context.Context, r *resolver, key string, load func(context.Context) (T, error)) (T, error) {
	value, err := r.get(ctx, key, func(loadCtx context.Context) (any, error) {
		return load(loadCtx)
	})
	if err != nil {
		var zero T
		return zero, err
	}
	typed, ok := value.(T)
	if !ok {
		var zero T
		return zero, fmt.Errorf("internal error: cached %q has type %T", key, value)
	}
	return typed, nil
}

// team resolves a team key, name, or ID. An empty ref uses the default team.
func (r *resolver) team(ctx context.Context, ref string) (teamInfo, error) {
	if ref == "" {
		ref = r.defaultTeam
	}
	if ref == "" {
		return teamInfo{}, fmt.Errorf("team is required and no default_team is configured")
	}
	return cached(ctx, r, "team/"+strings.ToLower(ref), func(ctx context.Context) (teamInfo, error) {
		filter := map[string]any{
			"or": append([]map[string]any{
				{"key": map[string]any{"eqIgnoreCase": ref}},
				{"name": map[string]any{"eqIgnoreCase": ref}},
			}, idBranches(ref)...),
		}
		found, err := r.client.teams(ctx, filter, listOptions{First: resolveMaxResults})
		if err != nil {
			return teamInfo{}, err
		}
		if len(found.Nodes) == 0 {
			return teamInfo{}, fmt.Errorf("no team matches %q", ref)
		}
		if r.names != nil {
			r.names.rememberTeam(found.Nodes[0])
		}
		return found.Nodes[0], nil
	})
}

// state resolves a workflow state name, type, or ID within a team.
func (r *resolver) state(ctx context.Context, teamID, ref string) (workflowStateInfo, error) {
	if ref == "" {
		return workflowStateInfo{}, fmt.Errorf("state is required")
	}
	key := "state/" + teamID + "/" + strings.ToLower(ref)
	return cached(ctx, r, key, func(ctx context.Context) (workflowStateInfo, error) {
		filter := map[string]any{
			"team": map[string]any{"id": map[string]any{"eq": teamID}},
		}
		branches := []map[string]any{{"name": map[string]any{"eqIgnoreCase": ref}}}
		branches = append(branches, stateTypeBranches(ref)...)
		branches = append(branches, idBranches(ref)...)
		filter["or"] = branches
		found, err := r.client.workflowStates(ctx, filter, listOptions{First: resolveMaxResults})
		if err != nil {
			return workflowStateInfo{}, err
		}
		if len(found.Nodes) == 0 {
			return workflowStateInfo{}, fmt.Errorf("no workflow state matches %q in team %s", ref, teamID)
		}
		if r.names != nil {
			r.names.rememberState(found.Nodes[0])
		}
		return found.Nodes[0], nil
	})
}

// user resolves "me", an email, a name, or a user ID.
func (r *resolver) user(ctx context.Context, ref string) (userInfo, error) {
	if ref == "" {
		return userInfo{}, fmt.Errorf("assignee is required")
	}
	if strings.EqualFold(ref, "me") {
		me, err := cached(ctx, r, "viewer", func(ctx context.Context) (viewerInfo, error) {
			return r.client.viewer(ctx)
		})
		if err != nil {
			return userInfo{}, err
		}
		user := userInfo{ID: me.ID, Name: me.Name, Email: me.Email}
		if r.names != nil {
			r.names.rememberUser(user)
		}
		return user, nil
	}
	return cached(ctx, r, "user/"+strings.ToLower(ref), func(ctx context.Context) (userInfo, error) {
		filter := map[string]any{
			"or": append([]map[string]any{
				{"email": map[string]any{"eqIgnoreCase": ref}},
				{"name": map[string]any{"eqIgnoreCase": ref}},
				{"displayName": map[string]any{"eqIgnoreCase": ref}},
			}, idBranches(ref)...),
		}
		found, err := r.client.users(ctx, filter, listOptions{First: resolveMaxResults})
		if err != nil {
			return userInfo{}, err
		}
		if len(found.Nodes) == 0 {
			return userInfo{}, fmt.Errorf("no user matches %q", ref)
		}
		if r.names != nil {
			r.names.rememberUser(found.Nodes[0])
		}
		return found.Nodes[0], nil
	})
}

// project resolves a project name or ID.
func (r *resolver) project(ctx context.Context, ref string) (projectRef, error) {
	if ref == "" {
		return projectRef{}, fmt.Errorf("project is required")
	}
	return cached(ctx, r, "project/"+strings.ToLower(ref), func(ctx context.Context) (projectRef, error) {
		filter := map[string]any{
			"or": append([]map[string]any{
				{"name": map[string]any{"eqIgnoreCase": ref}},
			}, idBranches(ref)...),
		}
		found, err := r.client.projects(ctx, filter, listOptions{First: resolveMaxResults})
		if err != nil {
			return projectRef{}, err
		}
		if len(found.Nodes) == 0 {
			return projectRef{}, fmt.Errorf("no project matches %q", ref)
		}
		if r.names != nil {
			r.names.rememberProject(projectRef{ID: found.Nodes[0].ID, Name: found.Nodes[0].Name})
		}
		return projectRef{ID: found.Nodes[0].ID, Name: found.Nodes[0].Name}, nil
	})
}

// label resolves a label name or ID. When teamID is set, a label owned by that
// team wins and a workspace-wide label is the fallback.
func (r *resolver) label(ctx context.Context, teamID, ref string) (labelInfo, error) {
	if ref == "" {
		return labelInfo{}, fmt.Errorf("label name is empty")
	}
	key := "label/" + teamID + "/" + strings.ToLower(ref)
	return cached(ctx, r, key, func(ctx context.Context) (labelInfo, error) {
		filter := map[string]any{
			"or": append([]map[string]any{
				{"name": map[string]any{"eqIgnoreCase": ref}},
			}, idBranches(ref)...),
		}
		// A team filter would drop workspace-wide labels, which Linear reports
		// with a null team, and those are exactly the labels meant to be
		// assignable to every team. Fetch every match and choose below.
		found, err := r.client.labels(ctx, filter, listOptions{First: resolveMaxResults})
		if err != nil {
			return labelInfo{}, err
		}
		label, ok := matchLabel(found.Nodes, teamID, ref)
		if !ok {
			return labelInfo{}, fmt.Errorf("no label matches %q", ref)
		}
		if r.names != nil {
			r.names.rememberLabel(label)
		}
		return label, nil
	})
}

// projectStatus resolves a project status name or type within a team. Statuses
// are not filterable by the API, so the list is fetched once and cached.
func (r *resolver) projectStatus(ctx context.Context, teamID, ref string) (projectStatusInfo, error) {
	if ref == "" {
		return projectStatusInfo{}, fmt.Errorf("project_status is required")
	}
	statuses, err := cached(ctx, r, "project_statuses", func(ctx context.Context) ([]projectStatusInfo, error) {
		found, err := r.client.projectStatuses(ctx, listOptions{First: resolveMaxResults})
		if err != nil {
			return nil, err
		}
		return found.Nodes, nil
	})
	if err != nil {
		return projectStatusInfo{}, err
	}
	return matchProjectStatus(statuses, teamID, ref)
}

// matchProjectStatus prefers a status owned by the team, then a workspace-level
// status, matching by ID, name, or type. With no team, the first match wins.
func matchProjectStatus(statuses []projectStatusInfo, teamID, ref string) (projectStatusInfo, error) {
	var workspace *projectStatusInfo
	for i := range statuses {
		status := statuses[i]
		if !matchesStatus(status, ref) {
			continue
		}
		if teamID == "" || status.TeamID == teamID {
			return status, nil
		}
		if status.TeamID == "" && workspace == nil {
			workspace = &statuses[i]
		}
	}
	if workspace != nil {
		return *workspace, nil
	}
	return projectStatusInfo{}, fmt.Errorf("no project status matches %q", ref)
}

func matchesStatus(status projectStatusInfo, ref string) bool {
	return status.ID == ref ||
		strings.EqualFold(status.Name, ref) ||
		strings.EqualFold(status.Type, ref)
}

// matchLabel prefers a label owned by the team, then a workspace-wide label,
// matching by ID or name. Linear reports a workspace-wide label with a null
// team, and those labels stay assignable to issues in every team, so they are
// the fallback rather than a reason to fail. With no team, the first match wins.
func matchLabel(labels []labelInfo, teamID, ref string) (labelInfo, bool) {
	var workspace *labelInfo
	for i := range labels {
		label := labels[i]
		if !matchesLabel(label, ref) {
			continue
		}
		if teamID == "" || label.teamID() == teamID {
			return label, true
		}
		if label.teamID() == "" && workspace == nil {
			workspace = &labels[i]
		}
	}
	if workspace != nil {
		return *workspace, true
	}
	return labelInfo{}, false
}

func matchesLabel(label labelInfo, ref string) bool {
	return label.ID == ref || strings.EqualFold(label.Name, ref)
}

// milestone resolves a project milestone name or ID within a project.
func (r *resolver) milestone(ctx context.Context, projectID, ref string) (milestoneInfo, error) {
	if ref == "" {
		return milestoneInfo{}, fmt.Errorf("project_milestone is required")
	}
	key := "milestone/" + projectID + "/" + strings.ToLower(ref)
	return cached(ctx, r, key, func(ctx context.Context) (milestoneInfo, error) {
		filter := map[string]any{
			"project": map[string]any{"id": map[string]any{"eq": projectID}},
			"or": append([]map[string]any{
				{"name": map[string]any{"eqIgnoreCase": ref}},
			}, idBranches(ref)...),
		}
		found, err := r.client.projectMilestones(ctx, filter, listOptions{First: resolveMaxResults})
		if err != nil {
			return milestoneInfo{}, err
		}
		if len(found.Nodes) == 0 {
			return milestoneInfo{}, fmt.Errorf("no milestone matches %q in project %s", ref, projectID)
		}
		if r.names != nil {
			r.names.rememberMilestone(found.Nodes[0])
		}
		return found.Nodes[0], nil
	})
}

// idBranches returns an id equality branch only when ref is a UUID. Linear
// rejects an id filter whose value is not a UUID, and it fails the whole query
// with "Argument Validation Error" rather than matching nothing.
func idBranches(ref string) []map[string]any {
	if !isUUID(ref) {
		return nil
	}
	return []map[string]any{{"id": map[string]any{"eq": ref}}}
}

// stateTypeBranches returns a workflow state type branch only for a known type.
// The field is an enum on Linear's side, so any other value, including a state
// name, fails the whole query.
func stateTypeBranches(ref string) []map[string]any {
	kind := strings.ToLower(ref)
	switch kind {
	case "backlog", "unstarted", "started", "completed", "canceled":
		return []map[string]any{{"type": map[string]any{"eq": kind}}}
	}
	return nil
}
