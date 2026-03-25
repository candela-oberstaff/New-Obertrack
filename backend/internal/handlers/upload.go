package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"gorm.io/gorm"

	"github.com/obertrack/backend/internal/middleware"
	"github.com/obertrack/backend/internal/models"
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

var notificationUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

var notifHub = &NotificationHub{
	clients:    make(map[uint]*websocket.Conn),
	userIDs:    make(map[*websocket.Conn]uint),
	broadcast:  make(chan NotificationWSMessage, 100),
	register:   make(chan *websocket.Conn),
	unregister: make(chan *websocket.Conn),
}

func init() {
	go notifHub.Run()
}

func (h *NotificationHub) Run() {
	for {
		select {
		case conn := <-h.register:
			userID := h.userIDs[conn]
			h.mu.Lock()
			h.clients[userID] = conn
			h.mu.Unlock()

		case conn := <-h.unregister:
			userID := h.userIDs[conn]
			h.mu.Lock()
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

func NotifyUser(userID uint, notifType string, data interface{}) {
	notifHub.broadcast <- NotificationWSMessage{
		Type:   notifType,
		UserID: userID,
		Data:   data,
	}
}

type UploadHandler struct {
	db          *gorm.DB
	uploadPath  string
	maxFileSize int64
}

func NewUploadHandler(db *gorm.DB, uploadPath string) *UploadHandler {
	if uploadPath == "" {
		uploadPath = "./uploads"
	}
	os.MkdirAll(uploadPath, 0755)
	return &UploadHandler{
		db:          db,
		uploadPath:  uploadPath,
		maxFileSize: 50 << 20, // 50MB
	}
}

var allowedMimeTypes = map[string]string{
	"application/pdf":    ".pdf",
	"application/msword": ".doc",
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document": ".docx",
	"application/vnd.ms-excel": ".xls",
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet": ".xlsx",
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/gif":  ".gif",
	"image/webp": ".webp",
	"audio/mpeg": ".mp3",
	"audio/wav":  ".wav",
	"audio/ogg":  ".ogg",
	"audio/webm": ".webm",
}

func (h *UploadHandler) UploadFile(c *gin.Context) {
	userID := middleware.GetUserID(c)

	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file uploaded"})
		return
	}

	if file.Size > h.maxFileSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File too large (max 50MB)"})
		return
	}

	contentType := file.Header.Get("Content-Type")
	ext, allowed := allowedMimeTypes[contentType]
	if !allowed {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File type not allowed. Allowed: PDF, DOC, DOCX, XLS, XLSX, JPG, PNG, GIF, WEBP, MP3, WAV, OGG, WEBM"})
		return
	}

	filename := fmt.Sprintf("%d_%d_%s%s", userID, time.Now().UnixNano(), sanitizeFilename(file.Filename), ext)
	filepath := filepath.Join(h.uploadPath, filename)

	if err := c.SaveUploadedFile(file, filepath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file"})
		return
	}

	fileURL := fmt.Sprintf("/api/uploads/%s", filename)

	c.JSON(http.StatusOK, gin.H{
		"url":      fileURL,
		"filename": filename,
		"size":     file.Size,
		"type":     contentType,
	})
}

func (h *UploadHandler) GetFile(c *gin.Context) {
	filename := c.Param("filename")
	if filename == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Filename required"})
		return
	}

	filepath := filepath.Join(h.uploadPath, filename)
	if _, err := os.Stat(filepath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}

	c.File(filepath)
}

func sanitizeFilename(name string) string {
	name = filepath.Base(name)
	name = strings.ReplaceAll(name, "..", "")
	name = strings.ReplaceAll(name, "/", "")
	name = strings.ReplaceAll(name, "\\", "")
	if len(name) > 50 {
		name = name[:50]
	}
	return name
}

type NotificationHandler struct {
	db *gorm.DB
}

func NewNotificationHandler(db *gorm.DB) *NotificationHandler {
	return &NotificationHandler{db: db}
}

func (h *NotificationHandler) CreateNotification(userID uint, notifType, title, message string, data map[string]interface{}) error {
	dataJSON := ""
	if data != nil {
		b, _ := json.Marshal(data)
		dataJSON = string(b)
	}

	notification := models.Notification{
		UserID:  userID,
		Type:    notifType,
		Title:   title,
		Message: message,
		Data:    dataJSON,
	}

	if err := h.db.Create(&notification).Error; err != nil {
		return err
	}

	NotifyUser(userID, notifType, map[string]interface{}{
		"id":      notification.ID,
		"type":    notifType,
		"title":   title,
		"message": message,
		"data":    dataJSON,
	})

	return nil
}

func (h *NotificationHandler) HandleWebSocket(c *gin.Context) {
	userID := middleware.GetUserID(c)

	conn, err := notificationUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}

	notifHub.register <- conn
	notifHub.userIDs[conn] = userID

	go func() {
		defer func() {
			notifHub.unregister <- conn
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

func (h *NotificationHandler) GetNotifications(c *gin.Context) {
	userID := middleware.GetUserID(c)

	var notifications []models.Notification
	if err := h.db.Where("user_id = ?", userID).Order("created_at DESC").Limit(50).Find(&notifications).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch notifications"})
		return
	}

	c.JSON(http.StatusOK, notifications)
}

func (h *NotificationHandler) MarkAsRead(c *gin.Context) {
	userID := middleware.GetUserID(c)
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid notification ID"})
		return
	}

	now := time.Now()
	if err := h.db.Model(&models.Notification{}).Where("id = ? AND user_id = ?", id, userID).Update("read_at", &now).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to mark as read"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Notification marked as read"})
}

func (h *NotificationHandler) MarkAllAsRead(c *gin.Context) {
	userID := middleware.GetUserID(c)
	now := time.Now()
	if err := h.db.Model(&models.Notification{}).Where("user_id = ? AND read_at IS NULL", userID).Update("read_at", &now).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to mark all as read"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "All notifications marked as read"})
}

func (h *NotificationHandler) GetUnreadCount(c *gin.Context) {
	userID := middleware.GetUserID(c)

	var count int64
	h.db.Model(&models.Notification{}).Where("user_id = ? AND read_at IS NULL", userID).Count(&count)

	c.JSON(http.StatusOK, gin.H{"count": count})
}
