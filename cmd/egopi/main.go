package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"

	"github.com/cgund98/gogent"
	"github.com/cgund98/gopi"

	"github.com/cgund98/gopi-tools/calendar"
	"github.com/cgund98/gopi-tools/linear"
)

const usage = `egopi [flags] [workspace]

Start the coding agent in a workspace, with the calendar and Linear tools.

  workspace             directory to work in (default: current directory)

Flags:
`

// calendarSettings is the [calendar] table in ~/.gopi/config.toml.
type calendarSettings struct {
	// DefaultCalendar is the Google Calendar ID passed to every operation.
	// Empty falls back to "primary".
	DefaultCalendar string `toml:"default_calendar"`
}

func main() {
	resumeFlag := flag.Bool("resume", false, "open the newest saved session")
	flag.Usage = func() {
		fmt.Fprint(os.Stderr, usage)
		flag.PrintDefaults()
	}
	flag.Parse()

	workspace := flag.Arg(0)
	if flag.NArg() > 1 {
		fmt.Fprintln(os.Stderr, "gopi: at most one workspace directory")
		flag.Usage()
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	var opts []gopi.Option
	if workspace != "" {
		opts = append(opts, gopi.WithWorkspace(workspace))
	}
	if *resumeFlag {
		opts = append(opts, gopi.WithResume())
	}
	opts = append(opts, gopi.WithToolFactory(gopi.ModeAgent, newCalendarTool))
	// The Linear tools register themselves: every tool in agent mode, and the
	// read-only form of each in plan and ask mode, so gopi can look tickets up
	// while planning with no chance of a write.
	opts = append(opts, linear.Options()...)

	if err := gopi.Run(ctx, opts...); err != nil {
		fmt.Fprintf(os.Stderr, "gopi: %v\n", err)
		os.Exit(1)
	}
}

// newCalendarTool reads the [calendar] table from ~/.gopi/config.toml, where
// default_calendar names the Google Calendar ID. It reads gcal_token from
// ~/.gopi/secrets.toml, usually as gcal_token = { file = "~/.secrets/gcal_token.json" }.
// A token stored as a file carries a bearer token that the client uses on its
// own, so the OAuth client is only read when the token is not a file. In that
// case the client ID and secret come from gcal_client_id and gcal_client_secret,
// falling back to the GCAL_CLIENT_ID and GCAL_CLIENT_SECRET environment variables.
func newCalendarTool(env gopi.ToolEnv) (gogent.Tool, error) {
	var settings calendarSettings
	if err := env.Config("calendar", &settings); err != nil {
		return nil, err
	}
	calendarID := settings.DefaultCalendar
	if calendarID == "" {
		calendarID = "primary"
	}

	token, err := env.Secret("gcal_token")
	if err != nil {
		return nil, fmt.Errorf("%w; run go run ./cmd/gcal-auth, then add gcal_token = { file = \"~/.secrets/gcal_token.json\" }", err)
	}

	// A token file is self-sufficient: its bearer token is used directly. The
	// OAuth client is only needed to refresh a token that is not stored as a file.
	var clientID, clientSecret string
	if _, pathErr := env.SecretPath("gcal_token"); pathErr != nil {
		var err error
		clientID, err = secretOrEnv(env, "gcal_client_id", "GCAL_CLIENT_ID")
		if err != nil {
			return nil, err
		}
		clientSecret, err = secretOrEnv(env, "gcal_client_secret", "GCAL_CLIENT_SECRET")
		if err != nil {
			return nil, err
		}
	}
	return calendar.New(calendar.Config{
		CalendarID:   calendarID,
		ClientID:     clientID,
		ClientSecret: clientSecret,
		TokenJSON:    []byte(token),
	}), nil
}

func secretOrEnv(env gopi.ToolEnv, name, envName string) (string, error) {
	if value, err := env.Secret(name); err == nil {
		return value, nil
	}
	if value := os.Getenv(envName); value != "" {
		return value, nil
	}
	return "", fmt.Errorf("set %s in ~/.gopi/secrets.toml or %s in the environment", name, envName)
}
