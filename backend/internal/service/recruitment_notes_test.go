package service

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/obertrack/backend/internal/apperrors"
	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
)

// Notas de Reclutamiento desde Obersuite. Es la primera escritura de la
// integración, así que lo que se fija aquí es el CONTRATO que se les mandó:
// qué es 400 y con qué texto, qué es idempotente, y que la nota queda con el
// autor y la vía traducidos como se prometió.

type fakeRecruitRepo struct {
	fakeNotesAdminRepo
	byExternal map[string]*models.CompanyEvent
	createErr  error
	deleted    []string
	deleteRows int64
	// apareceTrasCrear simula al otro escritor de una carrera: la fila que no
	// estaba antes de intentar crear y sí está después.
	apareceTrasCrear *models.CompanyEvent
	intentoCrear     bool
}

func (f *fakeRecruitRepo) FindCompanyEventByExternalID(_ uint, ext string) (*models.CompanyEvent, error) {
	if ev, ok := f.byExternal[ext]; ok {
		return ev, nil
	}
	if f.intentoCrear && f.apareceTrasCrear != nil && f.apareceTrasCrear.ExternalID == ext {
		return f.apareceTrasCrear, nil
	}
	return nil, nil
}

func (f *fakeRecruitRepo) CreateCompanyEvent(event *models.CompanyEvent) error {
	f.intentoCrear = true
	if f.createErr != nil {
		return f.createErr
	}
	event.ID = uint(100 + len(f.created))
	f.created = append(f.created, event)
	if f.byExternal == nil {
		f.byExternal = map[string]*models.CompanyEvent{}
	}
	f.byExternal[event.ExternalID] = event
	return nil
}

func (f *fakeRecruitRepo) DeleteCompanyEventByExternalID(_ uint, ext string) (int64, error) {
	f.deleted = append(f.deleted, ext)
	return f.deleteRows, nil
}

type fakeRecruitUserRepo struct {
	fakeNotesUserRepo
	// plantilla[companyID][userID] = nombre. Es lo que decide si un person_id
	// es de la empresa.
	plantilla map[uint]map[uint]string
}

func (f *fakeRecruitUserRepo) GetByEmail(email string) (*models.User, error) {
	if email == models.ObersuiteServiceEmail {
		return &models.User{ID: 999, Email: email, Name: models.ObersuiteServiceName, IsSystem: true}, nil
	}
	return nil, errors.New("not found")
}

func (f *fakeRecruitUserRepo) GetObersuiteProfessional(companyID, userID uint) (*repository.ObersuiteProfessional, error) {
	if name, ok := f.plantilla[companyID][userID]; ok {
		return &repository.ObersuiteProfessional{ID: userID, Name: name}, nil
	}
	return nil, nil
}

func newRecruitSvc() (*adminService, *fakeRecruitRepo) {
	repo := &fakeRecruitRepo{}
	users := &fakeRecruitUserRepo{
		fakeNotesUserRepo: fakeNotesUserRepo{users: map[uint]*models.User{36: company(36)}},
		plantilla:         map[uint]map[uint]string{36: {427: "Ana Pérez", 431: "Luis Gómez"}},
	}
	return &adminService{repo: repo, userRepo: users}, repo
}

func notaBase() RecruitmentNoteInput {
	return RecruitmentNoteInput{
		ExternalID: "obersuite-note-abc",
		Via:        "llamada",
		Content:    "Se acordó abrir búsqueda de un diseñador.",
		AuthorName: "Lorena Moujalli",
	}
}

func TestReclutamiento_GuardaLaNotaComoSePrometio(t *testing.T) {
	svc, repo := newRecruitSvc()

	res, err := svc.AddRecruitmentNote(36, notaBase())

	if err != nil || res.Status != "created" {
		t.Fatalf("debía crearse: res=%v err=%v", res, err)
	}
	ev := repo.created[0]
	if ev.Type != models.CompanyEventRecruitment {
		t.Errorf("tipo %q, quería recruitment", ev.Type)
	}
	if ev.ByUserID != 999 {
		t.Errorf("el autor técnico tiene que ser la cuenta de servicio, got %d", ev.ByUserID)
	}
	if ev.AuthorName != "Lorena Moujalli" {
		t.Errorf("author_name %q", ev.AuthorName)
	}
	// "llamada" se traduce a nuestro canal, no se guarda en español.
	if ev.Channel != models.CompanyContactCall {
		t.Errorf("via llamada debía ser channel call, got %q", ev.Channel)
	}
	if ev.ExternalID != "obersuite-note-abc" {
		t.Errorf("external_id %q", ev.ExternalID)
	}
}

// Idempotencia: el mismo external_id no crea otra nota ni modifica la que hay.
// Es lo que protege el reintento del 500 y una doble pulsación del reclutador.
func TestReclutamiento_ElMismoIdNoDuplica(t *testing.T) {
	svc, repo := newRecruitSvc()

	primera, _ := svc.AddRecruitmentNote(36, notaBase())
	otra := notaBase()
	otra.Content = "texto distinto que NO debe pisar el primero"
	segunda, err := svc.AddRecruitmentNote(36, otra)

	if err != nil || segunda.Status != "already_exists" || segunda.ID != primera.ID {
		t.Fatalf("el reintento debía devolver la misma nota: %v %v", segunda, err)
	}
	if len(repo.created) != 1 {
		t.Fatalf("se creó una segunda fila: %d", len(repo.created))
	}
	if !strings.Contains(repo.created[0].Detail, "diseñador") {
		t.Fatal("el reintento modificó la nota original")
	}
}

// Los destinatarios no existen en nuestro modelo: van al texto, con nombre, y
// validados. Un id que no es de la empresa es 400 con el id, nunca un
// "Para: <alguien de otro cliente>" silencioso.
func TestReclutamiento_LosDestinatariosVanAlTextoConSuNombre(t *testing.T) {
	svc, repo := newRecruitSvc()
	in := notaBase()
	in.PersonIDs = []uint{427, 431, 427} // repetido a propósito

	if _, err := svc.AddRecruitmentNote(36, in); err != nil {
		t.Fatal(err)
	}
	if got := repo.created[0].Detail; !strings.HasPrefix(got, "Para: Ana Pérez, Luis Gómez — ") {
		t.Fatalf("prefijo de destinatarios: %q", got)
	}
}

func TestReclutamiento_UnDestinatarioDeOtraEmpresaEs400ConElId(t *testing.T) {
	svc, repo := newRecruitSvc()
	in := notaBase()
	in.PersonIDs = []uint{427, 9999}

	_, err := svc.AddRecruitmentNote(36, in)

	if !errors.Is(err, apperrors.ErrInvalidInput) {
		t.Fatalf("debía ser 400, got %v", err)
	}
	if !strings.Contains(err.Error(), "9999") {
		t.Errorf("el mensaje tiene que decir QUÉ id falla: %s", err)
	}
	if len(repo.created) != 0 {
		t.Fatal("un 400 no escribe nada")
	}
}

func TestReclutamiento_ListaVaciaEsTodaLaEmpresa(t *testing.T) {
	svc, repo := newRecruitSvc()
	in := notaBase()
	in.PersonIDs = []uint{}

	if _, err := svc.AddRecruitmentNote(36, in); err != nil {
		t.Fatal(err)
	}
	if strings.HasPrefix(repo.created[0].Detail, "Para:") {
		t.Fatal("sin destinatarios no hay prefijo")
	}
}

// La tabla del 400, tal como se les mandó. Cada caso con el texto que verá el
// reclutador, porque Obersuite lo enseña tal cual.
func TestReclutamiento_Los400DelContrato(t *testing.T) {
	casos := []struct {
		nombre string
		muta   func(*RecruitmentNoteInput)
		frase  string
	}{
		{"sin external_id", func(i *RecruitmentNoteInput) { i.ExternalID = " " }, "external_id"},
		{"via desconocida", func(i *RecruitmentNoteInput) { i.Via = "paloma" }, "paloma"},
		{"sin content", func(i *RecruitmentNoteInput) { i.Content = "   " }, "content"},
		{"content largo", func(i *RecruitmentNoteInput) { i.Content = strings.Repeat("x", 2001) }, "2000"},
		{"sin author_name", func(i *RecruitmentNoteInput) { i.AuthorName = "" }, "author_name"},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			svc, repo := newRecruitSvc()
			in := notaBase()
			c.muta(&in)

			_, err := svc.AddRecruitmentNote(36, in)

			if !errors.Is(err, apperrors.ErrInvalidInput) {
				t.Fatalf("debía ser 400, got %v", err)
			}
			if !strings.Contains(err.Error(), c.frase) {
				t.Errorf("mensaje %q debía mencionar %q", err.Error(), c.frase)
			}
			if len(repo.created) != 0 {
				t.Error("un 400 no escribe nada")
			}
		})
	}
}

// Las cinco vías del formulario y a qué canal nuestro van. "general" es sin vía.
func TestReclutamiento_LasCincoViasSeTraducen(t *testing.T) {
	quiero := map[string]string{
		"reunion": models.CompanyContactMeeting, "whatsapp": models.CompanyContactWhatsApp,
		"llamada": models.CompanyContactCall, "correo": models.CompanyContactEmail, "general": "",
	}
	for via, canal := range quiero {
		svc, repo := newRecruitSvc()
		in := notaBase()
		in.Via = via
		if _, err := svc.AddRecruitmentNote(36, in); err != nil {
			t.Fatalf("via %q rechazada: %v", via, err)
		}
		if got := repo.created[0].Channel; got != canal {
			t.Errorf("via %q -> channel %q, quería %q", via, got, canal)
		}
	}
}

func TestReclutamiento_RespetaLaFechaQueMandan(t *testing.T) {
	svc, repo := newRecruitSvc()
	in := notaBase()
	cuando := time.Date(2026, 9, 11, 22, 10, 0, 0, time.UTC)
	in.CreatedAt = &cuando

	if _, err := svc.AddRecruitmentNote(36, in); err != nil {
		t.Fatal(err)
	}
	if !repo.created[0].CreatedAt.Equal(cuando) {
		t.Fatalf("created_at %v, quería %v", repo.created[0].CreatedAt, cuando)
	}
}

func TestReclutamiento_EmpresaInexistenteEs404(t *testing.T) {
	svc, _ := newRecruitSvc()
	_, err := svc.AddRecruitmentNote(77, notaBase())
	if !errors.Is(err, apperrors.ErrNotFound) {
		t.Fatalf("debía ser 404, got %v", err)
	}
}

// Dos POST simultáneos con el mismo id: uno gana el índice único, el otro
// choca. Para quien mandó el segundo es el mismo "ya existe", no un 500. Y si
// tras el choque no aparece ninguna fila —no era ese índice— sube como error.
func TestReclutamiento_UnChoqueDeIndiceEsYaExiste(t *testing.T) {
	svc, repo := newRecruitSvc()
	repo.createErr = errors.New(`ERROR: duplicate key value violates unique constraint "idx_company_events_external_id" (SQLSTATE 23505)`)

	// Sin fila del otro escritor: error.
	if _, err := svc.AddRecruitmentNote(36, notaBase()); err == nil {
		t.Fatal("sin fila que devolver, el choque tiene que subir como error")
	}

	// La fila del otro escritor aparece SOLO después del intento de crear:
	// antes de escribir no estaba (por eso se llegó a crear).
	repo.apareceTrasCrear = &models.CompanyEvent{ID: 55, ExternalID: "obersuite-note-abc"}
	res, err := svc.AddRecruitmentNote(36, notaBase())
	if err != nil || res.Status != "already_exists" || res.ID != 55 {
		t.Fatalf("debía resolverse como already_exists: %v %v", res, err)
	}
}

func TestReclutamiento_BorrarEsEnFirmeYElSegundoEs404(t *testing.T) {
	svc, repo := newRecruitSvc()
	repo.deleteRows = 1
	if err := svc.DeleteRecruitmentNote(36, "obersuite-note-abc"); err != nil {
		t.Fatalf("el primer borrado debía salir: %v", err)
	}
	repo.deleteRows = 0
	err := svc.DeleteRecruitmentNote(36, "obersuite-note-abc")
	if !errors.Is(err, apperrors.ErrNotFound) {
		t.Fatalf("el segundo DELETE es 404, got %v", err)
	}
}
