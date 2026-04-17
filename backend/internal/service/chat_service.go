package service

import (
	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
)

type ChatService interface {
	GetMessages(userID uint, role string, isSuperadmin bool, empleadorID uint) ([]models.Message, error)
	SendMessage(userID uint, role string, empleadorID uint, content string) (*ChatMessageResponse, error)
}

type ChatMessageResponse struct {
	Type      string    `json:"type"`
	Content   string    `json:"content"`
	UserID    uint      `json:"user_id"`
	CompanyID *uint     `json:"company_id,omitempty"`
}

type chatService struct {
	repo repository.ChatRepository
}

func NewChatService(repo repository.ChatRepository) ChatService {
	return &chatService{repo: repo}
}

func (s *chatService) GetMessages(userID uint, role string, isSuperadmin bool, empleadorID uint) ([]models.Message, error) {
	filters := make(map[string]interface{})

	if !isSuperadmin {
		if role == string(models.UserTypeEmployer) || role == "empleador" {
			filters["employer_id"] = userID
		} else if empleadorID > 0 {
			filters["employer_id"] = empleadorID
		} else {
			filters["user_id"] = userID
		}
	}

	return s.repo.FindAllWithFilters(filters, 100)
}

func (s *chatService) SendMessage(userID uint, role string, empleadorID uint, content string) (*ChatMessageResponse, error) {
	var companyID *uint
	if role == string(models.UserTypeEmployer) || role == "empleador" {
		companyID = &userID
	} else if empleadorID > 0 {
		companyID = &empleadorID
	}

	msg := models.Message{
		UserID:    userID,
		CompanyID: companyID,
		Content:   content,
	}

	if err := s.repo.Create(&msg); err != nil {
		return nil, err
	}

	return &ChatMessageResponse{
		Type:      "chat_message",
		Content:   content,
		UserID:    userID,
		CompanyID: companyID,
	}, nil
}
