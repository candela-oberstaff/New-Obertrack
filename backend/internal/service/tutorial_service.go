package service

import (
	"errors"
	"regexp"
	"strings"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
	"github.com/obertrack/backend/internal/utils"
)

type TutorialService interface {
	GetAll(onlyActive bool) ([]models.Tutorial, error)
	GetByID(id uint) (*models.Tutorial, error)
	Create(userID uint, title, description, googleDriveURL, iconName, category string, durationMin, orderIndex int, isActive bool) (*models.Tutorial, error)
	Update(id uint, updates map[string]interface{}) (*models.Tutorial, error)
	Delete(id uint) error
	Reorder(ids []uint) error
	RecordView(tutorialID, userID uint) error
	GetUserViewedIDs(userID uint) ([]uint, error)
}

type tutorialService struct {
	repo repository.TutorialRepository
}

func NewTutorialService(repo repository.TutorialRepository) TutorialService {
	return &tutorialService{repo: repo}
}

var (
	driveFileIDRegex = regexp.MustCompile(`/file/d/([a-zA-Z0-9_-]+)`)
	youtubeIDRegex   = regexp.MustCompile(`(?:youtube\.com/(?:watch\?(?:[^&]*&)*v=|embed/|v/|shorts/)|youtu\.be/)([a-zA-Z0-9_-]{11})`)
)

func validateVideoURL(url string) error {
	url = strings.TrimSpace(url)
	if url == "" {
		return errors.New("El link del video es obligatorio")
	}
	if strings.Contains(url, "drive.google.com") {
		if !driveFileIDRegex.MatchString(url) {
			return errors.New("El link de Google Drive debe tener el formato /file/d/{ID}/...")
		}
		return nil
	}
	if strings.Contains(url, "youtube.com") || strings.Contains(url, "youtu.be") {
		if !youtubeIDRegex.MatchString(url) {
			return errors.New("El link de YouTube no tiene un ID de video válido")
		}
		return nil
	}
	return errors.New("Solo se aceptan links de Google Drive o YouTube")
}

func (s *tutorialService) GetAll(onlyActive bool) ([]models.Tutorial, error) {
	return s.repo.FindAll(onlyActive)
}

func (s *tutorialService) GetByID(id uint) (*models.Tutorial, error) {
	return s.repo.GetByID(id)
}

func (s *tutorialService) Create(userID uint, title, description, googleDriveURL, iconName, category string, durationMin, orderIndex int, isActive bool) (*models.Tutorial, error) {
	if strings.TrimSpace(title) == "" {
		return nil, errors.New("El título es obligatorio")
	}
	if err := validateVideoURL(googleDriveURL); err != nil {
		return nil, err
	}
	if iconName == "" {
		iconName = "PlayCircle"
	}
	category = strings.TrimSpace(category)
	if category == "" {
		category = "General"
	}

	tutorial := &models.Tutorial{
		Title:          utils.SanitizeHTML(title),
		Description:    utils.SanitizeHTML(description),
		GoogleDriveURL: strings.TrimSpace(googleDriveURL),
		IconName:       iconName,
		Category:       utils.SanitizeHTML(category),
		DurationMin:    durationMin,
		OrderIndex:     orderIndex,
		IsActive:       isActive,
		CreatedBy:      userID,
	}

	if err := s.repo.Create(tutorial); err != nil {
		return nil, err
	}

	return s.repo.GetByID(tutorial.ID)
}

func (s *tutorialService) Update(id uint, updates map[string]interface{}) (*models.Tutorial, error) {
	tutorial, err := s.repo.GetByID(id)
	if err != nil {
		return nil, errors.New("Tutorial no encontrado")
	}

	if title, ok := updates["title"].(string); ok {
		if strings.TrimSpace(title) == "" {
			return nil, errors.New("El título es obligatorio")
		}
		updates["title"] = utils.SanitizeHTML(title)
	}
	if description, ok := updates["description"].(string); ok {
		updates["description"] = utils.SanitizeHTML(description)
	}
	if url, ok := updates["google_drive_url"].(string); ok {
		if err := validateVideoURL(url); err != nil {
			return nil, err
		}
		updates["google_drive_url"] = strings.TrimSpace(url)
	}
	if category, ok := updates["category"].(string); ok {
		trimmed := strings.TrimSpace(category)
		if trimmed == "" {
			trimmed = "General"
		}
		updates["category"] = utils.SanitizeHTML(trimmed)
	}

	if len(updates) == 0 {
		return tutorial, nil
	}

	if err := s.repo.Update(tutorial, updates); err != nil {
		return nil, err
	}

	return s.repo.GetByID(id)
}

func (s *tutorialService) Delete(id uint) error {
	return s.repo.Delete(id)
}

func (s *tutorialService) Reorder(ids []uint) error {
	if len(ids) == 0 {
		return errors.New("La lista de IDs no puede estar vacía")
	}
	return s.repo.Reorder(ids)
}

func (s *tutorialService) RecordView(tutorialID, userID uint) error {
	if tutorialID == 0 || userID == 0 {
		return errors.New("IDs inválidos")
	}
	return s.repo.RecordView(tutorialID, userID)
}

func (s *tutorialService) GetUserViewedIDs(userID uint) ([]uint, error) {
	return s.repo.GetUserViewedIDs(userID)
}
