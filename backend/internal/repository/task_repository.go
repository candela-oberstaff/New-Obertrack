package repository

import (
	"github.com/obertrack/backend/internal/models"
	"gorm.io/gorm"
)

type TaskRepository interface {
	GetAll(query *gorm.DB, offset, limit int) ([]models.Task, int64, error)
	GetByID(id uint) (*models.Task, error)
	Create(task *models.Task) error
	Update(task *models.Task, updates map[string]interface{}) error
	Delete(id uint) error
	AddComment(comment *models.Comment) error
	AddAttachment(attachment *models.TaskAttachment) error
	DeleteAttachment(attachment *models.TaskAttachment) error
	GetAttachmentByID(id uint) (*models.TaskAttachment, error)
	GetDB() *gorm.DB // Permite acceso bruto a gorm.DB para queries dinámicas complejas
}

type taskRepository struct {
	db *gorm.DB
}

func NewTaskRepository(db *gorm.DB) TaskRepository {
	return &taskRepository{db: db}
}

func (r *taskRepository) GetDB() *gorm.DB {
	return r.db
}

func (r *taskRepository) GetAll(query *gorm.DB, offset, limit int) ([]models.Task, int64, error) {
	var tasks []models.Task
	var total int64

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := query.Preload("Creator").Preload("Assignees").Preload("Board").Preload("Attachments").
		Offset(offset).Limit(limit).Find(&tasks).Error; err != nil {
		return nil, 0, err
	}

	return tasks, total, nil
}

func (r *taskRepository) GetByID(id uint) (*models.Task, error) {
	var task models.Task
	if err := r.db.Preload("Creator").Preload("Assignees").Preload("Comments").
		Preload("Comments.User").Preload("Attachments").First(&task, id).Error; err != nil {
		return nil, err
	}
	return &task, nil
}

func (r *taskRepository) Create(task *models.Task) error {
	return r.db.Create(task).Error
}

func (r *taskRepository) Update(task *models.Task, updates map[string]interface{}) error {
	return r.db.Model(task).Updates(updates).Error
}

func (r *taskRepository) Delete(id uint) error {
	return r.db.Delete(&models.Task{}, id).Error
}

func (r *taskRepository) AddComment(comment *models.Comment) error {
	return r.db.Create(comment).Error
}

func (r *taskRepository) AddAttachment(attachment *models.TaskAttachment) error {
	return r.db.Create(attachment).Error
}

func (r *taskRepository) GetAttachmentByID(id uint) (*models.TaskAttachment, error) {
	var attachment models.TaskAttachment
	if err := r.db.First(&attachment, id).Error; err != nil {
		return nil, err
	}
	return &attachment, nil
}

func (r *taskRepository) DeleteAttachment(attachment *models.TaskAttachment) error {
	return r.db.Delete(attachment).Error
}
