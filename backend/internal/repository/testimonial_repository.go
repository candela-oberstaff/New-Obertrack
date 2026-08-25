package repository

import (
	"github.com/obertrack/backend/internal/models"
	"gorm.io/gorm"
)

// TestimonialFilter acota el listado del panel interno. Los campos vacíos no
// filtran.
type TestimonialFilter struct {
	Status   string
	Audience string
	Search   string // Busca por nombre, correo o empresa de quien firma
}

// TestimonialRepository accede a las solicitudes de testimonio.
type TestimonialRepository interface {
	Create(t *models.Testimonial) error
	// Update actualiza por ID con un modelo limpio. Recibe el mapa de cambios
	// para no arrastrar asociaciones precargadas.
	Update(id uint, updates map[string]interface{}) error
	Delete(id uint) error

	GetByID(id uint) (*models.Testimonial, error)
	// GetByToken resuelve el enlace público. No precarga el usuario: la página
	// pública se arma con los datos congelados de la solicitud.
	GetByToken(token string) (*models.Testimonial, error)
	List(f TestimonialFilter) ([]models.Testimonial, error)
	// CountByStatus alimenta los contadores de las pestañas del panel.
	CountByStatus() (map[string]int64, error)
	// HasPendingForUser evita pedirle dos veces lo mismo a la misma persona.
	HasPendingForUser(userID uint, audience string) (bool, error)
}

type testimonialRepository struct {
	db *gorm.DB
}

func NewTestimonialRepository(db *gorm.DB) TestimonialRepository {
	return &testimonialRepository{db: db}
}

func (r *testimonialRepository) Create(t *models.Testimonial) error {
	return r.db.Create(t).Error
}

func (r *testimonialRepository) Update(id uint, updates map[string]interface{}) error {
	return r.db.Model(&models.Testimonial{}).Where("id = ?", id).Updates(updates).Error
}

func (r *testimonialRepository) Delete(id uint) error {
	return r.db.Delete(&models.Testimonial{}, id).Error
}

func (r *testimonialRepository) GetByID(id uint) (*models.Testimonial, error) {
	var t models.Testimonial
	if err := r.db.Preload("User").First(&t, id).Error; err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *testimonialRepository) GetByToken(token string) (*models.Testimonial, error) {
	var t models.Testimonial
	if err := r.db.Where("token = ?", token).First(&t).Error; err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *testimonialRepository) List(f TestimonialFilter) ([]models.Testimonial, error) {
	q := r.db.Model(&models.Testimonial{}).Preload("User")
	if f.Status != "" {
		q = q.Where("status = ?", f.Status)
	}
	if f.Audience != "" {
		q = q.Where("audience = ?", f.Audience)
	}
	if f.Search != "" {
		like := "%" + f.Search + "%"
		q = q.Where(
			"recipient_name ILIKE ? OR recipient_email ILIKE ? OR recipient_company ILIKE ?",
			like, like, like,
		)
	}
	var out []models.Testimonial
	// Los que esperan revisión primero (es la acción pendiente del panel), y
	// dentro de cada grupo lo más reciente arriba.
	err := q.Order("CASE WHEN status = 'submitted' THEN 0 ELSE 1 END, created_at DESC").
		Find(&out).Error
	return out, err
}

func (r *testimonialRepository) CountByStatus() (map[string]int64, error) {
	var rows []struct {
		Status string
		Total  int64
	}
	err := r.db.Model(&models.Testimonial{}).
		Select("status, COUNT(*) AS total").
		Group("status").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make(map[string]int64, len(rows))
	for _, row := range rows {
		out[row.Status] = row.Total
	}
	return out, nil
}

func (r *testimonialRepository) HasPendingForUser(userID uint, audience string) (bool, error) {
	var n int64
	err := r.db.Model(&models.Testimonial{}).
		Where("user_id = ? AND audience = ? AND status = ?", userID, audience, models.TestimonialPending).
		Where("expires_at > NOW()").
		Count(&n).Error
	return n > 0, err
}
