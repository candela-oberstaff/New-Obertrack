package websocket

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func newTestHub() *NotificationHub {
	h := &NotificationHub{
		clients:    make(map[*websocket.Conn]*notifClient),
		byUser:     make(map[uint]map[*websocket.Conn]*notifClient),
		broadcast:  make(chan NotificationWSMessage, broadcastBuffer),
		register:   make(chan notifRegistration),
		unregister: make(chan *websocket.Conn),
	}
	go h.Run()
	return h
}

func serveHub(t *testing.T, h *NotificationHub) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uid, err := strconv.ParseUint(r.URL.Query().Get("uid"), 10, 32)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		conn, err := NotificationUpgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		h.HandleConnection(conn, uint(uid))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func dialAs(t *testing.T, srv *httptest.Server, uid uint) *websocket.Conn {
	t.Helper()
	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "?uid=" + strconv.FormatUint(uint64(uid), 10)
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("dial as user %d: %v", uid, err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

func waitForConns(t *testing.T, h *NotificationHub, uid uint, n int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		h.mu.RLock()
		got := len(h.byUser[uid])
		h.mu.RUnlock()
		if got == n {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for user %d to have %d connection(s), got %d", uid, n, got)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func expectType(t *testing.T, conn *websocket.Conn, label, wantType string) {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	var msg NotificationWSMessage
	if err := conn.ReadJSON(&msg); err != nil {
		t.Fatalf("%s: expected notification %q, got read error: %v", label, wantType, err)
	}
	if msg.Type != wantType {
		t.Fatalf("%s: got notification type %q, want %q", label, msg.Type, wantType)
	}
}

func expectSilence(t *testing.T, conn *websocket.Conn, label string) {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
	var msg NotificationWSMessage
	err := conn.ReadJSON(&msg)
	if err == nil {
		t.Fatalf("%s: expected no notification, but received %+v", label, msg)
	}
	if netErr, ok := err.(net.Error); !ok || !netErr.Timeout() {
		t.Fatalf("%s: expected a read timeout, got %v", label, err)
	}
}

func TestNotifyUserReachesEveryTab(t *testing.T) {
	h := newTestHub()
	srv := serveHub(t, h)

	tab1 := dialAs(t, srv, 7)
	tab2 := dialAs(t, srv, 7)
	waitForConns(t, h, 7, 2)

	h.NotifyUser(7, "task_assigned", map[string]any{"title": "Nueva tarea"})

	expectType(t, tab1, "tab1", "task_assigned")
	expectType(t, tab2, "tab2", "task_assigned")
}

func TestClosingOneTabKeepsTheOtherReceiving(t *testing.T) {
	h := newTestHub()
	srv := serveHub(t, h)

	tab1 := dialAs(t, srv, 7)
	tab2 := dialAs(t, srv, 7)
	waitForConns(t, h, 7, 2)

	tab1.Close()
	waitForConns(t, h, 7, 1)

	h.NotifyUser(7, "task_assigned", map[string]any{"title": "Nueva tarea"})
	expectType(t, tab2, "surviving tab", "task_assigned")
}

func TestNotifyUserDoesNotLeakToOtherUsers(t *testing.T) {
	h := newTestHub()
	srv := serveHub(t, h)

	alice := dialAs(t, srv, 1)
	bob := dialAs(t, srv, 2)
	waitForConns(t, h, 1, 1)
	waitForConns(t, h, 2, 1)

	h.NotifyUser(1, "task_assigned", map[string]any{"title": "Solo para Alice"})

	expectType(t, alice, "alice", "task_assigned")
	expectSilence(t, bob, "bob")
}

func TestBroadcastToAllReachesEveryClientAndHubStaysLive(t *testing.T) {
	h := newTestHub()
	srv := serveHub(t, h)

	alice := dialAs(t, srv, 1)
	bob := dialAs(t, srv, 2)
	waitForConns(t, h, 1, 1)
	waitForConns(t, h, 2, 1)

	h.BroadcastToAll("new_ticket_message", map[string]any{"ticket_id": 3})
	expectType(t, alice, "alice", "new_ticket_message")
	expectType(t, bob, "bob", "new_ticket_message")

	h.NotifyUser(2, "task_assigned", map[string]any{"title": "Nueva tarea"})
	expectType(t, bob, "bob after broadcast", "task_assigned")
	expectSilence(t, alice, "alice after broadcast")
}

func TestNotifyUserWithNoConnectionsDoesNotBlock(t *testing.T) {
	h := newTestHub()

	done := make(chan struct{})
	go func() {
		h.NotifyUser(999, "task_assigned", map[string]any{"title": "Nadie conectado"})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("NotifyUser blocked when the target user had no live connection")
	}
}
