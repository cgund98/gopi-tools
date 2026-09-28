package linear

// The queries below alias camelCase API fields to snake_case output names, so a
// single set of structs in types.go both decodes a response and is marshaled
// back to the model.

const viewerQuery = `
query Viewer {
  viewer {
    id
    name
    email
  }
}`

const teamsQuery = `
query Teams($filter: TeamFilter, $first: Int, $after: String, $includeArchived: Boolean, $orderBy: PaginationOrderBy) {
  teams(filter: $filter, first: $first, after: $after, includeArchived: $includeArchived, orderBy: $orderBy) {
    nodes { id key name }
    page_info: pageInfo { has_next_page: hasNextPage end_cursor: endCursor }
  }
}`

const teamQuery = `
query Team($id: String!) {
  team(id: $id) {
    id
    key
    name
    description
    archived_at: archivedAt
    states(first: 100) { nodes { id name type } }
    labels(first: 200) { nodes { id name color } }
    members(first: 200) { nodes { id name display_name: displayName email } }
  }
}`

const workflowStatesQuery = `
query WorkflowStates($filter: WorkflowStateFilter, $first: Int, $after: String) {
  workflowStates(filter: $filter, first: $first, after: $after) {
    nodes { id name type }
  }
}`

const usersQuery = `
query Users($filter: UserFilter, $first: Int, $after: String, $includeArchived: Boolean, $orderBy: PaginationOrderBy) {
  users(filter: $filter, first: $first, after: $after, includeArchived: $includeArchived, orderBy: $orderBy) {
    nodes { id name display_name: displayName email }
    page_info: pageInfo { has_next_page: hasNextPage end_cursor: endCursor }
  }
}`

const labelsQuery = `
query Labels($filter: IssueLabelFilter, $first: Int, $after: String, $includeArchived: Boolean, $orderBy: PaginationOrderBy) {
  issueLabels(filter: $filter, first: $first, after: $after, includeArchived: $includeArchived, orderBy: $orderBy) {
    nodes { id name color team { id } }
    page_info: pageInfo { has_next_page: hasNextPage end_cursor: endCursor }
  }
}`

const projectStatusesQuery = `
query ProjectStatuses($first: Int, $after: String) {
  projectStatuses(first: $first, after: $after) {
    nodes { id name type team_id: teamId }
  }
}`

const projectMilestonesQuery = `
query ProjectMilestones($filter: ProjectMilestoneFilter, $first: Int, $after: String, $includeArchived: Boolean) {
  projectMilestones(filter: $filter, first: $first, after: $after, includeArchived: $includeArchived) {
    nodes { id name target_date: targetDate }
  }
}`

const issueSummaryFields = `
fragment IssueSummaryFields on Issue {
  id
  identifier
  title
  state { id name type }
  assignee { id name display_name: displayName email }
  project { id name }
  priority
  estimate
  due_date: dueDate
  updated_at: updatedAt
  archived_at: archivedAt
  url
}`

const projectSummaryFields = `
fragment ProjectSummaryFields on Project {
  id
  name
  status { id name type }
  lead { id name display_name: displayName email }
  progress
  start_date: startDate
  target_date: targetDate
  created_at: createdAt
  updated_at: updatedAt
  archived_at: archivedAt
}`

const issuesQuery = `
query Issues($filter: IssueFilter, $first: Int, $after: String, $includeArchived: Boolean, $orderBy: PaginationOrderBy) {
  issues(filter: $filter, first: $first, after: $after, includeArchived: $includeArchived, orderBy: $orderBy) {
    nodes { ...IssueSummaryFields }
    page_info: pageInfo { has_next_page: hasNextPage end_cursor: endCursor }
  }
}` + issueSummaryFields

const issueQuery = `
query Issue($id: String!) {
  issue(id: $id) {
    ...IssueSummaryFields
    description
    created_at: createdAt
    parent { id identifier title }
    team { id key name }
    children(first: 100) { nodes { id identifier title } }
    labels(first: 100) { nodes { id name color } }
    comments(first: 100) { nodes { id body user { id name display_name: displayName email } created_at: createdAt url } }
  }
}` + issueSummaryFields

const projectsQuery = `
query Projects($filter: ProjectFilter, $first: Int, $after: String, $includeArchived: Boolean, $orderBy: PaginationOrderBy) {
  projects(filter: $filter, first: $first, after: $after, includeArchived: $includeArchived, orderBy: $orderBy) {
    nodes { ...ProjectSummaryFields }
    page_info: pageInfo { has_next_page: hasNextPage end_cursor: endCursor }
  }
}` + projectSummaryFields

const projectQuery = `
query Project($id: String!) {
  project(id: $id) {
    ...ProjectSummaryFields
    description
    content
    teams(first: 50) { nodes { id key name } }
    milestones: projectMilestones(first: 100) { nodes { id name target_date: targetDate } }
  }
}` + projectSummaryFields

const commentQuery = `
query Comment($id: String!) {
  comment(id: $id) {
    id
    body
    created_at: createdAt
    url
    user { id name display_name: displayName email }
  }
}`

const issueCreateMutation = `
mutation IssueCreate($input: IssueCreateInput!) {
  issueCreate(input: $input) {
    success
    issue { ...IssueSummaryFields }
  }
}` + issueSummaryFields

const issueUpdateMutation = `
mutation IssueUpdate($id: String!, $input: IssueUpdateInput!) {
  issueUpdate(id: $id, input: $input) {
    success
    issue { ...IssueSummaryFields }
  }
}` + issueSummaryFields

const issueArchiveMutation = `
mutation IssueArchive($id: String!) {
  issueArchive(id: $id) {
    success
    entity { id identifier title }
  }
}`

const commentCreateMutation = `
mutation CommentCreate($input: CommentCreateInput!) {
  commentCreate(input: $input) {
    success
    comment { id body created_at: createdAt url user { id name display_name: displayName email } }
  }
}`

const commentUpdateMutation = `
mutation CommentUpdate($id: String!, $input: CommentUpdateInput!) {
  commentUpdate(id: $id, input: $input) {
    success
    comment { id body created_at: createdAt url user { id name display_name: displayName email } }
  }
}`

const projectCreateMutation = `
mutation ProjectCreate($input: ProjectCreateInput!) {
  projectCreate(input: $input) {
    success
    project { ...ProjectSummaryFields }
  }
}` + projectSummaryFields

const projectUpdateMutation = `
mutation ProjectUpdate($id: String!, $input: ProjectUpdateInput!) {
  projectUpdate(id: $id, input: $input) {
    success
    project { ...ProjectSummaryFields }
  }
}` + projectSummaryFields

const milestoneCreateMutation = `
mutation MilestoneCreate($input: ProjectMilestoneCreateInput!) {
  projectMilestoneCreate(input: $input) {
    success
    projectMilestone { id name target_date: targetDate }
  }
}`

const milestoneUpdateMutation = `
mutation MilestoneUpdate($id: String!, $input: ProjectMilestoneUpdateInput!) {
  projectMilestoneUpdate(id: $id, input: $input) {
    success
    projectMilestone { id name target_date: targetDate }
  }
}`

const labelCreateMutation = `
mutation LabelCreate($input: IssueLabelCreateInput!) {
  issueLabelCreate(input: $input) {
    success
    issueLabel { id name color }
  }
}`

const relationCreateMutation = `
mutation RelationCreate($input: IssueRelationCreateInput!) {
  issueRelationCreate(input: $input) {
    success
    issueRelation {
      id
      type
      issue { id identifier title }
      related_issue: relatedIssue { id identifier title }
    }
  }
}`
