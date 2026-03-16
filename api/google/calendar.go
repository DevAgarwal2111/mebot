package google

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

// CalendarService wraps the Google Calendar API
type CalendarService struct {
	srv *calendar.Service
}

// NewCalendarService creates a new Google Calendar service client
func NewCalendarService(ctx context.Context) (*CalendarService, error) {
	client, err := GetClient(ctx)
	if err != nil {
		return nil, err
	}

	srv, err := calendar.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve Calendar client: %v", err)
	}

	return &CalendarService{srv: srv}, nil
}

// ListUpcomingEvents lists the next 10 upcoming events from the user's primary calendar
func (c *CalendarService) ListUpcomingEvents() (string, error) {
	t := time.Now().Format(time.RFC3339)
	events, err := c.srv.Events.List("primary").ShowDeleted(false).
		SingleEvents(true).TimeMin(t).MaxResults(10).OrderBy("startTime").Do()
	if err != nil {
		return "", fmt.Errorf("unable to retrieve next ten of the user's events: %v", err)
	}

	if len(events.Items) == 0 {
		return "No upcoming events found.", nil
	}

	var result string
	for _, item := range events.Items {
		date := item.Start.DateTime
		if date == "" {
			date = item.Start.Date
		}
		result += fmt.Sprintf("- %v (%v)\n", item.Summary, date)
	}

	return result, nil
}

// CreateEvent creates a quick event on the user's primary calendar
// Use standard language like "Appointment at Tomorrow 10am-10:25am"
func (c *CalendarService) CreateEvent(text string) (string, error) {
	event, err := c.srv.Events.QuickAdd("primary", text).Do()
	if err != nil {
		return "", fmt.Errorf("unable to create quick event: %v", err)
	}

	return fmt.Sprintf("Event created: %s\nLink: %s", event.Summary, event.HtmlLink), nil
}
