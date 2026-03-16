package websocket

import "encoding/json"

// IncomingMessage is a message from the frontend.
type IncomingMessage struct {
	Type    string `json:"type"`    // "user_message", "user_response", "ping", "reset"
	Content string `json:"content"` // message text or response value
}

// Parse parses a raw JSON message from the WebSocket.
func Parse(data []byte) (*IncomingMessage, error) {
	var msg IncomingMessage
	err := json.Unmarshal(data, &msg)
	return &msg, err
}
