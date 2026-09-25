package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/cgund98/gogent"
	"github.com/cgund98/gopi"
	"github.com/cgund98/gopi-tools/calendar"
)

func main() {
	calendarID := flag.String("calendar", "primary", "Google Calendar ID")
	flag.Parse()

	err := gopi.Run(context.Background(),
		gopi.WithToolFactory(gopi.ModeAgent, func(env gopi.ToolEnv) (gogent.Tool, error) {
			return newCalendarTool(env, *calendarID)
		}),
	)
	if err != nil {
		log.Fatalf("gopi exited with error: %v", err)
	}
}

// newCalendarTool reads gcal_token from ~/.gopi/secrets.toml, usually as
// gcal_token = { file = "~/.secrets/gcal_token.json" }. The OAuth client ID and
// secret come from gcal_client_id and gcal_client_secret, falling back to the
// GCAL_CLIENT_ID and GCAL_CLIENT_SECRET environment variables.
func newCalendarTool(env gopi.ToolEnv, calendarID string) (gogent.Tool, error) {
	token, err := env.Secret("gcal_token")
	if err != nil {
		return nil, fmt.Errorf("%w; run go run ./cmd/gcal-auth, then add gcal_token = { file = \"~/.secrets/gcal_token.json\" }", err)
	}
	clientID, err := secretOrEnv(env, "gcal_client_id", "GCAL_CLIENT_ID")
	if err != nil {
		return nil, err
	}
	clientSecret, err := secretOrEnv(env, "gcal_client_secret", "GCAL_CLIENT_SECRET")
	if err != nil {
		return nil, err
	}
	return calendar.NewCalendarTool(calendar.CalendarConfig{
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
