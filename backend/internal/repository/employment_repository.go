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
	// CountActiveByManager cuenta las membresías activas que tienen a managerID
	// como manager (su equipo vigente). Sirve para impedir quitar el rol de
	// manager mientras aún tenga profesionales a su cargo.
	CountActiveByManager(managerID uint) (int64, error)
	// CountActiveByManagerInCompany cuenta las membresías activas que tienen a
	// managerID como manager dentro de una empresa concreta. Sirve para impedir
	// finalizar el empleo de un manager que aún tiene reportes en esa empresa.
	CountActiveByManagerInCompany(managerID, companyID uint) (int64, error)
	// ReassignManager mueve todas las membresías activas que tienen a oldManagerID
	// como manager (en todas las empresas) hacia newManagerID, o las desasigna si
	// newManagerID es nil. Devuelve cuántas filas se afectaron.
	ReassignManager(oldManagerID uint, newManagerID *uint, companyID uint) (int64, error)

	// --- Multi-manager N-a-N (employment_managers) ---
	// AddManager agrega managerID al empleo. Con isPrimary=false solo crea el
	// vínculo (sin tocar el principal); con isPrimary=true equivale a
	// SetPrimaryManager (lo marca principal y desmarca los demás).
	AddManager(employmentID, managerID uint, isPrimary bool) error
	// RemoveManager soft-borra el vínculo (employmentID, managerID).
	RemoveManager(employmentID, managerID uint) error
	// SetPrimaryManager marca managerID como principal del empleo (creando el
	// vínculo si falta o restaurándolo si estaba soft-borrado) y desmarca los
	// demás del mismo empleo. Respeta el índice único parcial.
	SetPrimaryManager(employmentID, managerID uint) error
	// ClearManagers soft-borra todos los vínculos vivos del empleo (desasignar).
	ClearManagers(employmentID uint) error
	// ReassignManagerLinks mueve, en los empleos activos de companyID cuyo
	// vínculo principal es oldManagerID, el principal hacia newManagerID (o lo
	// quita si newManagerID es nil).
	ReassignManagerLinks(oldManagerID, newManagerID *uint, companyID uint) error

	// --- Lecturas via-links (FASE 2, semántica "cualquier manager") ---
	// IsManagerOf indica si managerID es (alguno de) los managers del empleo
	// activo de (userID, companyID), vía un vínculo vivo en employment_managers.
	IsManagerOf(userID, companyID, managerID uint) (bool, error)
	// CountActiveByManagerViaLinks cuenta los empleos ACTIVOS con un vínculo vivo
	// a managerID (en cualquier empresa). Equivalente via-links de
	// CountActiveByManager.
	CountActiveByManagerViaLinks(managerID uint) (int64, error)
	// CountActiveByManagerInCompanyViaLinks idem, acotado a una empresa.
	CountActiveByManagerInCompanyViaLinks(managerID, companyID uint) (int64, error)
	// ListManagerIDs devuelve los IDs de los managers (vínculos vivos) del empleo
	// activo de (userID, companyID), para notificar a todos.
	ListManagerIDs(userID, companyID uint) ([]uint, error)
	// ListEmploymentManagers devuelve los vínculos vivos del empleo, principal
	// primero (ORDER BY is_primary DESC, id ASC).
	ListEmploymentManagers(employmentID uint) ([]models.EmploymentManager, error)

	// ListCompanyOrg devuelve una fila por empleo ACTIVO de la empresa, con su
	// manager principal, para dibujar el organigrama. Es plano a propósito: el
	// árbol se arma en el cliente, que es quien lo va a reordenar.
	ListCompanyOrg(companyID uint) ([]OrgNode, error)

	// --- Alcance transitivo (rol supervisor) ---
	// DescendantIDs devuelve los IDs de usuario que cuelgan de rootID en la
	// empresa, recorriendo employment_managers hacia abajo hasta maxDepth niveles
	// (1 = reportes directos, que es lo que ve un manager). No incluye a rootID.
	DescendantIDs(rootID, companyID uint, maxDepth int) ([]uint, error)
	// IsDescendantOf indica si userID cuelga de rootID dentro de la empresa, a
	// cualquier profundidad hasta maxDepth. Es DescendantIDs acotado a un usuario:
	// existe aparte para no traerse el árbol entero en un chequeo de permiso.
	IsDescendantOf(rootID, userID, companyID uint, maxDepth int) (bool, error)

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
	// usuario en una empresa dentro de un rango de fechas, más recientes primero.
	ListFollowUps(userID, companyID uint, start, end time.Time) ([]models.FollowUp, error)
	// CreateContact registra un intento de contacto sobre un profesional.
	CreateContact(contact *models.ContactLog) error
	// ListContacts lista los contactos a un usuario en una empresa dentro de un rango.
	ListContacts(userID, companyID uint, start, end time.Time) ([]models.ContactLog, error)
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

// joinLivingUser acota un conteo de empleos a los que pertenecen a un
// profesional que todavía existe y sigue activo. Sin esto se cuentan empleos
// huérfanos: borrar a un profesional no cierra su employment, así que su manager
// quedaba bloqueado para siempre por gente que ya no está.
func joinLivingUser(q *gorm.DB) *gorm.DB {
	return q.Joins("JOIN users ON users.id = employments.user_id AND users.deleted_at IS NULL AND users.is_active = ?", true)
}

func (r *employmentRepository) CountActiveByManager(managerID uint) (int64, error) {
	var count int64
	err := joinLivingUser(r.db.Model(&models.Employment{})).
		Where("employments.manager_id = ? AND employments.status = ?", managerID, models.EmploymentActive).
		Distinct("employments.user_id").
		Count(&count).Error
	return count, err
}

func (r *employmentRepository) CountActiveByManagerInCompany(managerID, companyID uint) (int64, error) {
	var count int64
	err := joinLivingUser(r.db.Model(&models.Employment{})).
		Where("employments.manager_id = ? AND employments.company_id = ? AND employments.status = ?", managerID, companyID, models.EmploymentActive).
		Distinct("employments.user_id").
		Count(&count).Error
	return count, err
}

func (r *employmentRepository) ReassignManager(oldManagerID uint, newManagerID *uint, companyID uint) (int64, error) {
	result := r.db.Model(&models.Employment{}).
		Where("manager_id = ? AND company_id = ? AND status = ?", oldManagerID, companyID, models.EmploymentActive).
		Update("manager_id", newManagerID)
	return result.RowsAffected, result.Error
}

// --- Multi-manager N-a-N (employment_managers) ---

func (r *employmentRepository) AddManager(employmentID, managerID uint, isPrimary bool) error {
	if isPrimary {
		return r.SetPrimaryManager(employmentID, managerID)
	}
	return r.upsertLink(r.db, employmentID, managerID, false)
}

func (r *employmentRepository) RemoveManager(employmentID, managerID uint) error {
	return r.db.
		Where("employment_id = ? AND manager_id = ?", employmentID, managerID).
		Delete(&models.EmploymentManager{}).Error
}

func (r *employmentRepository) SetPrimaryManager(employmentID, managerID uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		// Desmarca cualquier principal vivo del empleo para liberar el índice
		// único parcial antes de marcar el nuevo.
		if err := tx.Model(&models.EmploymentManager{}).
			Where("employment_id = ? AND is_primary = ? AND deleted_at IS NULL", employmentID, true).
			Update("is_primary", false).Error; err != nil {
			return err
		}
		return r.upsertLink(tx, employmentID, managerID, true)
	})
}

func (r *employmentRepository) ClearManagers(employmentID uint) error {
	return r.db.
		Where("employment_id = ?", employmentID).
		Delete(&models.EmploymentManager{}).Error
}

func (r *employmentRepository) ReassignManagerLinks(oldManagerID, newManagerID *uint, companyID uint) error {
	if oldManagerID == nil {
		return nil
	}
	return r.db.Transaction(func(tx *gorm.DB) error {
		// TODOS los vínculos vivos del manager saliente en empleos activos de la
		// empresa — principal O adicional (antes solo movía los principales, así
		// que un manager adicional no se reasignaba).
		var links []models.EmploymentManager
		if err := tx.Model(&models.EmploymentManager{}).
			Select("employment_managers.*").
			Joins("JOIN employments e ON e.id = employment_managers.employment_id").
			Where("employment_managers.manager_id = ? AND employment_managers.deleted_at IS NULL", *oldManagerID).
			Where("e.company_id = ? AND e.status = ? AND e.deleted_at IS NULL", companyID, models.EmploymentActive).
			Find(&links).Error; err != nil {
			return err
		}

		for _, link := range links {
			wasPrimary := link.IsPrimary
			// Soft-borra el vínculo del manager saliente (sea principal o adicional).
			if err := tx.
				Where("employment_id = ? AND manager_id = ?", link.EmploymentID, *oldManagerID).
				Delete(&models.EmploymentManager{}).Error; err != nil {
				return err
			}
			if newManagerID == nil {
				continue
			}
			// Si el nuevo manager ya está vinculado a ese empleo, no se duplica.
			var cnt int64
			if err := tx.Model(&models.EmploymentManager{}).
				Where("employment_id = ? AND manager_id = ? AND deleted_at IS NULL", link.EmploymentID, *newManagerID).
				Count(&cnt).Error; err != nil {
				return err
			}
			if cnt == 0 {
				// Conserva el rol del saliente: principal -> principal, adicional -> adicional.
				if err := r.upsertLink(tx, link.EmploymentID, *newManagerID, wasPrimary); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

// --- Lecturas via-links (FASE 2) ---

func (r *employmentRepository) IsManagerOf(userID, companyID, managerID uint) (bool, error) {
	var exists bool
	err := r.db.Raw(`
		SELECT EXISTS (
			SELECT 1
			FROM employments e
			JOIN employment_managers em
			  ON em.employment_id = e.id
			 AND em.manager_id = ?
			 AND em.deleted_at IS NULL
			WHERE e.user_id = ?
			  AND e.company_id = ?
			  AND e.status = ?
			  AND e.deleted_at IS NULL
		)`, managerID, userID, companyID, models.EmploymentActive).
		Scan(&exists).Error
	return exists, err
}

func (r *employmentRepository) CountActiveByManagerViaLinks(managerID uint) (int64, error) {
	var count int64
	err := r.db.Model(&models.EmploymentManager{}).
		Joins("JOIN employments e ON e.id = employment_managers.employment_id").
		Joins("JOIN users u ON u.id = e.user_id AND u.deleted_at IS NULL AND u.is_active = ?", true).
		Where("employment_managers.manager_id = ? AND employment_managers.deleted_at IS NULL", managerID).
		Where("e.status = ? AND e.deleted_at IS NULL", models.EmploymentActive).
		Distinct("e.user_id").
		Count(&count).Error
	return count, err
}

func (r *employmentRepository) CountActiveByManagerInCompanyViaLinks(managerID, companyID uint) (int64, error) {
	var count int64
	err := r.db.Model(&models.EmploymentManager{}).
		Joins("JOIN employments e ON e.id = employment_managers.employment_id").
		Joins("JOIN users u ON u.id = e.user_id AND u.deleted_at IS NULL AND u.is_active = ?", true).
		Where("employment_managers.manager_id = ? AND employment_managers.deleted_at IS NULL", managerID).
		Where("e.company_id = ? AND e.status = ? AND e.deleted_at IS NULL", companyID, models.EmploymentActive).
		Distinct("e.user_id").
		Count(&count).Error
	return count, err
}

func (r *employmentRepository) ListManagerIDs(userID, companyID uint) ([]uint, error) {
	var ids []uint
	err := r.db.Model(&models.EmploymentManager{}).
		Joins("JOIN employments e ON e.id = employment_managers.employment_id").
		Where("e.user_id = ? AND e.company_id = ? AND e.status = ? AND e.deleted_at IS NULL", userID, companyID, models.EmploymentActive).
		Where("employment_managers.deleted_at IS NULL").
		Distinct().
		Pluck("employment_managers.manager_id", &ids).Error
	return ids, err
}

func (r *employmentRepository) ListEmploymentManagers(employmentID uint) ([]models.EmploymentManager, error) {
	var links []models.EmploymentManager
	err := r.db.
		Where("employment_id = ? AND deleted_at IS NULL", employmentID).
		Order("is_primary DESC, id ASC").
		Find(&links).Error
	return links, err
}

// OrgNode es una persona dentro del organigrama de una empresa. ManagerID nulo
// = cuelga de la raíz (nadie por encima dentro de la empresa).
type OrgNode struct {
	UserID       uint   `json:"user_id"`
	EmploymentID uint   `json:"employment_id"`
	Name         string `json:"name"`
	Email        string `json:"email"`
	Avatar       string `json:"avatar"`
	JobTitle     string `json:"job_title"`
	IsManager    bool   `json:"is_manager"`
	IsSupervisor bool   `json:"is_supervisor"`
	IsActive     bool   `json:"is_active"`
	ManagerID    *uint  `json:"manager_id"`
	// IsCompany marca la cuenta de empresa, que es la raíz del organigrama. No
	// sale de employments (una empresa no se emplea a sí misma): la agrega el
	// servicio para que el árbol tenga cabeza.
	IsCompany bool `json:"is_company"`
	// ExtraManagers son los managers ADICIONALES al principal. El árbol solo
	// dibuja el principal (si no, dejaría de ser un árbol), así que este contador
	// es lo que avisa de que la persona además responde a alguien más.
	ExtraManagers int `json:"extra_managers"`
}

func (r *employmentRepository) ListCompanyOrg(companyID uint) ([]OrgNode, error) {
	if companyID == 0 {
		return []OrgNode{}, nil
	}
	var nodes []OrgNode
	// El manager principal se toma de employment_managers (is_primary), que es la
	// fuente que mantiene el dual-write, y se cae al puntero employments.manager_id
	// para los empleos que aún no tengan vínculo escrito.
	// DISTINCT ON: una fila POR PERSONA. Si por un arrastre de datos alguien
	// tuviera dos empleos activos en la misma empresa, aparecería dos veces en el
	// organigrama y se podría arrastrar cada copia por separado, que es peor que
	// quedarse con el empleo más antiguo.
	err := r.db.Raw(`
		SELECT * FROM (
		SELECT DISTINCT ON (e.user_id)
			e.user_id        AS user_id,
			e.id             AS employment_id,
			u.name           AS name,
			u.email          AS email,
			u.avatar         AS avatar,
			COALESCE(NULLIF(e.job_title, ''), u.job_title, '') AS job_title,
			u.is_manager     AS is_manager,
			u.is_supervisor  AS is_supervisor,
			u.is_active      AS is_active,
			COALESCE(
				(SELECT em.manager_id FROM employment_managers em
				  WHERE em.employment_id = e.id AND em.is_primary AND em.deleted_at IS NULL
				  LIMIT 1),
				e.manager_id
			) AS manager_id,
			(SELECT COUNT(*) FROM employment_managers em2
			  WHERE em2.employment_id = e.id AND NOT em2.is_primary AND em2.deleted_at IS NULL
			) AS extra_managers
		FROM employments e
		JOIN users u ON u.id = e.user_id AND u.deleted_at IS NULL
		WHERE e.company_id = ?
		  AND e.status = ?
		  AND e.deleted_at IS NULL
		ORDER BY e.user_id, e.id ASC
		) t ORDER BY t.name ASC
	`, companyID, models.EmploymentActive).Scan(&nodes).Error
	if nodes == nil {
		nodes = []OrgNode{}
	}
	return nodes, err
}

// --- Alcance transitivo (rol supervisor) ---

// subtreeCTE recorre la cadena de mando hacia ABAJO desde un usuario raíz,
// dentro de una empresa. Sus parámetros, en orden: rootID, companyID, estado
// activo, companyID, estado activo, maxDepth.
//
// Se apoya en employment_managers y no en el puntero employments.manager_id
// porque el dual-write la mantiene en espejo desde la fase 1 del multi-manager:
// está poblada valga lo que valga MULTI_MANAGER_READS, y además es la única que
// sabe expresar que un empleo tenga varios jefes.
//
// UNION —y no UNION ALL— deduplica. Junto con el tope de profundidad, eso evita
// que un ciclo heredado de datos viejos haga girar la recursión para siempre:
// aunque ensureNoManagerCycle impide crearlos hoy, esta consulta no depende de
// esa garantía.
const subtreeCTE = `
	WITH RECURSIVE subtree(user_id, depth) AS (
		SELECT e.user_id, 1
		FROM employments e
		JOIN employment_managers em
		  ON em.employment_id = e.id
		 AND em.deleted_at IS NULL
		WHERE em.manager_id = ?
		  AND e.company_id = ?
		  AND e.status = ?
		  AND e.deleted_at IS NULL
	  UNION
		SELECT e.user_id, s.depth + 1
		FROM subtree s
		JOIN employment_managers em
		  ON em.manager_id = s.user_id
		 AND em.deleted_at IS NULL
		JOIN employments e
		  ON e.id = em.employment_id
		 AND e.company_id = ?
		 AND e.status = ?
		 AND e.deleted_at IS NULL
		WHERE s.depth < ?
	)`

func (r *employmentRepository) DescendantIDs(rootID, companyID uint, maxDepth int) ([]uint, error) {
	if rootID == 0 || companyID == 0 || maxDepth < 1 {
		return nil, nil
	}
	var ids []uint
	err := r.db.Raw(subtreeCTE+`
		SELECT DISTINCT s.user_id
		FROM subtree s
		JOIN users u ON u.id = s.user_id AND u.deleted_at IS NULL AND u.is_active = ?
		WHERE s.user_id <> ?`,
		rootID, companyID, models.EmploymentActive,
		companyID, models.EmploymentActive, maxDepth,
		true, rootID,
	).Scan(&ids).Error
	return ids, err
}

func (r *employmentRepository) IsDescendantOf(rootID, userID, companyID uint, maxDepth int) (bool, error) {
	// La raíz nunca es descendiente de sí misma: eso mantiene la separación de
	// funciones (nadie aprueba sus propias jornadas) sin depender de que cada
	// llamador lo recuerde.
	if rootID == 0 || userID == 0 || companyID == 0 || maxDepth < 1 || rootID == userID {
		return false, nil
	}
	var exists bool
	err := r.db.Raw(subtreeCTE+`
		SELECT EXISTS (SELECT 1 FROM subtree s WHERE s.user_id = ?)`,
		rootID, companyID, models.EmploymentActive,
		companyID, models.EmploymentActive, maxDepth,
		userID,
	).Scan(&exists).Error
	return exists, err
}

// upsertLink crea o restaura el vínculo (employmentID, managerID) con el valor
// de is_primary indicado. Si existe vivo, lo actualiza; si está soft-borrado, lo
// restaura; si no existe, lo crea. Usa el tx/db recibido para encadenar en
// transacciones.
func (r *employmentRepository) upsertLink(db *gorm.DB, employmentID, managerID uint, isPrimary bool) error {
	// Busca cualquier fila (incluso soft-borrada) para ese par.
	var existing models.EmploymentManager
	err := db.Unscoped().
		Where("employment_id = ? AND manager_id = ?", employmentID, managerID).
		First(&existing).Error
	if err == gorm.ErrRecordNotFound {
		return db.Create(&models.EmploymentManager{
			EmploymentID: employmentID,
			ManagerID:    managerID,
			IsPrimary:    isPrimary,
		}).Error
	}
	if err != nil {
		return err
	}
	// Restaura (deleted_at = NULL) y fija is_primary.
	return db.Unscoped().Model(&models.EmploymentManager{}).
		Where("id = ?", existing.ID).
		Updates(map[string]interface{}{
			"deleted_at": nil,
			"is_primary": isPrimary,
		}).Error
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

func (r *employmentRepository) ListFollowUps(userID, companyID uint, start, end time.Time) ([]models.FollowUp, error) {
	var fus []models.FollowUp
	err := r.db.
		Where("user_id = ? AND company_id = ? AND created_at >= ? AND created_at <= ?", userID, companyID, start, end).
		Order("created_at DESC").
		Find(&fus).Error
	return fus, err
}

func (r *employmentRepository) CreateContact(contact *models.ContactLog) error {
	return r.db.Create(contact).Error
}

func (r *employmentRepository) ListContacts(userID, companyID uint, start, end time.Time) ([]models.ContactLog, error) {
	var contacts []models.ContactLog
	err := r.db.
		Where("user_id = ? AND company_id = ? AND created_at >= ? AND created_at <= ?", userID, companyID, start, end).
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
