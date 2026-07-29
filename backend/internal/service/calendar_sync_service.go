package service

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
)

// CalendarSyncService mantiene los eventos de Google Calendar en sincronía con
// las tareas de Obertrack. Es de una sola dirección (Obertrack → Google): al
// mutar una tarea se encola trabajo (rápido, dentro del request) y un worker en
// segundo plano lo aplica contra la API de Google con reintentos.
//
// Solo sincroniza tareas CON fecha para asignados que tengan su cuenta de Google
// conectada y activa; el resto se ignora en silencio.
type CalendarSyncService struct {
	syncRepo   repository.CalendarSyncRepository
	googleRepo repository.GoogleCalendarRepository
	taskRepo   repository.TaskRepository
	google     GoogleCalendarService

	// nudge despierta al worker en cuanto se encola algo, sin esperar al tick.
	// Con buffer 1 y envío no bloqueante: varias mutaciones seguidas colapsan en
	// un solo aviso, que es justo lo que se quiere.
	nudge chan struct{}
}

func NewCalendarSyncService(
	syncRepo repository.CalendarSyncRepository,
	googleRepo repository.GoogleCalendarRepository,
	taskRepo repository.TaskRepository,
	google GoogleCalendarService,
) *CalendarSyncService {
	return &CalendarSyncService{
		syncRepo:   syncRepo,
		googleRepo: googleRepo,
		taskRepo:   taskRepo,
		google:     google,
		nudge:      make(chan struct{}, 1),
	}
}

// Enabled indica si la integración está configurada. Cuando está apagada, los
// enganches de tareas no hacen nada.
func (s *CalendarSyncService) Enabled() bool {
	return s != nil && s.google != nil && s.google.Enabled()
}

// --- Enganches que llama el servicio de tareas (rápidos, solo encolan) ---

// OnTaskChanged reconcilia los eventos de una tarea tras crearla, editarla o
// completarla. Calcula el estado objetivo (qué asignados conectados deben tener
// evento) y lo compara con los enlaces existentes: encola upserts para los que
// deben tenerlo y borrados para los que dejaron de estar asignados o para cuando
// la tarea se quedó sin fecha. Best-effort: un error aquí se registra pero no
// rompe la operación de la tarea.
func (s *CalendarSyncService) OnTaskChanged(taskID uint) {
	if !s.Enabled() {
		return
	}
	if err := s.enqueueReconcile(taskID); err != nil {
		log.Printf("Calendar sync: no se pudo encolar la tarea %d: %v", taskID, err)
	}
	s.signal()
}

// OnTaskDeleted encola el borrado de todos los eventos de una tarea. Se llama
// tras eliminarla, cuando la tarea ya no es consultable pero sus enlaces siguen.
func (s *CalendarSyncService) OnTaskDeleted(taskID uint) {
	if !s.Enabled() {
		return
	}
	links, err := s.syncRepo.ListLinksForTask(taskID)
	if err != nil {
		log.Printf("Calendar sync: no se pudieron leer los enlaces de la tarea %d: %v", taskID, err)
		return
	}
	for _, link := range links {
		s.enqueueDelete(taskID, link.UserID, link.GoogleEventID, link.CalendarID)
	}
	if len(links) > 0 {
		s.signal()
	}
}

// OnAccountDisconnected limpia los enlaces de un usuario que acaba de desvincular
// su cuenta. Lo llama googleCalendarService.Disconnect por callback inyectado.
//
// Los eventos ya escritos SE QUEDAN en el calendario del usuario: la desconexión
// revoca el permiso primero, así que a partir de ese momento no hay credencial
// con la que borrarlos. Lo que sí desaparece es el enlace, que sin cuenta detrás
// solo afirma algo que ya no se puede comprobar ni mantener. La contrapartida es
// que si el usuario vuelve a conectar la MISMA cuenta, las tareas que se editen
// después generan un evento nuevo junto al que quedó huérfano; se prefiere eso a
// arrastrar enlaces que apuntan a credenciales que ya no existen.
func (s *CalendarSyncService) OnAccountDisconnected(userID uint) {
	if !s.Enabled() {
		return
	}
	if err := s.syncRepo.DeleteLinksForUser(userID); err != nil {
		log.Printf("Calendar sync: no se pudieron borrar los enlaces del usuario %d al desvincular: %v", userID, err)
	}
}

// enqueueReconcile calcula y encola la diferencia entre el estado deseado y los
// enlaces actuales de la tarea.
func (s *CalendarSyncService) enqueueReconcile(taskID uint) error {
	task, err := s.taskRepo.GetByID(taskID)
	if err != nil {
		// La tarea desapareció entre la mutación y este cálculo: trátalo como
		// borrado para no dejar eventos huérfanos.
		s.OnTaskDeleted(taskID)
		return nil
	}

	// Estado objetivo: si la tarea tiene fecha, cada asignado con cuenta conectada
	// y activa debe tener evento. Sin fecha, el objetivo es vacío (se borra todo).
	target := map[uint]bool{}
	if taskHasDate(task) {
		for _, assignee := range task.Assignees {
			if s.userHasActiveAccount(assignee.ID) {
				target[assignee.ID] = true
			}
		}
	}

	links, err := s.syncRepo.ListLinksForTask(taskID)
	if err != nil {
		return err
	}
	linkedUsers := map[uint]models.CalendarEventLink{}
	for _, link := range links {
		linkedUsers[link.UserID] = link
	}

	// Upsert para cada usuario objetivo.
	for userID := range target {
		s.enqueueUpsert(taskID, userID)
	}
	// Borrado para cada enlace cuyo usuario ya no está en el objetivo
	// (desasignado, o la tarea perdió la fecha, o revocó el acceso).
	for userID, link := range linkedUsers {
		if !target[userID] {
			s.enqueueDelete(taskID, userID, link.GoogleEventID, link.CalendarID)
		}
	}
	return nil
}

func (s *CalendarSyncService) enqueueUpsert(taskID, userID uint) {
	job := &models.CalendarSyncJob{
		TaskID: taskID,
		UserID: userID,
		Action: models.CalendarSyncActionUpsert,
		Status: models.CalendarSyncStatusPending,
	}
	if err := s.syncRepo.EnqueueJob(job); err != nil {
		log.Printf("Calendar sync: no se pudo encolar upsert (tarea %d, usuario %d): %v", taskID, userID, err)
		return
	}
	// Descarta jobs pendientes anteriores de la misma (tarea, usuario): solo
	// importa el último estado, no reproducir cada edición intermedia.
	_ = s.syncRepo.SupersedePendingJobs(taskID, userID, job.ID)
}

func (s *CalendarSyncService) enqueueDelete(taskID, userID uint, eventID, calendarID string) {
	// Sin evento que borrar (nunca se creó) no hay nada que hacer en Google; el
	// enlace, si existiera vacío, se limpia igualmente.
	job := &models.CalendarSyncJob{
		TaskID:        taskID,
		UserID:        userID,
		Action:        models.CalendarSyncActionDelete,
		GoogleEventID: eventID,
		CalendarID:    calendarID,
		Status:        models.CalendarSyncStatusPending,
	}
	if err := s.syncRepo.EnqueueJob(job); err != nil {
		log.Printf("Calendar sync: no se pudo encolar delete (tarea %d, usuario %d): %v", taskID, userID, err)
		return
	}
	_ = s.syncRepo.SupersedePendingJobs(taskID, userID, job.ID)
}

// --- Worker ---

// Start lanza el bucle del worker. Procesa por lotes cada 20s, o antes si una
// mutación lo despierta. Idempotente frente a reinicios: los jobs pendientes
// sobreviven en la BD.
func (s *CalendarSyncService) Start() {
	if !s.Enabled() {
		log.Println("Calendar sync: deshabilitado (integración de Google Calendar apagada)")
		return
	}
	go s.run()
	log.Println("Calendar sync: worker iniciado")
}

func (s *CalendarSyncService) run() {
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()
	for {
		s.processBatch()
		select {
		case <-ticker.C:
		case <-s.nudge:
		}
	}
}

func (s *CalendarSyncService) signal() {
	select {
	case s.nudge <- struct{}{}:
	default: // ya hay un aviso pendiente; no hace falta otro.
	}
}

// processBatch toma un lote de jobs pendientes ya vencidos y los procesa uno a
// uno. Los que están esperando su backoff quedan fuera de la selección y no
// frenan al resto.
func (s *CalendarSyncService) processBatch() {
	jobs, err := s.syncRepo.ClaimPendingJobs(50, time.Now())
	if err != nil {
		log.Printf("Calendar sync: no se pudieron leer jobs pendientes: %v", err)
		return
	}
	for i := range jobs {
		s.processJob(&jobs[i])
	}
}

func (s *CalendarSyncService) processJob(job *models.CalendarSyncJob) {
	var err error
	switch job.Action {
	case models.CalendarSyncActionUpsert:
		err = s.processUpsert(job)
	case models.CalendarSyncActionDelete:
		err = s.processDelete(job)
	default:
		err = fmt.Errorf("acción desconocida %q", job.Action)
	}

	if err == nil {
		if merr := s.syncRepo.MarkJobDone(job.ID); merr != nil {
			log.Printf("Calendar sync: no se pudo marcar job %d como hecho: %v", job.ID, merr)
		}
		return
	}

	attempts := job.Attempts + 1
	retryAt := retryAfterFailure(err, attempts)

	if merr := s.syncRepo.MarkJobFailed(job.ID, attempts, err.Error(), retryAt); merr != nil {
		log.Printf("Calendar sync: no se pudo marcar job %d como fallido: %v", job.ID, merr)
	}
	if retryAt == nil {
		log.Printf("Calendar sync: job %d (tarea %d, usuario %d) agotado tras %d intentos: %v",
			job.ID, job.TaskID, job.UserID, attempts, err)
		return
	}
	log.Printf("Calendar sync: job %d (tarea %d, usuario %d) falló en el intento %d, reintento en %s: %v",
		job.ID, job.TaskID, job.UserID, attempts, models.CalendarSyncRetryDelay(attempts), err)
}

// retryAfterFailure decide cuándo —o si— se reintenta un job. Devuelve nil para
// los fallos que no mejoran esperando, de forma que no gasten la ventana de
// reintentos ni cuota de la API:
//
//   - needs_reauth: hace falta que el usuario reconecte, no tiempo.
//   - rechazo permanente de Google (4xx que no sea 408/429): datos inválidos o
//     permiso denegado; repetir la misma petición da la misma respuesta.
//
// El resto —cortes de red, 429 por cuota, 5xx— son transitorios y sí escalan por
// la tabla de backoff hasta agotar CalendarSyncMaxAttempts.
func retryAfterFailure(err error, attempts int) *time.Time {
	if errors.Is(err, ErrNeedsReauth) || errors.Is(err, ErrGooglePermanent) {
		return nil
	}
	if attempts >= models.CalendarSyncMaxAttempts {
		return nil
	}
	at := time.Now().Add(models.CalendarSyncRetryDelay(attempts))
	return &at
}

func (s *CalendarSyncService) processUpsert(job *models.CalendarSyncJob) error {
	task, err := s.taskRepo.GetByID(job.TaskID)
	if err != nil {
		// La tarea ya no existe: nada que sincronizar. El borrado se encoló por
		// su propia vía; este upsert quedó obsoleto.
		return nil
	}
	if !taskHasDate(task) {
		return nil // perdió la fecha; el delete correspondiente ya se encoló.
	}

	account, err := s.googleRepo.GetByUser(job.UserID)
	if errors.Is(err, repository.ErrGoogleAccountNotFound) {
		return nil // desvinculó su cuenta; no hay dónde escribir.
	}
	if err != nil {
		return err
	}
	if account.Status == models.GoogleCalStatusNeedsReauth {
		return ErrNeedsReauth
	}

	input := buildTaskEventInput(task)

	link, err := s.syncRepo.GetLink(job.TaskID, job.UserID)
	if errors.Is(err, repository.ErrCalendarLinkNotFound) {
		return s.createAndLink(job.TaskID, job.UserID, account.CalendarID, input)
	}
	if err != nil {
		return err
	}

	// Ya existe evento: se actualiza. Si el usuario lo borró a mano en Google, se
	// vuelve a crear para no dejar la tarea sin reflejo.
	err = s.google.UpdateEvent(job.UserID, link.CalendarID, link.GoogleEventID, input)
	if errors.Is(err, ErrEventGone) {
		return s.createAndLink(job.TaskID, job.UserID, account.CalendarID, input)
	}
	return err
}

func (s *CalendarSyncService) createAndLink(taskID, userID uint, calendarID string, input CalendarEventInput) error {
	event, err := s.google.CreateEvent(userID, calendarID, input)
	if err != nil {
		return err
	}
	return s.syncRepo.UpsertLink(&models.CalendarEventLink{
		TaskID:        taskID,
		UserID:        userID,
		GoogleEventID: event.ID,
		CalendarID:    calendarID,
	})
}

func (s *CalendarSyncService) processDelete(job *models.CalendarSyncJob) error {
	if job.GoogleEventID != "" {
		err := s.google.DeleteEvent(job.UserID, job.CalendarID, job.GoogleEventID)
		// Sin cuenta vinculada no hay credencial con la que borrar nada en Google,
		// y no la va a haber por reintentar: el usuario desvinculó. Se sigue
		// adelante para limpiar el enlace. Sin esto, cada edición posterior de una
		// tarea que ese usuario tuvo sincronizada encolaba un delete que gastaba
		// sus cinco intentos y dejaba el enlace vivo para repetirlo en la
		// siguiente edición, para siempre.
		if err != nil && !errors.Is(err, repository.ErrGoogleAccountNotFound) {
			return err
		}
	}
	// Limpia el enlace pase lo que pase con Google (idempotente): el estado
	// deseado es "sin evento ni enlace".
	if err := s.syncRepo.DeleteLink(job.TaskID, job.UserID); err != nil {
		return err
	}
	return nil
}

// --- Helpers ---

func (s *CalendarSyncService) userHasActiveAccount(userID uint) bool {
	account, err := s.googleRepo.GetByUser(userID)
	if err != nil {
		return false
	}
	return account.Status == models.GoogleCalStatusActive
}

func taskHasDate(task *models.Task) bool {
	return task != nil && (task.StartDate != nil || task.EndDate != nil)
}

// buildTaskEventInput traduce una tarea a un evento de día completo. Usa la fecha
// de inicio si existe; si no, la de fin (una tarea con solo "vence" es un evento
// de ese día). El título lleva ✓ cuando está completada, para que se distinga de
// un vistazo en el calendario.
func buildTaskEventInput(task *models.Task) CalendarEventInput {
	var start, end time.Time
	if task.StartDate != nil {
		start = *task.StartDate
	}
	if task.EndDate != nil {
		end = *task.EndDate
	}
	if start.IsZero() {
		start = end
	}
	if end.IsZero() {
		end = start
	}

	summary := strings.TrimSpace(task.Title)
	if task.Completed {
		summary = "✓ " + summary
	}

	description := strings.TrimSpace(task.Description)
	descParts := []string{}
	if description != "" {
		descParts = append(descParts, description)
	}
	descParts = append(descParts, "— Sincronizado desde Obertrack")

	return CalendarEventInput{
		Summary:     summary,
		Description: strings.Join(descParts, "\n\n"),
		StartDate:   start,
		EndDate:     end,
	}
}
