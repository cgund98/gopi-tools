package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
)

func main() {
	log.Println("Starting gcal-auth...")

	clientID := flag.String("client-id", os.Getenv("GCAL_CLIENT_ID"), "OAuth client ID")
	clientSecret := flag.String("client-secret", os.Getenv("GCAL_CLIENT_SECRET"), "OAuth client secret")
	_ = flag.String("calendar-id", "primary", "Calendar ID (unused)")
	outPath := flag.String("out", os.Getenv("HOME")+"/.secrets/gcal_token.json", "Output path for token JSON")
	flag.Parse()

	if *clientID == "" || *clientSecret == "" {
		log.Fatalf("client-id and client-secret must be provided via flags or environment variables.")
	}

	// Create the OAuth2 config directly using ID and secret.
	config := &oauth2.Config{
		ClientID:     *clientID,
		ClientSecret: *clientSecret,
		RedirectURL:  "http://127.0.0.1:0/callback", // Temporary placeholder
		Scopes:       []string{calendar.CalendarScope},
		Endpoint:     google.Endpoint,
	}

	// Listen on a random available localhost port.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatalf("listen: %v", err)
	}
	defer listener.Close() // Ensure the listener is closed after main returns
	port := listener.Addr().(*net.TCPAddr).Port
	log.Printf("Listening on port %d\n", port)
	config.RedirectURL = fmt.Sprintf("http://127.0.0.1:%d/callback", port)

	// Prompt for user authorization
	authURL := config.AuthCodeURL("state", oauth2.AccessTypeOffline)
	log.Printf("Visit the URL for authorization: %s\n", authURL)

	// Accept the OAuth callback connection and parse the auth code.
	conn, err := listener.Accept()
	if err != nil {
		log.Fatalf("Failed to accept connection: %v", err)
	}
	defer conn.Close()

	reader := bufio.NewReader(conn)
	reqLine, err := reader.ReadString('\n')
	if err != nil {
		log.Fatalf("Failed to read request: %v", err)
	}

	// Parse the GET path to extract the authorization code.
	parts := strings.Split(reqLine, " ")
	if len(parts) < 2 {
		log.Fatalf("Invalid HTTP request")
	}
	parsedURL, err := url.Parse(parts[1])
	if err != nil {
		log.Fatalf("Failed to parse callback URL: %v", err)
	}
	authorizationCode := parsedURL.Query().Get("code")
	if authorizationCode == "" {
		log.Fatalf("No authorization code found in callback")
	}

	// Send a simple response back to the browser.
	response := "HTTP/1.1 200 OK\r\nContent-Type: text/html\r\nConnection: close\r\n\r\n<h1>Authentication successful!</h1><p>You can close this window.</p>"
	conn.Write([]byte(response))
	log.Println("Received authorization code")

	// Exchange the authorization code for a token
	token, err := config.Exchange(context.Background(), authorizationCode)
	if err != nil {
		log.Fatalf("Failed to exchange authorization code for token: %v", err)
	}

	log.Println("Token retrieved successfully!")

	// Convert token to JSON format and write it to the token file.
	tokenJSON, err := json.Marshal(token)
	if err != nil {
		log.Fatalf("Failed to marshal token: %v", err)
	}

	tokenFile := *outPath
	if err := os.MkdirAll(filepath.Dir(tokenFile), 0700); err != nil {
		log.Fatalf("Unable to create output directory: %v", err)
	}
	if err := os.WriteFile(tokenFile, tokenJSON, 0600); err != nil {
		log.Fatalf("Failed to write token to file: %v", err)
	}

	log.Println("Token saved to:", tokenFile)
}
