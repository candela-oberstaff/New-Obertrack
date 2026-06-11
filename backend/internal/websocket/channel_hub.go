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
	UserName  string      `json:"user_name,omitempty"`
	Timestamp time.Time   `json:"timestamp,omitempty"`
	Data      interface{} `json:"data,omitempty"`
}

var ChannelUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     checkWSOrigin,
}

const (
	pongWait   = 60 * time.Second
	pingPeriod = (pongWait * 9) / 10
)

type channelRegistration struct {
	conn   *websocket.Conn
	userID uint
}

type ChannelHub struct {
	clients        map[*websocket.Conn]uint
	broadcast      chan ChannelWSMessage
	register       chan channelRegistration
	unregister     chan *websocket.Conn
	mu             sync.RWMutex
	MessageHandler func(msg ChannelWSMessage)
	MemberResolver func(channelID uint) map[uint]bool
}

func NewChannelHub(messageHandler func(msg ChannelWSMessage)) *ChannelHub {
	return &ChannelHub{
		clients:        make(map[*websocket.Conn]uint),
		broadcast:      make(chan ChannelWSMessage),
		register:       make(chan channelRegistration),
		unregister:     make(chan *websocket.Conn),
		MessageHandler: messageHandler,
	}
}

func (h *ChannelHub) Run() {
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()

	for {
		select {
		case reg := <-h.register:
			h.mu.Lock()
			h.clients[reg.conn] = reg.userID
			h.mu.Unlock()
		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				client.Close()
			}
			h.mu.Unlock()
		case message := <-h.broadcast:
			if message.ChannelID == 0 {
				continue
			}
			var members map[uint]bool
			if h.MemberResolver != nil {
				members = h.MemberResolver(message.ChannelID)
			}
			h.mu.Lock()
			for client, uid := range h.clients {
				if members == nil || !members[uid] {
					continue
				}
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

	h.register <- channelRegistration{conn: conn, userID: userID}

	// Inbound frames are typing indicators only — keep them tiny.
	conn.SetReadLimit(1024)
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

		var lastTyping time.Time
		for {
			_, msgBytes, err := conn.ReadMessage()
			if err != nil {
				break
			}

			var in ChannelWSMessage
			if err := json.Unmarshal(msgBytes, &in); err != nil {
				continue
			}

			// Clients may only emit typing indicators. Every other event type
			// (message, message_edited, message_deleted, reactions, ...) is
			// server-authored by the HTTP handlers after persisting — relaying
			// them here would let a client forge UI events for other members.
			if in.Type != "typing" || in.ChannelID == 0 {
				continue
			}
			// Rate-limit typing to one event per second per connection.
			if time.Since(lastTyping) < time.Second {
				continue
			}
			// Only members of the channel can signal typing in it.
			if h.MemberResolver != nil && !h.MemberResolver(in.ChannelID)[userID] {
				continue
			}
			lastTyping = time.Now()

			h.broadcast <- ChannelWSMessage{
				Type:      "typing",
				ChannelID: in.ChannelID,
				UserID:    userID,
				UserName:  in.UserName,
				Timestamp: time.Now(),
			}
		}
	}()
}

func (h *ChannelHub) Broadcast(msg ChannelWSMessage) {
	h.broadcast <- msg
}
