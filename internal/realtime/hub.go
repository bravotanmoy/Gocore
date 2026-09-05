package realtime

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Event is the JSON envelope pushed to websocket clients.
type Event struct {
	Type    string      `json:"type"`    // e.g. notification, activity, order.created
	Payload interface{} `json:"payload"` // event-specific data
	At      time.Time   `json:"at"`
}

// Client is one connected websocket.
type Client struct {
	UserID uint
	Conn   *websocket.Conn
	Send   chan []byte
}

// Hub tracks connected clients and fans events out to them.
type Hub struct {
	mu      sync.RWMutex
	clients map[*Client]bool
}

// H is the global hub instance.
var H = &Hub{clients: map[*Client]bool{}}

// Register adds a client and starts its writer pump.
func (h *Hub) Register(c *Client) {
	h.mu.Lock()
	h.clients[c] = true
	h.mu.Unlock()

	go func() {
		defer h.Unregister(c)
		for msg := range c.Send {
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.Conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		}
	}()
}

// Unregister removes a client and closes its connection.
func (h *Hub) Unregister(c *Client) {
	h.mu.Lock()
	if _, ok := h.clients[c]; ok {
		delete(h.clients, c)
		close(c.Send)
	}
	h.mu.Unlock()
	_ = c.Conn.Close()
}

// Broadcast sends an event to every connected client.
func (h *Hub) Broadcast(eventType string, payload interface{}) {
	h.send(eventType, payload, 0)
}

// SendToUser sends an event to every connection belonging to one user.
func (h *Hub) SendToUser(userID uint, eventType string, payload interface{}) {
	h.send(eventType, payload, userID)
}

func (h *Hub) send(eventType string, payload interface{}, userID uint) {
	data, err := json.Marshal(Event{Type: eventType, Payload: payload, At: time.Now()})
	if err != nil {
		log.Printf("realtime: marshal failed: %v", err)
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients {
		if userID != 0 && c.UserID != userID {
			continue
		}
		select {
		case c.Send <- data:
		default: // slow client — drop the message rather than block
		}
	}
}

// Count returns the number of live connections (used on the dashboard).
func (h *Hub) Count() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}
