package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
	"github.com/obertrack/backend/internal/utils"
)

type ProfileChangeService interface {
	CreateRequest(userID uint, changes map[string]string, note string) (*models.ProfileChangeRequest, error)
	GetPending(userID uint) (*models.ProfileChangeRequest, error)
	Apply(reqID, actorID uint, finalValues map[string]string) error
	Reject(reqID, actorID uint, reason string) error
}

type profileChangeService struct {
	repo        repository.ProfileChangeRequestRepository
	userRepo    repository.UserRepository
	channelRepo repository.ChannelRepository
	channelSvc  ChannelService
	notifSvc    NotificationService
}

func NewProfileChangeService(
	repo repository.ProfileChangeRequestRepository,
	userRepo repository.UserRepository,
	channelRepo repository.ChannelRepository,
	channelSvc ChannelService,
	notifSvc NotificationService,
) ProfileChangeService {
	return &profileChangeService{repo: repo, userRepo: userRepo, channelRepo: channelRepo, channelSvc: channelSvc, notifSvc: notifSvc}
}

var profileFieldLabels = map[string]string{
	"name":              "Nombre",
	"phone_number":      "Teléfono",
	"country":           "País",
	"state":             "Provincia / Estado",
	"city":              "Ciudad",
	"location":          "Dirección",
	"job_title":         "Puesto / Cargo",
	"identity_document": "Documento de identidad",
}

func isLockedField(field string) bool {
	for _, f := range models.ProfileLockedFields {
		if f == field {
			return true
		}
	}
	return false
}

func currentValue(u *models.User, field string) string {
	switch field {
	case "name":
		return u.Name
	case "phone_number":
		return u.PhoneNumber
	case "country":
		return u.Country
	case "state":
		return u.State
	case "city":
		return u.City
	case "location":
		return u.Location
	case "job_title":
		return u.JobTitle
	case "identity_document":
		return u.IdentityDocument
	}
	return ""
}

func sanitizeChanges(u *models.User, raw map[string]string) map[string]string {
	out := map[string]string{}
	for field, val := range raw {
		if !isLockedField(field) {
			continue
		}
		if field != "identity_document" {
			val = utils.SanitizeHTML(val)
		}
		if strings.TrimSpace(val) == currentValue(u, field) {
			continue
		}
		out[field] = val
	}
	return out
}

func summarizeChanges(u *models.User, changes map[string]string) string {
	fields := make([]string, 0, len(changes))
	for f := range changes {
		fields = append(fields, f)
	}
	sort.Strings(fields)

	var b strings.Builder
	b.WriteString("Solicitud de actualización de datos de perfil:\n")
	for _, f := range fields {
		label := profileFieldLabels[f]
		if label == "" {
			label = f
		}
		old := currentValue(u, f)
		if old == "" {
			old = "(vacío)"
		}
		b.WriteString(fmt.Sprintf("• %s: %s → %s\n", label, old, changes[f]))
	}
	return b.String()
}

func (s *profileChangeService) CreateRequest(userID uint, changes map[string]string, note string) (*models.ProfileChangeRequest, error) {
	user, err := s.userRepo.GetByID(userID)
	if err != nil || user == nil {
		return nil, errors.New("Usuario no encontrado")
	}
	if user.UserType != models.UserTypeProfessional {
		return nil, errors.New("Solo los profesionales solicitan cambios de datos de perfil")
	}

	if existing, err := s.repo.GetPendingByUser(userID); err != nil {
		return nil, err
	} else if existing != nil {
		return nil, errors.New("Ya tienes una solicitud de cambio pendiente")
	}

	clean := sanitizeChanges(user, changes)
	if len(clean) == 0 {
		return nil, errors.New("No hay cambios para solicitar")
	}
	blob, err := json.Marshal(clean)
	if err != nil {
		return nil, err
	}

	message := summarizeChanges(user, clean)
	if strings.TrimSpace(note) != "" {
		message += "\nMotivo: " + utils.SanitizeHTML(note)
	}
	var ticketID *uint
	if ch, cerr := s.channelSvc.ContactSupport(userID, "Actualización de datos de perfil", message, "Media", "Actualización de Perfil", true); cerr == nil && ch != nil {
		if ticket, terr := s.channelRepo.GetActiveSupportTicketByChannel(ch.ID); terr == nil && ticket != nil {
			ticketID = &ticket.ID
		}
	}

	req := &models.ProfileChangeRequest{
		UserID:          userID,
		SupportTicketID: ticketID,
		Changes:         string(blob),
		Note:            utils.SanitizeHTML(note),
		Status:          models.ProfileChangePending,
	}
	if err := s.repo.Create(req); err != nil {
		return nil, err
	}
	return req, nil
}

func (s *profileChangeService) GetPending(userID uint) (*models.ProfileChangeRequest, error) {
	return s.repo.GetPendingByUser(userID)
}

func (s *profileChangeService) isReviewer(actorID uint) bool {
	actor, err := s.userRepo.GetByID(actorID)
	if err != nil || actor == nil {
		return false
	}
	return actor.IsSuperadmin ||
		actor.UserType == models.UserTypeCustomerSuccess ||
		actor.UserType == models.UserTypeITAnalyst
}

func (s *profileChangeService) Apply(reqID, actorID uint, finalValues map[string]string) error {
	if !s.isReviewer(actorID) {
		return errors.New("Solo Customer Success puede aplicar cambios de perfil")
	}
	req, err := s.repo.GetByID(reqID)
	if err != nil {
		return errors.New("Solicitud no encontrada")
	}
	if req.Status != models.ProfileChangePending {
		return errors.New("Esta solicitud ya fue procesada")
	}
	user, err := s.userRepo.GetByID(req.UserID)
	if err != nil || user == nil {
		return errors.New("Usuario no encontrado")
	}

	values := finalValues
	if len(values) == 0 {
		_ = json.Unmarshal([]byte(req.Changes), &values)
	}
	updates := map[string]interface{}{}
	for field, val := range values {
		if !isLockedField(field) {
			continue
		}
		if field != "identity_document" {
			val = utils.SanitizeHTML(val)
		}
		updates[field] = val
	}
	if len(updates) > 0 {
		if err := s.userRepo.Update(user, updates); err != nil {
			return err
		}
	}

	now := time.Now()
	if err := s.repo.Update(req, map[string]interface{}{
		"status":      models.ProfileChangeApplied,
		"reviewed_by": actorID,
		"reviewed_at": now,
	}); err != nil {
		return err
	}

	if req.SupportTicketID != nil {
		_, _ = s.channelSvc.ResolveSupportTicket(*req.SupportTicketID, actorID)
	}
	if s.notifSvc != nil {
		_ = s.notifSvc.CreateNotification(req.UserID, "profile_change",
			"Datos de perfil actualizados",
			"Customer Success aplicó los cambios que solicitaste en tu perfil.",
			map[string]interface{}{"request_id": req.ID})
	}
	return nil
}

func (s *profileChangeService) Reject(reqID, actorID uint, reason string) error {
	if !s.isReviewer(actorID) {
		return errors.New("Solo Customer Success puede rechazar cambios de perfil")
	}
	req, err := s.repo.GetByID(reqID)
	if err != nil {
		return errors.New("Solicitud no encontrada")
	}
	if req.Status != models.ProfileChangePending {
		return errors.New("Esta solicitud ya fue procesada")
	}

	now := time.Now()
	if err := s.repo.Update(req, map[string]interface{}{
		"status":      models.ProfileChangeRejected,
		"reviewed_by": actorID,
		"reviewed_at": now,
	}); err != nil {
		return err
	}

	if req.SupportTicketID != nil {
		_, _ = s.channelSvc.ResolveSupportTicket(*req.SupportTicketID, actorID)
	}
	if s.notifSvc != nil {
		msg := "Customer Success revisó tu solicitud de cambio de datos y no la aplicó."
		if strings.TrimSpace(reason) != "" {
			msg += " Motivo: " + reason
		}
		_ = s.notifSvc.CreateNotification(req.UserID, "profile_change", "Solicitud de cambio no aplicada", msg,
			map[string]interface{}{"request_id": req.ID})
	}
	return nil
}
