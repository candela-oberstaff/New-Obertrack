package repository

import (
	"strings"

	"github.com/obertrack/backend/internal/models"
	"gorm.io/gorm"
)

// BoardStatusCount is one row of the per-board, per-status task aggregation.
type BoardStatusCount struct {
	BoardID uint   `json:"board_id"`
	Status  string `json:"status"`
	Count   int64  `json:"count"`
}

type TaskRepository interface {
	FindAll(filters map[string]interface{}, offset, limit int) ([]models.Task, int64, error)
	CountByBoardAndStatus(tenantID uint) ([]BoardStatusCount, error)
	GetByID(id uint) (*models.Task, error)
	GetByIDAndTenant(id, tenantID uint) (*models.Task, error)
	Create(task *models.Task) error
	Update(task *models.Task, updates map[string]interface{}) error
	ReorderTasks(boardID uint, status string, orderedIDs []uint) error
	NextOrder(boardID uint, status string) int
	Delete(id uint) error
	AddComment(comment *models.Comment) error
	GetComment(id uint) (*models.Comment, error)
	UpdateComment(id uint, content string) error
	DeleteComment(id uint) error
	AddAttachment(attachment *models.TaskAttachment) error
	DeleteAttachment(attachment *models.TaskAttachment) error
	GetAttachmentByID(id uint) (*models.TaskAttachment, error)
	SyncAssignees(task *models.Task, userIDs []uint) error
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

// CountByBoardAndStatus returns task counts grouped by board and status, scoped to
// a tenant. Aggregates in the database instead of loading every task to count
// client-side (used by the board picker).
func (r *taskRepository) CountByBoardAndStatus(tenantID uint) ([]BoardStatusCount, error) {
	var rows []BoardStatusCount
	query := r.db.Model(&models.Task{}).
		Select("board_id, status, COUNT(*) as count")
	if tenantID > 0 {
		query = query.Where("tenant_id = ?", tenantID)
	}
	err := query.Group("board_id, status").Scan(&rows).Error
	return rows, err
}

func (r *taskRepository) FindAll(filters map[string]interface{}, offset, limit int) ([]models.Task, int64, error) {
	var tasks []models.Task
	var total int64
	query := r.db.Model(&models.Task{})

	if employerID, ok := filters["employer_id"].(uint); ok {
		query = query.Where("tasks.created_by IN (SELECT id FROM users WHERE empleador_id = ?)", employerID)
	}
	if boardID, ok := filters["board_id"].(uint); ok {
		query = query.Where("tasks.board_id = ?", boardID)
	}
	if status, ok := filters["status"].(string); ok {
		query = query.Where("tasks.status = ?", status)
	}
	if assigneeID, ok := filters["assignee_id"].(uint); ok {
		if creatorID, ok := filters["created_by"].(uint); ok {
			query = query.Where(r.db.Where("tasks.id IN (SELECT task_id FROM task_users WHERE user_id = ?)", assigneeID).Or("tasks.created_by = ?", creatorID))
			delete(filters, "created_by") // Handled by Or
		} else {
			query = query.Where("tasks.id IN (SELECT task_id FROM task_users WHERE user_id = ?)", assigneeID)
		}
	} else if creatorID, ok := filters["created_by"].(uint); ok {
		query = query.Where("tasks.created_by = ?", creatorID)
	}
	if tenantID, ok := filters["tenant_id"].(uint); ok {
		query = query.Where("tasks.tenant_id = ?", tenantID)
	} else if companyID, ok := filters["company_id"].(uint); ok {
		query = query.Where("tasks.tenant_id = ?", companyID)
	}

	// Restringe a tareas de tableros donde el usuario es miembro (o creador),
	// respetando la membresía de tableros (usado para profesionales regulares).
	if uid, ok := filters["member_board_user_id"].(uint); ok {
		query = query.Where(
			"tasks.board_id IN (SELECT id FROM boards WHERE created_by = ? OR id IN (SELECT board_id FROM board_members WHERE user_id = ?))",
			uid, uid,
		)
	}

	if startDate, ok := filters["start_date"].(string); ok && startDate != "" {
		query = query.Where("tasks.created_at >= ?", startDate)
	}
	if endDate, ok := filters["end_date"].(string); ok && endDate != "" {
		query = query.Where("tasks.created_at <= ?", endDate)
	}
	if search, ok := filters["search"].(string); ok {
		query = query.Where("tasks.title ILIKE ? OR tasks.description ILIKE ?", "%"+search+"%", "%"+search+"%")
	}

	if err := query.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := query.Select("tasks.id, tasks.title, tasks.status, tasks.priority, tasks.start_date, tasks.end_date, tasks.completed, tasks.created_by, tasks.board_id, tasks.tenant_id, tasks.order, tasks.visible_para, tasks.created_at, tasks.updated_at, tasks.deleted_at").
		Preload("Assignees").Preload("Attachments").
		Offset(offset).Limit(limit).Order("tasks.created_at DESC").Find(&tasks).Error; err != nil {
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

func (r *taskRepository) GetByIDAndTenant(id, tenantID uint) (*models.Task, error) {
	var task models.Task
	if err := r.db.Where("tenant_id = ?", tenantID).
		Preload("Creator").Preload("Assignees").Preload("Comments").
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

// ReorderTasks fija el orden manual de las tarjetas de una columna: order =
// posición en orderedIDs. Un solo UPDATE con CASE (no N statements): una
// columna de 50 tarjetas se reordena en un round-trip, y al ser SQL crudo no
// pisa updated_at de tarjetas que solo cambiaron de posición. El WHERE
// restringe a board y status para que un id ajeno colado en la lista no toque
// tareas de otro tablero/columna.
func (r *taskRepository) ReorderTasks(boardID uint, status string, orderedIDs []uint) error {
	if len(orderedIDs) == 0 {
		return nil
	}
	var b strings.Builder
	args := make([]interface{}, 0, len(orderedIDs)*3+2)
	b.WriteString(`UPDATE tasks SET "order" = CASE id`)
	for idx, id := range orderedIDs {
		b.WriteString(" WHEN ? THEN CAST(? AS integer)")
		args = append(args, id, idx)
	}
	b.WriteString(` END WHERE id IN (`)
	for idx, id := range orderedIDs {
		if idx > 0 {
			b.WriteString(",")
		}
		b.WriteString("?")
		args = append(args, id)
	}
	b.WriteString(`) AND board_id = ? AND status = ? AND deleted_at IS NULL`)
	args = append(args, boardID, status)
	return r.db.Exec(b.String(), args...).Error
}

// NextOrder devuelve el order para una tarea nueva: al final de su columna.
func (r *taskRepository) NextOrder(boardID uint, status string) int {
	var next int
	r.db.Model(&models.Task{}).
		Where("board_id = ? AND status = ?", boardID, status).
		Select(`COALESCE(MAX("order") + 1, 0)`).
		Scan(&next)
	return next
}

func (r *taskRepository) Delete(id uint) error {
	return r.db.Delete(&models.Task{}, id).Error
}

func (r *taskRepository) AddComment(comment *models.Comment) error {
	return r.db.Create(comment).Error
}

func (r *taskRepository) GetComment(id uint) (*models.Comment, error) {
	var comment models.Comment
	if err := r.db.Preload("User").First(&comment, id).Error; err != nil {
		return nil, err
	}
	return &comment, nil
}

func (r *taskRepository) UpdateComment(id uint, content string) error {
	return r.db.Model(&models.Comment{}).Where("id = ?", id).Update("content", content).Error
}

func (r *taskRepository) DeleteComment(id uint) error {
	return r.db.Delete(&models.Comment{}, id).Error
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

func (r *taskRepository) SyncAssignees(task *models.Task, userIDs []uint) error {
	var users []models.User
	if len(userIDs) > 0 {
		if err := r.db.Find(&users, userIDs).Error; err != nil {
			return err
		}
	}
	return r.db.Model(task).Association("Assignees").Replace(users)
}
