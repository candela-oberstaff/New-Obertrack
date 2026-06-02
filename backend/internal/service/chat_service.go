package service

import (
	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
)

type ChatService interface {
	GetMessages(userID uint, role string, isSuperadmin bool, tenantID uint) ([]models.Message, error)
	SendMessage(userID uint, role string, tenantID uint, content string) (*ChatMessageResponse, error)
}

type ChatMessageResponse struct {
	Type     string `json:"type"`
	Content  string `json:"content"`
	UserID   uint   `json:"user_id"`
	TenantID *uint  `json:"tenant_id,omitempty"`
}

type chatService struct {
	repo repository.ChatRepository
}

func NewChatService(repo repository.ChatRepository) ChatService {
	return &chatService{repo: repo}
}

func (s *chatService) GetMessages(userID uint, role string, isSuperadmin bool, tenantID uint) ([]models.Message, error) {
	filters := make(map[string]interface{})

	if !isSuperadmin {
		if role == string(models.UserTypeEmployer) || role == "empleador" {
			filters["tenant_id"] = userID
		} else if tenantID > 0 {
			filters["tenant_id"] = tenantID
		} else {
			filters["user_id"] = userID
		}
	}

	return s.repo.FindAllWithFilters(filters, 100)
}

func (s *chatService) SendMessage(userID uint, role string, tenantID uint, content string) (*ChatMessageResponse, error) {
	var tid *uint
	if role == string(models.UserTypeEmployer) || role == "empleador" {
		tid = &userID
	} else if tenantID > 0 {
		tid = &tenantID
	}

	msg := models.Message{
		UserID:   userID,
		TenantID: tid,
		Content:  content,
	}

	if err := s.repo.Create(&msg); err != nil {
		return nil, err
	}

	return &ChatMessageResponse{
		Type:     "chat_message",
		Content:  content,
		UserID:   userID,
		TenantID: tid,
	}, nil
}
