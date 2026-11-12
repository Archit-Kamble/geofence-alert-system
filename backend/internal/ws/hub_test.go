package ws

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// startHub runs a hub behind a test HTTP server and returns its ws:// address.
func startHub(t *testing.T) (*Hub, string) {
	t.Helper()
	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	go hub.Run(ctx)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hub.ServeWS(w, r, func(*http.Request) bool { return true })
	}))
	t.Cleanup(func() {
		cancel()
		server.Close()
	})
	return hub, "ws" + strings.TrimPrefix(server.URL, "http")
}

func dial(t *testing.T, url string) *websocket.Conn {
	t.Helper()
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

func (h *Hub) clientCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.clients)
}

// waitFor polls until check is true, or fails the test after one second.
func waitFor(t *testing.T, what string, check func() bool) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if check() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func TestEveryClientReceivesTheAlert(t *testing.T) {
	hub, url := startHub(t)
	first, second := dial(t, url), dial(t, url)
	waitFor(t, "two clients", func() bool { return hub.clientCount() == 2 })

	hub.Publish(map[string]string{"event_type": "entry"})

	for _, conn := range []*websocket.Conn{first, second} {
		conn.SetReadDeadline(time.Now().Add(time.Second))
		_, message, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		if string(message) != `{"event_type":"entry"}` {
			t.Errorf("got %s", message)
		}
	}
}

func TestDisconnectedClientIsRemoved(t *testing.T) {
	hub, url := startHub(t)
	conn := dial(t, url)
	waitFor(t, "client to connect", func() bool { return hub.clientCount() == 1 })

	conn.Close()
	waitFor(t, "client to be removed", func() bool { return hub.clientCount() == 0 })
}

// Run with "go test -race": many goroutines publish while clients connect and leave.
func TestConcurrentPublishAndConnect(t *testing.T) {
	hub, url := startHub(t)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				hub.Publish(map[string]int{"n": j})
			}
		}()
		go func() {
			defer wg.Done()
			conn, _, err := websocket.DefaultDialer.Dial(url, nil)
			if err != nil {
				return
			}
			conn.Close()
		}()
	}
	wg.Wait()
	waitFor(t, "all clients to leave", func() bool { return hub.clientCount() == 0 })
}
