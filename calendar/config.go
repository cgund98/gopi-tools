// Package calendar provides a gogent tool for interacting with Google Calendar.
package calendar

// CalendarConfig holds the configuration needed to authenticate with Google Calendar.
type CalendarConfig struct {
	// CalendarID is the Google Calendar identifier. Use "primary" for the user's primary calendar.
	CalendarID string

	// CredentialsJSON is the raw OAuth 2.0 client credentials JSON downloaded from
	// Google Cloud Console (APIs & Services → Credentials → Create → OAuth client ID → Desktop app).
	CredentialsJSON []byte

	// ClientID is the OAuth 2.0 client ID, used if CredentialsJSON is empty.
	ClientID string

	// ClientSecret is the OAuth 2.0 client secret, used if CredentialsJSON is empty.
	ClientSecret string

	// TokenJSON is the cached OAuth token JSON containing a refresh token.
	// Generate it once by running: go run ./cmd/gcal-auth
	// The file is long-lived (years) as long as your OAuth app is "Published"
	// in the Google Cloud Console consent screen settings.
	TokenJSON []byte
}
