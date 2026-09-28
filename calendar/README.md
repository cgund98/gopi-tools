# Google Calendar Tool

Package `calendar` provides a `gogent.Tool` implementation for interacting with Google Calendar.

## Installation

```bash
go get github.com/callum/gopi-tools/calendar
```

## Usage

The calendar ID and token come from gopi's config and secrets files, so a
`WithToolFactory` reads them at startup:

```go
package main

import (
    "context"

    "github.com/cgund98/gogent"
    "github.com/cgund98/gopi"
    "github.com/callum/gopi-tools/calendar"
)

// calendarSettings is the [calendar] table in ~/.gopi/config.toml.
type calendarSettings struct {
    DefaultCalendar string `toml:"default_calendar"`
}

func newCalendarTool(env gopi.ToolEnv) (gogent.Tool, error) {
    var settings calendarSettings
    if err := env.Config("calendar", &settings); err != nil {
        return nil, err
    }
    token, err := env.Secret("gcal_token")
    if err != nil {
        return nil, err
    }
    return calendar.New(calendar.Config{
        CalendarID: settings.DefaultCalendar,
        TokenJSON:  []byte(token),
    }), nil
}

func main() {
    err := gopi.Run(context.Background(),
        gopi.WithToolFactory(gopi.ModeAgent, newCalendarTool),
    )
    if err != nil {
        panic(err)
    }
}
```

## Configuration

The calendar ID lives in `~/.gopi/config.toml`, not on the command line:

```toml
[calendar]
default_calendar = "primary"
```

- `default_calendar` — the Google Calendar ID passed to every operation.
  `primary` is the signed-in account's main calendar. Copy another calendar's ID
  from **Settings and sharing → Integrate calendar**. An empty value falls back to
  `primary`.

A `WithToolFactory` reads the table with `env.Config("calendar", &settings)`. The
`[calendar]` table is required there: startup fails with a message naming the
tables that exist when it is missing. `calendar.New` itself takes the
ID directly, so a program that builds the tool without gopi's config does not
need the table.

The OAuth token comes from `gcal_token` in `~/.gopi/secrets.toml`. A token
stored as a file is used as-is, so the OAuth client ID and secret are not read.
For a token stored as an inline string, the client ID and secret come from
`gcal_client_id` and `gcal_client_secret`, falling back to the `GCAL_CLIENT_ID`
and `GCAL_CLIENT_SECRET` environment variables, and the tool refreshes the token
when it expires.

## Operations

| Operation | Approval Required | Description |
|-----------|------------------|-------------|
| `list_events` | No | List events in a time range. |
| `get_event` | No | Get a single event by ID. |
| `create_event` | **Yes** | Create a new event. |
| `update_event` | **Yes** | Update an existing event. |
| `delete_event` | **Yes** | Delete an event by ID. |

## OAuth Setup (One-Time)

This tool uses **OAuth 2.0 with refresh tokens** so it never expires (as long as your OAuth app is "Published"). You authorize once, save the token, and the tool auto-refreshes access tokens forever.

### 1. Create OAuth Credentials

1. Go to the [Google Cloud Console](https://console.cloud.google.com/).
2. Create or select a project, then enable the **Google Calendar API**.
3. Go to **APIs & Services → OAuth consent screen**.
4. Set it to **External** and click **Publish App** (not "Testing" — testing mode kills refresh tokens after 7 days).
5. Go to **Credentials → Create Credentials → OAuth client ID → Desktop app**.
6. Copy the **Client ID** and **Client secret**.

### 2. Authorize Once

Run the built-in auth helper:

```bash
cd path/to/gopi-tools
export GCAL_CLIENT_ID="..."
export GCAL_CLIENT_SECRET="..."
go run ./cmd/gcal-auth
```

This will:
- Open your browser to a Google consent screen.
- Ask you to click **Allow**.
- Capture the callback automatically (no copy-paste of codes).
- Save a long-lived `~/.secrets/gcal_token.json` containing your `refresh_token`.

Keep that file secure — it grants access to your calendar. Treat it like a password.

### 3. Wire Up the Secrets

Add the token to `~/.gopi/secrets.toml` (mode `0600`):

```toml
gcal_token = { file = "~/.secrets/gcal_token.json" }
```

A file token is used on its own, so the OAuth client is not needed. If you store
the token as an inline string instead, add the client so the tool can refresh it:

```toml
gcal_token = "..."
gcal_client_id = "..."
gcal_client_secret = "..."
```

Then set the calendar ID in `~/.gopi/config.toml`:

```toml
[calendar]
default_calendar = "primary"
```

## Tool Arguments

The model passes a JSON object with these fields:

- `operation` (string, required): One of `list_events`, `get_event`, `create_event`, `update_event`, `delete_event`.
- `event_id` (string): Required for `get_event`, `update_event`, `delete_event`.
- `summary` (string): Event title. Required for `create_event`.
- `description` (string): Event description. Optional.
- `location` (string): Event location. Optional.
- `start_time` (string): RFC3339 start time. Required for `create_event`; used as `min_time` for `list_events`.
- `end_time` (string): RFC3339 end time. Required for `create_event`; used as `max_time` for `list_events`.
- `max_results` (int): Max events to return for `list_events`. Default 10.

## How the Token Works

The `~/.secrets/gcal_token.json` file contains a `refresh_token` — this is the magic that makes it semi-permanent:

- **Access tokens** expire after ~1 hour.
- **Refresh tokens** are used to silently obtain new access tokens.
- If your OAuth app is **Published**, refresh tokens last indefinitely (until revoked or 6 months of inactivity).
- If your app stays in **Testing**, Google kills refresh tokens after 7 days.

The calendar tool sends the token's `access_token` as the bearer token. When
OAuth client credentials are configured it uses `golang.org/x/oauth2` with a
`TokenSource` that refreshes the token automatically. Without them the stored
token is used as-is, so run `gcal-auth` again when it expires.

## Errors

All errors are returned as structured JSON so the model can reason about them:

```json
{"error": "client_setup_failed", "message": "TokenJSON is required; run 'go run ./cmd/gcal-auth' to generate it"}
```

Common error codes:

- `client_setup_failed` — Missing token, or an expired token with no OAuth client configured.
- `invalid_operation` — Unknown operation name.
- `missing_argument` — Required field missing (e.g., `event_id` for `get_event`).
- `invalid_time` — RFC3339 parse error.
- `list_failed`, `get_failed`, `create_failed`, `update_failed`, `delete_failed` — Google API error.
