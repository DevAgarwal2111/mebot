package google

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

// GmailService wraps the Google Gmail API
type GmailService struct {
	srv *gmail.Service
}

// NewGmailService creates a new Gmail service client
func NewGmailService(ctx context.Context) (*GmailService, error) {
	client, err := GetClient(ctx)
	if err != nil {
		return nil, err
	}

	srv, err := gmail.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve Gmail client: %v", err)
	}

	return &GmailService{srv: srv}, nil
}

// ListUnreadEmails gets a summary of up to 5 unread emails
func (s *GmailService) ListUnreadEmails() (string, error) {
	user := "me"
	r, err := s.srv.Users.Messages.List(user).Q("is:unread").MaxResults(5).Do()
	if err != nil {
		return "", fmt.Errorf("unable to retrieve messages: %v", err)
	}

	if len(r.Messages) == 0 {
		return "No new unread emails.", nil
	}

	var result strings.Builder
	result.WriteString("Recent Unread Emails:\n")

	for _, m := range r.Messages {
		msg, err := s.srv.Users.Messages.Get(user, m.Id).Format("metadata").MetadataHeaders("Subject", "From").Do()
		if err != nil {
			continue
		}

		var subject, from string
		for _, header := range msg.Payload.Headers {
			if header.Name == "Subject" {
				subject = header.Value
			}
			if header.Name == "From" {
				from = header.Value
			}
		}
		result.WriteString(fmt.Sprintf("- From: %s | Subject: %s\n", from, subject))
	}
	return result.String(), nil
}

// SendEmail sends a plain text email to the given recipient
func (s *GmailService) SendEmail(to, subject, body string) (string, error) {
	messageStr := fmt.Sprintf("To: %s\r\nSubject: %s\r\n\r\n%s", to, subject, body)

	msg := &gmail.Message{
		Raw: base64.URLEncoding.EncodeToString([]byte(messageStr)),
	}

	_, err := s.srv.Users.Messages.Send("me", msg).Do()
	if err != nil {
		return "", fmt.Errorf("failed to send email: %v", err)
	}

	return fmt.Sprintf("Email successfully sent to %s", to), nil
}
