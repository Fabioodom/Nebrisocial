package websocket

import (
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 512
)

// Client represents a connected WebSocket client.
type Client struct {
	Hub    *Hub
	Conn   *websocket.Conn
	Send   chan []byte
	NodeID string
}

// ReadPump pumps messages from the websocket connection to the hub.
// The application runs ReadPump in a per-connection goroutine.
func (c *Client) ReadPump() {
	defer func() {
		c.Hub.Unregister(c)
		c.Conn.Close()
	}()
	c.Conn.SetReadLimit(maxMessageSize)
	c.Conn.SetReadDeadline(time.Now().Add(pongWait))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})
	for {
		_, _, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WS: read error for node %s: %v", c.NodeID, err)
			}
			break
		}
	}
}

// WritePump pumps messages from the hub to the websocket connection.
// A goroutine running WritePump is started for each connection.
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The hub closed the channel.
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.Conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Add queued chat messages to the current websocket message.
			n := len(c.Send)
			for i := 0; i < n; i++ {
				w.Write([]byte("\n"))
				w.Write(<-c.Send)
			}

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// Hub maintains the set of active clients and broadcasts messages to the clients.
type Hub struct {
	// Registered clients by nodeID.
	clients    map[string]map[*Client]bool
	clientsMu  sync.RWMutex

	// Register requests from the clients.
	register   chan *Client

	// Unregister requests from clients.
	unregister chan *Client
}

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[string]map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.clientsMu.Lock()
			if _, exists := h.clients[client.NodeID]; !exists {
				h.clients[client.NodeID] = make(map[*Client]bool)
			}
			h.clients[client.NodeID][client] = true
			h.clientsMu.Unlock()
			log.Printf("WS: client registered on node %s", client.NodeID)

		case client := <-h.unregister:
			h.clientsMu.Lock()
			if clients, exists := h.clients[client.NodeID]; exists {
				if _, ok := clients[client]; ok {
					delete(clients, client)
					close(client.Send)
					if len(clients) == 0 {
						delete(h.clients, client.NodeID)
					}
				}
			}
			h.clientsMu.Unlock()
			log.Printf("WS: client unregistered on node %s", client.NodeID)
		}
	}
}

func (h *Hub) Register(c *Client) {
	h.register <- c
}

func (h *Hub) Unregister(c *Client) {
	h.unregister <- c
}

// Broadcast sends a message to all clients in a specific node.
func (h *Hub) Broadcast(nodeID string, message []byte) {
	h.clientsMu.RLock()
	defer h.clientsMu.RUnlock()

	clients, exists := h.clients[nodeID]
	if !exists {
		return
	}

	for client := range clients {
		select {
		case client.Send <- message:
		default:
			// If buffer is full, unregister client
			go func(c *Client) {
				h.unregister <- c
			}(client)
		}
	}
}
