// Package linear provides a gogent tool for reading and writing Linear issues,
// projects, comments, labels, and milestones through Linear's GraphQL API.
//
// Authentication uses a personal Linear API key, sent as the raw Authorization
// header. Read operations run without approval; write operations require the
// user to approve them, and updates and archives show a before/after diff.
package linear

import "net/http"

// Config holds the configuration needed to authenticate with Linear.
type Config struct {
	// APIKey is a personal Linear API key. It is sent as the raw Authorization
	// header, without a "Bearer" prefix, matching Linear's API key
	// authentication. Create one under Settings -> Account -> Security & Access.
	APIKey string

	// DefaultTeam is a team key (such as "ENG"), name, or UUID used whenever an
	// operation omits the team. It removes ambiguity for issue and project
	// creation in multi-team workspaces. Unlike the model-supplied team
	// argument, which must be a UUID, this may be a human-friendly name.
	DefaultTeam string

	// Endpoint overrides the GraphQL endpoint. Empty uses the public Linear API
	// at https://api.linear.app/graphql.
	Endpoint string

	// HTTPClient overrides the HTTP client. Nil uses a client with a timeout.
	HTTPClient *http.Client
}
