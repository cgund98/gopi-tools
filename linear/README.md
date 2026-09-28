# Linear Tools

Package `linear` provides six `gogent.Tool` implementations for Linear's GraphQL
API, one per resource:

| Tool | File | Operations |
|------|------|------------|
| `linear_issues` | `linear/tool_issues.go` | list, get, create, update, archive issues; relate issues |
| `linear_projects` | `linear/tool_projects.go` | list, get, create, update projects; create and update milestones |
| `linear_comments` | `linear/tool_comments.go` | create and update comments |
| `linear_teams` | `linear/tool_teams.go` | list and get teams |
| `linear_users` | `linear/tool_users.go` | whoami and list users |
| `linear_labels` | `linear/tool_labels.go` | list and create labels |

All six share one client, one name resolver, and one display-name cache, so a
name one tool records shows up in another tool's output.

Read operations run without approval. Write operations require approval, and
updates and archives show a before/after diff.

## Setup

1. Open **Settings → Account → Security & Access** in Linear and create a
   personal API key.
2. Give the key **Read** and **Write** access, including **Create issues** and
   **Create comments**. A key restricted to specific teams limits every
   operation to those teams.
3. Save the key in `~/.gopi/secrets.toml` (mode `0600`):

   ```toml
   linear_api_key = "lin_api_..."
   ```

4. Add the `[linear]` table to `~/.gopi/config.toml`:

   ```toml
   [linear]
   default_team = "ENG"
   ```

5. Run `go run ./cmd/gopi`.

Linear sends the key as the raw `Authorization` header, without a `Bearer`
prefix. That is what Linear expects for API keys, and it differs from OAuth
tokens.

## Registering the tools

`linear.Options()` is the helper that registers everything. It returns the gopi
options for every tool: all six in agent mode, and the read-only form of each
tool that has a read operation in the plan and ask modes. It reads the `[linear]`
table and the `linear_api_key` secret itself, once, and the tools it builds share
one set of state.

```go
opts = append(opts, linear.Options()...)
```

A program that manages its own modes, or does not use gopi's config, calls
`linear.Tools(cfg)` instead. It returns the six tools built from a
`linear.Config`, with every write enabled:

```go
tools := linear.Tools(linear.Config{APIKey: key, DefaultTeam: "ENG"})
```

## Configuration

The `[linear]` table lives in `~/.gopi/config.toml`. A project
`.gopi/config.toml` is not loaded.

- `default_team` — a team key such as `ENG`, a team name, or a UUID.
  Create operations use it when the model omits `team`. List operations do not:
  they only filter by team when `team` is set, so a query can span teams. Unlike
  the model's `team` argument, which must be a UUID, this may be a name.

`linear.Tools` takes the team directly, so a program that builds the tools
without gopi's config does not need the table. `linear.Options` reads it.

The API key comes from `linear_api_key` in `~/.gopi/secrets.toml`, falling back
to the `LINEAR_API_KEY` environment variable.

## Tools and operations

Every tool takes an `operation` argument, and each advertises only its own
operations. `linear_teams` does not offer `priority`, and `linear_issues` does
not offer `target_date`.

### `linear_issues`

| Operation | Approval | Description |
|-----------|----------|-------------|
| `list_issues` | No | Filter issues by team, assignee, state, project, milestone, labels, priority, text, and update time. |
| `get_issue` | No | A full issue with labels, comments, parent, and sub-issues. |
| `create_issue` | **Yes** | Create an issue. |
| `update_issue` | **Yes** | Change an issue. Shows a diff against its current values. |
| `archive_issue` | **Yes** | Archive an issue. Shows the current issue. |
| `create_relation` | **Yes** | Relate two issues as `blocks`, `blocked_by`, `related`, or `duplicate`. |

### `linear_projects`

| Operation | Approval | Description |
|-----------|----------|-------------|
| `list_projects` | No | Filter projects by team, lead, status, and name. |
| `get_project` | No | A project with its teams and milestones. |
| `create_project` | **Yes** | Create a project. |
| `update_project` | **Yes** | Change a project. Shows a diff. |
| `create_milestone` | **Yes** | Add a project milestone. |
| `update_milestone` | **Yes** | Change a milestone. |

### `linear_comments`

| Operation | Approval | Description |
|-----------|----------|-------------|
| `create_comment` | **Yes** | Comment on an issue. |
| `update_comment` | **Yes** | Replace a comment body. Shows the current body. |

`linear_comments` has no read operation, so it is registered in agent mode only.

### `linear_teams`

| Operation | Approval | Description |
|-----------|----------|-------------|
| `list_teams` | No | Teams and their keys. |
| `get_team` | No | A team with its workflow states, labels, and members. |

### `linear_users`

| Operation | Approval | Description |
|-----------|----------|-------------|
| `get_viewer` | No | The authenticated user. |
| `list_users` | No | Workspace members, or one team's members. |

### `linear_labels`

| Operation | Approval | Description |
|-----------|----------|-------------|
| `list_labels` | No | Issue labels, optionally scoped to a team. |
| `create_label` | **Yes** | Create an issue label. |

## Arguments

The model passes one JSON object per tool. `operation` is required; the rest
apply per operation, and each tool's schema carries only the fields its
operations use. The query controls below are shared by the tools with a list
operation.

- **Identifiers** — `team`, `issue`, `project`, `project_milestone`, `state`,
  `assignee`, `lead`, `parent_issue`, `related_issue`, `comment_id`.
- **Issue fields** — `title`, `description`, `priority`, `estimate`, `due_date`,
  `labels`.
- **Project fields** — `name`, `summary`, `project_status`, `start_date`,
  `target_date`.
- **Comment** — `body`.
- **Relation** — `relation`.
- **Label** — `color`.
- **Query controls** — `query`, `include_archived`, `updated_since`,
  `created_since`, `limit`, `cursor`, `order_by`.

You name most things the way you see them in Linear, and the tool resolves them
to IDs:

- **team**, **project** — a UUID, from `list_teams` or `list_projects`. Names are
  not accepted for these two, which is why a write needs a list call first.
- **issue** — the shorthand identifier (`ENG-123`) or a UUID.
- **state** — a workflow state name, a UUID, or a type: `backlog`, `unstarted`,
  `started`, `completed`, `canceled`.
- **assignee**, **lead** — `me`, an email, a name, or a user ID.
- **project_milestone**, **label** — a name or a UUID. When the team owns a
  label of that name, the team's label wins; otherwise a workspace-wide label is
  used. `list_labels` scoped with `team` returns only the labels that team owns,
  so call it without `team` to also see the workspace-wide labels.

Resolved names are cached for five minutes.

## Names in the UI

Headlines and approval prompts show a name, not a UUID, when the session has
already seen the entity. A call that returns an entity records its ID and name,
and the renderer reads that map. Nothing is fetched while rendering, so a UUID
the session has not seen yet is shown as the raw UUID. The first write of a
session, before any `list_teams` or `list_projects` call, is the likely case.

That map is shared by all six tools, so `linear_teams`' `list_teams` warms the
team name that `linear_issues` then shows in its approval prompt. The map is per
session, in memory, and used for display only.

`priority` uses Linear's scale: `0` None, `1` Urgent, `2` High, `3` Medium,
`4` Low.

`limit` defaults to 25 and caps at 50. A list result carries
`page_info.end_cursor`; pass it back as `cursor` for the next page.

Setting `labels` on `create_issue` or `update_issue` replaces the issue's labels.
An empty list clears them.

Dates are `YYYY-MM-DD`. `updated_since` and `created_since` also accept ISO 8601
durations such as `-P2W`.

## Read-only mode

`linear.Options()` registers the read-only form of every tool that has a read
operation in the plan and ask modes, so gopi can look tickets up while planning
with no chance of a write. A read-only tool rejects a write with
`permission_denied`, never requests approval, and does not list its writes in the
schema at all.

## Errors

Every failure returns structured JSON so the model can react:

```json
{"error": "invalid_argument", "message": "title is required for create_issue"}
```

| Code | Meaning |
|------|---------|
| `invalid_operation` | The operation is not one this tool offers. |
| `invalid_argument` | A field is missing, a name matched nothing, or `team` or `project` was not a UUID. The message names the list operation to call. |
| `permission_denied` | The key lacks Write, is restricted to other teams, or the tool is read-only. |
| `rate_limited` | Linear's rate limit is hit. The message names the reset time. |
| `<operation>_failed` | Any other Linear or transport error. |

## Limits

- **Projects use statuses, not states.** Pass `project_status` as a status name
  or a type: `backlog`, `planned`, `started`, `paused`, `completed`, `canceled`.
  Linear deprecated the fixed project state field this reads.
- **Archiving is the only removal.** `delete_issue` is not exposed. Archiving is
  reversible in Linear's UI; deleting is not.
- **Cycles are not supported.** The tools have no cycle reads or writes.
- **`description` on an update means "leave unchanged" when empty.** You cannot
  clear a description or summary through these tools.
- **`team` and `project` take a UUID.** Pass the ID from `list_teams` or
  `list_projects`; a key or a name is rejected with `invalid_argument`.
- **The UI name map is per session.** It lives in memory, covers only entities an
  earlier call returned, and does not affect what the API receives.
- **Operations are spread across six tools.** Pick the tool for the resource,
  then the operation within it.

Rate limits for API keys are 2,500 requests and about 3,000,000 complexity points
per hour.
