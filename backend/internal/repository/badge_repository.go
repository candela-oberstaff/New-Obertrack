package repository

import (
	"github.com/obertrack/backend/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// BadgeRepository guarda y lista las insignias ganadas.
type BadgeRepository interface {
	// Award otorga la insignia. Si la persona ya la tenía (mismo origen) no
	// hace nada y devuelve false: reaprobar tras un reinicio no duplica.
	Award(badge *models.UserBadge) (bool, error)
	ListByUser(userID uint) ([]models.UserBadge, error)
}

type badgeRepository struct {
	db *gorm.DB
}

func NewBadgeRepository(db *gorm.DB) BadgeRepository {
	return &badgeRepository{db: db}
}

func (r *badgeRepository) Award(badge *models.UserBadge) (bool, error) {
	// El índice único (user_id, source_key) es quien decide; ON CONFLICT
	// evita la carrera de "consultar y luego insertar".
	res := r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}, {Name: "source_key"}},
		DoNothing: true,
	}).Create(badge)
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}

func (r *badgeRepository) ListByUser(userID uint) ([]models.UserBadge, error) {
	var badges []models.UserBadge
	err := r.db.Where("user_id = ?", userID).
		Order("earned_at DESC, id DESC").Find(&badges).Error
	return badges, err
}
