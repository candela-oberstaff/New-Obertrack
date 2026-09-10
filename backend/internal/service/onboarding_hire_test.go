package service

import (
	"errors"
	"testing"
	"time"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
)

// El puente Obersuite → Obertrack: identidad del profesional que llega y correo
// de inducción. Mismo patrón de fakes que el resto del paquete: se embebe la
// interfaz real y se sobrescribe solo lo que toca el camino bajo prueba.

type fakeHireUserRepo struct {
	repository.UserRepository

	byID          map[uint]*models.User
	byEmail       map[string]*models.User
	byObersuiteID map[string]*models.User

	created *models.User
	updates map[uint]map[string]interface{}
}

func (f *fakeHireUserRepo) GetByID(id uint) (*models.User, error) {
	if u, ok := f.byID[id]; ok {
		return u, nil
	}
	return nil, errors.New("not found")
}

func (f *fakeHireUserRepo) GetByEmail(email string) (*models.User, error) {
	if u, ok := f.byEmail[email]; ok {
		return u, nil
	}
	return nil, errors.New("not found")
}

func (f *fakeHireUserRepo) GetByObersuiteID(id string) (*models.User, error) {
	if u, ok := f.byObersuiteID[id]; ok {
		return u, nil
	}
	return nil, errors.New("not found")
}

func (f *fakeHireUserRepo) Create(user *models.User) error {
	user.ID = 900
	f.created = user
	return nil
}

func (f *fakeHireUserRepo) Update(user *models.User, updates map[string]interface{}) error {
	if f.updates == nil {
		f.updates = map[uint]map[string]interface{}{}
	}
	f.updates[user.ID] = updates
	return nil
}

func (f *fakeHireUserRepo) GetObersuiteCompanies(_ *time.Time) ([]repository.ObersuiteCompanyRecord, error) {
	var records []repository.ObersuiteCompanyRecord
	for _, u := range f.byID {
		if u.UserType == models.UserTypeEmployer {
			name := u.CompanyName
			if name == "" {
				name = u.Name
			}
			records = append(records, repository.ObersuiteCompanyRecord{
				ID:               u.ID,
				Name:             name,
				IsActive:         u.IsActive,
				ResponsibleName:  u.Name,
				ResponsibleEmail: u.Email,
				Industry:         u.Industry,
				Country:          u.Country,
				State:            u.State,
				City:             u.City,
				Address:          u.Address,
			})
		}
	}
	return records, nil
}

type fakeHireEmploymentRepo struct {
	repository.EmploymentRepository

	active  *models.Employment
	updates map[string]interface{}
}

func (f *fakeHireEmploymentRepo) GetActive(userID, companyID uint) (*models.Employment, error) {
	if f.active != nil {
		return f.active, nil
	}
	return nil, errors.New("not found")
}

func (f *fakeHireEmploymentRepo) Update(emp *models.Employment, updates map[string]interface{}) error {
	f.updates = updates
	return nil
}

type fakeHireEmploymentSvc struct {
	EmploymentService
	startReason string
}

func (f *fakeHireEmploymentSvc) AddEmployment(userID, companyID uint, jobTitle, startReason string, managerID *uint) (*models.Employment, error) {
	f.startReason = startReason
	return &models.Employment{ID: 77, UserID: userID, CompanyID: companyID}, nil
}

type fakeHireAuthSvc struct {
	AuthService
	welcomeTo []string
}

func (f *fakeHireAuthSvc) ForgotPassword(email string) error {
	f.welcomeTo = append(f.welcomeTo, email)
	return nil
}

// fakeHireInduction registra a quién se invitó. enabled=false imita la inducción
// apagada, que devuelve false SIN error (el puente no se rompe por eso).
type fakeHireInduction struct {
	InductionService
	enabled bool
	invited []uint
}

func (f *fakeHireInduction) InviteIfEnabled(user *models.User) (bool, error) {
	if !f.enabled {
		return false, nil
	}
	f.invited = append(f.invited, user.ID)
	return true, nil
}

// hireCompany es la empresa contratante: válida y activa salvo que el test diga
// lo contrario.
func hireCompany() *models.User {
	return &models.User{ID: 10, UserType: models.UserTypeEmployer, IsActive: true, CompanyName: "Acme"}
}

func newHireSvc(induccionEncendida bool, existentes ...*models.User) (*onboardingService, *fakeHireUserRepo, *fakeHireEmploymentRepo, *fakeHireInduction, *fakeHireAuthSvc) {
	company := hireCompany()
	userRepo := &fakeHireUserRepo{
		byID:          map[uint]*models.User{company.ID: company},
		byEmail:       map[string]*models.User{},
		byObersuiteID: map[string]*models.User{},
	}
	for _, u := range existentes {
		userRepo.byID[u.ID] = u
		userRepo.byEmail[u.Email] = u
		if u.ObersuiteID != "" {
			userRepo.byObersuiteID[u.ObersuiteID] = u
		}
	}
	empRepo := &fakeHireEmploymentRepo{}
	induction := &fakeHireInduction{enabled: induccionEncendida}
	auth := &fakeHireAuthSvc{}
	svc := &onboardingService{
		userRepo:       userRepo,
		employmentRepo: empRepo,
		employmentSvc:  &fakeHireEmploymentSvc{},
		authSvc:        auth,
		inductionSvc:   induction,
	}
	return svc, userRepo, empRepo, induction, auth
}

func baseHire() HireRequest {
	return HireRequest{
		ExternalID: "cand-123",
		Email:      "nuevo@x.com",
		Name:       "Nuevo Profesional",
		CompanyID:  10,
	}
}

// hiredPro es un profesional que ya trabaja aquí, con el estado de inducción
// que el test necesite.
func hiredPro(id uint, email, obersuiteID, onboarding string) *models.User {
	return &models.User{
		ID: id, Email: email, ObersuiteID: obersuiteID,
		UserType: models.UserTypeProfessional, IsActive: true,
		OnboardingStatus: onboarding,
	}
}

// --- Identidad ----------------------------------------------------------------

// El motivo del campo: el email solo identifica mientras nadie lo cambie. Si el
// candidato se postuló con un correo y la contratación llega con otro, resolver
// por email partía a la misma persona en dos profesionales distintos.
func TestHire_ReconoceAlCandidatoAunqueCambieDeCorreo(t *testing.T) {
	existente := hiredPro(5, "viejo@x.com", "cand-123", models.OnboardingPassed)
	svc, userRepo, _, _, _ := newHireSvc(true, existente)

	req := baseHire()
	req.Email = "nuevo@x.com" // se cambió el correo entre postulación y alta

	result, err := svc.Hire(req)
	if err != nil {
		t.Fatalf("hire: %v", err)
	}
	if userRepo.created != nil {
		t.Fatalf("no debía crearse un profesional nuevo: %+v", userRepo.created)
	}
	if result.UserID != 5 {
		t.Fatalf("debía reconocerse al profesional 5, got %d", result.UserID)
	}
	if result.Status != "rehired" {
		t.Fatalf("status = %q, se esperaba rehired", result.Status)
	}
}

// El alta nueva guarda el id: es lo que permite identificar después quién llegó
// por el puente y de qué candidato venía.
func TestHire_GuardaElIdDeObersuiteEnElAlta(t *testing.T) {
	svc, userRepo, empRepo, _, _ := newHireSvc(true)

	result, err := svc.Hire(baseHire())
	if err != nil {
		t.Fatalf("hire: %v", err)
	}
	if userRepo.created == nil {
		t.Fatal("se esperaba un profesional nuevo")
	}
	if userRepo.created.ObersuiteID != "cand-123" {
		t.Fatalf("obersuite_id del usuario = %q", userRepo.created.ObersuiteID)
	}
	// También en el empleo: identifica ESTA contratación.
	if empRepo.updates["obersuite_id"] != "cand-123" {
		t.Fatalf("obersuite_id del empleo = %v", empRepo.updates["obersuite_id"])
	}
	// Y vuelve en la respuesta, para que Obersuite case su candidato.
	if result.ObersuiteID != "cand-123" {
		t.Fatalf("la respuesta debe devolver el id, got %q", result.ObersuiteID)
	}
}

// Quien ya estaba aquí antes de que existiera el id queda vinculado en la
// primera contratación que llega por el puente.
func TestHire_VinculaHaciaAtrasAQuienYaEstaba(t *testing.T) {
	existente := hiredPro(5, "nuevo@x.com", "", models.OnboardingPassed)
	svc, userRepo, _, _, _ := newHireSvc(true, existente)

	if _, err := svc.Hire(baseHire()); err != nil {
		t.Fatalf("hire: %v", err)
	}
	if userRepo.updates[5]["obersuite_id"] != "cand-123" {
		t.Fatalf("debía estamparse el id al profesional existente: %v", userRepo.updates[5])
	}
}

// Un id ya estampado no se pisa: la identidad original manda.
func TestHire_NoPisaUnIdDeObersuiteYaExistente(t *testing.T) {
	existente := hiredPro(5, "nuevo@x.com", "cand-original", models.OnboardingPassed)
	svc, userRepo, _, _, _ := newHireSvc(true, existente)

	req := baseHire()
	req.ExternalID = "cand-otro"

	if _, err := svc.Hire(req); err != nil {
		t.Fatalf("hire: %v", err)
	}
	if _, pisado := userRepo.updates[5]["obersuite_id"]; pisado {
		t.Fatalf("no debía reescribirse el id original: %v", userRepo.updates[5])
	}
}

// --- Correo de inducción ------------------------------------------------------

func TestHire_EmiteLaInduccionEnElAlta(t *testing.T) {
	svc, _, _, induction, auth := newHireSvc(true)

	result, err := svc.Hire(baseHire())
	if err != nil {
		t.Fatalf("hire: %v", err)
	}
	if len(induction.invited) != 1 {
		t.Fatalf("se esperaba una invitación de inducción, got %v", induction.invited)
	}
	if !result.InductionPending {
		t.Fatal("el resultado debe avisar que quedó pendiente de inducción")
	}
	// Con inducción encendida NO se manda además la bienvenida: serían dos
	// correos contradictorios (uno da acceso, el otro dice que aún no lo tiene).
	if len(auth.welcomeTo) != 0 {
		t.Fatalf("no debía mandarse la bienvenida: %v", auth.welcomeTo)
	}
}

// El agujero que motivó el cambio: la inducción solo se emitía al CREAR el
// profesional, así que quien volvía a ser contratado no la recibía nunca.
func TestHire_EmiteLaInduccionTambienAlRecontratar(t *testing.T) {
	existente := hiredPro(5, "nuevo@x.com", "cand-123", models.OnboardingPending)
	svc, _, _, induction, _ := newHireSvc(true, existente)

	result, err := svc.Hire(baseHire())
	if err != nil {
		t.Fatalf("hire: %v", err)
	}
	if len(induction.invited) != 1 || induction.invited[0] != 5 {
		t.Fatalf("la re-contratación debe emitir la inducción, got %v", induction.invited)
	}
	if !result.InductionPending {
		t.Fatal("el resultado debe avisar que quedó pendiente de inducción")
	}
}

// Quien agotó sus intentos y vuelve a ser contratado empieza de nuevo.
func TestHire_ReemiteLaInduccionAQuienQuedoBloqueado(t *testing.T) {
	existente := hiredPro(5, "nuevo@x.com", "cand-123", models.OnboardingBlocked)
	svc, _, _, induction, _ := newHireSvc(true, existente)

	if _, err := svc.Hire(baseHire()); err != nil {
		t.Fatalf("hire: %v", err)
	}
	if len(induction.invited) != 1 {
		t.Fatalf("un bloqueado recontratado debe recibir la inducción, got %v", induction.invited)
	}
}

// Reinvitar a quien ya aprobó le QUITARÍA el acceso que se ganó: InviteIfEnabled
// deja el estado en 'pending'.
func TestHire_NoReenviaLaInduccionAQuienYaAprobo(t *testing.T) {
	existente := hiredPro(5, "nuevo@x.com", "cand-123", models.OnboardingPassed)
	svc, _, _, induction, _ := newHireSvc(true, existente)

	result, err := svc.Hire(baseHire())
	if err != nil {
		t.Fatalf("hire: %v", err)
	}
	if len(induction.invited) != 0 {
		t.Fatalf("no debía reinvitarse a quien ya aprobó: %v", induction.invited)
	}
	if result.InductionPending {
		t.Fatal("no quedó pendiente de inducción: ya la aprobó")
	}
}

// Las cuentas anteriores a la inducción ('not_required') trabajan con
// normalidad: invitarlas las dejaría fuera de la plataforma sin avisar.
func TestHire_NoDejaSinAccesoAQuienNuncaTuvoQueHacerLaInduccion(t *testing.T) {
	existente := hiredPro(5, "nuevo@x.com", "cand-123", models.OnboardingNotRequired)
	svc, _, _, induction, _ := newHireSvc(true, existente)

	if _, err := svc.Hire(baseHire()); err != nil {
		t.Fatalf("hire: %v", err)
	}
	if len(induction.invited) != 0 {
		t.Fatalf("no debía invitarse a una cuenta que ya opera: %v", induction.invited)
	}
}

// Con la inducción apagada el alta sigue el flujo directo de siempre: el puente
// nunca se rompe por una inducción sin configurar.
func TestHire_ConInduccionApagadaMandaLaBienvenida(t *testing.T) {
	svc, _, _, _, auth := newHireSvc(false)

	result, err := svc.Hire(baseHire())
	if err != nil {
		t.Fatalf("hire: %v", err)
	}
	if len(auth.welcomeTo) != 1 || auth.welcomeTo[0] != "nuevo@x.com" {
		t.Fatalf("se esperaba el correo de bienvenida, got %v", auth.welcomeTo)
	}
	if result.InductionPending {
		t.Fatal("con la inducción apagada nada queda pendiente")
	}
}

// Un reintento del webhook (o un doble clic) no debe volver a escribirle al
// profesional: recibiría el mismo enlace una y otra vez.
func TestHire_ElReintentoDelWebhookNoReenviaCorreos(t *testing.T) {
	existente := hiredPro(5, "nuevo@x.com", "cand-123", models.OnboardingPending)
	svc, _, empRepo, induction, auth := newHireSvc(true, existente)
	empRepo.active = &models.Employment{ID: 77, UserID: 5, CompanyID: 10}

	result, err := svc.Hire(baseHire())
	if err != nil {
		t.Fatalf("hire: %v", err)
	}
	if result.Status != "already_active" {
		t.Fatalf("status = %q, se esperaba already_active", result.Status)
	}
	if len(induction.invited) != 0 || len(auth.welcomeTo) != 0 {
		t.Fatalf("un reintento no debe mandar correos: inducción=%v bienvenida=%v",
			induction.invited, auth.welcomeTo)
	}
}

// Los correos salen DESPUÉS de que el empleo existe: si la contratación se cae a
// mitad, nadie recibe un correo por un empleo que no llegó a crearse.
func TestHire_NoAvisaSiLaEmpresaNoEsValida(t *testing.T) {
	svc, _, _, induction, auth := newHireSvc(true)

	req := baseHire()
	req.CompanyID = 999 // no existe

	if _, err := svc.Hire(req); err == nil {
		t.Fatal("se esperaba error con una empresa inválida")
	}
	if len(induction.invited) != 0 || len(auth.welcomeTo) != 0 {
		t.Fatalf("no debía salir ningún correo: inducción=%v bienvenida=%v",
			induction.invited, auth.welcomeTo)
	}
}

// --- Ticket de incorporación --------------------------------------------------

// fakeHireTicketSvc registra las incorporaciones que se abrieron en Soporte.
type fakeHireTicketSvc struct {
	TicketService
	abiertos []ObersuiteHireInput
	err      error
}

func (f *fakeHireTicketSvc) CreateObersuiteHireAlert(in ObersuiteHireInput) error {
	if f.err != nil {
		return f.err
	}
	f.abiertos = append(f.abiertos, in)
	return nil
}

// El motivo del ticket: hasta ahora la contratación solo se veía en el panel de
// profesionales, donde no se mira salvo que se sepa que hay que mirar.
func TestHire_AbreElTicketDeIncorporacionEnSoporte(t *testing.T) {
	svc, _, _, _, _ := newHireSvc(true)
	tickets := &fakeHireTicketSvc{}
	svc.ticketSvc = tickets

	req := baseHire()
	req.JobTitle = "Backend Developer"
	req.PhoneNumber = "+58414700000"

	if _, err := svc.Hire(req); err != nil {
		t.Fatalf("hire: %v", err)
	}
	if len(tickets.abiertos) != 1 {
		t.Fatalf("se esperaba 1 ticket de incorporación, hubo %d", len(tickets.abiertos))
	}
	in := tickets.abiertos[0]
	if in.ProfessionalEmail != "nuevo@x.com" || in.JobTitle != "Backend Developer" {
		t.Fatalf("el ticket no lleva los datos del contratado: %+v", in)
	}
	if in.CompanyName != "Acme" {
		t.Fatalf("company = %q, se esperaba la empresa contratante", in.CompanyName)
	}
	// El texto del ticket cambia según esto: no es lo mismo "reenvíale la
	// capacitación" que "no se le envió ninguna".
	if !in.InductionSent {
		t.Fatal("con la inducción encendida el ticket debe saber que salió el correo")
	}
}

// Con la inducción apagada el alta igual se avisa: hay alguien nuevo al que
// acompañar aunque no haya capacitación que enviarle.
func TestHire_AvisaAunqueLaInduccionEsteApagada(t *testing.T) {
	svc, _, _, _, _ := newHireSvc(false)
	tickets := &fakeHireTicketSvc{}
	svc.ticketSvc = tickets

	if _, err := svc.Hire(baseHire()); err != nil {
		t.Fatalf("hire: %v", err)
	}
	if len(tickets.abiertos) != 1 {
		t.Fatalf("se esperaba 1 ticket, hubo %d", len(tickets.abiertos))
	}
	if tickets.abiertos[0].InductionSent {
		t.Fatal("sin inducción el ticket no debe decir que se envió")
	}
}

// El ticket es un aviso, no parte de la contratación: si falla, la persona ya
// está contratada y el puente no puede devolverle un error a Obersuite.
func TestHire_UnFalloDelTicketNoRompeLaContratacion(t *testing.T) {
	svc, _, _, _, _ := newHireSvc(true)
	svc.ticketSvc = &fakeHireTicketSvc{err: errors.New("soporte caído")}

	result, err := svc.Hire(baseHire())
	if err != nil {
		t.Fatalf("la contratación no debía fallar por el ticket: %v", err)
	}
	if result.UserID == 0 || result.EmploymentID == 0 {
		t.Fatalf("la contratación quedó a medias: %+v", result)
	}
}

func TestListCompanies_IncluyeAtributosCompletos(t *testing.T) {
	svc, userRepo, _, _, _ := newHireSvc(false)
	company := userRepo.byID[10] // Empresa Ejemplo
	company.CompanyName = "Empresa Ejemplo"
	company.Name = "Hermann Tschernko"
	company.Email = "empresa@ejemplo.com"
	company.Industry = "Construcción"
	company.Country = "España"
	company.State = "Sevilla"
	company.City = "Sevilla"
	company.Address = "Av. República Argentina 24"

	// nil = todas, que es como llama Obersuite cuando no sincroniza en incremental.
	companies, err := svc.ListCompanies(nil)
	if err != nil {
		t.Fatalf("ListCompanies error: %v", err)
	}
	if len(companies) != 1 {
		t.Fatalf("se esperaba 1 empresa, hubo %d", len(companies))
	}

	c := companies[0]
	if c.ID != 10 {
		t.Errorf("ID esperado 10, obtenido %d", c.ID)
	}
	if c.Name != "Empresa Ejemplo" {
		t.Errorf("Name esperado 'Empresa Ejemplo', obtenido %s", c.Name)
	}
	if c.Status != "active" {
		t.Errorf("Status esperado 'active', obtenido %s", c.Status)
	}
	if c.ResponsibleName != "Hermann Tschernko" {
		t.Errorf("ResponsibleName esperado 'Hermann Tschernko', obtenido %s", c.ResponsibleName)
	}
	if c.ResponsibleEmail != "empresa@ejemplo.com" {
		t.Errorf("ResponsibleEmail esperado 'empresa@ejemplo.com', obtenido %s", c.ResponsibleEmail)
	}
	if c.Industry != "Construcción" {
		t.Errorf("Industry esperada 'Construcción', obtenida %s", c.Industry)
	}
	if c.Country != "España" {
		t.Errorf("Country esperado 'España', obtenido %s", c.Country)
	}
	if c.State != "Sevilla" {
		t.Errorf("State esperado 'Sevilla', obtenido %s", c.State)
	}
	if c.City != "Sevilla" {
		t.Errorf("City esperada 'Sevilla', obtenida %s", c.City)
	}
	if c.Address != "Av. República Argentina 24" {
		t.Errorf("Address esperada 'Av. República Argentina 24', obtenida %s", c.Address)
	}
	if c.LastContact != "nunca" {
		t.Errorf("LastContact esperado 'nunca', obtenido %s", c.LastContact)
	}
}

func TestFormatLastContact(t *testing.T) {
	if got := FormatLastContact(nil); got != "nunca" {
		t.Errorf("esperado 'nunca', obtenido %q", got)
	}

	now := time.Now()
	if got := FormatLastContact(&now); got != "hoy" {
		t.Errorf("esperado 'hoy', obtenido %q", got)
	}

	ayer := now.AddDate(0, 0, -1)
	if got := FormatLastContact(&ayer); got != "ayer" {
		t.Errorf("esperado 'ayer', obtenido %q", got)
	}

	sieteDias := now.AddDate(0, 0, -7)
	if got := FormatLastContact(&sieteDias); got != "hace 7 días" {
		t.Errorf("esperado 'hace 7 días', obtenido %q", got)
	}

	unMes := now.AddDate(0, -1, 0)
	if got := FormatLastContact(&unMes); got != "hace 1 mes" {
		t.Errorf("esperado 'hace 1 mes', obtenido %q", got)
	}

	dosMeses := now.AddDate(0, -2, 0)
	if got := FormatLastContact(&dosMeses); got != "hace 2 meses" {
		t.Errorf("esperado 'hace 2 meses', obtenido %q", got)
	}

	unAno := now.AddDate(-1, 0, 0)
	if got := FormatLastContact(&unAno); got != "hace 1 año" {
		t.Errorf("esperado 'hace 1 año', obtenido %q", got)
	}

	dosAnos := now.AddDate(-2, 0, 0)
	if got := FormatLastContact(&dosAnos); got != "hace 2 años" {
		t.Errorf("esperado 'hace 2 años', obtenido %q", got)
	}
}
