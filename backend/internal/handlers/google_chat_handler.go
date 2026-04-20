package handlers

import (
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/obertrack/backend/internal/service"
)

type GoogleChatHandler struct {
	workHourSvc service.WorkHourService
	userSvc     service.UserService
}

func NewGoogleChatHandler(whSvc service.WorkHourService, userSvc service.UserService) *GoogleChatHandler {
	return &GoogleChatHandler{
		workHourSvc: whSvc,
		userSvc:     userSvc,
	}
}

// Google Chat Event Structure (Simplified)
type chatEvent struct {
	Type string `json:"type"`
	User struct {
		Email string `json:"email"`
		Name  string `json:"name"`
	} `json:"user"`
	Common struct {
		Parameters map[string]string `json:"parameters"`
	} `json:"common"`
	Action struct {
		ActionMethodName string `json:"actionMethodName"`
	} `json:"action"`
}

func (h *GoogleChatHandler) HandleCallback(c *gin.Context) {
	var event chatEvent
	if err := c.ShouldBindJSON(&event); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	log.Printf("[Google Chat Callback] Received event: %s from %s", event.Type, event.User.Email)

	if event.Type == "CARD_CLICKED" {
		action := event.Action.ActionMethodName
		if action == "" {
			// In some versions it's in a different field
			action = event.Common.Parameters["action"] 
		}
		// Based on our card implementation, the function is "approve_workhour"
		// Google Chat might send it in actionMethodName or a parameter depending on version.
		// Our implementation used: Action: &chat.GoogleAppsCardV1Action{ Function: "approve_workhour" ... }
		
		// For simplicity, let's check both
		if action == "" {
			// Fallback check
		}

		whIDStr := event.Common.Parameters["workhour_id"]
		if whIDStr != "" {
			whID, err := strconv.ParseUint(whIDStr, 10, 32)
			if err != nil {
				h.sendResponse(c, "Error: ID de jornada inválido")
				return
			}

			// 1. Identify the user in Obertrack
			user, err := h.userSvc.GetByEmail(event.User.Email)
			if err != nil || user == nil {
				h.sendResponse(c, "⚠️ No se encontró una cuenta de Obertrack vinculada a este email.")
				return
			}

			// 2. Attempt Approval
			// Passing the user's role and flags to enforce security even from the bot
			err = h.workHourSvc.Approve([]uint{uint(whID)}, user.ID, string(user.UserType), user.IsSuperadmin, user.IsManager)
			if err != nil {
				h.sendResponse(c, fmt.Sprintf("❌ Error al aprobar: %v", err))
				return
			}

			h.sendResponse(c, "✅ ¡Jornada aprobada con éxito!")
			return
		}
	}

	// Default response for other events (like ADDED_TO_SPACE)
	c.JSON(http.StatusOK, gin.H{})
}

func (h *GoogleChatHandler) sendResponse(c *gin.Context, text string) {
	// Simple text response for the card update/reply
	c.JSON(http.StatusOK, gin.H{
		"actionResponse": gin.H{
			"type": "NEW_MESSAGE",
		},
		"text": text,
	})
}
