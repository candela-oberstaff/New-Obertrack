package service

import (
	"log"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
)

// AuditService records and queries app-wide audit events.
type AuditService interface {
	// Record persists an audit entry (best-effort; callers may run it in a goroutine).
	Record(entry models.AuditLog)
	// RecordAuth records an authentication event (login/logout/register/reset).
	RecordAuth(action string, actorID *uint, email, role string, success bool, ip, ua string)
	List(filters map[string]interface{}, offset, limit int) ([]models.AuditLog, int64, error)
}

type auditService struct {
	repo repository.AuditRepository
}

func NewAuditService(repo repository.AuditRepository) AuditService {
	return &auditService{repo: repo}
}

func (s *auditService) Record(entry models.AuditLog) {
	if err := s.repo.Create(&entry); err != nil {
		log.Printf("[Audit] failed to record %s %s: %v", entry.Method, entry.Path, err)
	}
}

func (s *auditService) RecordAuth(action string, actorID *uint, email, role string, success bool, ip, ua string) {
	status := 200
	if !success {
		status = 401
	}
	s.Record(models.AuditLog{
		Kind:       "activity",
		ActorID:    actorID,
		ActorEmail: email,
		ActorRole:  role,
		Action:     action,
		Module:     "auth",
		EntityType: "auth",
		Method:     "POST",
		Path:       "/api/" + action,
		Status:     status,
		Success:    success,
		IP:         ip,
		UserAgent:  ua,
	})
}

func (s *auditService) List(filters map[string]interface{}, offset, limit int) ([]models.AuditLog, int64, error) {
	return s.repo.FindAll(filters, offset, limit)
}
