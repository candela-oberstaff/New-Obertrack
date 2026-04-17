package websocket

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type ChannelWSMessage struct {
	Type      string      `json:"type"`
	ChannelID uint        `json:"channel_id,omitempty"`
	Content   string      `json:"content,omitempty"`
	UserID    uint        `json:"user_id,omitempty"`
	Timestamp time.Time   `json:"timestamp,omitempty"`
	Data      interface{} `json:"data,omitempty"`
}

var ChannelUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

const (
	pongWait   = 60 * time.Second
	pingPeriod = (pongWait * 9) / 10
)

type ChannelHub struct {
	clients        map[*websocket.Conn]uint
	broadcast      chan ChannelWSMessage
	register       chan *websocket.Conn
	unregister     chan *websocket.Conn
	mu             sync.RWMutex
	MessageHandler func(msg ChannelWSMessage)
}

func NewChannelHub(messageHandler func(msg ChannelWSMessage)) *ChannelHub {
	return &ChannelHub{
		clients:        make(map[*websocket.Conn]uint),
		broadcast:      make(chan ChannelWSMessage),
		register:       make(chan *websocket.Conn),
		unregister:     make(chan *websocket.Conn),
		MessageHandler: messageHandler,
	}
}

func (h *ChannelHub) Run() {
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()

	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = 0
			h.mu.Unlock()
		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				client.Close()
			}
			h.mu.Unlock()
		case message := <-h.broadcast:
			h.mu.Lock()
			for client := range h.clients {
				err := client.WriteJSON(message)
				if err != nil {
					client.Close()
					delete(h.clients, client)
				}
			}
			h.mu.Unlock()

			if h.MessageHandler != nil && message.Type == "chat_message" {
				h.MessageHandler(message)
			}
		case <-ticker.C:
			h.mu.Lock()
			for client := range h.clients {
				if err := client.WriteMessage(websocket.PingMessage, nil); err != nil {
					client.Close()
					delete(h.clients, client)
				}
			}
			h.mu.Unlock()
		}
	}
}

func (h *ChannelHub) HandleConnection(w http.ResponseWriter, r *http.Request, userID uint) {
	conn, err := ChannelUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	h.mu.Lock()
	h.clients[conn] = userID
	h.mu.Unlock()
	h.register <- conn

	conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	go func() {
		defer func() {
			h.unregister <- conn
			conn.Close()
		}()

		for {
			_, msgBytes, err := conn.ReadMessage()
			if err != nil {
				break
			}

			var msg ChannelWSMessage
			if err := json.Unmarshal(msgBytes, &msg); err != nil {
				continue
			}

			msg.UserID = userID
			h.broadcast <- msg
		}
	}()
}

func (h *ChannelHub) Broadcast(msg ChannelWSMessage) {
	h.broadcast <- msg
}
