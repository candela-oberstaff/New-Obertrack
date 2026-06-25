package service

import (
	"errors"
	"strings"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
)

type EmergencyTemplateService interface {
	List() ([]models.EmergencyTemplate, error)
	Create(title, subject, body string) (*models.EmergencyTemplate, error)
	Update(id uint, title, subject, body string) (*models.EmergencyTemplate, error)
	Delete(id uint) error
}

type emergencyTemplateService struct {
	repo repository.EmergencyTemplateRepository
}

func NewEmergencyTemplateService(repo repository.EmergencyTemplateRepository) EmergencyTemplateService {
	return &emergencyTemplateService{repo: repo}
}

func (s *emergencyTemplateService) List() ([]models.EmergencyTemplate, error) {
	return s.repo.List()
}

func (s *emergencyTemplateService) Create(title, subject, body string) (*models.EmergencyTemplate, error) {
	title = strings.TrimSpace(title)
	subject = strings.TrimSpace(subject)
	body = strings.TrimSpace(body)
	if title == "" || subject == "" || body == "" {
		return nil, errors.New("Título, asunto y cuerpo son requeridos")
	}
	template := &models.EmergencyTemplate{Title: title, Subject: subject, Body: body}
	if err := s.repo.Create(template); err != nil {
		return nil, err
	}
	return template, nil
}

func (s *emergencyTemplateService) Update(id uint, title, subject, body string) (*models.EmergencyTemplate, error) {
	title = strings.TrimSpace(title)
	subject = strings.TrimSpace(subject)
	body = strings.TrimSpace(body)
	if title == "" || subject == "" || body == "" {
		return nil, errors.New("Título, asunto y cuerpo son requeridos")
	}
	template, err := s.repo.GetByID(id)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Update(template, map[string]interface{}{
		"title":   title,
		"subject": subject,
		"body":    body,
	}); err != nil {
		return nil, err
	}
	template.Title = title
	template.Subject = subject
	template.Body = body
	return template, nil
}

func (s *emergencyTemplateService) Delete(id uint) error {
	return s.repo.Delete(id)
}
