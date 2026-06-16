package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
	"github.com/obertrack/backend/internal/utils"
)

// Audiencias del expediente: controlan qué entradas (notas/documentos) se ven.
const (
	AudienceCompany      = "company"      // RR.HH.: ve todo
	AudienceProfessional = "professional" // El profesional: solo lo compartido
)

// EmploymentView es una membresía con los nombres resueltos (empresa, manager)
// para la UI del expediente.
type EmploymentView struct {
	models.Employment
	CompanyName string `json:"company_name"`
	ManagerName string `json:"manager_name"`
}

// ExpedienteSummary es el resumen cuantitativo de un empleo (horas, tareas,
// antigüedad). Se calcula en vivo para empleos activos y se CONGELA en JSON
// (employments.end_summary) al terminar, para que el histórico no cambie aunque
// luego se borren horas o tareas.
type ExpedienteSummary struct {
	DaysEmployed   int        `json:"days_employed"`
	TotalHours     float64    `json:"total_hours"`
	ApprovedHours  float64    `json:"approved_hours"`
	PendingHours   float64    `json:"pending_hours"`
	TasksAssigned  int64      `json:"tasks_assigned"`
	TasksCompleted int64      `json:"tasks_completed"`
	Absences       int        `json:"absences"`
	FrozenAt       *time.Time `json:"frozen_at,omitempty"`
}

// AbsenceEntry es una ausencia registrada durante el empleo (para el detalle
// del expediente: fecha, motivo, horas y si quedó aprobada/justificada).
type AbsenceEntry struct {
	Date     time.Time `json:"date"`
	Reason   string    `json:"reason"`
	Hours    float64   `json:"hours"`
	Approved bool      `json:"approved"`
}

// GestionEntry es una gestión de customer success (seguimiento de inactividad o
// ausencia) registrada sobre el profesional durante el empleo.
type GestionEntry struct {
	Kind      string    `json:"kind"`   // inactivity | absence
	Status    string    `json:"status"` // contacted | justified | escalated
	Note      string    `json:"note"`
	ByName    string    `json:"by_name"`
	CreatedAt time.Time `json:"created_at"`
}

// ContactEntry es un intento de contacto (email/WhatsApp/chat) al profesional.
type ContactEntry struct {
	Channel   string    `json:"channel"` // email | whatsapp | chat
	ByName    string    `json:"by_name"`
	Note      string    `json:"note,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// ExpedienteNoteView es una nota con el nombre de su autor resuelto.
type ExpedienteNoteView struct {
	models.EmploymentNote
	AuthorName string `json:"author_name"`
}

// FrozenExpediente es el snapshot que se guarda en employments.end_summary al
// dar de baja: el resumen cuantitativo MÁS las evaluaciones/notas y gestiones
// tal como estaban a la salida. Sella el legajo (inmutable). Embebe el resumen,
// así que sus campos quedan al tope del JSON (compatibilidad con snapshots
// viejos que solo tenían el resumen).
type FrozenExpediente struct {
	ExpedienteSummary
	Notes     []ExpedienteNoteView `json:"frozen_notes,omitempty"`
	Gestiones []GestionEntry       `json:"frozen_gestiones,omitempty"`
}

// CVEntry es una empresa en el CV del profesional: el empleo, su resumen y lo
// que esa empresa decidió compartir (evaluaciones y documentos). NO incluye
// datos internos de CS (gestiones/contactos).
type CVEntry struct {
	Employment EmploymentView              `json:"employment"`
	Summary    ExpedienteSummary           `json:"summary"`
	Notes      []ExpedienteNoteView        `json:"notes"`
	Documents  []models.EmploymentDocument `json:"documents"`
}

// CVView es el CV vivo del profesional: su trayectoria unificada en todas las
// empresas (activas y pasadas) con métricas agregadas.
type CVView struct {
	Entries         []CVEntry `json:"entries"`
	TotalCompanies  int       `json:"total_companies"`
	ActiveCompanies int       `json:"active_companies"`
	TotalDays       int       `json:"total_days"`
}

// ExpedienteView es el expediente completo de un empleo para una audiencia.
type ExpedienteView struct {
	Employment EmploymentView              `json:"employment"`
	Summary    ExpedienteSummary           `json:"summary"`
	Notes      []ExpedienteNoteView        `json:"notes"`
	Documents  []models.EmploymentDocument `json:"documents"`
	Absences   []AbsenceEntry              `json:"absences"`
	Gestiones  []GestionEntry              `json:"gestiones"`
	Contactos  []ContactEntry              `json:"contactos"`
}

// EmploymentService gestiona las membresías de un profesional en empresas
// (employments): fuente de verdad del vínculo multi-empresa y base del expediente.
//
// FASE 0: dual-write (SyncActiveForUser) mantiene la tabla espejo de empleador_id.
// FASE 1: alta/baja de membresías adicionales (multi-empresa). users.empleador_id
// sigue siendo la "empresa activa"; al terminar la activa se reasigna a otra.
type EmploymentService interface {
	SyncActiveForUser(user *models.User) error
	ListForUser(userID uint) ([]EmploymentView, error)
	AddEmployment(userID, companyID uint, jobTitle, startReason string, managerID *uint) (*models.Employment, error)
	EndEmployment(userID, employmentID uint, endReason string) error
	// ReactivateEmployment revierte una baja: el empleo vuelve a estar activo y,
	// si el usuario no tenía empresa activa, esta vuelve a serlo.
	ReactivateEmployment(employmentID uint) error
	// ActiveCompanies lista las empresas donde el usuario tiene empleo activo
	// (para el switcher multi-empresa).
	ActiveCompanies(userID uint) ([]models.CompanyRef, error)
	// SwitchActive cambia la empresa activa del usuario (empleador_id) a otra
	// donde tenga empleo activo. Devuelve el usuario actualizado; el caller
	// re-emite el JWT con el nuevo tenant.
	SwitchActive(userID, companyID uint) (*models.User, error)

	// --- Expediente (FASE 3) ---
	// GetExpediente arma el expediente de un empleo para una audiencia
	// (AudienceCompany ve todo; AudienceProfessional solo lo compartido).
	GetExpediente(employmentID uint, audience string) (*ExpedienteView, error)
	AddNote(employmentID, authorID uint, kind string, rating *int, content, visibility string) (*models.EmploymentNote, error)
	DeleteNote(noteID uint) error
	AddDocument(employmentID, uploaderID uint, title, fileName, fileURL string, fileSize int64, mimeType, visibility string, expiresAt *time.Time) (*models.EmploymentDocument, error)
	DeleteDocument(docID uint) error
	// DocumentForDownload devuelve un documento si el solicitante puede verlo: la
	// empresa (RR.HH.) ve todo; el profesional solo los compartidos de su empleo.
	DocumentForDownload(docID uint, audience string, requesterID uint) (*models.EmploymentDocument, error)
	// LogContact registra un intento de contacto (email/WhatsApp/chat) a un
	// profesional; aparece en el historial de su expediente.
	LogContact(userID, byUserID uint, channel string) error
	// GetCV arma el CV vivo del profesional: su trayectoria unificada en todas
	// las empresas, con lo que cada una compartió.
	GetCV(userID uint) (*CVView, error)
	// GetCVPDF genera el PDF del CV del profesional. Devuelve bytes + el nombre.
	GetCVPDF(userID uint) ([]byte, string, error)
	// GetExpedientePDF genera el PDF del expediente completo de un empleo
	// (audiencia empresa). Devuelve bytes + el nombre del profesional.
	GetExpedientePDF(employmentID uint) ([]byte, string, error)
	// UpdateNote edita una evaluación/nota existente.
	UpdateNote(noteID uint, kind string, rating *int, content, visibility string) (*models.EmploymentNote, error)
	// UpdateDocument edita los metadatos de un documento (título, visibilidad,
	// vencimiento); no toca el archivo.
	UpdateDocument(docID uint, title, visibility string, expiresAt *time.Time) (*models.EmploymentDocument, error)
	// --- Acotamiento por empresa (gestión del empleador) ---
	// EmploymentCompanyID / NoteCompanyID / DocCompanyID devuelven la empresa
	// dueña del recurso, para validar que el empleador solo toque lo suyo.
	EmploymentCompanyID(employmentID uint) (uint, error)
	NoteCompanyID(noteID uint) (uint, error)
	DocCompanyID(docID uint) (uint, error)
	// EmploymentForUserInCompany resuelve el empleo de un profesional en una
	// empresa (para que el empleador abra su expediente por user_id).
	EmploymentForUserInCompany(userID, companyID uint) (*EmploymentView, error)
}

type employmentService struct {
	repo         repository.EmploymentRepository
	userRepo     repository.UserRepository
	workHourRepo repository.WorkHourRepository
	notifSvc     NotificationService
}

func NewEmploymentService(repo repository.EmploymentRepository, userRepo repository.UserRepository, workHourRepo repository.WorkHourRepository, notifSvc NotificationService) EmploymentService {
	return &employmentService{repo: repo, userRepo: userRepo, workHourRepo: workHourRepo, notifSvc: notifSvc}
}

func (s *employmentService) SyncActiveForUser(user *models.User) error {
	if user == nil {
		return nil
	}
	// Solo profesionales y customer success se vinculan a una empresa.
	if user.UserType != models.UserTypeProfessional && user.UserType != models.UserTypeCustomerSuccess {
		return nil
	}
	if user.EmpleadorID == nil || *user.EmpleadorID == 0 {
		return nil
	}
	companyID := *user.EmpleadorID

	existing, err := s.repo.GetActive(user.ID, companyID)
	if err == nil && existing != nil {
		return s.repo.Update(existing, map[string]interface{}{
			"job_title":  user.JobTitle,
			"manager_id": user.ManagerID,
		})
	}

	started := user.CreatedAt
	if started.IsZero() {
		started = time.Now()
	}
	return s.repo.Create(&models.Employment{
		UserID:    user.ID,
		CompanyID: companyID,
		JobTitle:  user.JobTitle,
		ManagerID: user.ManagerID,
		Status:    models.EmploymentActive,
		StartedAt: started,
	})
}

func (s *employmentService) ActiveCompanies(userID uint) ([]models.CompanyRef, error) {
	employments, err := s.repo.ListActiveByUser(userID)
	if err != nil {
		return nil, err
	}
	refs := make([]models.CompanyRef, 0, len(employments))
	for _, e := range employments {
		name := ""
		if company, err := s.userRepo.GetByID(e.CompanyID); err == nil {
			name = company.CompanyName
			if name == "" {
				name = company.Name
			}
		}
		refs = append(refs, models.CompanyRef{ID: e.CompanyID, Name: name})
	}
	return refs, nil
}

func (s *employmentService) SwitchActive(userID, companyID uint) (*models.User, error) {
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return nil, errors.New("Usuario no encontrado")
	}
	// Ya es la empresa activa: no-op.
	if user.EmpleadorID != nil && *user.EmpleadorID == companyID {
		return user, nil
	}
	// Debe tener empleo ACTIVO en esa empresa.
	if _, err := s.repo.GetActive(userID, companyID); err != nil {
		return nil, errors.New("No tienes un empleo activo en esa empresa")
	}
	// Sin bump de token_version: re-emitimos el token de ESTA sesión con el
	// nuevo tenant; las otras sesiones conservan su empresa hasta refrescar.
	if err := s.userRepo.Update(user, map[string]interface{}{"empleador_id": companyID}); err != nil {
		return nil, err
	}
	cid := companyID
	user.EmpleadorID = &cid
	return user, nil
}

func (s *employmentService) ListForUser(userID uint) ([]EmploymentView, error) {
	employments, err := s.repo.ListByUser(userID)
	if err != nil {
		return nil, err
	}
	views := make([]EmploymentView, 0, len(employments))
	for _, e := range employments {
		v := EmploymentView{Employment: e}
		if company, err := s.userRepo.GetByID(e.CompanyID); err == nil {
			v.CompanyName = company.CompanyName
			if v.CompanyName == "" {
				v.CompanyName = company.Name
			}
		}
		if e.ManagerID != nil {
			if mgr, err := s.userRepo.GetByID(*e.ManagerID); err == nil {
				v.ManagerName = mgr.Name
			}
		}
		views = append(views, v)
	}
	return views, nil
}

func (s *employmentService) AddEmployment(userID, companyID uint, jobTitle, startReason string, managerID *uint) (*models.Employment, error) {
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return nil, errors.New("Usuario no encontrado")
	}
	if user.UserType != models.UserTypeProfessional && user.UserType != models.UserTypeCustomerSuccess {
		return nil, errors.New("Solo profesionales y customer success pueden vincularse a empresas")
	}

	company, err := s.userRepo.GetByID(companyID)
	if err != nil || company.UserType != models.UserTypeEmployer {
		return nil, errors.New("La empresa seleccionada no es válida")
	}
	if !company.IsActive {
		return nil, errors.New("La empresa seleccionada está suspendida")
	}

	if existing, err := s.repo.GetActive(userID, companyID); err == nil && existing != nil {
		return nil, errors.New("El usuario ya pertenece a esta empresa")
	}

	employment := &models.Employment{
		UserID:      userID,
		CompanyID:   companyID,
		JobTitle:    utils.SanitizeHTML(jobTitle),
		ManagerID:   managerID,
		Status:      models.EmploymentActive,
		StartedAt:   time.Now(),
		StartReason: utils.SanitizeHTML(startReason),
	}
	if err := s.repo.Create(employment); err != nil {
		return nil, err
	}

	// Si el usuario no tenía empresa activa, esta pasa a serlo (e invalida su
	// sesión para que el nuevo tenant entre en el JWT).
	if user.EmpleadorID == nil || *user.EmpleadorID == 0 {
		_ = s.userRepo.Update(user, map[string]interface{}{
			"empleador_id":  companyID,
			"token_version": user.TokenVersion + 1,
		})
	}
	return employment, nil
}

func (s *employmentService) EndEmployment(userID, employmentID uint, endReason string) error {
	employments, err := s.repo.ListByUser(userID)
	if err != nil {
		return err
	}
	var target *models.Employment
	for i := range employments {
		if employments[i].ID == employmentID {
			target = &employments[i]
			break
		}
	}
	if target == nil {
		return errors.New("Membresía no encontrada")
	}
	if target.Status != models.EmploymentActive {
		return errors.New("Esta membresía ya está finalizada")
	}

	now := time.Now()

	// Congela el legajo en el momento de la salida: resumen cuantitativo +
	// evaluaciones/notas + gestiones, tal como estaban. No cambia aunque luego
	// se editen/borren esos registros.
	summary := s.computeSummary(target, now)
	summary.FrozenAt = &now
	frozen := FrozenExpediente{ExpedienteSummary: summary}

	notes, _ := s.repo.ListNotes(target.ID)
	for _, n := range notes {
		nv := ExpedienteNoteView{EmploymentNote: n}
		if a, err := s.userRepo.GetByID(n.AuthorID); err == nil {
			nv.AuthorName = a.Name
		}
		frozen.Notes = append(frozen.Notes, nv)
	}
	if fus, err := s.repo.ListFollowUps(target.UserID, target.StartedAt, now); err == nil {
		for _, f := range fus {
			g := GestionEntry{Kind: f.Kind, Status: f.Status, Note: f.Note, CreatedAt: f.CreatedAt}
			if by, err := s.userRepo.GetByID(f.CreatedBy); err == nil {
				g.ByName = by.Name
			}
			frozen.Gestiones = append(frozen.Gestiones, g)
		}
	}

	endSummary := ""
	if blob, err := json.Marshal(frozen); err == nil {
		endSummary = string(blob)
	}

	if err := s.repo.Update(target, map[string]interface{}{
		"status":      models.EmploymentEnded,
		"ended_at":    now,
		"end_reason":  utils.SanitizeHTML(endReason),
		"end_summary": endSummary,
	}); err != nil {
		return err
	}

	// Si era la empresa activa, reasigna empleador_id a otra membresía activa
	// (o nil) e invalida la sesión para reflejar el nuevo tenant.
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return nil // membresía finalizada igual; el usuario se relee aparte
	}
	if user.EmpleadorID != nil && *user.EmpleadorID == target.CompanyID {
		var nextCompany *uint
		for _, e := range employments {
			if e.ID != employmentID && e.Status == models.EmploymentActive {
				c := e.CompanyID
				nextCompany = &c
				break
			}
		}
		_ = s.userRepo.Update(user, map[string]interface{}{
			"empleador_id":  nextCompany,
			"token_version": user.TokenVersion + 1,
		})
	}
	return nil
}

func (s *employmentService) ReactivateEmployment(employmentID uint) error {
	emp, err := s.repo.GetByID(employmentID)
	if err != nil {
		return errors.New("Empleo no encontrado")
	}
	if emp.Status == models.EmploymentActive {
		return errors.New("Este empleo ya está activo")
	}
	// Si ya existe OTRO empleo activo en la misma empresa, no se puede duplicar.
	if existing, err := s.repo.GetActive(emp.UserID, emp.CompanyID); err == nil && existing != nil {
		return errors.New("El usuario ya tiene un empleo activo en esta empresa")
	}

	if err := s.repo.Update(emp, map[string]interface{}{
		"status":      models.EmploymentActive,
		"ended_at":    nil,
		"end_reason":  "",
		"end_summary": "",
	}); err != nil {
		return err
	}

	// Si el usuario no tenía empresa activa, esta vuelve a serlo (e invalida su
	// sesión para que el tenant entre de nuevo en el JWT).
	if user, err := s.userRepo.GetByID(emp.UserID); err == nil {
		if user.EmpleadorID == nil || *user.EmpleadorID == 0 {
			_ = s.userRepo.Update(user, map[string]interface{}{
				"empleador_id":  emp.CompanyID,
				"token_version": user.TokenVersion + 1,
			})
		}
	}
	return nil
}

// computeSummary calcula horas/tareas/antigüedad de un empleo hasta la fecha
// `at` (now para activos, ended_at para terminados). Tolera errores parciales:
// si una fuente falla, ese campo queda en cero pero el resto se devuelve.
func (s *employmentService) computeSummary(e *models.Employment, at time.Time) ExpedienteSummary {
	summary := ExpedienteSummary{}

	end := e.StartedAt
	if at.After(end) {
		end = at
	}
	summary.DaysEmployed = int(end.Sub(e.StartedAt).Hours() / 24)

	if hours, err := s.workHourRepo.GetSummary(map[string]interface{}{
		"user_id":    e.UserID,
		"tenant_id":  e.CompanyID,
		"start_date": e.StartedAt,
		"end_date":   at,
	}); err == nil {
		summary.TotalHours = hours["total_hours"]
		summary.ApprovedHours = hours["approved_hours"]
		summary.PendingHours = hours["pending_hours"]
	}

	if assigned, completed, err := s.repo.CountTasks(e.UserID, e.CompanyID); err == nil {
		summary.TasksAssigned = assigned
		summary.TasksCompleted = completed
	}

	if absences, err := s.workHourRepo.ListAbsences(e.UserID, e.CompanyID, e.StartedAt, at); err == nil {
		summary.Absences = len(absences)
	}
	return summary
}

// listAbsences arma el detalle de ausencias de un empleo (fecha, motivo, horas,
// si quedó aprobada) hasta la fecha `at`.
func (s *employmentService) listAbsences(e *models.Employment, at time.Time) []AbsenceEntry {
	rows, err := s.workHourRepo.ListAbsences(e.UserID, e.CompanyID, e.StartedAt, at)
	if err != nil {
		return []AbsenceEntry{}
	}
	entries := make([]AbsenceEntry, 0, len(rows))
	for _, r := range rows {
		hours := r.AbsenceHours
		if hours == 0 {
			hours = r.HoursWorked
		}
		entries = append(entries, AbsenceEntry{
			Date:     r.WorkDate,
			Reason:   r.AbsenceReason,
			Hours:    hours,
			Approved: r.Approved,
		})
	}
	return entries
}

// GetExpediente arma el expediente de un empleo: datos del empleo, resumen
// (congelado si terminó, en vivo si sigue activo), notas y documentos. La
// audiencia filtra la visibilidad: el profesional solo ve lo compartido.
func (s *employmentService) GetExpediente(employmentID uint, audience string) (*ExpedienteView, error) {
	emp, err := s.repo.GetByID(employmentID)
	if err != nil {
		return nil, errors.New("Empleo no encontrado")
	}

	view := EmploymentView{Employment: *emp}
	if company, err := s.userRepo.GetByID(emp.CompanyID); err == nil {
		view.CompanyName = company.CompanyName
		if view.CompanyName == "" {
			view.CompanyName = company.Name
		}
	}
	if emp.ManagerID != nil {
		if mgr, err := s.userRepo.GetByID(*emp.ManagerID); err == nil {
			view.ManagerName = mgr.Name
		}
	}

	// Si el empleo terminó, el legajo está SELLADO: resumen, notas y gestiones
	// salen del snapshot congelado (no cambian aunque se editen los registros).
	useFrozen := emp.Status == models.EmploymentEnded && emp.EndSummary != ""
	var frozen FrozenExpediente
	if useFrozen {
		_ = json.Unmarshal([]byte(emp.EndSummary), &frozen)
	}

	var summary ExpedienteSummary
	if useFrozen {
		summary = frozen.ExpedienteSummary
	} else {
		summary = s.computeSummary(emp, time.Now())
	}

	// Notas (filtradas por audiencia) con autor resuelto.
	noteViews := make([]ExpedienteNoteView, 0)
	if useFrozen {
		for _, nv := range frozen.Notes {
			if audience == AudienceProfessional && nv.Visibility != models.ExpedienteShared {
				continue
			}
			noteViews = append(noteViews, nv)
		}
	} else {
		notes, _ := s.repo.ListNotes(employmentID)
		for _, n := range notes {
			if audience == AudienceProfessional && n.Visibility != models.ExpedienteShared {
				continue
			}
			nv := ExpedienteNoteView{EmploymentNote: n}
			if author, err := s.userRepo.GetByID(n.AuthorID); err == nil {
				nv.AuthorName = author.Name
			}
			noteViews = append(noteViews, nv)
		}
	}

	// Documentos (filtrados por audiencia).
	docs, _ := s.repo.ListDocuments(employmentID)
	docViews := make([]models.EmploymentDocument, 0, len(docs))
	for _, d := range docs {
		if audience == AudienceProfessional && d.Visibility != models.ExpedienteShared {
			continue
		}
		docViews = append(docViews, d)
	}

	// Detalle de ausencias hasta el cierre del empleo (o ahora si sigue activo).
	absUntil := time.Now()
	if emp.EndedAt != nil {
		absUntil = *emp.EndedAt
	}
	absences := s.listAbsences(emp, absUntil)

	// Gestiones de CS: congeladas si el empleo terminó; en vivo si sigue activo.
	gestiones := make([]GestionEntry, 0)
	if useFrozen {
		gestiones = append(gestiones, frozen.Gestiones...)
	} else if fus, err := s.repo.ListFollowUps(emp.UserID, emp.StartedAt, absUntil); err == nil {
		for _, f := range fus {
			g := GestionEntry{Kind: f.Kind, Status: f.Status, Note: f.Note, CreatedAt: f.CreatedAt}
			if by, err := s.userRepo.GetByID(f.CreatedBy); err == nil {
				g.ByName = by.Name
			}
			gestiones = append(gestiones, g)
		}
	}

	// Contactos (email/WhatsApp/chat) durante el empleo, con autor resuelto.
	contactos := make([]ContactEntry, 0)
	if cs, err := s.repo.ListContacts(emp.UserID, emp.StartedAt, absUntil); err == nil {
		for _, c := range cs {
			ce := ContactEntry{Channel: c.Channel, Note: c.Note, CreatedAt: c.CreatedAt}
			if by, err := s.userRepo.GetByID(c.ByUserID); err == nil {
				ce.ByName = by.Name
			}
			contactos = append(contactos, ce)
		}
	}

	return &ExpedienteView{
		Employment: view,
		Summary:    summary,
		Notes:      noteViews,
		Documents:  docViews,
		Absences:   absences,
		Gestiones:  gestiones,
		Contactos:  contactos,
	}, nil
}

func (s *employmentService) GetCV(userID uint) (*CVView, error) {
	emps, err := s.repo.ListByUser(userID)
	if err != nil {
		return nil, err
	}
	cv := &CVView{Entries: []CVEntry{}}
	companies := map[uint]bool{}

	for i := range emps {
		emp := emps[i]

		view := EmploymentView{Employment: emp}
		if company, err := s.userRepo.GetByID(emp.CompanyID); err == nil {
			view.CompanyName = company.CompanyName
			if view.CompanyName == "" {
				view.CompanyName = company.Name
			}
		}
		if emp.ManagerID != nil {
			if mgr, err := s.userRepo.GetByID(*emp.ManagerID); err == nil {
				view.ManagerName = mgr.Name
			}
		}

		// Resumen y notas: congelados si terminó (legajo sellado), en vivo si no.
		useFrozen := emp.Status == models.EmploymentEnded && emp.EndSummary != ""
		var frozen FrozenExpediente
		if useFrozen {
			_ = json.Unmarshal([]byte(emp.EndSummary), &frozen)
		}

		var summary ExpedienteSummary
		if useFrozen {
			summary = frozen.ExpedienteSummary
		} else {
			summary = s.computeSummary(&emp, time.Now())
		}

		// Solo lo COMPARTIDO con el profesional (evaluaciones y documentos).
		sharedNotes := make([]ExpedienteNoteView, 0)
		if useFrozen {
			for _, nv := range frozen.Notes {
				if nv.Visibility == models.ExpedienteShared {
					sharedNotes = append(sharedNotes, nv)
				}
			}
		} else {
			notes, _ := s.repo.ListNotes(emp.ID)
			for _, n := range notes {
				if n.Visibility != models.ExpedienteShared {
					continue
				}
				nv := ExpedienteNoteView{EmploymentNote: n}
				if author, err := s.userRepo.GetByID(n.AuthorID); err == nil {
					nv.AuthorName = author.Name
				}
				sharedNotes = append(sharedNotes, nv)
			}
		}
		docs, _ := s.repo.ListDocuments(emp.ID)
		sharedDocs := make([]models.EmploymentDocument, 0)
		for _, d := range docs {
			if d.Visibility == models.ExpedienteShared {
				sharedDocs = append(sharedDocs, d)
			}
		}

		cv.Entries = append(cv.Entries, CVEntry{
			Employment: view,
			Summary:    summary,
			Notes:      sharedNotes,
			Documents:  sharedDocs,
		})
		companies[emp.CompanyID] = true
		if emp.Status == models.EmploymentActive {
			cv.ActiveCompanies++
		}
		cv.TotalDays += summary.DaysEmployed
	}
	cv.TotalCompanies = len(companies)
	return cv, nil
}

func (s *employmentService) GetCVPDF(userID uint) ([]byte, string, error) {
	cv, err := s.GetCV(userID)
	if err != nil {
		return nil, "", err
	}
	name, email := "", ""
	if u, err := s.userRepo.GetByID(userID); err == nil {
		name, email = u.Name, u.Email
	}
	bytes, err := generateCVPDF(cv, name, email)
	return bytes, name, err
}

func (s *employmentService) GetExpedientePDF(employmentID uint) ([]byte, string, error) {
	exp, err := s.GetExpediente(employmentID, AudienceCompany)
	if err != nil {
		return nil, "", err
	}
	name := ""
	if u, err := s.userRepo.GetByID(exp.Employment.UserID); err == nil {
		name = u.Name
	}
	bytes, err := generateExpedientePDF(exp, name)
	return bytes, name, err
}

func (s *employmentService) LogContact(userID, byUserID uint, channel string) error {
	if !models.IsValidContactChannel(channel) {
		return errors.New("Canal de contacto inválido")
	}
	return s.repo.CreateContact(&models.ContactLog{
		UserID:   userID,
		ByUserID: byUserID,
		Channel:  channel,
	})
}

func normalizeVisibility(v string) string {
	if v == models.ExpedienteShared {
		return models.ExpedienteShared
	}
	return models.ExpedientePrivate
}

func (s *employmentService) AddNote(employmentID, authorID uint, kind string, rating *int, content, visibility string) (*models.EmploymentNote, error) {
	if _, err := s.repo.GetByID(employmentID); err != nil {
		return nil, errors.New("Empleo no encontrado")
	}
	content = utils.SanitizeHTML(content)
	if content == "" {
		return nil, errors.New("La nota no puede estar vacía")
	}
	if kind != models.NoteKindEvaluation {
		kind = models.NoteKindNote
	}
	// El rating solo aplica a evaluaciones y debe estar en 1..5.
	if kind != models.NoteKindEvaluation {
		rating = nil
	} else if rating != nil && (*rating < 1 || *rating > 5) {
		return nil, errors.New("La calificación debe estar entre 1 y 5")
	}

	note := &models.EmploymentNote{
		EmploymentID: employmentID,
		AuthorID:     authorID,
		Kind:         kind,
		Rating:       rating,
		Content:      content,
		Visibility:   normalizeVisibility(visibility),
	}
	if err := s.repo.CreateNote(note); err != nil {
		return nil, err
	}
	if note.Visibility == models.ExpedienteShared {
		s.notifyShared(employmentID, noteWord(kind))
	}
	return note, nil
}

// noteWord devuelve "una evaluación" o "una nota" según el tipo.
func noteWord(kind string) string {
	if kind == models.NoteKindEvaluation {
		return "una evaluación"
	}
	return "una nota"
}

func (s *employmentService) UpdateNote(noteID uint, kind string, rating *int, content, visibility string) (*models.EmploymentNote, error) {
	note, err := s.repo.GetNote(noteID)
	if err != nil {
		return nil, errors.New("Nota no encontrada")
	}
	content = utils.SanitizeHTML(content)
	if content == "" {
		return nil, errors.New("La nota no puede estar vacía")
	}
	if kind != models.NoteKindEvaluation {
		kind = models.NoteKindNote
		rating = nil
	} else if rating != nil && (*rating < 1 || *rating > 5) {
		return nil, errors.New("La calificación debe estar entre 1 y 5")
	}
	wasShared := note.Visibility == models.ExpedienteShared
	vis := normalizeVisibility(visibility)
	if err := s.repo.UpdateNote(note, map[string]interface{}{
		"kind": kind, "rating": rating, "content": content, "visibility": vis,
	}); err != nil {
		return nil, err
	}
	if vis == models.ExpedienteShared && !wasShared {
		s.notifyShared(note.EmploymentID, noteWord(kind))
	}
	note.Kind, note.Rating, note.Content, note.Visibility = kind, rating, content, vis
	return note, nil
}

func (s *employmentService) DeleteNote(noteID uint) error {
	if _, err := s.repo.GetNote(noteID); err != nil {
		return errors.New("Nota no encontrada")
	}
	return s.repo.DeleteNote(noteID)
}

func (s *employmentService) AddDocument(employmentID, uploaderID uint, title, fileName, fileURL string, fileSize int64, mimeType, visibility string, expiresAt *time.Time) (*models.EmploymentDocument, error) {
	if _, err := s.repo.GetByID(employmentID); err != nil {
		return nil, errors.New("Empleo no encontrado")
	}
	if fileURL == "" || fileName == "" {
		return nil, errors.New("Falta el archivo")
	}
	doc := &models.EmploymentDocument{
		EmploymentID: employmentID,
		UploadedBy:   uploaderID,
		Title:        utils.SanitizeHTML(title),
		FileName:     fileName,
		FileURL:      fileURL,
		FileSize:     fileSize,
		MimeType:     mimeType,
		Visibility:   normalizeVisibility(visibility),
		ExpiresAt:    expiresAt,
	}
	if err := s.repo.CreateDocument(doc); err != nil {
		return nil, err
	}
	if doc.Visibility == models.ExpedienteShared {
		s.notifyShared(employmentID, "un documento")
	}
	return doc, nil
}

func (s *employmentService) UpdateDocument(docID uint, title, visibility string, expiresAt *time.Time) (*models.EmploymentDocument, error) {
	doc, err := s.repo.GetDocument(docID)
	if err != nil {
		return nil, errors.New("Documento no encontrado")
	}
	wasShared := doc.Visibility == models.ExpedienteShared
	vis := normalizeVisibility(visibility)
	if err := s.repo.UpdateDocument(doc, map[string]interface{}{
		"title": utils.SanitizeHTML(title), "visibility": vis, "expires_at": expiresAt,
		// Al renovar/cambiar el vencimiento, vuelve a ser alertable.
		"expiry_alerted_at": nil,
	}); err != nil {
		return nil, err
	}
	if vis == models.ExpedienteShared && !wasShared {
		s.notifyShared(doc.EmploymentID, "un documento")
	}
	doc.Title, doc.Visibility, doc.ExpiresAt = utils.SanitizeHTML(title), vis, expiresAt
	return doc, nil
}

func (s *employmentService) EmploymentCompanyID(employmentID uint) (uint, error) {
	emp, err := s.repo.GetByID(employmentID)
	if err != nil {
		return 0, err
	}
	return emp.CompanyID, nil
}

func (s *employmentService) NoteCompanyID(noteID uint) (uint, error) {
	note, err := s.repo.GetNote(noteID)
	if err != nil {
		return 0, err
	}
	return s.EmploymentCompanyID(note.EmploymentID)
}

func (s *employmentService) DocCompanyID(docID uint) (uint, error) {
	doc, err := s.repo.GetDocument(docID)
	if err != nil {
		return 0, err
	}
	return s.EmploymentCompanyID(doc.EmploymentID)
}

func (s *employmentService) EmploymentForUserInCompany(userID, companyID uint) (*EmploymentView, error) {
	emp, err := s.repo.GetByUserAndCompany(userID, companyID)
	if err != nil {
		return nil, errors.New("El profesional no tiene empleo en tu empresa")
	}
	view := EmploymentView{Employment: *emp}
	if company, err := s.userRepo.GetByID(emp.CompanyID); err == nil {
		view.CompanyName = firstNonEmpty(company.CompanyName, company.Name)
	}
	if emp.ManagerID != nil {
		if mgr, err := s.userRepo.GetByID(*emp.ManagerID); err == nil {
			view.ManagerName = mgr.Name
		}
	}
	return &view, nil
}

// notifyShared avisa al profesional que la empresa compartió algo en su
// expediente (best-effort; un fallo no rompe la operación).
func (s *employmentService) notifyShared(employmentID uint, what string) {
	if s.notifSvc == nil {
		return
	}
	emp, err := s.repo.GetByID(employmentID)
	if err != nil {
		return
	}
	companyName := "Una empresa"
	if c, err := s.userRepo.GetByID(emp.CompanyID); err == nil {
		if n := firstNonEmpty(c.CompanyName, c.Name); n != "" {
			companyName = n
		}
	}
	_ = s.notifSvc.CreateNotification(
		emp.UserID,
		"expediente",
		"Novedad en tu expediente",
		fmt.Sprintf("%s compartió %s contigo.", companyName, what),
		map[string]interface{}{"employment_id": employmentID},
	)
}

func (s *employmentService) DeleteDocument(docID uint) error {
	if _, err := s.repo.GetDocument(docID); err != nil {
		return errors.New("Documento no encontrado")
	}
	return s.repo.DeleteDocument(docID)
}

func (s *employmentService) DocumentForDownload(docID uint, audience string, requesterID uint) (*models.EmploymentDocument, error) {
	doc, err := s.repo.GetDocument(docID)
	if err != nil {
		return nil, errors.New("Documento no encontrado")
	}
	// El profesional solo puede bajar documentos COMPARTIDOS de su propio empleo.
	if audience == AudienceProfessional {
		emp, err := s.repo.GetByID(doc.EmploymentID)
		if err != nil || emp.UserID != requesterID || doc.Visibility != models.ExpedienteShared {
			return nil, errors.New("No autorizado")
		}
	}
	return doc, nil
}
