---
todos:
  - id: "init-mod"
    content: "Initialize go module with replace directive for ../gopi"
    status: "completed"
  - id: "read-gopi"
    content: "Read ../gopi source to discover Tool interface and registration patterns"
    status: "completed"
  - id: "scaffold-calendar"
    content: "Create calendar/ package with tool struct, config, and constructor"
    status: "completed"
  - id: "add-deps"
    content: "Add google.golang.org/api/calendar/v3 and run go mod tidy"
    status: "completed"
  - id: "implement-tool"
    content: "Implement calendar tool conforming to gogent.Tool interface"
    status: "completed"
  - id: "add-tests"
    content: "Add tests for approval decisions, execute dispatch, and error handling"
    status: "completed"
  - id: "add-docs"
    content: "Add README.md with usage example and OAuth setup instructions"
    status: "completed"
---
# Bootstrap gopi-tools with Google Calendar Module

## Goal
Create a fresh Go module (`gopi-tools`) that extends the `gopi` personal coding assistant with a modular Google Calendar tool, following the exact patterns and architecture found in the `gopi` repository.

## Context
- Workspace is currently empty at `~/Coding/personal/gopi-tools`.
- Base `gopi` library is at `../gopi` (module: `github.com/cgund98/gopi`).
- `gopi` depends on `github.com/cgund98/gogent` as the agent framework.
- Tools in `gopi` implement `gogent.Tool` and are registered via `gopi.WithTool(mode, tool)`.

## Design Decisions
- **Module path**: `github.com/callum/gopi-tools`
- **Replace directive**: `replace github.com/cgund98/gopi => ../gopi`
- **Public API**: A single exported constructor `NewCalendarTool(cfg CalendarConfig) gogent.Tool` in package `calendar/`.
- **Package layout**: Flat public package `calendar/` at repo root (gopi keeps public API flat, internals in `internal/`).

## gopi Patterns to Follow

### Tool Interface (`gogent.Tool`)
Every tool must implement:
```go
type Tool interface {
    Name() string
    Description() string
    Parameters() json.RawMessage
    RequiresApproval(ctx context.Context, args json.RawMessage) (ApprovalDecision, error)
    Execute(ctx context.Context, args json.RawMessage) (json.RawMessage, error)
}
```

### Schema Generation
Use `github.com/invopop/jsonschema` with struct tags:
```go
type myArgs struct {
    Field string `json:"field" jsonschema:"description=Human-readable description"`
}
func schemaFor(v any) json.RawMessage {
    reflector := jsonschema.Reflector{DoNotReference: true}
    body, _ := json.Marshal(reflector.Reflect(v))
    return body
}
```

### Tool Structure Pattern (from `internal/tools/search.go`, `internal/tools/read.go`)
- Define an `args` struct with `json` + `jsonschema` tags.
- Define a struct type (e.g., `type CalendarTool struct{...}`) implementing `gogent.Tool` with pointer receivers.
- `Name()` returns the tool name (snake_case).
- `Description()` returns a concise description for the model.
- `Parameters()` returns `schemaFor(&argsStruct{})`.
- `RequiresApproval()` — calendar API calls that read events need no approval; write operations (create/update/delete) should require approval with a reason.
- `Execute()` unmarshals args, calls the service, and returns `json.Marshal(map[string]any{...})`.

### Error Handling
- Use `fmt.Errorf("...: %w", err)` for wrapped errors.
- For access/policy denials, return a JSON payload like `{"error":"access_denied","message":"..."}` instead of a Go error.

### Testing Pattern (from `internal/tools/tools_test.go`)
- Table-driven tests with `t.Run` subtests.
- Test `RequiresApproval` and `Execute` separately.
- Use `json.RawMessage` for tool args in tests.
- Mock external dependencies (Google Calendar API) with interfaces.

## Steps

### 1. Initialize Go Module
- `go mod init github.com/callum/gopi-tools`
- Add `replace github.com/cgund98/gopi => ../gopi`
- `go mod tidy` to pull in gopi/gogent dependencies.

### 2. Scaffold `calendar/` Package
Create `calendar/calendar.go` with:
- `CalendarConfig` struct for OAuth credentials, calendar ID, etc.
- `CalendarTool` struct implementing `gogent.Tool`.
- `NewCalendarTool(cfg CalendarConfig) gogent.Tool` constructor.

### 3. Implement Tool Methods
- `Name()`: `"google_calendar"`
- `Description()`: Describe what the tool can do (list events, create events, etc.).
- `Parameters()`: Schema for args (operation type, date range, event details).
- `RequiresApproval()`: Approve write operations (`create`, `update`, `delete`). Reads (`list`, `get`) auto-approve.
- `Execute()`: Dispatch to internal service methods based on operation.

### 4. Add Google Calendar API Client
- `go get google.golang.org/api/calendar/v3`
- Create `calendar/client.go` with a thin wrapper around the Google Calendar API.
- Define a local interface so tests can mock it.

### 5. Add Tests
- `calendar/calendar_test.go` with tests for:
  - Schema generation
  - Approval decisions (read vs write)
  - Execute with mocked client
  - Error handling

### 6. Add Example / Documentation
- `README.md` with usage example:
  ```go
  err := gopi.Run(ctx,
      gopi.WithTool(gopi.ModeAgent, calendar.NewCalendarTool(cfg)),
  )
  ```
- `calendar/config.go` documenting required OAuth setup.

## Acceptance Criteria
- `go build ./...` succeeds.
- `go test ./...` passes.
- The tool satisfies `gogent.Tool` and can be passed to `gopi.WithTool`.
- Read operations do not require approval; write operations do.
- Errors return structured JSON, not raw Go error strings.
