package websocket

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"

	"mebot/engine"
	"mebot/types"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins in dev
	},
}

// Handler manages WebSocket connections.
type Handler struct {
	engine      *engine.Engine
	connections map[*websocket.Conn]bool
	mu          sync.Mutex
}

// NewHandler creates a new WebSocket handler.
func NewHandler(eng *engine.Engine) *Handler {
	return &Handler{
		engine:      eng,
		connections: make(map[*websocket.Conn]bool),
	}
}

// Broadcast sends an event to all connected clients.
func (h *Handler) Broadcast(event types.WSEvent) {
	data, err := json.Marshal(event)
	if err != nil {
		log.Printf("[WS] Broadcast marshal error: %v", err)
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	for conn := range h.connections {
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			log.Printf("[WS] Broadcast write error: %v", err)
			conn.Close()
			delete(h.connections, conn)
		}
	}
}

// ServeWS handles the /ws endpoint.
func (h *Handler) ServeWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[WS] Upgrade error: %v", err)
		return
	}

	h.mu.Lock()
	h.connections[conn] = true
	h.mu.Unlock()

	defer func() {
		h.mu.Lock()
		delete(h.connections, conn)
		h.mu.Unlock()
		conn.Close()
	}()

	log.Println("[WS] Client connected")

	// Send session start
	sendEvent := func(event types.WSEvent) {
		data, err := json.Marshal(event)
		if err != nil {
			log.Printf("[WS] Marshal error: %v", err)
			return
		}
		h.mu.Lock()
		defer h.mu.Unlock()
		if _, ok := h.connections[conn]; ok {
			if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
				log.Printf("[WS] Write error: %v", err)
			}
		}
	}

	sendEvent(types.WSEvent{Type: "session_start", Content: "Connected to MeBot"})

	// Read loop
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	for {
		_, msgData, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				log.Printf("[WS] Read error: %v", err)
			}
			break
		}

		msg, err := Parse(msgData)
		if err != nil {
			log.Printf("[WS] Parse error: %v", err)
			sendEvent(types.WSEvent{Type: "error", Content: "Invalid message format"})
			continue
		}

		switch msg.Type {
		case "user_message":
			if msg.Content == "" {
				sendEvent(types.WSEvent{Type: "error", Content: "Empty message"})
				continue
			}
			log.Printf("[WS] User message: %s (attachments: %d)", msg.Content, len(msg.Attachments))
			// Run the engine in a goroutine so we don't block the read loop
			go h.engine.HandleMessage(ctx, msg.Content, msg.Attachments, sendEvent)

		case "reset":
			h.engine.ResetConversation()
			sendEvent(types.WSEvent{Type: "session_start", Content: "Conversation reset"})

		case "cancel_run":
			log.Println("[WS] Received cancel request from user")
			h.engine.CancelActiveRun()
			// The engine loop will exit and send its own event, but we can also send an ack.

		case "ping":
			sendEvent(types.WSEvent{Type: "pong"})

		default:
			log.Printf("[WS] Unknown message type: %s", msg.Type)
		}
	}

	log.Println("[WS] Client disconnected")
}
