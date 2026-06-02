package websocket

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

var ChatUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type ChatWSMessage struct {
	Type      string    `json:"type"`
	Content   string    `json:"content"`
	UserID    uint      `json:"user_id"`
	TenantID  *uint     `json:"tenant_id,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

type chatClient struct {
	userID       uint
	tenantID     uint
	isSuperadmin bool
}

type chatRegistration struct {
	conn *websocket.Conn
	meta chatClient
}

type ChatHub struct {
	clients        map[*websocket.Conn]chatClient
	broadcast      chan ChatWSMessage
	register       chan chatRegistration
	unregister     chan *websocket.Conn
	MessageHandler func(msg ChatWSMessage)
}

func NewChatHub(messageHandler func(msg ChatWSMessage)) *ChatHub {
	return &ChatHub{
		clients:        make(map[*websocket.Conn]chatClient),
		broadcast:      make(chan ChatWSMessage),
		register:       make(chan chatRegistration),
		unregister:     make(chan *websocket.Conn),
		MessageHandler: messageHandler,
	}
}

func (h *ChatHub) Run() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case reg := <-h.register:
			h.clients[reg.conn] = reg.meta
		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				client.Close()
			}
		case message := <-h.broadcast:
			msgTenant := uint(0)
			if message.TenantID != nil {
				msgTenant = *message.TenantID
			}
			for client, meta := range h.clients {
				if !meta.isSuperadmin && meta.tenantID != msgTenant {
					continue
				}
				err := client.WriteJSON(message)
				if err != nil {
					log.Printf("error: %v", err)
					client.Close()
					delete(h.clients, client)
				}
			}

			if message.Type == "chat_message" && h.MessageHandler != nil {
				h.MessageHandler(message)
			}
		case <-ticker.C:
			for client := range h.clients {
				if err := client.WriteMessage(websocket.PingMessage, nil); err != nil {
					client.Close()
					delete(h.clients, client)
				}
			}
		}
	}
}

func (h *ChatHub) HandleConnection(w http.ResponseWriter, r *http.Request, userID, tenantID uint, isSuperadmin bool) {
	conn, err := ChatUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("WebSocket upgrade error:", err)
		return
	}

	h.register <- chatRegistration{conn: conn, meta: chatClient{userID: userID, tenantID: tenantID, isSuperadmin: isSuperadmin}}

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

			var msg ChatWSMessage
			if err := json.Unmarshal(msgBytes, &msg); err != nil {
				continue
			}

			msg.UserID = userID
			msg.Timestamp = time.Now()
			t := tenantID
			msg.TenantID = &t
			h.broadcast <- msg
		}
	}()
}

func (h *ChatHub) Broadcast(msg ChatWSMessage) {
	h.broadcast <- msg
}
