package service

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
)

func dateP(y int, m time.Month, d int) *time.Time {
	t := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	return &t
}

// El 'end.date' de un evento all-day de Google es EXCLUSIVO: una tarea de un solo
// día (start==end) debe ir de ese día al siguiente, o Google la muestra sin
// duración / en el día equivocado.
func TestBuildEventPayloadSingleDay(t *testing.T) {
	ev := CalendarEventInput{
		Summary:   "Tarea",
		StartDate: time.Date(2026, 7, 23, 0, 0, 0, 0, time.UTC),
		EndDate:   time.Date(2026, 7, 23, 0, 0, 0, 0, time.UTC),
	}
	p := buildEventPayload(ev)
	if p.Start.Date != "2026-07-23" {
		t.Errorf("start = %q, se esperaba 2026-07-23", p.Start.Date)
	}
	if p.End.Date != "2026-07-24" {
		t.Errorf("end = %q, se esperaba 2026-07-24 (exclusivo)", p.End.Date)
	}
}

func TestBuildEventPayloadMultiDay(t *testing.T) {
	ev := CalendarEventInput{
		StartDate: time.Date(2026, 7, 23, 0, 0, 0, 0, time.UTC),
		EndDate:   time.Date(2026, 7, 25, 0, 0, 0, 0, time.UTC),
	}
	p := buildEventPayload(ev)
	if p.Start.Date != "2026-07-23" || p.End.Date != "2026-07-26" {
		t.Errorf("rango = %s..%s, se esperaba 2026-07-23..2026-07-26", p.Start.Date, p.End.Date)
	}
}

// Un end anterior al start (dato inconsistente) no debe producir un rango
// invertido que Google rechace: se colapsa a un solo día.
func TestBuildEventPayloadEndBeforeStart(t *testing.T) {
	ev := CalendarEventInput{
		StartDate: time.Date(2026, 7, 23, 0, 0, 0, 0, time.UTC),
		EndDate:   time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC),
	}
	p := buildEventPayload(ev)
	if p.Start.Date != "2026-07-23" || p.End.Date != "2026-07-24" {
		t.Errorf("rango = %s..%s, se esperaba colapsar a 2026-07-23..2026-07-24", p.Start.Date, p.End.Date)
	}
}

func TestTaskHasDate(t *testing.T) {
	if taskHasDate(&models.Task{}) {
		t.Error("una tarea sin fechas no tiene fecha")
	}
	if !taskHasDate(&models.Task{StartDate: dateP(2026, 7, 23)}) {
		t.Error("una tarea con start_date sí tiene fecha")
	}
	if !taskHasDate(&models.Task{EndDate: dateP(2026, 7, 23)}) {
		t.Error("una tarea con solo end_date (vence) sí tiene fecha")
	}
	if taskHasDate(nil) {
		t.Error("nil no tiene fecha")
	}
}

// Una tarea con solo fecha de vencimiento debe generar un evento en ese día
// (no en el año cero por un start vacío).
func TestBuildTaskEventInputOnlyEndDate(t *testing.T) {
	task := &models.Task{
		Title:   "Entregar informe",
		EndDate: dateP(2026, 7, 30),
	}
	in := buildTaskEventInput(task)
	if in.StartDate.Format("2006-01-02") != "2026-07-30" {
		t.Errorf("start = %s, se esperaba 2026-07-30 (cae al end)", in.StartDate.Format("2006-01-02"))
	}
	if in.EndDate.Format("2006-01-02") != "2026-07-30" {
		t.Errorf("end = %s, se esperaba 2026-07-30", in.EndDate.Format("2006-01-02"))
	}
}

// El título completado lleva ✓ para distinguirlo de un vistazo en el calendario.
func TestBuildTaskEventInputCompletedPrefix(t *testing.T) {
	task := &models.Task{
		Title:     "Revisar PR",
		Completed: true,
		EndDate:   dateP(2026, 7, 23),
	}
	in := buildTaskEventInput(task)
	if in.Summary != "✓ Revisar PR" {
		t.Errorf("summary = %q, se esperaba con prefijo ✓", in.Summary)
	}

	task.Completed = false
	if got := buildTaskEventInput(task).Summary; got != "Revisar PR" {
		t.Errorf("summary sin completar = %q, no debe llevar ✓", got)
	}
}

// La descripción siempre lleva la firma de Obertrack; si la tarea trae texto, va
// antes, separado.
func TestBuildTaskEventInputDescription(t *testing.T) {
	withText := buildTaskEventInput(&models.Task{Title: "T", Description: "Detalle", EndDate: dateP(2026, 7, 23)})
	if withText.Description != "Detalle\n\n— Sincronizado desde Obertrack" {
		t.Errorf("descripción con texto inesperada: %q", withText.Description)
	}
	noText := buildTaskEventInput(&models.Task{Title: "T", EndDate: dateP(2026, 7, 23)})
	if noText.Description != "— Sincronizado desde Obertrack" {
		t.Errorf("descripción sin texto inesperada: %q", noText.Description)
	}
}

// El calendario destino cae a 'primary' cuando no se especifica, y el path se
// arma bien para ids con caracteres especiales (correos).
func TestCalendarEventsURL(t *testing.T) {
	if got := calendarEventsURL(""); got != "https://www.googleapis.com/calendar/v3/calendars/primary/events" {
		t.Errorf("URL con calendario vacío = %q", got)
	}
	got := calendarEventsURL("user@example.com")
	want := "https://www.googleapis.com/calendar/v3/calendars/user@example.com/events"
	if got != want {
		t.Errorf("URL = %q, se esperaba %q", got, want)
	}
}

// --- Dobles de prueba del worker ---

// fakeSyncRepo registra las llamadas que importan para comprobar la limpieza de
// enlaces; el resto de métodos son inertes.
type fakeSyncRepo struct {
	deletedLinks     [][2]uint
	deletedForUser   []uint
	deleteLinksErr   error
	deleteForUserErr error

	doneJobs   []uint
	failedJobs []failedJob
}

// failedJob captura lo que el worker decidió al fallar: cuántos intentos lleva y
// cuándo (o si) se reintenta.
type failedJob struct {
	attempts int
	errMsg   string
	retryAt  *time.Time
}

func (f *fakeSyncRepo) GetLink(taskID, userID uint) (*models.CalendarEventLink, error) {
	return nil, repository.ErrCalendarLinkNotFound
}
func (f *fakeSyncRepo) ListLinksForTask(uint) ([]models.CalendarEventLink, error) { return nil, nil }
func (f *fakeSyncRepo) UpsertLink(*models.CalendarEventLink) error                { return nil }
func (f *fakeSyncRepo) DeleteLink(taskID, userID uint) error {
	f.deletedLinks = append(f.deletedLinks, [2]uint{taskID, userID})
	return f.deleteLinksErr
}
func (f *fakeSyncRepo) DeleteLinksForUser(userID uint) error {
	f.deletedForUser = append(f.deletedForUser, userID)
	return f.deleteForUserErr
}
func (f *fakeSyncRepo) EnqueueJob(*models.CalendarSyncJob) error { return nil }
func (f *fakeSyncRepo) ClaimPendingJobs(int, time.Time) ([]models.CalendarSyncJob, error) {
	return nil, nil
}
func (f *fakeSyncRepo) MarkJobDone(jobID uint) error {
	f.doneJobs = append(f.doneJobs, jobID)
	return nil
}
func (f *fakeSyncRepo) MarkJobFailed(jobID uint, attempts int, errMsg string, retryAt *time.Time) error {
	f.failedJobs = append(f.failedJobs, failedJob{attempts: attempts, errMsg: errMsg, retryAt: retryAt})
	return nil
}
func (f *fakeSyncRepo) SupersedePendingJobs(uint, uint, uint) error { return nil }

// fakeGoogle simula el cliente de Google. Los campos van con valor cero por
// defecto (crear funciona, borrar funciona), y cada test inyecta solo la rama
// que quiere ejercitar.
type fakeGoogle struct {
	deleteErr   error
	deleteCalls int

	createResult *CalendarEvent
	createErr    error
	updateErr    error
	getResult    *CalendarEvent
	presence     *MeetPresence
	presenceErr  error
	// lastCreateInput deja ver qué se le pidió a Google sin llamar a Google.
	lastCreateInput CalendarEventInput
	createCalls     int
}

func (f *fakeGoogle) Enabled() bool                        { return true }
func (f *fakeGoogle) AuthURL(uint, string) (string, error) { return "", nil }
func (f *fakeGoogle) HandleCallback(string, string) (*models.GoogleCalendarAccount, string, error) {
	return nil, "", nil
}
func (f *fakeGoogle) Status(uint) (*models.GoogleCalendarAccount, error) { return nil, nil }
func (f *fakeGoogle) Disconnect(uint) error                              { return nil }
func (f *fakeGoogle) SetDisconnectHook(func(uint))                       {}
func (f *fakeGoogle) AccessToken(uint) (string, error)                   { return "token", nil }
func (f *fakeGoogle) CreateEvent(_ uint, _ string, in CalendarEventInput) (*CalendarEvent, error) {
	f.createCalls++
	f.lastCreateInput = in
	if f.createErr != nil {
		return nil, f.createErr
	}
	if f.createResult != nil {
		return f.createResult, nil
	}
	return &CalendarEvent{ID: "evt"}, nil
}
func (f *fakeGoogle) UpdateEvent(uint, string, string, CalendarEventInput) error {
	return f.updateErr
}
func (f *fakeGoogle) GetEvent(uint, string, string) (*CalendarEvent, error) {
	if f.getResult != nil {
		return f.getResult, nil
	}
	return &CalendarEvent{ID: "evt"}, nil
}
func (f *fakeGoogle) MeetPresence(uint, string) (*MeetPresence, error) {
	if f.presenceErr != nil {
		return nil, f.presenceErr
	}
	if f.presence != nil {
		return f.presence, nil
	}
	return &MeetPresence{}, nil
}
func (f *fakeGoogle) DeleteEvent(uint, string, string) error {
	f.deleteCalls++
	return f.deleteErr
}

func deleteJob() *models.CalendarSyncJob {
	return &models.CalendarSyncJob{
		ID: 1, TaskID: 7, UserID: 42,
		Action:        models.CalendarSyncActionDelete,
		GoogleEventID: "evt-1",
		CalendarID:    "primary",
	}
}

// Un usuario que desvinculó su cuenta no tiene credencial con la que borrar el
// evento, y no la va a tener por reintentar. El job debe darse por hecho y el
// enlace limpiarse igual; si no, cada edición posterior de esa tarea gastaba
// cinco intentos y dejaba el enlace vivo para repetirlo en la siguiente.
func TestProcessDeleteWithoutAccountCleansLink(t *testing.T) {
	repo := &fakeSyncRepo{}
	svc := NewCalendarSyncService(repo, nil, nil, &fakeGoogle{
		deleteErr: repository.ErrGoogleAccountNotFound,
	})

	if err := svc.processDelete(deleteJob()); err != nil {
		t.Fatalf("processDelete devolvió error con cuenta desvinculada: %v", err)
	}
	if len(repo.deletedLinks) != 1 || repo.deletedLinks[0] != [2]uint{7, 42} {
		t.Errorf("el enlace no se limpió: %v", repo.deletedLinks)
	}
}

// En cambio un fallo transitorio de Google SÍ debe propagarse: el enlace se
// conserva para que el reintento vuelva a intentar el borrado del evento.
func TestProcessDeleteKeepsLinkOnTransientError(t *testing.T) {
	repo := &fakeSyncRepo{}
	boom := errors.New("Google respondió 503")
	svc := NewCalendarSyncService(repo, nil, nil, &fakeGoogle{deleteErr: boom})

	if err := svc.processDelete(deleteJob()); !errors.Is(err, boom) {
		t.Fatalf("processDelete devolvió %v, se esperaba el error de Google", err)
	}
	if len(repo.deletedLinks) != 0 {
		t.Errorf("el enlace se borró pese al fallo transitorio: %v", repo.deletedLinks)
	}
}

// --- Backoff de reintentos ---

// La tabla de espera debe crecer y saturarse en el último escalón, para que subir
// CalendarSyncMaxAttempts nunca la desborde.
func TestCalendarSyncRetryDelayEscalatesAndSaturates(t *testing.T) {
	prev := time.Duration(0)
	for attempts := 1; attempts <= models.CalendarSyncMaxAttempts+3; attempts++ {
		d := models.CalendarSyncRetryDelay(attempts)
		if d <= 0 {
			t.Fatalf("espera no positiva en el intento %d: %s", attempts, d)
		}
		if d < prev {
			t.Errorf("la espera bajó en el intento %d: %s tras %s", attempts, d, prev)
		}
		prev = d
	}
	// Por debajo de 1 no se indexa fuera de rango.
	if got := models.CalendarSyncRetryDelay(0); got != models.CalendarSyncRetryDelay(1) {
		t.Errorf("intento 0 = %s, se esperaba igual que el primero", got)
	}
}

// Un fallo transitorio programa el reintento en el futuro en vez de dejar el job
// listo para reintentarse de inmediato (que era lo que quemaba los intentos en
// dos minutos ante una caída de Google).
func TestProcessJobSchedulesBackoffOnTransientError(t *testing.T) {
	repo := &fakeSyncRepo{}
	svc := NewCalendarSyncService(repo, nil, nil, &fakeGoogle{
		deleteErr: errors.New("Google respondió 503 Service Unavailable"),
	})

	before := time.Now()
	svc.processJob(deleteJob())

	if len(repo.failedJobs) != 1 {
		t.Fatalf("se esperaba un job fallido, hubo %d", len(repo.failedJobs))
	}
	got := repo.failedJobs[0]
	if got.attempts != 1 {
		t.Errorf("intentos = %d, se esperaba 1", got.attempts)
	}
	if got.retryAt == nil {
		t.Fatal("un 503 debe reintentarse, no agotarse")
	}
	if !got.retryAt.After(before) {
		t.Errorf("el reintento quedó en el pasado (%s): no habría espera", got.retryAt)
	}
}

// El último intento disponible no programa otro: el job queda agotado.
func TestProcessJobExhaustsAtMaxAttempts(t *testing.T) {
	repo := &fakeSyncRepo{}
	svc := NewCalendarSyncService(repo, nil, nil, &fakeGoogle{
		deleteErr: errors.New("Google respondió 503 Service Unavailable"),
	})

	job := deleteJob()
	job.Attempts = models.CalendarSyncMaxAttempts - 1
	svc.processJob(job)

	if len(repo.failedJobs) != 1 || repo.failedJobs[0].retryAt != nil {
		t.Errorf("el job debía agotarse en el intento %d: %+v", models.CalendarSyncMaxAttempts, repo.failedJobs)
	}
}

// Los fallos que no mejoran esperando no gastan la ventana de reintentos.
func TestProcessJobDoesNotRetryPermanentFailures(t *testing.T) {
	cases := map[string]error{
		"needs_reauth": ErrNeedsReauth,
		"rechazo permanente de Google": fmt.Errorf("%w (400 Bad Request): datos inválidos",
			ErrGooglePermanent),
	}
	for name, failure := range cases {
		t.Run(name, func(t *testing.T) {
			repo := &fakeSyncRepo{}
			svc := NewCalendarSyncService(repo, nil, nil, &fakeGoogle{deleteErr: failure})

			svc.processJob(deleteJob())

			if len(repo.failedJobs) != 1 {
				t.Fatalf("se esperaba un job fallido, hubo %d", len(repo.failedJobs))
			}
			if repo.failedJobs[0].retryAt != nil {
				t.Errorf("no debía programarse reintento, se programó para %s", repo.failedJobs[0].retryAt)
			}
		})
	}
}

// La clasificación decide si se gasta la ventana de reintentos, así que conviene
// fijarla: cuota e incidentes esperan; el resto de 4xx no.
func TestIsTransientStatus(t *testing.T) {
	transient := []int{408, 429, 500, 502, 503, 504}
	permanent := []int{400, 403, 404, 409, 422}

	for _, code := range transient {
		if !isTransientStatus(code) {
			t.Errorf("%d debería reintentarse", code)
		}
	}
	for _, code := range permanent {
		if isTransientStatus(code) {
			t.Errorf("%d no debería gastar reintentos", code)
		}
	}
}

func TestOnAccountDisconnectedDropsLinks(t *testing.T) {
	repo := &fakeSyncRepo{}
	svc := NewCalendarSyncService(repo, nil, nil, &fakeGoogle{})

	svc.OnAccountDisconnected(42)

	if len(repo.deletedForUser) != 1 || repo.deletedForUser[0] != 42 {
		t.Errorf("no se borraron los enlaces del usuario: %v", repo.deletedForUser)
	}
}
