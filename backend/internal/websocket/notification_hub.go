package websocket

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

type NotificationHub struct {
	clients    map[uint]*websocket.Conn
	userIDs    map[*websocket.Conn]uint
	broadcast  chan NotificationWSMessage
	register   chan *websocket.Conn
	unregister chan *websocket.Conn
	mu         sync.RWMutex
}

type NotificationWSMessage struct {
	Type   string      `json:"type"`
	UserID uint        `json:"user_id,omitempty"`
	Data   interface{} `json:"data,omitempty"`
}

var NotificationUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

var GlobalNotifHub = &NotificationHub{
	clients:    make(map[uint]*websocket.Conn),
	userIDs:    make(map[*websocket.Conn]uint),
	broadcast:  make(chan NotificationWSMessage, 100),
	register:   make(chan *websocket.Conn),
	unregister: make(chan *websocket.Conn),
}

func init() {
	go GlobalNotifHub.Run()
}

func (h *NotificationHub) Run() {
	for {
		select {
		case conn := <-h.register:
			h.mu.Lock()
			userID := h.userIDs[conn]
			h.clients[userID] = conn
			h.mu.Unlock()

		case conn := <-h.unregister:
			h.mu.Lock()
			userID := h.userIDs[conn]
			delete(h.clients, userID)
			delete(h.userIDs, conn)
			h.mu.Unlock()

		case message := <-h.broadcast:
			h.mu.RLock()
			if conn, ok := h.clients[message.UserID]; ok {
				data, _ := json.Marshal(message)
				conn.WriteMessage(websocket.TextMessage, data)
			}
			h.mu.RUnlock()
		}
	}
}

func (h *NotificationHub) HandleConnection(conn *websocket.Conn, userID uint) {
	h.mu.Lock()
	h.userIDs[conn] = userID
	h.mu.Unlock()
	h.register <- conn

	go func() {
		defer func() {
			h.unregister <- conn
			conn.Close()
		}()

		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				break
			}
		}
	}()
}

func (h *NotificationHub) NotifyUser(userID uint, notifType string, data interface{}) {
	h.broadcast <- NotificationWSMessage{
		Type:   notifType,
		UserID: userID,
		Data:   data,
	}
}

func (h *NotificationHub) BroadcastToAll(notifType string, data interface{}) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for userID := range h.clients {
		h.NotifyUser(userID, notifType, data)
	}
}
