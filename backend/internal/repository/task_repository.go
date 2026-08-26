package repository

import (
	"strings"
	"time"

	"github.com/obertrack/backend/internal/models"
	"gorm.io/gorm"
)

// BoardStatusCount is one row of the per-board, per-status task aggregation.
type BoardStatusCount struct {
	BoardID uint   `json:"board_id"`
	Status  string `json:"status"`
	Count   int64  `json:"count"`
}

// UserLoad es la carga de trabajo VIVA de una persona: lo que todavía tiene por
// hacer. Lo terminado no cuenta, que es lo que permite que quien va al día vuelva a
// aparecer como disponible.
type UserLoad struct {
	UserID uint `json:"user_id"`
	// Abiertas son las tareas asignadas que no están completadas ni en finalizado.
	Abiertas int `json:"abiertas"`
	// Vencidas son las abiertas cuya fecha de fin ya pasó. Sirve de desempate: tres
	// tareas todas atrasadas pesan más que tres tranquilas.
	Vencidas int `json:"vencidas"`
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
	// AddStatusHistory registra un movimiento de columna. Lo llama el servicio en
	// modo best-effort: la bitácora no debe poder tumbar la operación que la
	// origina.
	AddStatusHistory(entry *models.TaskStatusHistory) error
	// StatusHistory devuelve los movimientos de una tarea, del más reciente al más
	// antiguo. limit <= 0 devuelve todos.
	StatusHistory(taskID uint, limit int) ([]models.TaskStatusHistory, error)
	// UpdateWithStatusHistory aplica los cambios y escribe la bitácora en la MISMA
	// transacción. Lo usa el camino de PUERTA de fase, donde el formulario tiene el
	// mismo peso que el movimiento: si el registro de quién aprobó qué no se puede
	// guardar, el movimiento no debe ocurrir. En el camino normal la bitácora sigue
	// siendo best-effort, porque ahí perderla sólo cuesta una línea de historial.
	UpdateWithStatusHistory(task *models.Task, updates map[string]interface{}, entry *models.TaskStatusHistory) error
	// ListByDueDate devuelve las tareas SIN terminar de una empresa cuya fecha de fin
	// cae en el rango [desde, hasta], con asignados y tablero ya cargados. La usa el
	// barrido del tiempo, que necesita la tarea entera para armar el snapshot: sin
	// asignados no hay a quién avisar, y sin tablero no hay ámbito que comprobar.
	ListByDueDate(tenantID uint, desde, hasta time.Time, limit int) ([]models.Task, error)
	// OpenLoadByUser cuenta la carga viva de cada uno de `userIDs` en la empresa.
	// Se cuenta en TODA la empresa y no sólo en un tablero: alguien hasta arriba de
	// trabajo en otro tablero está igual de ocupado, y repartirle más por no mirar
	// allí es justo el problema que esto viene a resolver.
	//
	// Quien no aparezca en el resultado es que no tiene ninguna: carga cero.
	OpenLoadByUser(tenantID uint, userIDs []uint) (map[uint]UserLoad, error)
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

	// status_changed_at viaja en la lista (no sólo en el detalle) porque la
	// antigüedad en columna se muestra en la tarjeta del tablero; omitirla aquí la
	// dejaría en cero en toda la vista de kanban sin que nada fallara.
	if err := query.Select("tasks.id, tasks.title, tasks.status, tasks.priority, tasks.start_date, tasks.end_date, tasks.completed, tasks.created_by, tasks.board_id, tasks.tenant_id, tasks.order, tasks.revision, tasks.status_changed_at, tasks.visible_para, tasks.created_at, tasks.updated_at, tasks.deleted_at").
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

// ListByDueDate acota por fecha, no por "vencida": quien llama decide qué ventana
// mira. Las completadas quedan fuera porque una tarea terminada no vence.
func (r *taskRepository) ListByDueDate(tenantID uint, desde, hasta time.Time, limit int) ([]models.Task, error) {
	var tasks []models.Task
	q := r.db.
		Where("tenant_id = ? AND completed = ? AND end_date IS NOT NULL", tenantID, false).
		Where("end_date >= ? AND end_date <= ?", desde, hasta).
		Preload("Assignees").
		Order("end_date ASC, id ASC")
	if limit > 0 {
		q = q.Limit(limit)
	}
	return tasks, q.Find(&tasks).Error
}

func (r *taskRepository) OpenLoadByUser(tenantID uint, userIDs []uint) (map[uint]UserLoad, error) {
	out := map[uint]UserLoad{}
	if len(userIDs) == 0 {
		return out, nil
	}
	var rows []UserLoad
	// Model(&Task{}) y no una consulta cruda: así el filtro de borrado lógico lo
	// pone GORM. Escrito a mano se olvida, y las tareas borradas volverían a contar
	// como carga de alguien.
	err := r.db.Model(&models.Task{}).
		Select(`task_users.user_id AS user_id,
		        COUNT(*) AS abiertas,
		        SUM(CASE WHEN tasks.end_date IS NOT NULL AND tasks.end_date < ? THEN 1 ELSE 0 END) AS vencidas`,
			time.Now()).
		Joins("JOIN task_users ON task_users.task_id = tasks.id").
		Where("tasks.tenant_id = ? AND tasks.completed = ? AND tasks.status <> ?",
			tenantID, false, models.TaskStatusDone).
		Where("task_users.user_id IN ?", userIDs).
		Group("task_users.user_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.UserID] = row
	}
	return out, nil
}

func (r *taskRepository) Create(task *models.Task) error {
	return r.db.Create(task).Error
}

// Update aplica `updates` sobre la tarea e incrementa su revisión EN EL MISMO
// statement. La revisión es la "versión del cambio" de la que el motor de workflows
// deriva su clave de idempotencia; hacerlo en una escritura aparte abriría una
// ventana en la que dos cambios distintos comparten revisión.
//
// El mapa del llamador no se muta: taskService lo sigue inspeccionando después.
func (r *taskRepository) Update(task *models.Task, updates map[string]interface{}) error {
	patch := make(map[string]interface{}, len(updates)+1)
	for k, v := range updates {
		patch[k] = v
	}
	patch["revision"] = gorm.Expr("revision + 1")
	return r.db.Model(task).Updates(patch).Error
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

// SyncAssignees reemplaza el conjunto de asignados y bumpea la revisión de la tarea.
//
// El bump es necesario aquí y no basta con el de Update: una edición que sólo toque
// asignados no pasa por Update (taskService únicamente lo llama si hay campos que
// escribir), así que sin esto dos reasignaciones consecutivas compartirían revisión y
// la segunda se descartaría como duplicada. Va aparte del Replace porque este opera
// sobre task_users, no sobre tasks.
//
// Que una edición con campos Y asignados bumpee dos veces es inocuo: la revisión sólo
// tiene que ser monotónica y cambiar en cada mutación, y el emisor lee el valor final
// de la tarea recargada.
func (r *taskRepository) SyncAssignees(task *models.Task, userIDs []uint) error {
	var users []models.User
	if len(userIDs) > 0 {
		if err := r.db.Find(&users, userIDs).Error; err != nil {
			return err
		}
	}
	if err := r.db.Model(task).Association("Assignees").Replace(users); err != nil {
		return err
	}
	return r.db.Model(&models.Task{}).Where("id = ?", task.ID).
		UpdateColumn("revision", gorm.Expr("revision + 1")).Error
}

func (r *taskRepository) AddStatusHistory(entry *models.TaskStatusHistory) error {
	return r.db.Create(entry).Error
}

func (r *taskRepository) UpdateWithStatusHistory(task *models.Task, updates map[string]interface{}, entry *models.TaskStatusHistory) error {
	patch := make(map[string]interface{}, len(updates)+1)
	for k, v := range updates {
		patch[k] = v
	}
	patch["revision"] = gorm.Expr("revision + 1")

	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(task).Updates(patch).Error; err != nil {
			return err
		}
		return tx.Create(entry).Error
	})
}

func (r *taskRepository) StatusHistory(taskID uint, limit int) ([]models.TaskStatusHistory, error) {
	var rows []models.TaskStatusHistory
	query := r.db.Where("task_id = ?", taskID).Order("changed_at DESC, id DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	return rows, query.Find(&rows).Error
}
