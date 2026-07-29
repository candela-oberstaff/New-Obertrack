package service

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
)

// ---------------------------------------------------------------------------
// Payload de Google: la parte nueva es el evento CON HORA y la sala de Meet.
// Los tests de día completo (tareas) viven en calendar_sync_service_test.go y
// siguen valiendo tal cual: son la red que protege la sincronización de tareas.
// ---------------------------------------------------------------------------

// Un evento con hora debe emitir dateTime+timeZone y NINGÚN `date`: Google
// rechaza un evento que traiga las dos formas a la vez.
func TestBuildEventPayloadTimed(t *testing.T) {
	start := time.Date(2026, 8, 5, 15, 0, 0, 0, time.UTC)
	p := buildEventPayload(CalendarEventInput{
		Summary:  "Seguimiento",
		StartAt:  start,
		EndAt:    start.Add(45 * time.Minute),
		TimeZone: "America/Bogota",
	})

	if p.Start.Date != "" || p.End.Date != "" {
		t.Errorf("un evento con hora no debe llevar `date`: %+v / %+v", p.Start, p.End)
	}
	if p.Start.DateTime != start.Format(time.RFC3339) {
		t.Errorf("start = %q, se esperaba %q", p.Start.DateTime, start.Format(time.RFC3339))
	}
	if p.Start.TimeZone != "America/Bogota" || p.End.TimeZone != "America/Bogota" {
		t.Errorf("falta la zona IANA: %+v / %+v", p.Start, p.End)
	}
}

// Un fin que no es posterior al inicio produciría un 400 de Google; se corrige a
// una hora en vez de dejar que reviente la petición.
func TestBuildEventPayloadTimedFixesInvalidEnd(t *testing.T) {
	start := time.Date(2026, 8, 5, 15, 0, 0, 0, time.UTC)
	for name, end := range map[string]time.Time{
		"vacío":    {},
		"anterior": start.Add(-time.Hour),
		"igual":    start,
	} {
		t.Run(name, func(t *testing.T) {
			p := buildEventPayload(CalendarEventInput{StartAt: start, EndAt: end, TimeZone: "UTC"})
			want := start.Add(time.Hour).Format(time.RFC3339)
			if p.End.DateTime != want {
				t.Errorf("end = %q, se esperaba %q", p.End.DateTime, want)
			}
		})
	}
}

// El día completo debe seguir intacto tras añadir la rama con hora.
func TestBuildEventPayloadAllDayStillHasNoDateTime(t *testing.T) {
	p := buildEventPayload(CalendarEventInput{
		StartDate: time.Date(2026, 8, 5, 0, 0, 0, 0, time.UTC),
		EndDate:   time.Date(2026, 8, 5, 0, 0, 0, 0, time.UTC),
	})
	if p.Start.DateTime != "" || p.Start.TimeZone != "" {
		t.Errorf("el evento de día completo no debe llevar dateTime/timeZone: %+v", p.Start)
	}
	if p.Start.Date != "2026-08-05" || p.End.Date != "2026-08-06" {
		t.Errorf("rango all-day = %s..%s, se esperaba 2026-08-05..2026-08-06", p.Start.Date, p.End.Date)
	}
}

func TestBuildEventPayloadRequestsMeet(t *testing.T) {
	p := buildEventPayload(CalendarEventInput{
		StartAt:          time.Now(),
		TimeZone:         "UTC",
		CreateConference: true,
		Attendees:        []string{"a@x.com", "b@x.com"},
		Recurrence:       []string{"RRULE:FREQ=WEEKLY"},
	})

	if p.ConferenceData == nil || p.ConferenceData.CreateRequest == nil {
		t.Fatal("no se pidió la sala de Meet")
	}
	if got := p.ConferenceData.CreateRequest.ConferenceSolutionKey.Type; got != hangoutsMeet {
		t.Errorf("tipo de conferencia = %q, se esperaba %q", got, hangoutsMeet)
	}
	if p.ConferenceData.CreateRequest.RequestID == "" {
		t.Error("el requestId no puede ir vacío: Google lo rechaza")
	}
	if len(p.Attendees) != 2 || p.Attendees[0].Email != "a@x.com" {
		t.Errorf("invitados mal traducidos: %+v", p.Attendees)
	}
	if len(p.Recurrence) != 1 {
		t.Errorf("recurrencia = %v", p.Recurrence)
	}
}

// Google deduplica por requestId: si dos sesiones compartieran el mismo, la
// segunda reusaría la sala de la primera.
func TestConferenceRequestIDIsUnique(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 50; i++ {
		id := newConferenceRequestID()
		if id == "" {
			t.Fatal("requestId vacío")
		}
		if seen[id] {
			t.Fatalf("requestId repetido: %q", id)
		}
		seen[id] = true
	}
}

// Sin conferencia ni invitados no se manda ningún parámetro: es el caso de la
// sincronización de tareas, que no debe cambiar de comportamiento.
func TestEventQuery(t *testing.T) {
	if got := eventQuery(false, false); got != "" {
		t.Errorf("query sin invitados ni conferencia = %q, se esperaba vacía", got)
	}
	if got := eventQuery(true, false); !strings.Contains(got, "sendUpdates=all") {
		t.Errorf("con invitados falta sendUpdates: %q", got)
	}
	if got := eventQuery(false, true); strings.Contains(got, "sendUpdates") {
		t.Errorf("sin invitados no debe pedirse sendUpdates: %q", got)
	}
	if got := eventQuery(false, true); !strings.Contains(got, "conferenceDataVersion=1") {
		t.Errorf("con conferencia falta conferenceDataVersion: %q", got)
	}
}

func TestToCalendarEventPrefersHangoutLink(t *testing.T) {
	resp := &googleEventResponse{
		ID:          "e1",
		HangoutLink: "https://meet.google.com/abc-def-ghi",
		ConferenceData: &googleConferenceDataResponse{
			EntryPoints: []googleEntryPoint{
				{EntryPointType: "video", URI: "https://meet.google.com/otro"},
			},
		},
	}

	ev := resp.toCalendarEvent()
	if ev.MeetURL != "https://meet.google.com/abc-def-ghi" {
		t.Errorf("MeetURL = %q, debía ganar hangoutLink", ev.MeetURL)
	}
	if ev.ConferencePending {
		t.Error("sin createRequest pendiente no debe marcarse pending")
	}
}

func TestToCalendarEventFallsBackToEntryPoint(t *testing.T) {
	resp := &googleEventResponse{
		ID: "e1",
		ConferenceData: &googleConferenceDataResponse{
			EntryPoints: []googleEntryPoint{
				{EntryPointType: "phone", URI: "tel:+123"},
				{EntryPointType: "video", URI: "https://meet.google.com/xyz"},
			},
		},
	}

	if got := resp.toCalendarEvent().MeetURL; got != "https://meet.google.com/xyz" {
		t.Errorf("MeetURL = %q, se esperaba el entryPoint de vídeo (no el teléfono)", got)
	}
}

func TestToCalendarEventDetectsPendingConference(t *testing.T) {
	resp := &googleEventResponse{
		ID: "e1",
		ConferenceData: &googleConferenceDataResponse{
			CreateRequest: &googleCreateRequestStatusWrap{
				Status: googleCreateRequestStatus{StatusCode: "pending"},
			},
		},
	}

	ev := resp.toCalendarEvent()
	if !ev.ConferencePending {
		t.Error("una conferencia 'pending' debe marcarse para volver a consultarla")
	}
	if ev.MeetURL != "" {
		t.Errorf("MeetURL = %q, todavía no debería existir", ev.MeetURL)
	}
}

// ---------------------------------------------------------------------------
// Servicio de sesiones
// ---------------------------------------------------------------------------

type fakeMeetingRepo struct {
	repository.MeetingRepository

	session   *models.MeetingSession
	getErr    error
	created   *models.MeetingSession
	createErr error
	updates   map[string]interface{}
	cancelled bool
}

func (f *fakeMeetingRepo) Create(s *models.MeetingSession) error {
	if f.createErr != nil {
		return f.createErr
	}
	s.ID = 1
	f.created = s
	return nil
}

func (f *fakeMeetingRepo) GetByID(uint) (*models.MeetingSession, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	if f.session != nil {
		return f.session, nil
	}
	// Create recarga la sesión al terminar para devolverla con sus asociaciones;
	// el fake devuelve la que acaba de guardar.
	if f.created != nil {
		return f.created, nil
	}
	return nil, repository.ErrMeetingNotFound
}

func (f *fakeMeetingRepo) UpdateFields(_ uint, updates map[string]interface{}) error {
	f.updates = updates
	if updates["status"] == models.MeetingStatusCancelled {
		f.cancelled = true
	}
	return nil
}

func (f *fakeMeetingRepo) ReplaceAttendees(uint, []models.MeetingAttendee) error { return nil }
func (f *fakeMeetingRepo) IsParticipant(uint, uint) (bool, error)                { return true, nil }

type fakeMeetingUserRepo struct {
	repository.UserRepository

	byID    map[uint]*models.User
	byEmail map[string]*models.User
}

func (f *fakeMeetingUserRepo) GetByID(id uint) (*models.User, error) {
	if u, ok := f.byID[id]; ok {
		return u, nil
	}
	return nil, errors.New("no existe")
}

func (f *fakeMeetingUserRepo) GetByEmail(email string) (*models.User, error) {
	if u, ok := f.byEmail[email]; ok {
		return u, nil
	}
	return nil, errors.New("no existe")
}

type fakeGoogleAccountRepo struct {
	repository.GoogleCalendarRepository

	account *models.GoogleCalendarAccount
	err     error
}

func (f *fakeGoogleAccountRepo) GetByUser(uint) (*models.GoogleCalendarAccount, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.account, nil
}

type fakeNotifier struct {
	NotificationService
	created int
}

func (f *fakeNotifier) CreateNotification(uint, string, string, string, map[string]interface{}) error {
	f.created++
	return nil
}

func newMeetingSvc(
	repo *fakeMeetingRepo, users *fakeMeetingUserRepo,
	accounts *fakeGoogleAccountRepo, google *fakeGoogle,
) *meetingService {
	return NewMeetingService(repo, users, accounts, google, &fakeNotifier{}).(*meetingService)
}

func validInput() MeetingInput {
	start := time.Now().Add(time.Hour)
	return MeetingInput{
		Title:           "Seguimiento",
		StartAt:         start,
		EndAt:           start.Add(30 * time.Minute),
		TimeZone:        "America/Bogota",
		AttendeeUserIDs: []uint{2},
	}
}

func TestMeetingValidation(t *testing.T) {
	svc := &meetingService{}
	start := time.Now().Add(time.Hour)

	cases := map[string]MeetingInput{
		"sin título":      {StartAt: start, EndAt: start.Add(time.Hour), TimeZone: "UTC", AttendeeUserIDs: []uint{2}},
		"sin inicio":      {Title: "T", EndAt: start, TimeZone: "UTC", AttendeeUserIDs: []uint{2}},
		"fin antes":       {Title: "T", StartAt: start, EndAt: start.Add(-time.Hour), TimeZone: "UTC", AttendeeUserIDs: []uint{2}},
		"sin zona":        {Title: "T", StartAt: start, EndAt: start.Add(time.Hour), AttendeeUserIDs: []uint{2}},
		"zona inventada":  {Title: "T", StartAt: start, EndAt: start.Add(time.Hour), TimeZone: "Marte/Olympus", AttendeeUserIDs: []uint{2}},
		"sin invitados":   {Title: "T", StartAt: start, EndAt: start.Add(time.Hour), TimeZone: "UTC"},
		"correo inválido": {Title: "T", StartAt: start, EndAt: start.Add(time.Hour), TimeZone: "UTC", AttendeeEmails: []string{"no-es-correo"}},
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			if err := svc.validate(in); !errors.Is(err, ErrMeetingValidation) {
				t.Errorf("validate devolvió %v, se esperaba ErrMeetingValidation", err)
			}
		})
	}

	if err := svc.validate(validInput()); err != nil {
		t.Errorf("una entrada válida no debería fallar: %v", err)
	}
}

// El organizador ya es dueño del evento en Google: volver a listarlo lo
// duplicaría en la lista de asistentes.
func TestResolveAttendeesExcludesOrganizerAndDedupes(t *testing.T) {
	users := &fakeMeetingUserRepo{
		byID: map[uint]*models.User{
			1: {ID: 1, Name: "Organizador", Email: "org@x.com"},
			2: {ID: 2, Name: "Ana", Email: "ana@x.com"},
		},
		byEmail: map[string]*models.User{},
	}
	svc := newMeetingSvc(&fakeMeetingRepo{}, users, &fakeGoogleAccountRepo{}, &fakeGoogle{})

	got, err := svc.resolveAttendees(MeetingInput{
		AttendeeUserIDs: []uint{1, 2, 2},
		// El mismo correo por la vía externa, y en otra caja, no debe duplicar.
		AttendeeEmails: []string{"ANA@x.com", "externo@fuera.com"},
	}, 1)
	if err != nil {
		t.Fatalf("resolveAttendees: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("se esperaban 2 invitados (Ana + externo), hubo %d: %+v", len(got), got)
	}
	for _, a := range got {
		if a.Email == "org@x.com" {
			t.Error("el organizador no debe ir como invitado")
		}
	}
}

// Un correo metido "a mano" que resulta ser de alguien con cuenta se trata como
// interno: así recibe campanita y DM, no solo el correo de Google.
func TestResolveAttendeesPromotesKnownEmail(t *testing.T) {
	users := &fakeMeetingUserRepo{
		byID:    map[uint]*models.User{},
		byEmail: map[string]*models.User{"ana@x.com": {ID: 2, Name: "Ana", Email: "ana@x.com"}},
	}
	svc := newMeetingSvc(&fakeMeetingRepo{}, users, &fakeGoogleAccountRepo{}, &fakeGoogle{})

	got, err := svc.resolveAttendees(MeetingInput{AttendeeEmails: []string{"ana@x.com"}}, 1)
	if err != nil {
		t.Fatalf("resolveAttendees: %v", err)
	}
	if len(got) != 1 || got[0].UserID == nil || *got[0].UserID != 2 {
		t.Errorf("el correo conocido debía resolverse como usuario interno: %+v", got)
	}
}

func TestCreateRequiresConnectedGoogleAccount(t *testing.T) {
	accounts := &fakeGoogleAccountRepo{err: repository.ErrGoogleAccountNotFound}
	svc := newMeetingSvc(&fakeMeetingRepo{}, &fakeMeetingUserRepo{}, accounts, &fakeGoogle{})

	_, err := svc.Create(1, 10, validInput())
	if !errors.Is(err, ErrGoogleNotConnected) {
		t.Errorf("Create devolvió %v, se esperaba ErrGoogleNotConnected", err)
	}
}

func TestCreateRejectsAccountNeedingReauth(t *testing.T) {
	accounts := &fakeGoogleAccountRepo{account: &models.GoogleCalendarAccount{
		CalendarID: "primary", Status: models.GoogleCalStatusNeedsReauth,
	}}
	svc := newMeetingSvc(&fakeMeetingRepo{}, &fakeMeetingUserRepo{}, accounts, &fakeGoogle{})

	_, err := svc.Create(1, 10, validInput())
	if !errors.Is(err, ErrNeedsReauth) {
		t.Errorf("Create devolvió %v, se esperaba ErrNeedsReauth", err)
	}
}

// Si Google creó el evento pero la sesión no se pudo guardar, el evento debe
// deshacerse: si no, queda una reunión en el calendario del organizador que
// Obertrack no sabe ni que existe y que nadie puede cancelar desde la app.
func TestCreateRollsBackGoogleEventWhenSaveFails(t *testing.T) {
	users := &fakeMeetingUserRepo{byID: map[uint]*models.User{
		2: {ID: 2, Name: "Ana", Email: "ana@x.com"},
	}}
	accounts := &fakeGoogleAccountRepo{account: &models.GoogleCalendarAccount{
		CalendarID: "primary", Status: models.GoogleCalStatusActive,
	}}
	google := &fakeGoogle{createResult: &CalendarEvent{ID: "evt-1", MeetURL: "https://meet.google.com/abc"}}
	repo := &fakeMeetingRepo{createErr: errors.New("la base de datos falló")}

	svc := newMeetingSvc(repo, users, accounts, google)
	if _, err := svc.Create(1, 10, validInput()); err == nil {
		t.Fatal("Create debía propagar el fallo de guardado")
	}
	if google.deleteCalls != 1 {
		t.Errorf("se esperaba 1 borrado de compensación en Google, hubo %d", google.deleteCalls)
	}
}

func TestCreatePassesConferenceAndAttendeesToGoogle(t *testing.T) {
	users := &fakeMeetingUserRepo{byID: map[uint]*models.User{
		2: {ID: 2, Name: "Ana", Email: "ana@x.com"},
	}}
	accounts := &fakeGoogleAccountRepo{account: &models.GoogleCalendarAccount{
		CalendarID: "primary", Status: models.GoogleCalStatusActive,
	}}
	google := &fakeGoogle{createResult: &CalendarEvent{ID: "evt-1", MeetURL: "https://meet.google.com/abc"}}
	repo := &fakeMeetingRepo{}

	svc := newMeetingSvc(repo, users, accounts, google)
	if _, err := svc.Create(1, 10, validInput()); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if !google.lastCreateInput.CreateConference {
		t.Error("una sesión siempre debe pedir sala de Meet")
	}
	if len(google.lastCreateInput.Attendees) != 1 || google.lastCreateInput.Attendees[0] != "ana@x.com" {
		t.Errorf("invitados enviados a Google: %v", google.lastCreateInput.Attendees)
	}
	if repo.created == nil || repo.created.MeetURL != "https://meet.google.com/abc" {
		t.Errorf("la sesión guardada no conservó el enlace: %+v", repo.created)
	}
}

// Solo el organizador manda sobre su reunión: un invitado no puede reprogramarla
// ni cancelarla, aunque tenga permiso 'edit' del módulo.
func TestOnlyOrganizerCanEditOrCancel(t *testing.T) {
	repo := &fakeMeetingRepo{session: &models.MeetingSession{
		ID: 1, OrganizerID: 1, Status: models.MeetingStatusScheduled, GoogleEventID: "evt",
	}}
	svc := newMeetingSvc(repo, &fakeMeetingUserRepo{}, &fakeGoogleAccountRepo{}, &fakeGoogle{})

	const invitado = uint(9)
	if _, err := svc.Update(1, invitado, validInput()); !errors.Is(err, ErrMeetingForbidden) {
		t.Errorf("Update por un invitado devolvió %v, se esperaba ErrMeetingForbidden", err)
	}
	if err := svc.Cancel(1, invitado); !errors.Is(err, ErrMeetingForbidden) {
		t.Errorf("Cancel por un invitado devolvió %v, se esperaba ErrMeetingForbidden", err)
	}
	if repo.cancelled {
		t.Error("la sesión no debía cancelarse")
	}
}

// ---------------------------------------------------------------------------
// Presencia en la sala (API de Meet)
// ---------------------------------------------------------------------------

func TestMeetingCodeFromURL(t *testing.T) {
	cases := map[string]string{
		"https://meet.google.com/nnv-fbhe-wpc":        "nnv-fbhe-wpc",
		"https://meet.google.com/nnv-fbhe-wpc?hs=122": "nnv-fbhe-wpc",
		"meet.google.com/abc-defg-hij":                "abc-defg-hij",
		"https://MEET.GOOGLE.COM/ABC-DEFG-HIJ":        "abc-defg-hij",
		"https://zoom.us/j/123456":                    "",
		"":                                            "",
	}
	for input, want := range cases {
		if got := meetingCodeFromURL(input); got != want {
			t.Errorf("meetingCodeFromURL(%q) = %q, se esperaba %q", input, got, want)
		}
	}
}

// Una sesión sin sala, o ya cancelada, no debe llegar a preguntarle nada a
// Google: sería gastar cuota para saber algo que ya sabemos.
func TestPresenceSkipsGoogleWhenThereIsNoRoom(t *testing.T) {
	for name, session := range map[string]*models.MeetingSession{
		"sin enlace": {ID: 1, OrganizerID: 1, Status: models.MeetingStatusScheduled},
		"cancelada": {
			ID: 1, OrganizerID: 1, Status: models.MeetingStatusCancelled,
			MeetURL: "https://meet.google.com/abc-defg-hij",
		},
	} {
		t.Run(name, func(t *testing.T) {
			google := &fakeGoogle{presence: &MeetPresence{Live: true, Active: 3}}
			svc := newMeetingSvc(&fakeMeetingRepo{session: session}, &fakeMeetingUserRepo{},
				&fakeGoogleAccountRepo{}, google)

			got, err := svc.Presence(1, 1)
			if err != nil {
				t.Fatalf("Presence: %v", err)
			}
			if got.Live || got.Active != 0 {
				t.Errorf("se esperaba sala vacía, hubo %+v", got)
			}
		})
	}
}

// Que la sala no exista todavía (nadie ha entrado nunca) es una sala vacía, no
// un error: el frontend consulta esto en bucle y no debe pintar rojo por ello.
func TestPresenceTreatsMissingRoomAsEmpty(t *testing.T) {
	session := &models.MeetingSession{
		ID: 1, OrganizerID: 1, Status: models.MeetingStatusScheduled,
		MeetURL: "https://meet.google.com/abc-defg-hij",
	}
	google := &fakeGoogle{presenceErr: ErrEventGone}
	svc := newMeetingSvc(&fakeMeetingRepo{session: session}, &fakeMeetingUserRepo{},
		&fakeGoogleAccountRepo{}, google)

	got, err := svc.Presence(1, 1)
	if err != nil {
		t.Fatalf("una sala inexistente no es error: %v", err)
	}
	if got.Live {
		t.Errorf("se esperaba sala vacía, hubo %+v", got)
	}
}

// En cambio la falta de scope sí se propaga: la UI necesita distinguirla para
// ofrecer "reconecta" en vez de mentir con un contador a cero.
func TestPresencePropagatesMissingScope(t *testing.T) {
	session := &models.MeetingSession{
		ID: 1, OrganizerID: 1, Status: models.MeetingStatusScheduled,
		MeetURL: "https://meet.google.com/abc-defg-hij",
	}
	google := &fakeGoogle{presenceErr: ErrMeetScopeMissing}
	svc := newMeetingSvc(&fakeMeetingRepo{session: session}, &fakeMeetingUserRepo{},
		&fakeGoogleAccountRepo{}, google)

	if _, err := svc.Presence(1, 1); !errors.Is(err, ErrMeetScopeMissing) {
		t.Errorf("Presence devolvió %v, se esperaba ErrMeetScopeMissing", err)
	}
}

// El scope nuevo tiene que estar en la URL de consentimiento, o el token que se
// emita no servirá para leer la sala.
func TestAuthURLIncludesMeetScope(t *testing.T) {
	if !strings.Contains(googleScopes, "meetings.space.readonly") {
		t.Error("falta meetings.space.readonly en los scopes")
	}
	// meetings.space.created NO sirve: solo alcanza salas creadas por la app a
	// través de la API de Meet, y las nuestras las crea Calendar.
	if strings.Contains(googleScopes, "meetings.space.created") {
		t.Error("meetings.space.created no cubre las salas creadas por Calendar")
	}
}

func TestMeetParticipantsDisplayNames(t *testing.T) {
	var resp meetParticipantsResponse
	if err := json.Unmarshal([]byte(`{"participants":[
		{"name":"p1","signedinUser":{"displayName":"Ana"}},
		{"name":"p2","anonymousUser":{"displayName":"Invitado"}},
		{"name":"p3","phoneUser":{"displayName":"+58 412..."}},
		{"name":"p4"}
	]}`), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	names := resp.displayNames()
	// El cuarto no trae nombre por ninguna vía: cuenta como asistente pero no
	// aporta nombre, y eso no debe producir una entrada vacía en la lista.
	if len(names) != 3 {
		t.Errorf("nombres = %v, se esperaban 3", names)
	}
	if len(resp.Participants) != 4 {
		t.Errorf("participantes = %d, se esperaban 4", len(resp.Participants))
	}
}

// Cancelar dos veces deja el mismo estado y no vuelve a llamar a Google.
func TestCancelIsIdempotent(t *testing.T) {
	repo := &fakeMeetingRepo{session: &models.MeetingSession{
		ID: 1, OrganizerID: 1, Status: models.MeetingStatusCancelled, GoogleEventID: "evt",
	}}
	google := &fakeGoogle{}
	svc := newMeetingSvc(repo, &fakeMeetingUserRepo{}, &fakeGoogleAccountRepo{}, google)

	if err := svc.Cancel(1, 1); err != nil {
		t.Fatalf("cancelar una sesión ya cancelada no es error: %v", err)
	}
	if google.deleteCalls != 0 {
		t.Errorf("no debía volver a llamarse a Google, hubo %d borrados", google.deleteCalls)
	}
}
