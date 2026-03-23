package api

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type client struct {
	conn   *websocket.Conn
	send   chan []byte
	closed chan struct{}
}

type Hub struct {
	mu      sync.RWMutex
	clients map[*client]bool
}

func NewHub() *Hub {
	return &Hub{
		clients: make(map[*client]bool),
	}
}

func (h *Hub) HandleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	c := &client{conn: conn, send: make(chan []byte, 64), closed: make(chan struct{})}

	h.mu.Lock()
	h.clients[c] = true
	h.mu.Unlock()

	log.Printf("WebSocket client connected (%d total)", h.ClientCount())

	// Read pump — detects disconnect
	go func() {
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				break
			}
		}
		close(c.closed)
	}()

	// Write pump — serializes writes, exits on disconnect
	go func() {
		defer func() {
			h.mu.Lock()
			delete(h.clients, c)
			h.mu.Unlock()
			conn.Close()
			log.Printf("WebSocket client disconnected (%d remaining)", h.ClientCount())
		}()
		for {
			select {
			case msg, ok := <-c.send:
				if !ok {
					return
				}
				conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
				if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
					return
				}
			case <-c.closed:
				return
			}
		}
	}()
}

func (h *Hub) Broadcast(msg WSMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for c := range h.clients {
		// Safely send — channel may be closed if client disconnected
		func() {
			defer func() { recover() }()
			select {
			case c.send <- data:
			default:
			}
		}()
	}
}

func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}
