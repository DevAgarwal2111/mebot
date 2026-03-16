package google

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"mebot/config"
	"net/http"
	"os"
	"path/filepath"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/gmail/v1"
)

var (
	oauthConfig *oauth2.Config
	tokenCache  = "token.json"
)

// InitAuth initializes the Google OAuth2 configuration
func InitAuth(cfg *config.Config) error {
	if cfg.GoogleClientID == "" || cfg.GoogleClientSecret == "" {
		log.Println("Google API Credentials not provided, skipping Google integration.")
		return nil
	}

	oauthConfig = &oauth2.Config{
		ClientID:     cfg.GoogleClientID,
		ClientSecret: cfg.GoogleClientSecret,
		Endpoint:     google.Endpoint,
		RedirectURL:  "http://localhost:8085/auth/google/callback",
		Scopes: []string{
			calendar.CalendarScope,   // Read/Write Calendar
			gmail.GmailSendScope,     // Send emails
			gmail.GmailReadonlyScope, // Read emails
		},
	}
	log.Println("Google OAuth API Initialized")
	return nil
}

// GetClient retrieves a cached token or triggers the OAuth flow
func GetClient(ctx context.Context) (*http.Client, error) {
	if oauthConfig == nil {
		return nil, fmt.Errorf("Google API not configured (missing CLIENT_ID / CLIENT_SECRET)")
	}

	tok, err := tokenFromFile(tokenCache)
	if err != nil {
		// If token doesn't exist, we must prompt the user
		// In a real headless agent, this flow needs to be handled via the UI or CLI
		log.Println("No Google API token found. User must authenticate!")
		return nil, fmt.Errorf("authentication required")
	}

	return oauthConfig.Client(ctx, tok), nil
}

// Retrieves a token from a local file.
func tokenFromFile(file string) (*oauth2.Token, error) {
	configDir, _ := os.UserConfigDir()
	path := filepath.Join(configDir, "mebot", file)

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

// Gets Auth URL for the user to visit
func GetAuthURL() string {
	if oauthConfig == nil {
		return ""
	}
	return oauthConfig.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
}

// Saves a token to a file path after callback
func SaveToken(path string, token *oauth2.Token) error {
	configDir, _ := os.UserConfigDir()
	fullPath := filepath.Join(configDir, "mebot")
	os.MkdirAll(fullPath, 0700)

	f, err := os.OpenFile(filepath.Join(fullPath, path), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
	return nil
}

// HandleCallback exchanges the auth code for a token
func HandleCallback(code string) (*oauth2.Token, error) {
	tok, err := oauthConfig.Exchange(context.TODO(), code)
	if err != nil {
		return nil, err
	}
	err = SaveToken(tokenCache, tok)
	return tok, err
}
