package tools

import (
	"context"
	"fmt"
	"mebot/api/google"
	"mebot/types"
	"strings"
)

// googleAuthErrorResult checks if an error is an auth error and returns a
// user-friendly tool result prompting the user to connect their Google account.
// Returns nil if the error is not auth-related.
func googleAuthErrorResult(err error, serviceName string) *types.ToolResult {
	if err == nil {
		return nil
	}
	errMsg := err.Error()
	if strings.Contains(errMsg, "authentication required") || strings.Contains(errMsg, "not configured") {
		authURL := google.GetAuthURL()
		var msg string
		if authURL != "" {
			msg = fmt.Sprintf(
				"🔐 Google account not connected yet! To use %s, please connect your Google account by visiting this link:\n\n%s\n\nAfter authorizing, try your request again.",
				serviceName, authURL,
			)
		} else {
			msg = fmt.Sprintf(
				"🔐 Google API is not configured. To use %s, please add GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET to your .env file and restart the server.",
				serviceName,
			)
		}
		return &types.ToolResult{Status: "error", Output: msg}
	}
	return nil
}

// ---- google_calendar_list_events ----

type GoogleCalendarListEventsTool struct{}

func (t *GoogleCalendarListEventsTool) Name() string { return "google_calendar_list_events" }
func (t *GoogleCalendarListEventsTool) Description() string {
	return "List the user's upcoming Google Calendar events"
}

func (t *GoogleCalendarListEventsTool) Declaration() *types.ToolDeclaration {
	return &types.ToolDeclaration{
		Name:        "google_calendar_list_events",
		Description: "Fetches the next 10 upcoming events from the user's primary Google Calendar.",
		Parameters: &types.Schema{
			Type:       "OBJECT",
			Properties: map[string]*types.Schema{},
		},
	}
}

func (t *GoogleCalendarListEventsTool) Execute(args map[string]any) types.ToolResult {
	srv, err := google.NewCalendarService(context.Background())
	if err != nil {
		if authErr := googleAuthErrorResult(err, "Google Calendar"); authErr != nil {
			return *authErr
		}
		return types.ToolResult{Status: "error", Output: "Failed to initialize Calendar API: " + err.Error()}
	}

	res, err := srv.ListUpcomingEvents()
	if err != nil {
		return types.ToolResult{Status: "error", Output: "Error fetching events: " + err.Error()}
	}

	return types.ToolResult{Status: "success", Output: res}
}

// ---- google_calendar_create_event ----

type GoogleCalendarCreateEventTool struct{}

func (t *GoogleCalendarCreateEventTool) Name() string { return "google_calendar_create_event" }
func (t *GoogleCalendarCreateEventTool) Description() string {
	return "Create a quick Google Calendar event"
}

func (t *GoogleCalendarCreateEventTool) Declaration() *types.ToolDeclaration {
	return &types.ToolDeclaration{
		Name:        "google_calendar_create_event",
		Description: "Creates an event on the user's Google Calendar using natural language. Ensure you include the date/time.",
		Parameters: &types.Schema{
			Type: "OBJECT",
			Properties: map[string]*types.Schema{
				"text": {
					Type:        "STRING",
					Description: "Natural language text for the event, e.g. 'Coffee with John tomorrow at 10am'",
				},
			},
			Required: []string{"text"},
		},
	}
}

func (t *GoogleCalendarCreateEventTool) Execute(args map[string]any) types.ToolResult {
	text, _ := args["text"].(string)
	if text == "" {
		return types.ToolResult{Status: "error", Output: "Missing required argument: text"}
	}

	srv, err := google.NewCalendarService(context.Background())
	if err != nil {
		if authErr := googleAuthErrorResult(err, "Google Calendar"); authErr != nil {
			return *authErr
		}
		return types.ToolResult{Status: "error", Output: "Failed to initialize Calendar API: " + err.Error()}
	}

	res, err := srv.CreateEvent(text)
	if err != nil {
		return types.ToolResult{Status: "error", Output: "Error creating event: " + err.Error()}
	}

	return types.ToolResult{Status: "success", Output: res}
}

// ---- google_gmail_list_unread ----

type GoogleGmailListUnreadTool struct{}

func (t *GoogleGmailListUnreadTool) Name() string { return "google_gmail_list_unread" }
func (t *GoogleGmailListUnreadTool) Description() string {
	return "List recent unread emails from Gmail"
}

func (t *GoogleGmailListUnreadTool) Declaration() *types.ToolDeclaration {
	return &types.ToolDeclaration{
		Name:        "google_gmail_list_unread",
		Description: "Fetches a summary of the 5 most recent unread emails in the user's Gmail inbox.",
		Parameters: &types.Schema{
			Type:       "OBJECT",
			Properties: map[string]*types.Schema{},
		},
	}
}

func (t *GoogleGmailListUnreadTool) Execute(args map[string]any) types.ToolResult {
	srv, err := google.NewGmailService(context.Background())
	if err != nil {
		if authErr := googleAuthErrorResult(err, "Gmail"); authErr != nil {
			return *authErr
		}
		return types.ToolResult{Status: "error", Output: "Failed to initialize Gmail API: " + err.Error()}
	}

	res, err := srv.ListUnreadEmails()
	if err != nil {
		return types.ToolResult{Status: "error", Output: "Error fetching emails: " + err.Error()}
	}

	return types.ToolResult{Status: "success", Output: res}
}

// ---- google_gmail_send_email ----

type GoogleGmailSendEmailTool struct{}

func (t *GoogleGmailSendEmailTool) Name() string        { return "google_gmail_send_email" }
func (t *GoogleGmailSendEmailTool) Description() string { return "Send a plain-text email via Gmail" }

func (t *GoogleGmailSendEmailTool) Declaration() *types.ToolDeclaration {
	return &types.ToolDeclaration{
		Name:        "google_gmail_send_email",
		Description: "Sends an email from the user's Gmail account.",
		Parameters: &types.Schema{
			Type: "OBJECT",
			Properties: map[string]*types.Schema{
				"to": {
					Type:        "STRING",
					Description: "Recipient email address",
				},
				"subject": {
					Type:        "STRING",
					Description: "Subject of the email",
				},
				"body": {
					Type:        "STRING",
					Description: "Plain text body of the email",
				},
			},
			Required: []string{"to", "subject", "body"},
		},
	}
}

func (t *GoogleGmailSendEmailTool) Execute(args map[string]any) types.ToolResult {
	to, _ := args["to"].(string)
	subject, _ := args["subject"].(string)
	body, _ := args["body"].(string)

	if to == "" || subject == "" || body == "" {
		return types.ToolResult{Status: "error", Output: "Missing required arguments"}
	}

	srv, err := google.NewGmailService(context.Background())
	if err != nil {
		if authErr := googleAuthErrorResult(err, "Gmail"); authErr != nil {
			return *authErr
		}
		return types.ToolResult{Status: "error", Output: "Failed to initialize Gmail API: " + err.Error()}
	}

	res, err := srv.SendEmail(to, subject, body)
	if err != nil {
		return types.ToolResult{Status: "error", Output: "Error sending email: " + err.Error()}
	}

	return types.ToolResult{Status: "success", Output: res}
}
