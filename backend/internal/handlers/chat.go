package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"gorm.io/gorm"

	"github.com/obertrack/backend/internal/middleware"
	"github.com/obertrack/backend/internal/models"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Hub struct {
	clients    map[*websocket.Conn]uint
	broadcast  chan Message
	register   chan *websocket.Conn
	unregister chan *websocket.Conn
	db         *gorm.DB
}

type Message struct {
	Type      string    `json:"type"`
	Content   string    `json:"content"`
	UserID    uint      `json:"user_id"`
	CompanyID *uint     `json:"company_id,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

func NewHub(db *gorm.DB) *Hub {
	return &Hub{
		clients:    make(map[*websocket.Conn]uint),
		broadcast:  make(chan Message),
		register:   make(chan *websocket.Conn),
		unregister: make(chan *websocket.Conn),
		db:         db,
	}
}

func (h *Hub) Run() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case client := <-h.register:
			h.clients[client] = 0
		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				client.Close()
			}
		case message := <-h.broadcast:
			for client := range h.clients {
				err := client.WriteJSON(message)
				if err != nil {
					log.Printf("error: %v", err)
					client.Close()
					delete(h.clients, client)
				}
			}

			if message.Type == "chat_message" {
				h.saveMessage(message)
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

func (h *Hub) saveMessage(msg Message) {
	message := models.Message{
		UserID:    msg.UserID,
		CompanyID: msg.CompanyID,
		Content:   msg.Content,
	}
	h.db.Create(&message)
}

type ChatHandler struct {
	hub *Hub
	db  *gorm.DB
}

func NewChatHandler(db *gorm.DB) *ChatHandler {
	hub := NewHub(db)
	go hub.Run()
	return &ChatHandler{hub: hub, db: db}
}

func (h *ChatHandler) HandleWebSocket(c *gin.Context) {
	userID := middleware.GetUserID(c)

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println("WebSocket upgrade error:", err)
		return
	}

	h.hub.register <- conn
	h.hub.clients[conn] = userID

	go func() {
		defer func() {
			h.hub.unregister <- conn
			conn.Close()
		}()

		for {
			_, msgBytes, err := conn.ReadMessage()
			if err != nil {
				break
			}

			var msg Message
			if err := json.Unmarshal(msgBytes, &msg); err != nil {
				continue
			}

			msg.UserID = userID
			msg.Timestamp = time.Now()
			h.hub.broadcast <- msg
		}
	}()
}

func (h *ChatHandler) GetMessages(c *gin.Context) {
	userID := middleware.GetUserID(c)
	role := middleware.GetUserRole(c)
	isSuperadmin := middleware.IsSuperadmin(c)
	empleadorID := middleware.GetEmpleadorID(c)

	var messages []models.Message
	query := h.db.Model(&models.Message{})

	if !isSuperadmin {
		if role == string(models.UserTypeEmployer) || role == "empleador" {
			subquery := h.db.Model(&models.User{}).Where("empleador_id = ?", userID).Select("id")
			query = query.Where("user_id IN (?) OR (company_id = ? AND company_id IS NOT NULL)", subquery, userID)
		} else if empleadorID > 0 {
			subquery := h.db.Model(&models.User{}).Where("empleador_id = ?", empleadorID).Select("id")
			query = query.Where("user_id IN (?) OR company_id = ?", subquery, empleadorID)
		} else {
			query = query.Where("user_id = ?", userID)
		}
	}

	if err := query.Order("created_at DESC").Limit(100).Find(&messages).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch messages"})
		return
	}

	c.JSON(http.StatusOK, messages)
}

func (h *ChatHandler) SendMessage(c *gin.Context) {
	var req struct {
		Content string `json:"content" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := middleware.GetUserID(c)
	role := middleware.GetUserRole(c)
	empleadorID := middleware.GetEmpleadorID(c)

	var companyID *uint
	if role == string(models.UserTypeEmployer) || role == "empleador" {
		companyID = &userID
	} else if empleadorID > 0 {
		companyID = &empleadorID
	}

	msg := Message{
		Type:      "chat_message",
		Content:   req.Content,
		UserID:    userID,
		CompanyID: companyID,
		Timestamp: time.Now(),
	}

	h.hub.broadcast <- msg

	c.JSON(http.StatusOK, gin.H{"status": "sent"})
}
