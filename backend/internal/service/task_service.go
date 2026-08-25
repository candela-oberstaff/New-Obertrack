package service

import (
	"errors"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
	"github.com/obertrack/backend/internal/utils"
)

type TaskService interface {
	GetAll(userID uint, role string, isManager, crossCompany bool, tenantID, companyFilter uint, boardIDStr, status, priority, assigneeIDStr, startDate, endDate string, offset, limit int) ([]models.Task, int64, error)
	GetBoardStatusCounts(isSuperadmin bool, tenantID, companyFilter uint) (map[uint]map[string]int, error)
	GetByID(id uint, tenantID uint, isSuperadmin bool) (*models.Task, error)
	Create(userID uint, isSuperadmin bool, tenantID uint, title, description, priority string, endDate *string, assignees []uint, boardID uint) (*models.Task, []models.User, error)
	Update(id uint, tenantID uint, updaterUserID uint, role string, isManager, isSuperadmin bool, reqData map[string]interface{}, assignees *[]uint, gate map[string]any) (*models.Task, []models.User, error)
	Reorder(boardID, tenantID uint, isSuperadmin bool, status string, orderedIDs []uint) error
	Delete(id uint, tenantID uint, userID uint, role string, isManager, isSuperadmin bool) error
	ToggleCompletion(id uint, tenantID uint, updaterUserID uint, role string, isManager, isSuperadmin bool) (*models.Task, error)
	// StatusHistory devuelve los movimientos de columna de una tarea, del más
	// reciente al más antiguo, con el nombre de quien los hizo ya resuelto.
	StatusHistory(id uint, tenantID uint, isSuperadmin bool) ([]TaskStatusEntry, error)
	AddComment(id uint, tenantID uint, userID uint, content string, isSuperadmin bool) (*models.Comment, error)
	AddAttachment(taskID uint, tenantID uint, fileName, fileURL string, fileSize int64, mimeType string, uploadedBy uint, isSuperadmin bool) (*models.TaskAttachment, error)
	DeleteAttachment(attachmentID uint, tenantID uint, isSuperadmin bool) error

	// SetSystemDM cablea el emisor de DMs de sistema al chat interno (lo apunta a
	// channelService.PostSystemDM en deps.go). Callback inyectado para no acoplar
	// taskService→channelService, mismo patrón que ChannelService.SetBroadcaster.
	// Puede quedar sin cablear (nil): en ese caso no se envían DMs.
	SetSystemDM(fn func(recipientID uint, content string))

	// SetCalendarSync cablea los enganches de Google Calendar (Fase 2). Mismo
	// patrón inyectado que SetSystemDM para no acoplar taskService→calendarSync.
	// onChanged se llama tras crear/editar/completar; onDeleted tras eliminar.
	// Sin cablear (nil): la sincronización simplemente no ocurre.
	SetCalendarSync(onChanged, onDeleted func(taskID uint))

	// SetWorkflowEmitter cablea el motor de automatizaciones. Mismo patrón
	// inyectado que los dos anteriores, y por la misma razón: taskService no debe
	// saber que existe un motor de workflows. Sin cablear (nil), no se emite nada
	// y el módulo de Tareas se comporta exactamente como antes.
	SetWorkflowEmitter(fn func(WorkflowEvent))

	// SetGateChecker cablea las PUERTAS de fase: la mitad síncrona del motor, que
	// se interpone ANTES de que un cambio de columna ocurra. Sin cablear (nil), no
	// hay puertas y el tablero se comporta como siempre — el Modo Libre.
	SetGateChecker(fn GateChecker)

	// ApplyAsSystem aplica un cambio hecho por el MOTOR, no por una persona.
	//
	// Existe porque Update exige un actor humano y comprueba canModifyTask, y una
	// automatización no es nadie: no creó la tarea, no está asignada y no es manager
	// de nadie. Saltarse esa comprobación es justamente el privilegio que hay que
	// conceder de forma explícita y acotada, no colándolo por la puerta de atrás con
	// un usuario falso que resultara ser superadmin.
	//
	// El alcance de tenant SÍ se comprueba: el motor tiene menos permisos que un
	// empleador, no más.
	ApplyAsSystem(taskID uint, updates map[string]interface{}, assignees *[]uint, cause WorkflowCause) (*models.Task, error)
}

type taskService struct {
	repo      repository.TaskRepository
	userRepo  repository.UserRepository
	boardRepo repository.BoardRepository
	notifSvc  NotificationService
	// postSystemDM publica un DM del bot "Obertrack" en el chat interno. Inyectado
	// por SetSystemDM; nil = sin DMs (p. ej. en tests que no lo cablean).
	postSystemDM func(recipientID uint, content string)
	// calendarChanged/calendarDeleted enganchan la sincronización con Google
	// Calendar. Inyectados por SetCalendarSync; nil = sin sincronización.
	calendarChanged func(taskID uint)
	calendarDeleted func(taskID uint)
	// emitWorkflow enlaza con el motor de automatizaciones. Inyectado por
	// SetWorkflowEmitter; nil = no se emite nada.
	emitWorkflow func(WorkflowEvent)
	// gateChecker resuelve las puertas de fase. Inyectado por SetGateChecker;
	// nil = ninguna columna bloquea.
	gateChecker GateChecker
}

func (s *taskService) SetSystemDM(fn func(recipientID uint, content string)) {
	s.postSystemDM = fn
}

func (s *taskService) SetCalendarSync(onChanged, onDeleted func(taskID uint)) {
	s.calendarChanged = onChanged
	s.calendarDeleted = onDeleted
}

func (s *taskService) SetWorkflowEmitter(fn func(WorkflowEvent)) {
	s.emitWorkflow = fn
}

func (s *taskService) SetGateChecker(fn GateChecker) {
	s.gateChecker = fn
}

// checkGate consulta la puerta de la columna destino. Devuelve (nil, nil) cuando no
// hay puerta cableada o esa columna no tiene ninguna, que es el caso corriente.
func (s *taskService) checkGate(task *models.Task, toStatus string, gate map[string]any) (*GateResult, error) {
	if s.gateChecker == nil || task == nil {
		return nil, nil
	}
	return s.gateChecker(task.TenantID, task.BoardID, toStatus, gate)
}

// emit dispara el enganche del motor si está cableado. Best-effort y síncrono: el
// emisor sólo encola una fila, así que es barato; si un día dejara de serlo, esto
// es lo que habría que mover a una goroutine.
func (s *taskService) emit(ev WorkflowEvent) {
	if s.emitWorkflow == nil || ev.Task == nil {
		return
	}
	s.emitWorkflow(ev)
}

// emitTaskChange traduce una mutación de tarea a los eventos que le corresponden.
// Un mismo guardado puede cambiar estado, prioridad y asignados a la vez, y eso son
// TRES hechos distintos: una regla que escucha "cambió la prioridad" no debe
// dispararse porque además se movió de columna.
func (s *taskService) emitTaskChange(task *models.Task, prevStatus, prevPriority string, newAssignees []uint, actorID uint) {
	if s.emitWorkflow == nil || task == nil {
		return
	}
	base := WorkflowEvent{
		TenantID: task.TenantID,
		Task:     task,
		ActorID:  actorID,
	}

	if prevStatus != "" && prevStatus != string(task.Status) {
		ev := base
		ev.Type = models.TriggerTaskStatusChanged
		ev.PrevStatus = prevStatus
		s.emit(ev)
	}
	if prevPriority != "" && prevPriority != string(task.Priority) {
		ev := base
		ev.Type = models.TriggerTaskPriorityChanged
		ev.PrevPriority = prevPriority
		s.emit(ev)
	}
	if len(newAssignees) > 0 {
		ev := base
		ev.Type = models.TriggerTaskAssigned
		ev.NewAssignees = newAssignees
		s.emit(ev)
	}
}

// syncCalendarChanged/Deleted disparan el enganche si está cableado. Best-effort:
// el enganche solo encola (no llama a Google en el request), así que es barato.
func (s *taskService) syncCalendarChanged(taskID uint) {
	if s.calendarChanged != nil {
		s.calendarChanged(taskID)
	}
}

func (s *taskService) syncCalendarDeleted(taskID uint) {
	if s.calendarDeleted != nil {
		s.calendarDeleted(taskID)
	}
}

// sendSystemDM envía un DM del bot si el emisor está cableado. Best-effort: el
// aviso principal es la notificación de campanita.
func (s *taskService) sendSystemDM(recipientID uint, content string) {
	if s.postSystemDM != nil {
		s.postSystemDM(recipientID, content)
	}
}

// taskHistoryLimit acota lo que devuelve el detalle de una tarea. Una tarjeta que
// lleva meses rebotando entre columnas puede acumular muchas filas, y el panel sólo
// muestra las últimas; el corte evita traerlas todas para descartarlas en el cliente.
const taskHistoryLimit = 100

// TaskStatusEntry es una fila de la bitácora enriquecida con el nombre de quien hizo
// el cambio. La tabla guarda sólo el id —que es lo correcto: el nombre puede cambiar
// y la bitácora no debe congelar una copia— así que el nombre se resuelve al leer.
type TaskStatusEntry struct {
	models.TaskStatusHistory
	// ActorName vacío = el cambio no lo hizo una persona, o su cuenta ya no existe.
	ActorName string `json:"actor_name"`
}

func (s *taskService) StatusHistory(id uint, tenantID uint, isSuperadmin bool) ([]TaskStatusEntry, error) {
	task, err := s.authorizeTaskByID(id, tenantID, isSuperadmin)
	if err != nil {
		return nil, err
	}

	rows, err := s.repo.StatusHistory(task.ID, taskHistoryLimit)
	if err != nil {
		return nil, err
	}

	// Los nombres se resuelven una sola vez por actor: casi siempre son las mismas
	// dos o tres personas las que mueven una tarjeta, y así una tarea con 100
	// movimientos no dispara 100 consultas.
	names := make(map[uint]string)
	out := make([]TaskStatusEntry, 0, len(rows))
	for _, row := range rows {
		entry := TaskStatusEntry{TaskStatusHistory: row}
		if row.ChangedBy != nil {
			actorID := *row.ChangedBy
			name, cached := names[actorID]
			if !cached {
				if u, uerr := s.userRepo.GetByID(actorID); uerr == nil && u != nil {
					name = u.Name
				}
				names[actorID] = name
			}
			entry.ActorName = name
		}
		out = append(out, entry)
	}
	return out, nil
}

// recordStatusChange anota un movimiento de columna en la bitácora
// (task_status_history). Best-effort a propósito: la bitácora alimenta la antigüedad
// que se muestra al usuario y, más adelante, el disparador schedule.task_stale, pero
// nunca debe tumbar la operación que la origina.
//
// actorID 0 = el cambio no lo hizo una persona; la fila queda con changed_by nulo.
func (s *taskService) recordStatusChange(task *models.Task, from, to string, actorID uint) {
	if task == nil || from == to {
		return
	}
	entry := &models.TaskStatusHistory{
		TaskID:     task.ID,
		TenantID:   task.TenantID,
		FromStatus: from,
		ToStatus:   to,
		ChangedAt:  time.Now(),
	}
	if actorID > 0 {
		entry.ChangedBy = &actorID
	}
	if err := s.repo.AddStatusHistory(entry); err != nil {
		log.Printf("[TaskService] no se pudo registrar el movimiento de la tarea %d (%q → %q): %v",
			task.ID, from, to, err)
	}
}

// ApplyAsSystem es el camino de escritura del MOTOR. Deliberadamente parco: no hay
// notificaciones nativas, no hay DMs y no hay comprobación de permisos de persona.
// Lo que sí hay es todo lo que protege la integridad del dato.
func (s *taskService) ApplyAsSystem(taskID uint, updates map[string]interface{}, assignees *[]uint, cause WorkflowCause) (*models.Task, error) {
	task, err := s.repo.GetByID(taskID)
	if err != nil || task == nil {
		return nil, errors.New("Tarea no encontrada")
	}

	oldStatus := string(task.Status)
	oldPriority := string(task.Priority)
	newStatus := oldStatus
	if st, ok := updates["status"].(string); ok {
		newStatus = st
	}

	// PUERTA. Una acción automática sólo atraviesa una columna cerrada si su
	// ejecución nace de otra puerta YA cruzada: entonces la justificación existe, la
	// dio una persona, y viaja con la cadena causal. Si no, se rechaza — dejar pasar
	// al motor abriría exactamente el agujero que la puerta previene.
	if newStatus != oldStatus && !cause.GateJustified {
		if _, gerr := s.checkGate(task, newStatus, nil); gerr != nil {
			return nil, gerr
		}
	}

	if newStatus != oldStatus {
		updates["status_changed_at"] = time.Now()
		// Misma sincronía que en el camino humano: entrar en Finalizado completa la
		// tarea y salir de ahí la reabre.
		if _, explicit := updates["completed"]; !explicit {
			if newStatus == string(models.TaskStatusDone) {
				updates["completed"] = true
			} else if task.Completed {
				updates["completed"] = false
			}
		}
	}

	if len(updates) > 0 {
		if err := s.repo.Update(task, updates); err != nil {
			return nil, err
		}
	}

	var added []uint
	if assignees != nil {
		previous := make(map[uint]bool, len(task.Assignees))
		for _, a := range task.Assignees {
			previous[a.ID] = true
		}
		if err := s.validateAssignees(task.BoardID, *assignees, false); err != nil {
			return nil, err
		}
		if err := s.repo.SyncAssignees(task, *assignees); err != nil {
			return nil, err
		}
		for _, id := range *assignees {
			if !previous[id] {
				added = append(added, id)
			}
		}
	}

	finalTask, err := s.repo.GetByID(taskID)
	if err != nil {
		return task, nil
	}

	// La bitácora se anota con ChangedBy nulo: el movimiento no lo hizo una persona,
	// y firmarlo con una sería mentir sobre quién responde de él.
	s.recordStatusChange(finalTask, oldStatus, newStatus, 0)
	s.syncCalendarChanged(finalTask.ID)

	// Los eventos que genere heredan la cadena: un nivel más de profundidad y la
	// ejecución que los causó. Sin esto el antibucle no contaría y dos reglas que se
	// llamen entre sí girarían para siempre.
	s.emitSystemChange(finalTask, oldStatus, oldPriority, added, cause)

	return finalTask, nil
}

// emitSystemChange es emitTaskChange marcando el origen automático.
func (s *taskService) emitSystemChange(task *models.Task, prevStatus, prevPriority string, added []uint, cause WorkflowCause) {
	if s.emitWorkflow == nil || task == nil {
		return
	}
	runID := cause.RunID
	base := WorkflowEvent{
		TenantID: task.TenantID,
		Task:     task,
		// Sin actor humano: es la marca por la que una regla puede decir "sólo
		// cuando lo mueva alguien" y no dispararse con sus propias consecuencias.
		ActorIsSystem: true,
		Depth:         cause.Depth + 1,
	}
	if runID > 0 {
		base.CauseRunID = &runID
	}

	if prevStatus != "" && prevStatus != string(task.Status) {
		ev := base
		ev.Type = models.TriggerTaskStatusChanged
		ev.PrevStatus = prevStatus
		s.emit(ev)
	}
	if prevPriority != "" && prevPriority != string(task.Priority) {
		ev := base
		ev.Type = models.TriggerTaskPriorityChanged
		ev.PrevPriority = prevPriority
		s.emit(ev)
	}
	if len(added) > 0 {
		ev := base
		ev.Type = models.TriggerTaskAssigned
		ev.NewAssignees = added
		s.emit(ev)
	}
}

// taskDeepLink arma el enlace que abre LA TARJETA, no la pantalla de tareas.
//
// La pantalla ya sabía recibir ?company=&board=&task= y abrir la ficha, pero ninguna
// notificación lo aprovechaba: todas mandaban "/tasks" a secas y dejaban al lector
// buscando a mano la tarjeta de la que le acababan de hablar.
//
// Van los tres parámetros porque quien lee puede no tener ese tablero seleccionado, y
// un superadmin ni siquiera esa empresa en foco: con sólo la tarea, el enlace se
// quedaría a medio camino en cuanto el destinatario no estuviera ya mirando el sitio
// exacto.
func taskDeepLink(taskID, boardID, companyID uint) string {
	link := fmt.Sprintf("/tasks?task=%d", taskID)
	if boardID > 0 {
		link += fmt.Sprintf("&board=%d", boardID)
	}
	if companyID > 0 {
		link += fmt.Sprintf("&company=%d", companyID)
	}
	return link
}

// taskDueSuffix devuelve " · vence DD/MM/AAAA" si la tarea tiene fecha límite, o
// "" si no. Se anexa al DM de asignación para dar contexto sin otra línea.
func taskDueSuffix(t *models.Task) string {
	if t != nil && t.EndDate != nil {
		return " · vence " + t.EndDate.Format("02/01/2006")
	}
	return ""
}

func (s *taskService) authorizeBoardTenant(boardID, tenantID uint, isSuperadmin bool) error {
	if isSuperadmin {
		return nil
	}

	board, err := s.boardRepo.GetByID(boardID)
	if err != nil {
		return errors.New("El tablero especificado no existe o fue eliminado")
	}

	// A qué empresa pertenece el tablero lo dice SU PROPIA columna, no quién lo creó.
	//
	// Deducirlo del creador dejaba fuera a la empresa de cualquier tablero que hubiera
	// creado un superadmin desde soporte: el creador no pertenece a ninguna empresa,
	// así que ninguna de las dos comprobaciones de abajo se cumplía y el dueño real
	// del tablero no podía ni crear una tarea en él.
	if board.TenantID != 0 && board.TenantID == tenantID {
		return nil
	}

	// Las dos de abajo se conservan para los tableros anteriores a que existiera la
	// columna, donde tenant_id puede venir vacío.
	if board.CreatedBy == tenantID {
		return nil
	}
	if board.Creator.EmpleadorID != nil && *board.Creator.EmpleadorID == tenantID {
		return nil
	}

	return errors.New("No tienes permiso para acceder a ese tablero")
}

func (s *taskService) authorizeTaskTenant(task *models.Task, tenantID uint, isSuperadmin bool) error {
	if isSuperadmin {
		return nil
	}

	// La tarea pertenece a un tenant, y es el mismo campo por el que GetAll
	// filtra la lista. Comprobarlo aquí es lo que hace que lista y detalle
	// coincidan.
	//
	// Antes la pertenencia se deducía del CREADOR DEL TABLERO, y eso dejaba
	// fuera cualquier tablero creado por un superadmin: su empleador_id es
	// nulo, así que no coincidía con ningún tenant. La lista mostraba la tarea
	// —filtrada por tenant_id, correctamente— y al abrirla el detalle la
	// negaba. Como el handler colapsa cualquier error en 404, salía
	// "Task not found" sobre una tarea que sí existía y sí era suya.
	if task.TenantID != 0 && task.TenantID == tenantID {
		return nil
	}

	// Compatibilidad con tareas antiguas sin tenant_id: para esas se sigue
	// resolviendo por el tablero, como hasta ahora.
	board, err := s.boardRepo.GetByID(task.BoardID)
	if err != nil {
		return errors.New("Tarea no encontrada")
	}

	if board.CreatedBy == tenantID {
		return nil
	}
	if board.Creator.EmpleadorID != nil && *board.Creator.EmpleadorID == tenantID {
		return nil
	}

	return errors.New("No tienes permiso para acceder a esta tarea")
}

func (s *taskService) canModifyTask(task *models.Task, userID uint, role string, isManager bool) bool {
	if isEmployerRole(role) || isManager {
		return true
	}
	if task.CreatedBy == userID {
		return true
	}
	for _, a := range task.Assignees {
		if a.ID == userID {
			return true
		}
	}
	return false
}

func (s *taskService) authorizeTaskByID(id, tenantID uint, isSuperadmin bool) (*models.Task, error) {
	var task *models.Task
	var err error
	if isSuperadmin || tenantID == 0 {
		task, err = s.repo.GetByID(id)
	} else {
		task, err = s.repo.GetByIDAndTenant(id, tenantID)
	}
	if err != nil {
		return nil, errors.New("Tarea no encontrada")
	}

	if err := s.authorizeTaskTenant(task, tenantID, isSuperadmin); err != nil {
		return nil, err
	}

	return task, nil
}

func NewTaskService(
	repo repository.TaskRepository,
	userRepo repository.UserRepository,
	boardRepo repository.BoardRepository,
	notifSvc NotificationService,
) TaskService {
	return &taskService{
		repo:      repo,
		userRepo:  userRepo,
		boardRepo: boardRepo,
		notifSvc:  notifSvc,
	}
}

// crossCompany: quien lee a nivel plataforma (superadmin y customer success) y
// elige la empresa en vez de quedar atado a la suya. Permiso de LECTURA; las
// escrituras de tareas siguen decidiéndose con isSuperadmin.
func (s *taskService) GetAll(userID uint, role string, isManager, crossCompany bool, tenantID, companyFilter uint, boardIDStr, status, priority, assigneeIDStr, startDate, endDate string, offset, limit int) ([]models.Task, int64, error) {
	filters := make(map[string]interface{})

	if boardIDStr != "" && boardIDStr != "all" {
		boardID, err := strconv.ParseUint(boardIDStr, 10, 32)
		if err == nil {
			filters["board_id"] = uint(boardID)
		}
	}

	if assigneeIDStr != "" && assigneeIDStr != "all" {
		assigneeID, err := strconv.ParseUint(assigneeIDStr, 10, 32)
		if err == nil {
			filters["assignee_id"] = uint(assigneeID)
		}
	}

	if startDate != "" {
		filters["start_date"] = startDate
	}
	if endDate != "" {
		filters["end_date"] = endDate
	}

	if crossCompany {
		// Debe elegir empresa explícitamente. Sin eso no se devuelve nada, para
		// no mezclar nunca tareas de tenants distintos en la misma vista.
		if companyFilter == 0 {
			return []models.Task{}, 0, nil
		}
		filters["tenant_id"] = companyFilter
	} else if tenantID > 0 {
		filters["tenant_id"] = tenantID
		// Empresas y managers supervisan al equipo: ven todas las tareas del
		// tenant. Un profesional regular solo ve las tareas de los tableros a
		// los que pertenece (igual que la lista de tableros, que es por
		// membresía); así no aparecen en su dashboard tareas inaccesibles.
		if !isManager && role != string(models.UserTypeEmployer) {
			filters["member_board_user_id"] = userID
		}
	}

	if status != "" {
		filters["status"] = status
	}
	if priority != "" {
		filters["priority"] = priority
	}

	return s.repo.FindAll(filters, offset, limit)
}

func (s *taskService) GetBoardStatusCounts(isSuperadmin bool, tenantID, companyFilter uint) (map[uint]map[string]int, error) {
	var scope uint
	if isSuperadmin {
		// Superadmin must scope to a company; without it return nothing.
		if companyFilter == 0 {
			return map[uint]map[string]int{}, nil
		}
		scope = companyFilter
	} else {
		scope = tenantID
	}

	rows, err := s.repo.CountByBoardAndStatus(scope)
	if err != nil {
		return nil, err
	}

	result := make(map[uint]map[string]int)
	for _, r := range rows {
		if result[r.BoardID] == nil {
			result[r.BoardID] = make(map[string]int)
		}
		result[r.BoardID][r.Status] = int(r.Count)
	}
	return result, nil
}

func (s *taskService) GetByID(id uint, tenantID uint, isSuperadmin bool) (*models.Task, error) {
	task, err := s.authorizeTaskByID(id, tenantID, isSuperadmin)
	if err != nil {
		return nil, err
	}
	return task, nil
}

func (s *taskService) validateAssignees(boardID uint, assignees []uint, isSuperadmin bool) error {
	if isSuperadmin {
		return nil
	}
	board, err := s.boardRepo.GetByID(boardID)
	if err != nil {
		return errors.New("El tablero especificado no existe o fue eliminado")
	}

	memberIDs := make(map[uint]bool)
	for _, m := range board.Members {
		memberIDs[m.ID] = true
	}
	if board.CreatedBy != 0 {
		memberIDs[board.CreatedBy] = true
	}

	for _, assigneeID := range assignees {
		if !memberIDs[assigneeID] {
			user, _ := s.userRepo.GetByID(assigneeID)
			userName := fmt.Sprintf("ID %d", assigneeID)
			if user != nil {
				userName = user.Name
			}
			return fmt.Errorf("%s no es miembro del tablero", userName)
		}
	}
	return nil
}

func (s *taskService) Create(userID uint, isSuperadmin bool, tenantID uint, title, description, priority string, endDate *string, assignees []uint, boardID uint) (*models.Task, []models.User, error) {

	if title == "" {
		return nil, nil, errors.New("Title is required")
	}

	if boardID == 0 {
		return nil, nil, errors.New("Debes seleccionar un tablero para crear una tarea")
	}

	if boardID > 0 {
		if err := s.authorizeBoardTenant(boardID, tenantID, isSuperadmin); err != nil {
			return nil, nil, err
		}
	}

	if len(assignees) > 0 && boardID > 0 {
		if err := s.validateAssignees(boardID, assignees, isSuperadmin); err != nil {
			return nil, nil, err
		}

		if isSuperadmin {
			// Auto-add assignees to board if they are not members
			board, _ := s.boardRepo.GetByID(boardID)
			if board != nil {
				memberIDs := make(map[uint]bool)
				for _, m := range board.Members {
					memberIDs[m.ID] = true
				}
				for _, mid := range assignees {
					if !memberIDs[mid] {
						u, _ := s.userRepo.GetByID(mid)
						if u != nil {
							s.boardRepo.AddMember(board, u)
						}
					}
				}
			}
		}
	}

	var boardTenant uint
	// La tarea nueva nace en la primera fase del tablero (la columna de más a la
	// izquierda), no siempre en "por_hacer": si el tablero no tiene una fase con
	// ese status (p. ej. se eliminó o reordenó), la tarea quedaría invisible.
	initialStatus := models.TaskStatusTodo
	if b, err := s.boardRepo.GetByID(boardID); err == nil {
		boardTenant = b.TenantID
		if len(b.Phases) > 0 {
			initialStatus = models.TaskStatus(PhaseColumnID(b.Phases[0]))
		}
	}

	now := time.Now()
	task := &models.Task{
		Title:       utils.SanitizeHTML(title),
		Description: utils.SanitizeHTML(description),
		Status:      initialStatus,
		Priority:    models.PriorityMedium,
		CreatedBy:   userID,
		BoardID:     boardID,
		TenantID:    boardTenant,
		// Al final de su columna: sin esto (order 0) la tarea nueva se mete en el
		// grupo de arriba de una columna reordenada a mano.
		Order: s.repo.NextOrder(boardID, string(initialStatus)),
		// La tarea entra en su primera columna AHORA. Sellarlo aquí evita que una
		// tarea recién creada aparezca sin antigüedad hasta que alguien la mueva.
		StatusChangedAt: &now,
	}

	if priority != "" {
		task.Priority = models.TaskPriority(priority)
	}

	if endDate != nil && *endDate != "" {
		parsedEndDate, err := time.Parse("2006-01-02", *endDate)
		if err == nil {
			task.EndDate = &parsedEndDate
		}
	}

	if err := s.repo.Create(task); err != nil {
		return nil, nil, err
	}

	// Primera fila de la bitácora: FromStatus vacío marca el nacimiento de la
	// tarea, para que el historial de una tarjeta sea completo desde el origen.
	s.recordStatusChange(task, "", string(initialStatus), userID)

	if len(assignees) > 0 {
		s.repo.SyncAssignees(task, assignees)
	}

	finalTask, err := s.repo.GetByID(task.ID)
	if err != nil {
		log.Printf("[TaskService] Error refreshing task %d for notifications: %v", task.ID, err)
		return task, nil, nil // Return the original task but continue
	}

	log.Printf("[TaskService] Notifying %d assignees for new task: %s", len(finalTask.Assignees), task.Title)

	for _, assignee := range finalTask.Assignees {
		err := s.notifSvc.CreateNotification(assignee.ID, "task_assigned", "Nueva tarea asignada", fmt.Sprintf("Se te asignó la tarea: %s", task.Title), map[string]interface{}{
			"task_id":  task.ID,
			"board_id": task.BoardID,
			"link":     taskDeepLink(task.ID, task.BoardID, task.TenantID),
		})
		if err != nil {
			log.Printf("[TaskService] Error creating internal notification for user %d: %v", assignee.ID, err)
		}

		s.sendSystemDM(assignee.ID, fmt.Sprintf("📋 Se te asignó la tarea: %s%s", task.Title, taskDueSuffix(finalTask)))
	}

	// Notify employer/company
	creator, _ := s.userRepo.GetByID(userID)
	board, _ := s.boardRepo.GetByID(task.BoardID)
	employerIDs := make(map[uint]bool)

	// If the task creator is a professional and has an employer
	if creator != nil && creator.UserType == models.UserTypeProfessional && creator.EmpleadorID != nil {
		employerIDs[*creator.EmpleadorID] = true
	}

	// Also, if the board creator is an employer and not the task creator
	if board != nil && board.CreatedBy != userID {
		boardCreator, _ := s.userRepo.GetByID(board.CreatedBy)
		if boardCreator != nil && (boardCreator.UserType == models.UserTypeEmployer || boardCreator.IsManager) {
			employerIDs[board.CreatedBy] = true
		}
	}

	for empID := range employerIDs {
		if empID == userID {
			continue
		}

		creatorName := "Alguien"
		if creator != nil {
			creatorName = creator.Name
		}

		err := s.notifSvc.CreateNotification(empID, "task_created", "Nueva tarea creada", fmt.Sprintf("%s creó la tarea: %s", creatorName, task.Title), map[string]interface{}{
			"task_id":  task.ID,
			"board_id": task.BoardID,
			"link":     taskDeepLink(task.ID, task.BoardID, task.TenantID),
		})
		if err != nil {
			log.Printf("[TaskService] Error creating task_created notification for employer %d: %v", empID, err)
		}
	}

	// Google Calendar: crea el evento en el calendario de cada asignado conectado
	// (si tiene fecha). Solo encola; el worker lo aplica en segundo plano.
	s.syncCalendarChanged(finalTask.ID)

	// Automatizaciones. Crear con asignados es a la vez task.created y
	// task.assigned: quien escribió "avísame cuando me asignen algo" espera el
	// aviso tanto si le asignan una tarea existente como si nace ya con su nombre.
	s.emit(WorkflowEvent{
		Type:     models.TriggerTaskCreated,
		TenantID: finalTask.TenantID,
		Task:     finalTask,
		ActorID:  userID,
	})
	if len(finalTask.Assignees) > 0 {
		assigned := make([]uint, 0, len(finalTask.Assignees))
		for _, a := range finalTask.Assignees {
			assigned = append(assigned, a.ID)
		}
		s.emit(WorkflowEvent{
			Type:         models.TriggerTaskAssigned,
			TenantID:     finalTask.TenantID,
			Task:         finalTask,
			NewAssignees: assigned,
			ActorID:      userID,
		})
	}

	return finalTask, finalTask.Assignees, nil
}

func (s *taskService) Update(id uint, tenantID uint, updaterUserID uint, role string, isManager, isSuperadmin bool, reqData map[string]interface{}, assignees *[]uint, gate map[string]any) (*models.Task, []models.User, error) {
	task, err := s.authorizeTaskByID(id, tenantID, isSuperadmin)
	if err != nil {
		return nil, nil, err
	}

	if !isSuperadmin && !s.canModifyTask(task, updaterUserID, role, isManager) {
		return nil, nil, errors.New("Access denied")
	}

	// Keep track of assignees before update (to detect who is new vs existing)
	currentAssigneeIDs := make(map[uint]bool)
	for _, a := range task.Assignees {
		currentAssigneeIDs[a.ID] = true
	}

	// Fecha límite ANTES del update. El frontend reenvía end_date en CADA edición
	// (aunque solo cambies la prioridad), así que la presencia de la clave en
	// reqData no significa que la fecha cambió: hay que comparar el valor real.
	var oldEndDate string
	if task.EndDate != nil {
		oldEndDate = task.EndDate.Format("2006-01-02")
	}

	// Misma sanitización que en Create: la web sanitiza al renderizar, pero otros
	// consumidores (p. ej. la app móvil) leen el HTML tal cual quedó en la base.
	if title, ok := reqData["title"].(string); ok {
		reqData["title"] = utils.SanitizeHTML(title)
	}
	if description, ok := reqData["description"].(string); ok {
		reqData["description"] = utils.SanitizeHTML(description)
	}

	// Estado ANTES del update. Hay que capturarlo aquí porque repo.Update escribe
	// con Model(task).Updates(mapa) y GORM refleja el mapa sobre la instancia: a
	// partir de esa llamada, task.Status ya es el valor nuevo.
	oldStatus := string(task.Status)
	newStatus := oldStatus
	// La prioridad anterior se guarda por el mismo motivo que el estado: tras
	// repo.Update, GORM refleja el mapa sobre la instancia y el valor viejo se pierde.
	oldPriority := string(task.Priority)

	// Mantiene `completed` en sincronía con el status (igual que ToggleCompletion):
	// arrastrar a "Finalizado" completa la tarea y sacarla de ahí la reabre. Solo
	// cuando el status CAMBIA de verdad: el formulario de edición reenvía el
	// status actual en cada guardado, y sincronizar con un status sin cambios
	// reabriría/completaría tareas por editar un título. Un `completed` explícito
	// en el request tiene prioridad.
	if status, ok := reqData["status"].(string); ok && status != oldStatus {
		newStatus = status
		// El sello de entrada en la columna viaja en el MISMO statement que el
		// status, de forma que no puedan divergir ni siquiera si la escritura
		// falla a medias.
		reqData["status_changed_at"] = time.Now()
		if _, explicit := reqData["completed"]; !explicit {
			if status == string(models.TaskStatusDone) {
				reqData["completed"] = true
			} else if task.Completed {
				reqData["completed"] = false
			}
		}
	}

	// PUERTA DE FASE. Se evalúa ANTES de escribir nada: si la columna destino exige
	// un formulario y no viene, o viene incompleto, el movimiento no ocurre y el
	// error viaja hasta el handler, que lo traduce a un 422 con la definición del
	// formulario para que el cliente pueda pedirlo.
	//
	// Sólo se consulta cuando el estado CAMBIA de verdad: el formulario de edición
	// reenvía el status actual en cada guardado, y cobrar peaje por editar un título
	// convertiría la puerta en un castigo.
	var gateResult *GateResult
	if newStatus != oldStatus {
		res, gerr := s.checkGate(task, newStatus, gate)
		if gerr != nil {
			return nil, nil, gerr
		}
		gateResult = res
	}

	if len(reqData) > 0 {
		if gateResult != nil {
			// Se cruzó una puerta: el formulario y el movimiento se guardan juntos o
			// no se guarda ninguno. Un movimiento sin su registro dejaría una
			// aprobación sin rastro de quién la dio.
			entry := &models.TaskStatusHistory{
				TaskID: task.ID, TenantID: task.TenantID,
				FromStatus: oldStatus, ToStatus: newStatus,
				ChangedAt:      time.Now(),
				GateWorkflowID: &gateResult.WorkflowID,
				FormData:       ptrString(mustJSON(models.GateSubmission{Fields: gateResult.Fields})),
			}
			if updaterUserID > 0 {
				entry.ChangedBy = &updaterUserID
			}
			if err := s.repo.UpdateWithStatusHistory(task, reqData, entry); err != nil {
				return nil, nil, err
			}
		} else if err := s.repo.Update(task, reqData); err != nil {
			return nil, nil, err
		}
	}

	// recordStatusChange descarta por sí solo el caso from == to, así que no hace
	// falta repetir la condición aquí. Si se cruzó una puerta, la bitácora ya quedó
	// escrita dentro de la transacción y repetirla duplicaría la fila.
	if gateResult == nil {
		s.recordStatusChange(task, oldStatus, newStatus, updaterUserID)
	}

	if assignees != nil {
		if err := s.validateAssignees(task.BoardID, *assignees, isSuperadmin); err != nil {
			return nil, nil, err
		}

		if isSuperadmin {
			// Auto-add assignees to board if they are not members
			board, _ := s.boardRepo.GetByID(task.BoardID)
			if board != nil {
				memberIDs := make(map[uint]bool)
				for _, m := range board.Members {
					memberIDs[m.ID] = true
				}
				for _, mid := range *assignees {
					if !memberIDs[mid] {
						u, _ := s.userRepo.GetByID(mid)
						if u != nil {
							s.boardRepo.AddMember(board, u)
						}
					}
				}
			}
		}

		s.repo.SyncAssignees(task, *assignees)
	}

	// Fetch final refreshed task with preloads (assignees, board, creator)
	finalTask, err := s.repo.GetByID(task.ID)
	if err != nil {
		log.Printf("[TaskService] Error refreshing task %d for update notifications: %v", task.ID, err)
		return task, task.Assignees, nil
	}
	task = finalTask

	// ¿Cambió la fecha límite DE VERDAD? Comparamos el valor viejo con el nuevo
	// (recargado en task/finalTask), no la mera presencia de end_date en reqData.
	// Sirve para avisar por DM solo a los asignados que ya estaban (los nuevos
	// reciben el DM de asignación, más completo).
	var newEndDate string
	if task.EndDate != nil {
		newEndDate = task.EndDate.Format("2006-01-02")
	}
	deadlineChanged := oldEndDate != newEndDate

	// Notify new assignees (only if assignees changed)
	if assignees != nil {
		for _, u := range task.Assignees {
			if !currentAssigneeIDs[u.ID] {
				err := s.notifSvc.CreateNotification(u.ID, "task_assigned", "Nueva tarea asignada", fmt.Sprintf("Se te asignó la tarea: %s", task.Title), map[string]interface{}{
					"task_id":  task.ID,
					"board_id": task.BoardID,
					"link":     taskDeepLink(task.ID, task.BoardID, task.TenantID),
				})
				if err != nil {
					log.Printf("[TaskService] Error creating internal notification for user %d: %v", u.ID, err)
				}

				s.sendSystemDM(u.ID, fmt.Sprintf("📋 Se te asignó la tarea: %s%s", task.Title, taskDueSuffix(task)))
			}
		}
	}

	// Now handle modification notifications for employer and other assignees
	updaterName := "Alguien"
	var updater *models.User
	if updaterUserID > 0 {
		updater, _ = s.userRepo.GetByID(updaterUserID)
		if updater != nil {
			updaterName = updater.Name
		}
	}

	// Employer IDs to notify
	board, _ := s.boardRepo.GetByID(task.BoardID)
	employerIDs := make(map[uint]bool)

	if updater != nil && updater.UserType == models.UserTypeProfessional && updater.EmpleadorID != nil {
		employerIDs[*updater.EmpleadorID] = true
	}

	if board != nil && board.CreatedBy != updaterUserID {
		boardCreator, _ := s.userRepo.GetByID(board.CreatedBy)
		if boardCreator != nil && (boardCreator.UserType == models.UserTypeEmployer || boardCreator.IsManager) {
			employerIDs[board.CreatedBy] = true
		}
	}

	for empID := range employerIDs {
		if empID == updaterUserID {
			continue
		}

		err := s.notifSvc.CreateNotification(empID, "task_updated", "Tarea modificada", fmt.Sprintf("%s modificó la tarea: %s", updaterName, task.Title), map[string]interface{}{
			"task_id":  task.ID,
			"board_id": task.BoardID,
			"link":     taskDeepLink(task.ID, task.BoardID, task.TenantID),
		})
		if err != nil {
			log.Printf("[TaskService] Error creating task_updated notification for employer %d: %v", empID, err)
		}

	}

	// Notify other assignees who were already assigned or whose assignment is preserved
	for _, assignee := range task.Assignees {
		if assignee.ID == updaterUserID {
			continue
		}
		if assignees != nil && !currentAssigneeIDs[assignee.ID] {
			// Skip newly assigned users, since they already got task_assigned
			continue
		}

		err := s.notifSvc.CreateNotification(assignee.ID, "task_updated", "Tarea modificada", fmt.Sprintf("%s modificó la tarea: %s", updaterName, task.Title), map[string]interface{}{
			"task_id":  task.ID,
			"board_id": task.BoardID,
			"link":     taskDeepLink(task.ID, task.BoardID, task.TenantID),
		})
		if err != nil {
			log.Printf("[TaskService] Error creating task_updated notification for assignee %d: %v", assignee.ID, err)
		}

		if deadlineChanged {
			var dueMsg string
			if task.EndDate != nil {
				dueMsg = fmt.Sprintf("📅 Cambió la fecha de \"%s\": ahora vence %s", task.Title, task.EndDate.Format("02/01/2006"))
			} else {
				dueMsg = fmt.Sprintf("📅 La tarea \"%s\" ya no tiene fecha límite", task.Title)
			}
			s.sendSystemDM(assignee.ID, dueMsg)
		}
	}

	// Google Calendar: reconcilia eventos (cambios de fecha, reasignaciones y
	// desasignaciones se resuelven dentro del enganche comparando asignados
	// actuales contra los enlaces existentes).
	s.syncCalendarChanged(task.ID)

	// Automatizaciones. Se emite al final, con la tarea ya recargada: el snapshot
	// que viaja al motor tiene que ser el estado definitivo, no uno intermedio.
	var addedAssignees []uint
	if assignees != nil {
		for _, a := range task.Assignees {
			if !currentAssigneeIDs[a.ID] {
				addedAssignees = append(addedAssignees, a.ID)
			}
		}
	}
	s.emitTaskChange(task, oldStatus, oldPriority, addedAssignees, updaterUserID)

	// Puerta cruzada: además de los eventos del cambio, la regla que puso la puerta
	// recibe su propia ejecución con lo que la persona respondió. Es lo que convierte
	// una puerta en una decisión con consecuencia —aprobar cierra, rechazar devuelve—
	// y no sólo en un peaje. Va después del cambio porque la consecuencia se sigue de
	// él, y con la tarea ya recargada para que el snapshot cuadre con lo guardado.
	if gateResult != nil {
		s.emit(WorkflowEvent{
			Type:           models.TriggerTaskEnteringPhase,
			TenantID:       task.TenantID,
			Task:           task,
			PrevStatus:     oldStatus,
			ActorID:        updaterUserID,
			GateWorkflowID: gateResult.WorkflowID,
			GateAnswers:    gateResult.Data,
		})
	}

	return task, task.Assignees, nil
}

// Reorder persiste el orden manual de las tarjetas de una columna. Es un cambio
// puramente cosmético (no toca status, asignados ni contenido), así que basta
// con la autorización de tenant sobre el tablero: no se exige canModifyTask por
// tarjeta porque la columna puede contener tarjetas de otros miembros.
func (s *taskService) Reorder(boardID, tenantID uint, isSuperadmin bool, status string, orderedIDs []uint) error {
	if boardID == 0 || status == "" || len(orderedIDs) == 0 {
		return errors.New("Datos de reordenamiento incompletos")
	}
	if err := s.authorizeBoardTenant(boardID, tenantID, isSuperadmin); err != nil {
		return err
	}
	return s.repo.ReorderTasks(boardID, status, orderedIDs)
}

func (s *taskService) Delete(id uint, tenantID uint, userID uint, role string, isManager, isSuperadmin bool) error {
	task, err := s.authorizeTaskByID(id, tenantID, isSuperadmin)
	if err != nil {
		return err
	}
	if !isSuperadmin && !s.canModifyTask(task, userID, role, isManager) {
		return errors.New("Access denied")
	}
	// Delete related notifications
	_ = s.notifSvc.DeleteByTaskID(id)
	if err := s.repo.Delete(id); err != nil {
		return err
	}
	// Google Calendar: borra los eventos de la tarea en todos los calendarios.
	s.syncCalendarDeleted(id)
	return nil
}

func (s *taskService) ToggleCompletion(id uint, tenantID uint, updaterUserID uint, role string, isManager, isSuperadmin bool) (*models.Task, error) {
	task, err := s.authorizeTaskByID(id, tenantID, isSuperadmin)
	if err != nil {
		return nil, err
	}

	if !isSuperadmin && !s.canModifyTask(task, updaterUserID, role, isManager) {
		return nil, errors.New("Access denied")
	}

	completed := !task.Completed
	// Al reabrir, la tarea vuelve a la primera fase del tablero (no a un
	// "por_hacer" fijo que quizás ya no existe como columna).
	status := models.TaskStatusTodo
	if completed {
		status = models.TaskStatusDone
	} else if b, err := s.boardRepo.GetByID(task.BoardID); err == nil && len(b.Phases) > 0 {
		status = models.TaskStatus(PhaseColumnID(b.Phases[0]))
	}

	oldStatus := string(task.Status)

	// La puerta también vigila este camino. Sin esto, el botón "Finalizar tarea"
	// sería un atajo para entrar en una columna cerrada sin rellenar su formulario,
	// y una puerta con un agujero conocido no es una puerta.
	//
	// Aquí no hay forma de aportar el formulario —este endpoint no lo recibe—, así
	// que se devuelve el error tal cual: el cliente lo traduce en abrir el modal y
	// reintentar por PUT /tasks/:id, que sí lo admite.
	if string(status) != oldStatus {
		if _, gerr := s.checkGate(task, string(status), nil); gerr != nil {
			return nil, gerr
		}
	}

	updates := map[string]interface{}{
		"completed": completed,
		"status":    status,
	}
	// Completar y reabrir también son movimientos de columna, así que sellan la
	// entrada igual que una edición. Reabrir una tarea que ya estaba en la primera
	// fase no mueve nada: en ese caso no se toca el sello ni se anota bitácora.
	if string(status) != oldStatus {
		updates["status_changed_at"] = time.Now()
	}

	if err := s.repo.Update(task, updates); err != nil {
		return nil, err
	}

	s.recordStatusChange(task, oldStatus, string(status), updaterUserID)

	// Fetch final refreshed task with preloads
	finalTask, err := s.repo.GetByID(id)
	if err != nil {
		return task, nil
	}
	task = finalTask

	updaterName := "Alguien"
	var updater *models.User
	if updaterUserID > 0 {
		updater, _ = s.userRepo.GetByID(updaterUserID)
		if updater != nil {
			updaterName = updater.Name
		}
	}

	actionVerb := "reabrió"
	notifType := "task_updated"
	title := "Tarea reabierta"
	if completed {
		actionVerb = "completó"
		notifType = "task_completed"
		title = "Tarea completada"
	}

	// Employer IDs to notify
	board, _ := s.boardRepo.GetByID(task.BoardID)
	employerIDs := make(map[uint]bool)

	if updater != nil && updater.UserType == models.UserTypeProfessional && updater.EmpleadorID != nil {
		employerIDs[*updater.EmpleadorID] = true
	}

	if board != nil && board.CreatedBy != updaterUserID {
		boardCreator, _ := s.userRepo.GetByID(board.CreatedBy)
		if boardCreator != nil && (boardCreator.UserType == models.UserTypeEmployer || boardCreator.IsManager) {
			employerIDs[board.CreatedBy] = true
		}
	}

	for empID := range employerIDs {
		if empID == updaterUserID {
			continue
		}

		err := s.notifSvc.CreateNotification(empID, notifType, title, fmt.Sprintf("%s %s la tarea: %s", updaterName, actionVerb, task.Title), map[string]interface{}{
			"task_id":  task.ID,
			"board_id": task.BoardID,
			"link":     taskDeepLink(task.ID, task.BoardID, task.TenantID),
		})
		if err != nil {
			log.Printf("[TaskService] Error creating ToggleCompletion notification for employer %d: %v", empID, err)
		}

	}

	// Notify other assignees
	for _, assignee := range task.Assignees {
		if assignee.ID == updaterUserID {
			continue
		}

		err := s.notifSvc.CreateNotification(assignee.ID, notifType, title, fmt.Sprintf("%s %s la tarea: %s", updaterName, actionVerb, task.Title), map[string]interface{}{
			"task_id":  task.ID,
			"board_id": task.BoardID,
			"link":     taskDeepLink(task.ID, task.BoardID, task.TenantID),
		})
		if err != nil {
			log.Printf("[TaskService] Error creating ToggleCompletion notification for assignee %d: %v", assignee.ID, err)
		}

		// Solo al completar (no al reabrir): el DM de "✅ completada" es señal de
		// cierre; reabrir es un cambio menor que no amerita un mensaje.
		if completed {
			s.sendSystemDM(assignee.ID, fmt.Sprintf("✅ Se completó la tarea: %s", task.Title))
		}
	}

	// Google Calendar: refleja el ✓ (o lo quita al reabrir) en el título del
	// evento de cada asignado conectado.
	s.syncCalendarChanged(task.ID)

	// Automatizaciones. Completar y reabrir son cambios de columna como cualquier
	// otro: una regla sobre "llega a Finalizado" debe dispararse tanto si la
	// tarjeta se arrastró como si se pulsó el botón de finalizar.
	s.emitTaskChange(task, oldStatus, "", nil, updaterUserID)

	return task, nil
}

// ptrString evita que el JSON del formulario se pierda por el camino: la columna es
// jsonb y nulable, así que sólo las filas con puerta llevan valor.
func ptrString(v string) *string { return &v }
