package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestHubBroadcastsToChatAndClosesConnections(t *testing.T) {
	hub := NewHub()
	registered := make(chan struct{}, 2)
	upgrader := websocket.Upgrader{
		CheckOrigin: func(*http.Request) bool { return true },
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		ctx, cancel := context.WithCancel(r.Context())
		client := hub.register(42, conn, cancel)
		if client == nil {
			cancel()
			return
		}
		go client.writePump(ctx)
		registered <- struct{}{}
		<-client.done
		hub.unregister(client)
	}))
	defer server.Close()

	url := "ws" + strings.TrimPrefix(server.URL, "http")
	connections := make([]*websocket.Conn, 0, 2)
	for i := 0; i < 2; i++ {
		conn, _, err := websocket.DefaultDialer.Dial(url, nil)
		if err != nil {
			t.Fatalf("dial websocket: %v", err)
		}
		connections = append(connections, conn)
		select {
		case <-registered:
		case <-time.After(time.Second):
			t.Fatal("websocket client was not registered")
		}
	}
	defer func() {
		for _, conn := range connections {
			_ = conn.Close()
		}
	}()

	payload := []byte(`{"type":"message","message":{"body":"hello"}}`)
	hub.broadcast(42, payload)
	for i, conn := range connections {
		_ = conn.SetReadDeadline(time.Now().Add(time.Second))
		messageType, got, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("read broadcast for client %d: %v", i, err)
		}
		if messageType != websocket.TextMessage || string(got) != string(payload) {
			t.Fatalf("unexpected broadcast for client %d: type=%d payload=%s", i, messageType, got)
		}
	}

	hub.CloseAll()
	for i, conn := range connections {
		_ = conn.SetReadDeadline(time.Now().Add(time.Second))
		if _, _, err := conn.ReadMessage(); err == nil {
			t.Fatalf("client %d connection remained open after shutdown", i)
		}
	}

	// Shutdown is idempotent.
	hub.CloseAll()
}

func TestHubCloseAllConcurrentCallsWaitForHandlers(t *testing.T) {
	hub := NewHub()
	registered := make(chan *client, 1)
	allowUnregister := make(chan struct{})
	upgrader := websocket.Upgrader{
		CheckOrigin: func(*http.Request) bool { return true },
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		_, cancel := context.WithCancel(r.Context())
		client := hub.register(42, conn, cancel)
		if client == nil {
			cancel()
			return
		}
		registered <- client
		<-client.done
		<-allowUnregister
		hub.unregister(client)
	}))
	defer server.Close()

	url := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	defer conn.Close()

	var wsClient *client
	select {
	case wsClient = <-registered:
	case <-time.After(time.Second):
		t.Fatal("websocket client was not registered")
	}

	firstDone := make(chan struct{})
	go func() {
		hub.CloseAll()
		close(firstDone)
	}()
	select {
	case <-wsClient.done:
	case <-time.After(time.Second):
		t.Fatal("first shutdown did not close the websocket client")
	}

	secondDone := make(chan struct{})
	go func() {
		hub.CloseAll()
		close(secondDone)
	}()
	select {
	case <-secondDone:
		t.Fatal("concurrent shutdown returned before the handler finished")
	case <-time.After(20 * time.Millisecond):
	}

	close(allowUnregister)
	for name, done := range map[string]<-chan struct{}{"first": firstDone, "second": secondDone} {
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatalf("%s shutdown did not finish", name)
		}
	}
}
