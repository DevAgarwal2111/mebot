package websocket

import (
	"encoding/json"
	
	"mebot/types"
)

// IncomingMessage is a message from the frontend.
type IncomingMessage struct {
	Type        string              `json:"type"`        // "user_message", "user_response", "ping", "reset"
	Content     string              `json:"content"`     // message text or response value
	Attachments []types.ContentPart `json:"attachments"` // Optional files/images from frontend
}

// Parse parses a raw JSON message from the WebSocket.
func Parse(data []byte) (*IncomingMessage, error) {
	var msg IncomingMessage
	err := json.Unmarshal(data, &msg)
	return &msg, err
}
