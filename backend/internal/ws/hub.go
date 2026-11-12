// Package ws delivers alerts to browsers over WebSocket.
//
// The Hub is the one place that knows every connected client.
//
//	HTTP handler --Publish--> queue (channel) --Run--> each client's send channel --writeLoop--> browser
//
// The handler only drops the alert into a channel and returns, so sending
// alerts never slows down the location update response.
package ws

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	queueSize      = 256              // alerts waiting to be broadcast
	clientBuffer   = 32               // alerts waiting for one slow client
	writeTimeout   = 10 * time.Second // max time to write one message
	pongTimeout    = 60 * time.Second // drop a client that is silent this long
	pingInterval   = 30 * time.Second // must be shorter than pongTimeout
	maxMessageSize = 1024             // clients only send small control messages
)

// client is one connected browser tab.
type client struct {
	conn *websocket.Conn
	send chan []byte // alerts waiting to be written to this client
}

// Hub tracks connected clients and broadcasts alerts to all of them.
type Hub struct {
	mu      sync.Mutex           // protects clients: several goroutines add, remove and loop over it
	clients map[*client]struct{} // a set of clients
	queue   chan []byte          // alerts waiting to be broadcast
}

// NewHub creates an empty hub. Call Run in a goroutine to start broadcasting.
func NewHub() *Hub {
	return &Hub{
		clients: make(map[*client]struct{}),
		queue:   make(chan []byte, queueSize),
	}
}

// Run takes alerts from the queue and broadcasts them until ctx is cancelled.
func (h *Hub) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case message := <-h.queue:
			h.broadcast(message)
		}
	}
}

// Publish queues an alert for all clients and returns immediately.
// If the queue is full the alert is not pushed live (it is still in the database).
func (h *Hub) Publish(alert any) {
	message, err := json.Marshal(alert)
	if err != nil {
		log.Printf("ws: cannot encode alert: %v", err)
		return
	}
	select {
	case h.queue <- message:
	default:
		log.Printf("ws: queue full, alert not pushed live")
	}
}

// broadcast hands the message to every client. A client whose buffer is
// full is too slow or dead, so it is dropped instead of blocking the others.
func (h *Hub) broadcast(message []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.clients {
		select {
		case c.send <- message:
		default:
			delete(h.clients, c)
			close(c.send) // tells writeLoop to stop
		}
	}
}

func (h *Hub) add(c *client) {
	h.mu.Lock()
	h.clients[c] = struct{}{}
	count := len(h.clients)
	h.mu.Unlock()
	log.Printf("ws: client connected (%d online)", count)
}

// remove is safe to call more than once for the same client.
func (h *Hub) remove(c *client) {
	h.mu.Lock()
	if _, ok := h.clients[c]; ok {
		delete(h.clients, c)
		close(c.send)
	}
	count := len(h.clients)
	h.mu.Unlock()
	log.Printf("ws: client disconnected (%d online)", count)
}

// ServeWS upgrades an HTTP request to a WebSocket and registers the client.
// checkOrigin decides which websites may connect.
func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request, checkOrigin func(*http.Request) bool) {
	upgrader := websocket.Upgrader{CheckOrigin: checkOrigin}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return // Upgrade already wrote the HTTP error
	}

	c := &client{conn: conn, send: make(chan []byte, clientBuffer)}
	h.add(c)

	// Two goroutines per client: one writes, one reads.
	// A WebSocket connection allows only one writer at a time, and writeLoop is that writer.
	go c.writeLoop()
	go c.readLoop(h)
}

// writeLoop sends queued alerts to the browser, and a ping every 30 seconds
// so that dead connections are noticed.
func (c *client) writeLoop() {
	ticker := time.NewTicker(pingInterval)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeTimeout))
			if !ok {
				// The hub closed the channel: say goodbye and stop.
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeTimeout))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// readLoop waits for the browser to answer pings or to disconnect.
// When reading fails, the client is gone and is removed from the hub.
func (c *client) readLoop(h *Hub) {
	defer func() {
		h.remove(c)
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongTimeout))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongTimeout))
		return nil
	})

	for {
		if _, _, err := c.conn.ReadMessage(); err != nil {
			return
		}
	}
}
