package websocket

import (
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	pingSentinel         = "__ping"
	broadcastUserID uint = 0
)

type NotificationHub struct {
	clients    map[*websocket.Conn]*notifClient
	byUser     map[uint]map[*websocket.Conn]*notifClient
	broadcast  chan NotificationWSMessage
	register   chan notifRegistration
	unregister chan *websocket.Conn
	mu         sync.RWMutex
}

type notifClient struct {
	conn      *websocket.Conn
	userID    uint
	send      chan NotificationWSMessage
	done      chan struct{}
	closeOnce sync.Once
}

func (c *notifClient) closeSend() {
	c.closeOnce.Do(func() { close(c.done) })
}

type notifRegistration struct {
	conn   *websocket.Conn
	userID uint
}

type NotificationWSMessage struct {
	Type   string      `json:"type"`
	UserID uint        `json:"user_id,omitempty"`
	Data   interface{} `json:"data,omitempty"`
}

var NotificationUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     checkWSOrigin,
}

var GlobalNotifHub = &NotificationHub{
	clients:    make(map[*websocket.Conn]*notifClient),
	byUser:     make(map[uint]map[*websocket.Conn]*notifClient),
	broadcast:  make(chan NotificationWSMessage, broadcastBuffer),
	register:   make(chan notifRegistration),
	unregister: make(chan *websocket.Conn),
}

func init() {
	go GlobalNotifHub.Run()
}

func (h *NotificationHub) Run() {
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()

	for {
		select {
		case reg := <-h.register:
			client := &notifClient{
				conn:   reg.conn,
				userID: reg.userID,
				send:   make(chan NotificationWSMessage, sendBuffer),
				done:   make(chan struct{}),
			}
			h.mu.Lock()
			h.clients[reg.conn] = client
			if h.byUser[reg.userID] == nil {
				h.byUser[reg.userID] = make(map[*websocket.Conn]*notifClient)
			}
			h.byUser[reg.userID][reg.conn] = client
			h.mu.Unlock()
			go h.writePump(client)

		case conn := <-h.unregister:
			h.removeClient(conn)

		case message := <-h.broadcast:
			h.dispatch(message)

		case <-ticker.C:
			h.mu.RLock()
			clients := make([]*notifClient, 0, len(h.clients))
			for _, c := range h.clients {
				clients = append(clients, c)
			}
			h.mu.RUnlock()
			for _, c := range clients {
				h.enqueue(c, NotificationWSMessage{Type: pingSentinel})
			}
		}
	}
}

func (h *NotificationHub) dispatch(message NotificationWSMessage) {
	h.mu.RLock()
	var targets []*notifClient
	if message.UserID == broadcastUserID {
		targets = make([]*notifClient, 0, len(h.clients))
		for _, c := range h.clients {
			targets = append(targets, c)
		}
	} else {
		targets = make([]*notifClient, 0, len(h.byUser[message.UserID]))
		for _, c := range h.byUser[message.UserID] {
			targets = append(targets, c)
		}
	}
	h.mu.RUnlock()

	for _, c := range targets {
		h.enqueue(c, message)
	}
}

func (h *NotificationHub) enqueue(c *notifClient, message NotificationWSMessage) {
	select {
	case c.send <- message:
	case <-c.done:
	default:
		go h.removeClient(c.conn)
	}
}

func (h *NotificationHub) removeClient(conn *websocket.Conn) {
	h.mu.Lock()
	client, ok := h.clients[conn]
	if ok {
		delete(h.clients, conn)
		if conns := h.byUser[client.userID]; conns != nil {
			delete(conns, conn)
			if len(conns) == 0 {
				delete(h.byUser, client.userID)
			}
		}
	}
	h.mu.Unlock()

	if ok {
		client.closeSend()
	}
}

func (h *NotificationHub) writePump(c *notifClient) {
	defer c.conn.Close()
	for {
		select {
		case msg := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if msg.Type == pingSentinel {
				if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					return
				}
				continue
			}
			if err := c.conn.WriteJSON(msg); err != nil {
				return
			}
		case <-c.done:
			return
		}
	}
}

func (h *NotificationHub) HandleConnection(conn *websocket.Conn, userID uint) {
	h.register <- notifRegistration{conn: conn, userID: userID}

	conn.SetReadLimit(512)
	_ = conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	go func() {
		defer func() {
			h.unregister <- conn
		}()

		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				break
			}
		}
	}()
}

func (h *NotificationHub) NotifyUser(userID uint, notifType string, data interface{}) {
	if userID == broadcastUserID {
		return
	}
	h.send(NotificationWSMessage{Type: notifType, UserID: userID, Data: data})
}

func (h *NotificationHub) BroadcastToAll(notifType string, data interface{}) {
	h.send(NotificationWSMessage{Type: notifType, UserID: broadcastUserID, Data: data})
}

func (h *NotificationHub) send(msg NotificationWSMessage) {
	select {
	case h.broadcast <- msg:
	default:
	}
}
