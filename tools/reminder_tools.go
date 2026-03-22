package tools

import (
	"fmt"
	"log"
	"strconv"
	"time"

	"mebot/types"
)

// ---- set_reminder ----

type SetReminderTool struct {
	// Broadcast is a callback to send messages to all active connected clients.
	Broadcast func(event types.WSEvent)
}

func (t *SetReminderTool) Name() string        { return "set_reminder" }
func (t *SetReminderTool) Description() string { return "Set a timed reminder or delayed message" }

func (t *SetReminderTool) Declaration() *types.ToolDeclaration {
	return &types.ToolDeclaration{
		Name:        "set_reminder",
		Description: "Set a one-time reminder or delayed message to be sent to the user after a specific number of seconds. Use this INSTEAD of 'run_command' with sleep for short-term timers/alarms. The timer runs asynchronously and will not block your current execution.",
		Parameters: &types.Schema{
			Type: "OBJECT",
			Properties: map[string]*types.Schema{
				"delay_seconds": {
					Type:        "INTEGER",
					Description: "How many seconds to wait before sending the reminder",
				},
				"message": {
					Type:        "STRING",
					Description: "The exact message/reminder to send to the user when the time is up",
				},
			},
			Required: []string{"delay_seconds", "message"},
		},
	}
}

func (t *SetReminderTool) Execute(args map[string]any) types.ToolResult {
	var delaySecs int
	switch v := args["delay_seconds"].(type) {
	case float64:
		delaySecs = int(v)
	case string:
		var err error
		if delaySecs, err = strconv.Atoi(v); err != nil {
			return types.ToolResult{Status: "error", Output: "delay_seconds must be a valid number"}
		}
	default:
		return types.ToolResult{Status: "error", Output: "Missing or invalid required argument: delay_seconds"}
	}

	message, ok := args["message"].(string)
	if !ok || message == "" {
		return types.ToolResult{Status: "error", Output: "Missing required argument: message"}
	}
	if delaySecs <= 0 {
		return types.ToolResult{Status: "error", Output: "delay_seconds must be > 0"}
	}
	if delaySecs > 86400 {
		// Prevent super long dangling goroutines
		return types.ToolResult{Status: "error", Output: "delay_seconds cannot exceed 24 hours (86400s). For longer recurring jobs, consider creating a skill with a cron expression."}
	}

	go func() {
		time.Sleep(time.Duration(delaySecs) * time.Second)
		log.Printf("[Reminder] Firing reminder: %s", message)
		if t.Broadcast != nil {
			t.Broadcast(types.WSEvent{
				Type:    "assistant",
				Content: fmt.Sprintf("⏰ **Reminder:** %s", message),
			})
		}
	}()

	return types.ToolResult{
		Status: "success",
		Output: fmt.Sprintf("Successfully scheduled reminder to fire in %d seconds. You can now reply to the user confirming it.", delaySecs),
	}
}
