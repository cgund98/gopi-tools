# gopi-tools

Modular tools that extend the [gopi](https://github.com/cgund98/gopi) personal coding assistant.

## Google Calendar Tool

Package `calendar` provides a `gogent.Tool` implementation for interacting with Google Calendar.

### Installation

```bash
go get github.com/callum/gopi-tools/calendar
```

### Usage

```go
package main

import (
    "context"
    "os"

    "github.com/cgund98/gopi"
    "github.com/callum/gopi-tools/calendar"
)

func main() {
    ctx := context.Background()

    cfg := calendar.CalendarConfig{
        CalendarID:      "primary",
        CredentialsJSON: os.ReadFile("credentials.json"),
        TokenJSON:       os.ReadFile("token.json"),
    }

    err := gopi.Run(ctx,
        gopi.WithTool(gopi.ModeAgent, calendar.NewCalendarTool(cfg)),
    )
    if err != nil {
        panic(err)
    }
}
```

### Operations

| Operation | Approval Required | Description |
|-----------|------------------|-------------|
| `list_events` | No | List events in a time range. |
| `get_event` | No | Get a single event by ID. |
| `create_event` | **Yes** | Create a new event. |
| `update_event` | **Yes** | Update an existing event. |
| `delete_event` | **Yes** | Delete an event by ID. |

### OAuth Setup (One-Time)

This tool uses **OAuth 2.0 with refresh tokens** so it never expires (as long as your OAuth app is "Published"). You authorize once, save the token, and the tool auto-refreshes access tokens forever.

#### 1. Create OAuth Credentials

1. Go to the [Google Cloud Console](https://console.cloud.google.com/).
2. Create or select a project, then enable the **Google Calendar API**.
3. Go to **APIs & Services → OAuth consent screen**.
4. Set it to **External** and click **Publish App** (not "Testing" — testing mode kills refresh tokens after 7 days).
5. Go to **Credentials → Create Credentials → OAuth client ID → Desktop app**.
6. Download the JSON and save it as `credentials.json`.

#### 2. Authorize Once

Run the built-in auth helper:

```bash
cd path/to/gopi-tools
go run ./cmd/gcal-auth -credentials credentials.json -out token.json
```

This will:
- Open your browser to a Google consent screen.
- Ask you to click **Allow**.
- Capture the callback automatically (no copy-paste of codes).
- Save a long-lived `token.json` containing your `refresh_token`.

Keep `token.json` secure — it grants access to your calendar. Treat it like a password.

#### 3. Use the Token

```go
cfg := calendar.CalendarConfig{
    CalendarID:      "primary",
    CredentialsJSON: os.ReadFile("credentials.json"),
    TokenJSON:       os.ReadFile("token.json"),
}
```

### Tool Arguments

The model passes a JSON object with these fields:

- `operation` (string, required): One of `list_events`, `get_event`, `create_event`, `update_event`, `delete_event`.
- `event_id` (string): Required for `get_event`, `update_event`, `delete_event`.
- `summary` (string): Event title. Required for `create_event`.
- `description` (string): Event description. Optional.
- `location` (string): Event location. Optional.
- `start_time` (string): RFC3339 start time. Required for `create_event`; used as `min_time` for `list_events`.
- `end_time` (string): RFC3339 end time. Required for `create_event`; used as `max_time` for `list_events`.
- `max_results` (int): Max events to return for `list_events`. Default 10.

### How the Token Works

The `token.json` file contains a `refresh_token` — this is the magic that makes it semi-permanent:

- **Access tokens** expire after ~1 hour.
- **Refresh tokens** are used to silently obtain new access tokens.
- If your OAuth app is **Published**, refresh tokens last indefinitely (until revoked or 6 months of inactivity).
- If your app stays in **Testing**, Google kills refresh tokens after 7 days.

The calendar tool uses `golang.org/x/oauth2` with a `TokenSource` that handles auto-refresh automatically — you never touch the token again after the one-time setup.

### Errors

All errors are returned as structured JSON so the model can reason about them:

```json
{"error": "client_setup_failed", "message": "CredentialsJSON is required"}
```

Common error codes:

- `client_setup_failed` — Missing credentials or token.
- `invalid_operation` — Unknown operation name.
- `missing_argument` — Required field missing (e.g., `event_id` for `get_event`).
- `invalid_time` — RFC3339 parse error.
- `list_failed`, `get_failed`, `create_failed`, `update_failed`, `delete_failed` — Google API error.
