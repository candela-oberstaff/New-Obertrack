package repository

import (
	"time"

	"gorm.io/gorm"

	"github.com/obertrack/backend/internal/models"
)

// CrmNonceRepository guarda los jti de los tokens del CRM ya canjeados.
type CrmNonceRepository interface {
	// Insert consume el jti. Un jti repetido devuelve el error de restricción
	// única de la base de datos tal cual: quien llama decide qué significa.
	Insert(n *models.CrmLoginNonce) error
	// DeleteExpired borra los jti cuyo token ya no podría validar.
	DeleteExpired(before time.Time) error
}

type crmNonceRepository struct {
	db *gorm.DB
}

func NewCrmNonceRepository(db *gorm.DB) CrmNonceRepository {
	return &crmNonceRepository{db: db}
}

func (r *crmNonceRepository) Insert(n *models.CrmLoginNonce) error {
	return r.db.Create(n).Error
}

func (r *crmNonceRepository) DeleteExpired(before time.Time) error {
	return r.db.Where("expires_at < ?", before).Delete(&models.CrmLoginNonce{}).Error
}
