// Command demoseed prepara un tablero de demostración con automatizaciones ya
// activas y un historial de ejecuciones con el que enseñar el módulo.
//
// La actividad NO se inventa: se genera pasando por los servicios reales
// (taskService → emisor → workflowService → worker), así que cada fila del historial
// corresponde a una ejecución que de verdad ocurrió, con sus pasos, sus motivos y
// sus destinatarios resueltos como en producción. Lo único que se retoca después son
// las fechas, para que la bitácora no parezca creada toda en el mismo minuto.
//
//	go run ./cmd/demoseed -tenant 2            # sembrar
//	go run ./cmd/demoseed -tenant 2 -reset     # borrar lo sembrado y volver a sembrar
//	go run ./cmd/demoseed -tenant 2 -reset-only
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
	"github.com/obertrack/backend/internal/service"
)

// boardName identifica todo lo sembrado. El borrado cuelga de aquí: si cambias el
// nombre, cambia también el de -reset o quedará huérfano lo anterior.
const boardName = "Lanzamiento Q3"

func main() {
	tenantID := flag.Uint("tenant", 2, "empresa (tenant) donde sembrar")
	reset := flag.Bool("reset", false, "borrar lo sembrado antes de volver a sembrar")
	resetOnly := flag.Bool("reset-only", false, "sólo borrar lo sembrado")
	flag.Parse()

	db := connect()

	if *reset || *resetOnly {
		if err := purge(db, uint(*tenantID)); err != nil {
			log.Fatalf("borrando lo sembrado: %v", err)
		}
		fmt.Println("✓ demo anterior eliminada")
		if *resetOnly {
			return
		}
	}

	if err := seed(db, uint(*tenantID)); err != nil {
		log.Fatalf("sembrando: %v", err)
	}
}

func connect() *gorm.DB {
	for _, p := range []string{".env", "../.env", "../../.env"} {
		if _, err := os.Stat(p); err == nil {
			_ = godotenv.Load(p)
			break
		}
	}
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=%s",
		env("DB_HOST", "localhost"), env("DB_USER", "postgres"), env("DB_PASSWORD", ""),
		env("DB_NAME", "obertrack"), env("DB_PORT", "5432"), env("DB_SSL_MODE", "disable"))

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		// Silencioso: la salida útil es la del propio seeder, no cien SELECTs.
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		log.Fatalf("conectando a la base: %v", err)
	}
	return db
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// ---------------------------------------------------------------------------
// Siembra
// ---------------------------------------------------------------------------

type rig struct {
	db        *gorm.DB
	tenantID  uint
	ownerID   uint
	taskSvc   service.TaskService
	workflow  *service.WorkflowService
	boardRepo repository.BoardRepository
}

func seed(db *gorm.DB, tenantID uint) error {
	owner, err := employerOf(db, tenantID)
	if err != nil {
		return err
	}

	r := buildRig(db, tenantID, owner.ID)

	board, err := r.ensureBoard()
	if err != nil {
		return err
	}
	fmt.Printf("✓ tablero %q (id %d)\n", board.Name, board.ID)

	team, err := r.team(board)
	if err != nil {
		return err
	}
	fmt.Printf("✓ equipo: %d personas (%d con manager)\n", len(team.all), len(team.withManager))

	if err := r.backlog(board, team); err != nil {
		return err
	}

	if err := r.enableRecipes(board); err != nil {
		return err
	}
	fmt.Println("✓ 4 automatizaciones activas")

	// El worker se arranca ANTES de mover nada: así cada operación se procesa como
	// en producción, con su encolado y su ejecución, en vez de a posteriori.
	r.workflow.Start()

	if err := r.timeline(board, team); err != nil {
		return err
	}

	if err := r.addFailedExample(board); err != nil {
		return err
	}

	if err := r.spreadOverTime(board); err != nil {
		return err
	}

	return r.report(board)
}

func buildRig(db *gorm.DB, tenantID, ownerID uint) *rig {
	userRepo := repository.NewUserRepository(db)
	boardRepo := repository.NewBoardRepository(db)
	taskRepo := repository.NewTaskRepository(db)
	empRepo := repository.NewEmploymentRepository(db)
	channelRepo := repository.NewChannelRepository(db)

	notifSvc := service.NewNotificationService(repository.NewNotificationRepository(db), nil)
	channelSvc := service.NewChannelService(channelRepo, userRepo, notifSvc)
	taskSvc := service.NewTaskService(taskRepo, userRepo, boardRepo, notifSvc)
	taskSvc.SetSystemDM(channelSvc.PostSystemDM)

	wf := service.NewWorkflowService(
		repository.NewWorkflowRepository(db), taskRepo, boardRepo, userRepo, empRepo,
		notifSvc, nil,
	)
	wf.SetSystemDM(channelSvc.PostSystemDM)
	taskSvc.SetWorkflowEmitter(wf.OnEvent)

	return &rig{db: db, tenantID: tenantID, ownerID: ownerID, taskSvc: taskSvc, workflow: wf, boardRepo: boardRepo}
}

func employerOf(db *gorm.DB, tenantID uint) (*models.User, error) {
	var u models.User
	if err := db.First(&u, tenantID).Error; err != nil {
		return nil, fmt.Errorf("no existe la empresa %d: %w", tenantID, err)
	}
	if u.UserType != models.UserTypeEmployer {
		return nil, fmt.Errorf("el usuario %d no es una cuenta de empresa", tenantID)
	}
	return &u, nil
}

func (r *rig) ensureBoard() (*models.Board, error) {
	var board models.Board
	err := r.db.Where("name = ? AND tenant_id = ?", boardName, r.tenantID).First(&board).Error
	if err == nil {
		return &board, nil
	}

	board = models.Board{
		Name:        boardName,
		Description: "Tablero de demostración del módulo de automatizaciones.",
		Color:       "#cc33cc",
		CreatedBy:   r.ownerID,
		TenantID:    r.tenantID,
	}
	if err := r.db.Create(&board).Error; err != nil {
		return nil, err
	}

	// Mismas fases que abre board_service desde la UI. La cuarta es custom, para
	// enseñar que las reglas funcionan igual sobre una columna propia.
	phases := []struct{ name, color, status string }{
		{"Por hacer", "#6b7280", string(models.TaskStatusTodo)},
		{"En proceso", "#3b82f6", string(models.TaskStatusInProcess)},
		{"En revisión", "#f59e0b", ""},
		{"Finalizado", "#22c55e", string(models.TaskStatusDone)},
	}
	for i, p := range phases {
		phase := models.Phase{Name: p.name, Color: p.color, Status: p.status, Order: i}
		if err := r.db.Create(&phase).Error; err != nil {
			return nil, err
		}
		if err := r.db.Create(&models.BoardPhase{BoardID: board.ID, PhaseID: phase.ID}).Error; err != nil {
			return nil, err
		}
	}
	return &board, nil
}

type demoTeam struct {
	all         []models.User
	withManager []models.User // disparan el aviso al manager
	noManager   []models.User // producen "sin efecto" con motivo
}

// team elige gente real del tenant y la mete en el tablero. Se buscan a propósito
// las dos clases: quien tiene manager hace que el aviso al manager llegue, y quien
// no lo tiene produce un "sin efecto" explicado, que es justo lo que hay que poder
// enseñar de la bitácora.
func (r *rig) team(board *models.Board) (*demoTeam, error) {
	var users []models.User
	err := r.db.Where("empleador_id = ? AND is_active = ? AND is_system = ?", r.tenantID, true, false).
		Where("user_type = ?", models.UserTypeProfessional).
		Order("manager_id IS NULL, id").Limit(6).Find(&users).Error
	if err != nil {
		return nil, err
	}
	if len(users) == 0 {
		return nil, fmt.Errorf("la empresa %d no tiene profesionales activos que asignar", r.tenantID)
	}

	t := &demoTeam{all: users}
	for _, u := range users {
		if u.ManagerID != nil {
			t.withManager = append(t.withManager, u)
		} else {
			t.noManager = append(t.noManager, u)
		}
	}

	// El dueño entra siempre: es quien presenta y quien recibe los avisos de
	// "creador del tablero".
	members := append([]models.User{{ID: r.ownerID}}, users...)
	for _, m := range members {
		if err := r.db.Clauses(clause.OnConflict{DoNothing: true}).
			Create(&models.BoardMember{BoardID: board.ID, UserID: m.ID}).Error; err != nil {
			return nil, err
		}
	}
	return t, nil
}

// backlog crea las tarjetas que "ya estaban" antes de encender las automatizaciones.
// Van directas por GORM y no por taskService justamente para que NO disparen nada:
// un tablero recién encendido tiene historia previa, y esa historia no generó avisos.
func (r *rig) backlog(board *models.Board, team *demoTeam) error {
	now := time.Now()
	specs := []struct {
		title    string
		status   models.TaskStatus
		priority models.TaskPriority
		assignee *uint
		endDays  int
	}{
		{"Migrar la pasarela de pagos", models.TaskStatusInProcess, models.PriorityHigh, pick(team.all, 0), 5},
		{"Rediseñar el onboarding de clientes", models.TaskStatusTodo, models.PriorityMedium, pick(team.all, 1), 12},
		{"Auditoría de accesibilidad", "en_revisión", models.PriorityMedium, pick(team.all, 2), 3},
		{"Documentar la API pública", models.TaskStatusTodo, models.PriorityLow, nil, 20},
		{"Cerrar incidencias de la release 2.4", models.TaskStatusDone, models.PriorityHigh, pick(team.all, 0), -2},
		{"Preparar informe trimestral", models.TaskStatusInProcess, models.PriorityMedium, pick(team.all, 3), 8},
	}

	for i, s := range specs {
		var existing models.Task
		if err := r.db.Where("board_id = ? AND title = ?", board.ID, s.title).First(&existing).Error; err == nil {
			continue
		}
		end := now.AddDate(0, 0, s.endDays)
		task := models.Task{
			Title: s.title, Status: s.status, Priority: s.priority,
			EndDate: &end, Completed: s.status == models.TaskStatusDone,
			CreatedBy: r.ownerID, BoardID: board.ID, TenantID: board.TenantID,
			Order: i, StatusChangedAt: ptr(now.AddDate(0, 0, -(i + 2))),
		}
		if err := r.db.Create(&task).Error; err != nil {
			return err
		}
		if s.assignee != nil {
			if err := r.db.Clauses(clause.OnConflict{DoNothing: true}).
				Create(&models.TaskUser{TaskID: task.ID, UserID: *s.assignee}).Error; err != nil {
				return err
			}
		}
	}
	fmt.Printf("✓ %d tarjetas de contexto\n", len(specs))
	return nil
}

func (r *rig) enableRecipes(board *models.Board) error {
	actor := service.WorkflowActor{
		UserID: r.ownerID, TenantID: r.tenantID, IsEmployer: true,
	}
	for _, key := range []string{
		"en_proceso_sin_responsable", "prioridad_urgente",
		"asignacion_por_chat", "creada_sin_fecha",
	} {
		if _, err := r.workflow.SetRecipeEnabled(actor, board.ID, key, true, 0); err != nil {
			return fmt.Errorf("activando %q: %w", key, err)
		}
	}
	return nil
}

// timeline reproduce una semana de trabajo normal sobre el tablero. Cada operación
// pasa por taskService, así que dispara los mismos eventos que dispararía una
// persona usando la aplicación.
func (r *rig) timeline(board *models.Board, team *demoTeam) error {
	role := string(models.UserTypeEmployer)

	// 1. Tarea nueva sin fecha de fin y con responsable: dispara "creada sin fecha"
	//    y el aviso por chat al asignado.
	if len(team.all) > 0 {
		if _, _, err := r.taskSvc.Create(r.ownerID, false, r.tenantID,
			"Revisar contrato del proveedor", "Pendiente de validar cláusulas de SLA.",
			string(models.PriorityMedium), nil, []uint{team.all[0].ID}, board.ID); err != nil {
			return err
		}
	}

	// 2. Tarea sin responsable que entra en "En proceso": dispara el aviso al líder
	//    del proyecto. Es el caso que más se pide y el que peor se detecta a ojo.
	huerfana, _, err := r.taskSvc.Create(r.ownerID, false, r.tenantID,
		"Actualizar dependencias críticas", "Nadie asignado todavía.",
		string(models.PriorityMedium), nil, nil, board.ID)
	if err != nil {
		return err
	}
	if _, _, err := r.taskSvc.Update(huerfana.ID, r.tenantID, r.ownerID, role, false, false,
		map[string]interface{}{"status": string(models.TaskStatusInProcess)}, nil, nil); err != nil {
		return err
	}

	// 3. Urgente sobre alguien CON manager: los dos pasos del aviso se completan.
	if len(team.withManager) > 0 {
		t, _, cerr := r.taskSvc.Create(r.ownerID, false, r.tenantID,
			"Caída intermitente en el login", "Reportado por dos clientes.",
			string(models.PriorityMedium), ptrStr(time.Now().AddDate(0, 0, 2).Format("2006-01-02")),
			[]uint{team.withManager[0].ID}, board.ID)
		if cerr != nil {
			return cerr
		}
		if _, _, err := r.taskSvc.Update(t.ID, r.tenantID, r.ownerID, role, false, false,
			map[string]interface{}{"priority": string(models.PriorityUrgent)}, nil, nil); err != nil {
			return err
		}
	}

	// 4. Urgente sobre alguien SIN manager: el primer paso avisa, el segundo queda
	//    "sin efecto" explicando que no hay manager. Es la prueba de que el motor
	//    dice por qué no hizo algo en vez de callar.
	if len(team.noManager) > 0 {
		t, _, cerr := r.taskSvc.Create(r.ownerID, false, r.tenantID,
			"Ajustar límites de la API", "Throttling demasiado agresivo.",
			string(models.PriorityMedium), nil, []uint{team.noManager[0].ID}, board.ID)
		if cerr != nil {
			return cerr
		}
		if _, _, err := r.taskSvc.Update(t.ID, r.tenantID, r.ownerID, role, false, false,
			map[string]interface{}{"priority": string(models.PriorityUrgent)}, nil, nil); err != nil {
			return err
		}
	}

	// 5. Reasignación sobre una tarjeta del backlog: dispara el aviso por chat.
	if len(team.all) > 1 {
		var t models.Task
		if err := r.db.Where("board_id = ? AND title = ?", board.ID, "Documentar la API pública").
			First(&t).Error; err == nil {
			assignees := []uint{team.all[1].ID}
			if _, _, err := r.taskSvc.Update(t.ID, r.tenantID, r.ownerID, role, false, false,
				map[string]interface{}{}, &assignees, nil); err != nil {
				return err
			}
		}
	}

	// 6. Un par de movimientos de columna que NO cumplen condiciones: dejan
	//    ejecuciones "sin efecto" y enseñan que una regla activa no dispara siempre.
	var conResponsable models.Task
	if err := r.db.Where("board_id = ? AND title = ?", board.ID, "Rediseñar el onboarding de clientes").
		First(&conResponsable).Error; err == nil {
		if _, _, err := r.taskSvc.Update(conResponsable.ID, r.tenantID, r.ownerID, role, false, false,
			map[string]interface{}{"status": string(models.TaskStatusInProcess)}, nil, nil); err != nil {
			return err
		}
	}

	// El worker despierta con cada encolado; se le da margen para vaciar la cola.
	fmt.Print("… ejecutando automatizaciones")
	for i := 0; i < 10; i++ {
		time.Sleep(700 * time.Millisecond)
		fmt.Print(".")
		var pending int64
		r.db.Model(&models.WorkflowRun{}).
			Where("tenant_id = ? AND status IN ?", r.tenantID,
				[]string{models.WorkflowRunPending, models.WorkflowRunRunning}).
			Count(&pending)
		if pending == 0 {
			break
		}
	}
	fmt.Println(" listo")
	return nil
}

// addFailedExample es LO ÚNICO FABRICADO de toda la siembra: una ejecución fallida
// con su error y sus reintentos agotados.
//
// Va aparte y bien señalado porque no se puede provocar de verdad sin romper algo
// (haría falta que Brevo o la base fallaran en mitad de un paso), y aun así hay que
// poder enseñar el camino de diagnóstico: qué se ve cuando una automatización no
// consigue entregar. La fila tiene la misma forma que produciría el motor —estado
// 'failed', intentos agotados, error del paso— para que lo que se enseña sea
// exactamente lo que se vería en producción.
func (r *rig) addFailedExample(board *models.Board) error {
	var wf models.Workflow
	if err := r.db.Where("board_id = ? AND recipe_key = ?", board.ID, "prioridad_urgente").
		First(&wf).Error; err != nil {
		return nil // sin esa regla no hay dónde colgarlo; no es motivo para fallar
	}

	var already int64
	r.db.Model(&models.WorkflowRun{}).
		Where("workflow_id = ? AND status = ?", wf.ID, models.WorkflowRunFailed).Count(&already)
	if already > 0 {
		return nil
	}

	var task models.Task
	if err := r.db.Where("board_id = ?", board.ID).Order("id").First(&task).Error; err != nil {
		return nil
	}

	run := models.WorkflowRun{
		WorkflowID: wf.ID, TenantID: r.tenantID,
		DedupKey:   fmt.Sprintf("demo-fallo-%d", wf.ID),
		EntityType: "task", EntityID: task.ID,
		Context:  "{}",
		Status:   models.WorkflowRunFailed,
		Attempts: models.WorkflowMaxAttempts,
		LastError: "notificando al usuario " + fmt.Sprint(r.ownerID) +
			": context deadline exceeded",
	}
	if err := r.db.Create(&run).Error; err != nil {
		return err
	}

	var step models.WorkflowStep
	if err := r.db.Where("workflow_id = ?", wf.ID).Order(`"order"`).First(&step).Error; err == nil {
		if err := r.db.Create(&models.WorkflowStepRun{
			RunID: run.ID, StepID: step.ID, Order: step.Order,
			Status: models.WorkflowStepFailed, Output: "{}",
			Error: run.LastError,
		}).Error; err != nil {
			return err
		}
	}
	return nil
}

// spreadOverTime reparte las ejecuciones por los últimos días. Sin esto la bitácora
// aparece entera con la misma hora y se nota que se sembró de golpe; lo único que se
// retoca son las marcas de tiempo, nunca el resultado.
func (r *rig) spreadOverTime(board *models.Board) error {
	var runs []models.WorkflowRun
	if err := r.db.
		Joins("JOIN workflows w ON w.id = workflow_runs.workflow_id").
		Where("w.board_id = ?", board.ID).
		Order("workflow_runs.id").Find(&runs).Error; err != nil {
		return err
	}

	now := time.Now()
	for i, run := range runs {
		// De hace seis días hasta hace un rato, en orden.
		offset := time.Duration(len(runs)-i) * 22 * time.Hour
		at := now.Add(-offset).Add(time.Duration(i*37) * time.Minute)
		if err := r.db.Model(&models.WorkflowRun{}).Where("id = ?", run.ID).
			UpdateColumns(map[string]interface{}{
				"created_at": at, "started_at": at, "finished_at": at.Add(2 * time.Second),
			}).Error; err != nil {
			return err
		}
		if err := r.db.Model(&models.WorkflowStepRun{}).Where("run_id = ?", run.ID).
			UpdateColumn("created_at", at.Add(time.Second)).Error; err != nil {
			return err
		}
	}
	return nil
}

func (r *rig) report(board *models.Board) error {
	type row struct {
		RecipeKey string
		Status    string
		N         int
	}
	var rows []row
	if err := r.db.Raw(`
		SELECT w.recipe_key, r.status, count(*) AS n
		FROM workflow_runs r JOIN workflows w ON w.id = r.workflow_id
		WHERE w.board_id = ? GROUP BY 1,2 ORDER BY 1,2`, board.ID).Scan(&rows).Error; err != nil {
		return err
	}

	fmt.Println("\n── Historial generado ──")
	for _, x := range rows {
		fmt.Printf("  %-28s %-8s %d\n", x.RecipeKey, x.Status, x.N)
	}

	var skipped []string
	r.db.Raw(`
		SELECT DISTINCT sr.error FROM workflow_step_runs sr
		JOIN workflow_runs r ON r.id = sr.run_id
		JOIN workflows w ON w.id = r.workflow_id
		WHERE w.board_id = ? AND sr.status = 'skipped' AND sr.error <> ''`, board.ID).Scan(&skipped)
	if len(skipped) > 0 {
		fmt.Println("\n  Motivos de 'sin efecto' que se pueden enseñar:")
		for _, s := range skipped {
			fmt.Printf("    · %s\n", s)
		}
	}
	fmt.Printf("\n✓ Listo. Entra en Automatizaciones y elige el tablero %q.\n", board.Name)
	return nil
}

// ---------------------------------------------------------------------------
// Borrado
// ---------------------------------------------------------------------------

// purge deja el tenant como estaba. Todo lo sembrado cuelga del tablero, así que el
// borrado se ancla ahí y no puede alcanzar nada ajeno.
func purge(db *gorm.DB, tenantID uint) error {
	var board models.Board
	if err := db.Where("name = ? AND tenant_id = ?", boardName, tenantID).First(&board).Error; err != nil {
		return nil // no había nada sembrado
	}

	return db.Transaction(func(tx *gorm.DB) error {
		var taskIDs []uint
		tx.Model(&models.Task{}).Where("board_id = ?", board.ID).Pluck("id", &taskIDs)

		if len(taskIDs) > 0 {
			for _, q := range []string{
				"DELETE FROM task_users WHERE task_id IN ?",
				"DELETE FROM comments WHERE task_id IN ?",
				"DELETE FROM task_attachments WHERE task_id IN ?",
				"DELETE FROM task_status_history WHERE task_id IN ?",
			} {
				if err := tx.Exec(q, taskIDs).Error; err != nil {
					return err
				}
			}
			// Las notificaciones se localizan por el task_id de su payload, igual
			// que hace notificationRepository (con el cast a text que la columna
			// json exige).
			for _, id := range taskIDs {
				if err := tx.Exec(
					`DELETE FROM notifications WHERE data::text LIKE ?`,
					fmt.Sprintf("%%\"task_id\":%d%%", id)).Error; err != nil {
					return err
				}
			}
			if err := tx.Exec("DELETE FROM tasks WHERE id IN ?", taskIDs).Error; err != nil {
				return err
			}
		}

		if err := tx.Exec(`
			DELETE FROM workflow_step_runs WHERE run_id IN (
				SELECT r.id FROM workflow_runs r JOIN workflows w ON w.id = r.workflow_id
				WHERE w.board_id = ?)`, board.ID).Error; err != nil {
			return err
		}
		for _, q := range []string{
			"DELETE FROM workflow_runs WHERE workflow_id IN (SELECT id FROM workflows WHERE board_id = ?)",
			"DELETE FROM workflow_steps WHERE workflow_id IN (SELECT id FROM workflows WHERE board_id = ?)",
			"DELETE FROM workflows WHERE board_id = ?",
			"DELETE FROM board_members WHERE board_id = ?",
		} {
			if err := tx.Exec(q, board.ID).Error; err != nil {
				return err
			}
		}

		// Las fases van al final y en dos tiempos: board_phases tiene una clave
		// foránea física hacia phases, así que hay que anotarse los ids ANTES de
		// romper el vínculo y borrar la puente antes que las filas a las que apunta.
		var phaseIDs []uint
		tx.Table("board_phases").Where("board_id = ?", board.ID).Pluck("phase_id", &phaseIDs)
		if err := tx.Exec("DELETE FROM board_phases WHERE board_id = ?", board.ID).Error; err != nil {
			return err
		}
		if len(phaseIDs) > 0 {
			// Sólo las que no comparte ningún otro tablero: el modelo lo permite
			// aunque hoy no ocurra, y borrar una fase viva rompería ese tablero.
			if err := tx.Exec(`
				DELETE FROM phases WHERE id IN ?
				  AND id NOT IN (SELECT phase_id FROM board_phases)`, phaseIDs).Error; err != nil {
				return err
			}
		}
		return tx.Exec("DELETE FROM boards WHERE id = ?", board.ID).Error
	})
}

func pick(users []models.User, i int) *uint {
	if i >= len(users) {
		return nil
	}
	return &users[i].ID
}

func ptr(t time.Time) *time.Time { return &t }
func ptrStr(s string) *string    { return &s }
