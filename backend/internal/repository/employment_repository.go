package repository

// EmploymentRepository gestiona las membresías de profesionales en empresas
// (employments), fuente de verdad del vínculo multi-empresa y del expediente.

import (
	"time"

	"github.com/obertrack/backend/internal/models"
	"gorm.io/gorm"
)

type EmploymentRepository interface {
	// GetActive devuelve la membresía activa de un usuario en una empresa, o nil.
	GetActive(userID, companyID uint) (*models.Employment, error)
	// GetByID devuelve una membresía por su ID (cualquier estado).
	GetByID(id uint) (*models.Employment, error)
	// GetByUserAndCompany devuelve la membresía de un usuario en una empresa
	// (prefiere la activa), o error si no existe. Para acotar al empleador.
	GetByUserAndCompany(userID, companyID uint) (*models.Employment, error)
	Create(employment *models.Employment) error
	Update(employment *models.Employment, updates map[string]interface{}) error
	// ListByUser lista todas las membresías de un usuario (activas y terminadas).
	ListByUser(userID uint) ([]models.Employment, error)
	// ListActiveByUser lista solo las membresías activas de un usuario.
	ListActiveByUser(userID uint) ([]models.Employment, error)

	// --- Expediente (FASE 3) ---
	CreateNote(note *models.EmploymentNote) error
	ListNotes(employmentID uint) ([]models.EmploymentNote, error)
	GetNote(id uint) (*models.EmploymentNote, error)
	UpdateNote(note *models.EmploymentNote, updates map[string]interface{}) error
	DeleteNote(id uint) error
	CreateDocument(doc *models.EmploymentDocument) error
	ListDocuments(employmentID uint) ([]models.EmploymentDocument, error)
	GetDocument(id uint) (*models.EmploymentDocument, error)
	UpdateDocument(doc *models.EmploymentDocument, updates map[string]interface{}) error
	DeleteDocument(id uint) error
	// CountTasks devuelve (asignadas, completadas) para un usuario dentro de un
	// tenant; alimenta el resumen congelado al terminar un empleo.
	CountTasks(userID, tenantID uint) (assigned int64, completed int64, err error)
	// ListFollowUps lista las gestiones de CS (inactividad/ausencia) de un
	// usuario dentro de un rango de fechas, más recientes primero.
	ListFollowUps(userID uint, start, end time.Time) ([]models.FollowUp, error)
	// CreateContact registra un intento de contacto sobre un profesional.
	CreateContact(contact *models.ContactLog) error
	// ListContacts lista los contactos a un usuario dentro de un rango.
	ListContacts(userID uint, start, end time.Time) ([]models.ContactLog, error)
	// ListDocumentsExpiringSoon lista documentos que vencen antes de `before` y
	// que aún no fueron alertados (para el watcher de vencimientos).
	ListDocumentsExpiringSoon(before time.Time) ([]models.EmploymentDocument, error)
	// MarkDocumentAlerted marca un documento como ya alertado por vencimiento.
	MarkDocumentAlerted(id uint, at time.Time) error
}

type employmentRepository struct {
	db *gorm.DB
}

func NewEmploymentRepository(db *gorm.DB) EmploymentRepository {
	return &employmentRepository{db: db}
}

func (r *employmentRepository) GetActive(userID, companyID uint) (*models.Employment, error) {
	var e models.Employment
	err := r.db.Where("user_id = ? AND company_id = ? AND status = ?", userID, companyID, models.EmploymentActive).
		First(&e).Error
	if err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *employmentRepository) Create(employment *models.Employment) error {
	return r.db.Create(employment).Error
}

func (r *employmentRepository) Update(employment *models.Employment, updates map[string]interface{}) error {
	return r.db.Model(employment).Updates(updates).Error
}

func (r *employmentRepository) ListByUser(userID uint) ([]models.Employment, error) {
	var employments []models.Employment
	err := r.db.Where("user_id = ?", userID).
		Order("status ASC, started_at DESC").
		Find(&employments).Error
	return employments, err
}

func (r *employmentRepository) ListActiveByUser(userID uint) ([]models.Employment, error) {
	var employments []models.Employment
	err := r.db.Where("user_id = ? AND status = ?", userID, models.EmploymentActive).
		Order("started_at DESC").
		Find(&employments).Error
	return employments, err
}

func (r *employmentRepository) GetByID(id uint) (*models.Employment, error) {
	var e models.Employment
	if err := r.db.First(&e, id).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *employmentRepository) GetByUserAndCompany(userID, companyID uint) (*models.Employment, error) {
	var e models.Employment
	// status ASC pone 'active' antes que 'ended'; started_at DESC, la más reciente.
	err := r.db.Where("user_id = ? AND company_id = ?", userID, companyID).
		Order("status ASC, started_at DESC").
		First(&e).Error
	if err != nil {
		return nil, err
	}
	return &e, nil
}

// --- Expediente (FASE 3) ---

func (r *employmentRepository) CreateNote(note *models.EmploymentNote) error {
	return r.db.Create(note).Error
}

func (r *employmentRepository) ListNotes(employmentID uint) ([]models.EmploymentNote, error) {
	var notes []models.EmploymentNote
	err := r.db.Where("employment_id = ?", employmentID).
		Order("created_at DESC").
		Find(&notes).Error
	return notes, err
}

func (r *employmentRepository) GetNote(id uint) (*models.EmploymentNote, error) {
	var n models.EmploymentNote
	if err := r.db.First(&n, id).Error; err != nil {
		return nil, err
	}
	return &n, nil
}

func (r *employmentRepository) UpdateNote(note *models.EmploymentNote, updates map[string]interface{}) error {
	return r.db.Model(note).Updates(updates).Error
}

func (r *employmentRepository) DeleteNote(id uint) error {
	return r.db.Delete(&models.EmploymentNote{}, id).Error
}

func (r *employmentRepository) CreateDocument(doc *models.EmploymentDocument) error {
	return r.db.Create(doc).Error
}

func (r *employmentRepository) ListDocuments(employmentID uint) ([]models.EmploymentDocument, error) {
	var docs []models.EmploymentDocument
	err := r.db.Where("employment_id = ?", employmentID).
		Order("created_at DESC").
		Find(&docs).Error
	return docs, err
}

func (r *employmentRepository) GetDocument(id uint) (*models.EmploymentDocument, error) {
	var d models.EmploymentDocument
	if err := r.db.First(&d, id).Error; err != nil {
		return nil, err
	}
	return &d, nil
}

func (r *employmentRepository) UpdateDocument(doc *models.EmploymentDocument, updates map[string]interface{}) error {
	return r.db.Model(doc).Updates(updates).Error
}

func (r *employmentRepository) DeleteDocument(id uint) error {
	return r.db.Delete(&models.EmploymentDocument{}, id).Error
}

// CountTasks cuenta las tareas asignadas y completadas de un usuario dentro de
// un tenant, vía la tabla de unión task_users. Alimenta el resumen congelado.
func (r *employmentRepository) CountTasks(userID, tenantID uint) (int64, int64, error) {
	var assigned, completed int64
	base := r.db.Table("task_users").
		Joins("JOIN tasks ON tasks.id = task_users.task_id").
		Where("task_users.user_id = ? AND tasks.tenant_id = ? AND tasks.deleted_at IS NULL", userID, tenantID)

	if err := base.Session(&gorm.Session{}).Count(&assigned).Error; err != nil {
		return 0, 0, err
	}
	if err := base.Session(&gorm.Session{}).Where("tasks.completed = true").Count(&completed).Error; err != nil {
		return 0, 0, err
	}
	return assigned, completed, nil
}

func (r *employmentRepository) ListFollowUps(userID uint, start, end time.Time) ([]models.FollowUp, error) {
	var fus []models.FollowUp
	err := r.db.
		Where("user_id = ? AND created_at >= ? AND created_at <= ?", userID, start, end).
		Order("created_at DESC").
		Find(&fus).Error
	return fus, err
}

func (r *employmentRepository) CreateContact(contact *models.ContactLog) error {
	return r.db.Create(contact).Error
}

func (r *employmentRepository) ListContacts(userID uint, start, end time.Time) ([]models.ContactLog, error) {
	var contacts []models.ContactLog
	err := r.db.
		Where("user_id = ? AND created_at >= ? AND created_at <= ?", userID, start, end).
		Order("created_at DESC").
		Find(&contacts).Error
	return contacts, err
}

func (r *employmentRepository) ListDocumentsExpiringSoon(before time.Time) ([]models.EmploymentDocument, error) {
	var docs []models.EmploymentDocument
	err := r.db.
		Where("expires_at IS NOT NULL AND expires_at <= ? AND expiry_alerted_at IS NULL", before).
		Order("expires_at ASC").
		Find(&docs).Error
	return docs, err
}

func (r *employmentRepository) MarkDocumentAlerted(id uint, at time.Time) error {
	return r.db.Model(&models.EmploymentDocument{}).Where("id = ?", id).
		Update("expiry_alerted_at", at).Error
}
