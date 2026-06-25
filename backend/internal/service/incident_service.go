package service

import (
	"errors"
	"time"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
	"github.com/obertrack/backend/internal/utils"
)

type IncidentCounts struct {
	Affected     int `json:"affected"`
	Pendiente    int `json:"pendiente"`
	Contactado   int `json:"contactado"`
	Ok           int `json:"ok"`
	SinRespuesta int `json:"sin_respuesta"`
}

type IncidentSummary struct {
	ID        uint           `json:"id"`
	Title     string         `json:"title"`
	Kind      string         `json:"kind"`
	Country   string         `json:"country"`
	State     string         `json:"state"`
	Status    string         `json:"status"`
	CreatedAt time.Time      `json:"created_at"`
	ClosedAt  *time.Time     `json:"closed_at"`
	Counts    IncidentCounts `json:"counts"`
}

type IncidentProfessional struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	Email       string `json:"email"`
	PhoneNumber string `json:"phone_number"`
	Company     string `json:"company"`
	Country     string `json:"country"`
	State       string `json:"state"`
	City        string `json:"city"`
	IsActive    bool   `json:"is_active"`
	Status      string `json:"status"`
}

type IncidentService interface {
	List() ([]IncidentSummary, error)
	Create(title, description, kind, country, state string, createdBy uint) (*models.Incident, error)
	GetByID(id uint) (*models.Incident, error)
	Detail(id uint) (*models.Incident, []IncidentProfessional, error)
	Close(id uint) (*models.Incident, error)
	Broadcast(id uint, subject, body string) (BulkEmailResult, error)
	UpsertResponse(id, userID uint, status, note string) error
}

type incidentService struct {
	repo     repository.IncidentRepository
	userRepo repository.UserRepository
	brevoSvc *BrevoService
}

func NewIncidentService(repo repository.IncidentRepository, userRepo repository.UserRepository, brevoSvc *BrevoService) IncidentService {
	return &incidentService{repo: repo, userRepo: userRepo, brevoSvc: brevoSvc}
}

func (s *incidentService) affected(incident *models.Incident) ([]models.User, map[uint]string, error) {
	professionals, _, err := s.userRepo.GetAll(string(models.UserTypeProfessional), "", "", 0, 0, -1)
	if err != nil {
		return nil, nil, err
	}

	matched := make([]models.User, 0, len(professionals))
	companyByID := map[uint]string{}
	for _, p := range professionals {
		if incident.Country != "" && p.Country != incident.Country {
			continue
		}
		if incident.State != "" && p.State != incident.State {
			continue
		}
		matched = append(matched, p)
		if p.EmpleadorID != nil {
			companyByID[*p.EmpleadorID] = ""
		}
	}
	for id := range companyByID {
		if employer, err := s.userRepo.GetByID(id); err == nil {
			companyByID[id] = employer.CompanyName
		}
	}
	return matched, companyByID, nil
}

func (s *incidentService) List() ([]IncidentSummary, error) {
	incidents, err := s.repo.List()
	if err != nil {
		return nil, err
	}
	out := make([]IncidentSummary, 0, len(incidents))
	for i := range incidents {
		inc := incidents[i]
		matched, _, err := s.affected(&inc)
		if err != nil {
			return nil, err
		}
		responses, err := s.repo.GetResponses(inc.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, IncidentSummary{
			ID:        inc.ID,
			Title:     inc.Title,
			Kind:      inc.Kind,
			Country:   inc.Country,
			State:     inc.State,
			Status:    inc.Status,
			CreatedAt: inc.CreatedAt,
			ClosedAt:  inc.ClosedAt,
			Counts:    countStatuses(matched, responses),
		})
	}
	return out, nil
}

func countStatuses(matched []models.User, responses []models.IncidentResponse) IncidentCounts {
	byUser := map[uint]string{}
	for _, r := range responses {
		byUser[r.UserID] = r.Status
	}
	counts := IncidentCounts{Affected: len(matched)}
	for _, p := range matched {
		switch byUser[p.ID] {
		case models.IncidentResponseContactado:
			counts.Contactado++
		case models.IncidentResponseOk:
			counts.Ok++
		case models.IncidentResponseSinRespuesta:
			counts.SinRespuesta++
		default:
			counts.Pendiente++
		}
	}
	return counts
}

func (s *incidentService) Create(title, description, kind, country, state string, createdBy uint) (*models.Incident, error) {
	if title == "" {
		return nil, errors.New("Título requerido")
	}
	incident := &models.Incident{
		Title:       title,
		Description: utils.SanitizeHTML(description),
		Kind:        kind,
		Country:     country,
		State:       state,
		Status:      models.IncidentStatusOpen,
		CreatedBy:   createdBy,
	}
	if err := s.repo.Create(incident); err != nil {
		return nil, err
	}
	return incident, nil
}

func (s *incidentService) GetByID(id uint) (*models.Incident, error) {
	return s.repo.GetByID(id)
}

func (s *incidentService) Detail(id uint) (*models.Incident, []IncidentProfessional, error) {
	incident, err := s.repo.GetByID(id)
	if err != nil {
		return nil, nil, err
	}
	matched, companyByID, err := s.affected(incident)
	if err != nil {
		return nil, nil, err
	}
	responses, err := s.repo.GetResponses(id)
	if err != nil {
		return nil, nil, err
	}
	byUser := map[uint]string{}
	for _, r := range responses {
		byUser[r.UserID] = r.Status
	}

	professionals := make([]IncidentProfessional, 0, len(matched))
	for _, p := range matched {
		company := ""
		if p.EmpleadorID != nil {
			company = companyByID[*p.EmpleadorID]
		}
		status := byUser[p.ID]
		if status == "" {
			status = models.IncidentResponsePendiente
		}
		professionals = append(professionals, IncidentProfessional{
			ID:          p.ID,
			Name:        p.Name,
			Email:       p.Email,
			PhoneNumber: p.PhoneNumber,
			Company:     company,
			Country:     p.Country,
			State:       p.State,
			City:        p.City,
			IsActive:    p.IsActive,
			Status:      status,
		})
	}
	return incident, professionals, nil
}

func (s *incidentService) Close(id uint) (*models.Incident, error) {
	incident, err := s.repo.GetByID(id)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	if err := s.repo.Update(incident, map[string]interface{}{
		"status":    models.IncidentStatusClosed,
		"closed_at": now,
	}); err != nil {
		return nil, err
	}
	incident.Status = models.IncidentStatusClosed
	incident.ClosedAt = &now
	return incident, nil
}

func (s *incidentService) Broadcast(id uint, subject, body string) (BulkEmailResult, error) {
	result := BulkEmailResult{Failed: []BulkEmailFailure{}}
	incident, err := s.repo.GetByID(id)
	if err != nil {
		return result, err
	}
	matched, _, err := s.affected(incident)
	if err != nil {
		return result, err
	}
	for _, p := range matched {
		if err := s.brevoSvc.SendEmail(p.Email, p.Name, subject, body); err != nil {
			result.Failed = append(result.Failed, BulkEmailFailure{ID: p.ID, Email: p.Email, Error: err.Error()})
			continue
		}
		result.Sent++
		if err := s.repo.UpsertResponseIfPending(id, p.ID, models.IncidentResponseContactado); err != nil {
			continue
		}
	}
	return result, nil
}

func (s *incidentService) UpsertResponse(id, userID uint, status, note string) error {
	if !models.IsValidIncidentResponseStatus(status) {
		return errors.New("Estado inválido")
	}
	if _, err := s.repo.GetByID(id); err != nil {
		return err
	}
	return s.repo.UpsertResponse(id, userID, status, utils.SanitizeHTML(note))
}
