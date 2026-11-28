package http

import (
	"context"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	maxSocketMessageSize = 20 << 10
	writeWait            = 10 * time.Second
	pongWait             = 60 * time.Second
	pingPeriod           = 54 * time.Second
	clientSendBuffer     = 32
)

// Hub keeps WebSocket clients grouped by chat. Each client has one writer
// goroutine, so a connection is never written concurrently by two handlers.
type Hub struct {
	mu     sync.RWMutex
	rooms  map[int]map[*client]struct{}
	closed bool
	wg     sync.WaitGroup
}

type client struct {
	chatID    int
	conn      *websocket.Conn
	cancel    context.CancelFunc
	send      chan []byte
	done      chan struct{}
	closeOnce sync.Once
}

func NewHub() *Hub {
	return &Hub{rooms: make(map[int]map[*client]struct{})}
}

func (h *Hub) register(chatID int, conn *websocket.Conn, cancel context.CancelFunc) *client {
	c := &client{
		chatID: chatID,
		conn:   conn,
		cancel: cancel,
		send:   make(chan []byte, clientSendBuffer),
		done:   make(chan struct{}),
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		_ = conn.Close()
		return nil
	}
	if h.rooms[chatID] == nil {
		h.rooms[chatID] = make(map[*client]struct{})
	}
	h.rooms[chatID][c] = struct{}{}
	h.wg.Add(1)
	return c
}

func (h *Hub) unregister(c *client) {
	h.mu.Lock()
	if room := h.rooms[c.chatID]; room != nil {
		delete(room, c)
		if len(room) == 0 {
			delete(h.rooms, c.chatID)
		}
	}
	h.mu.Unlock()
	c.close()
	h.wg.Done()
}

// CloseAll terminates every WebSocket connection and waits for their handlers
// to finish before the service closes its database pool.
func (h *Hub) CloseAll() {
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		h.wg.Wait()
		return
	}
	h.closed = true
	clients := make([]*client, 0)
	for _, room := range h.rooms {
		for c := range room {
			clients = append(clients, c)
		}
	}
	h.mu.Unlock()

	for _, c := range clients {
		c.close()
	}
	h.wg.Wait()
}

func (h *Hub) broadcast(chatID int, payload []byte) {
	h.mu.RLock()
	clients := make([]*client, 0, len(h.rooms[chatID]))
	for c := range h.rooms[chatID] {
		clients = append(clients, c)
	}
	h.mu.RUnlock()

	for _, c := range clients {
		select {
		case <-c.done:
		case c.send <- payload:
		default:
			// A slow client must not block delivery to other participants.
			// Closing the socket unblocks its read loop, which unregisters it.
			c.close()
		}
	}
}

func (c *client) close() {
	c.closeOnce.Do(func() {
		if c.cancel != nil {
			c.cancel()
		}
		close(c.done)
		_ = c.conn.Close()
	})
}

func (c *client) enqueue(payload []byte) bool {
	select {
	case <-c.done:
		return false
	case c.send <- payload:
		return true
	default:
		c.close()
		return false
	}
}

func (c *client) writePump(ctx context.Context) {
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-c.done:
			return
		case payload := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.TextMessage, payload); err != nil {
				c.close()
				return
			}
		case <-ticker.C:
			if err := c.conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(writeWait)); err != nil {
				c.close()
				return
			}
		}
	}
}

func (c *client) readPump(ctx context.Context, onMessage func([]byte)) {
	c.conn.SetReadLimit(maxSocketMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})
	for {
		messageType, payload, err := c.conn.ReadMessage()
		if err != nil {
			return
		}
		if messageType != websocket.TextMessage {
			continue
		}
		select {
		case <-ctx.Done():
			return
		case <-c.done:
			return
		default:
			onMessage(payload)
		}
	}
}
