package service

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
	"time"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
)

// --- Dobles ---

type tsUserRepo struct {
	repository.UserRepository
	users map[uint]*models.User
}

func (f *tsUserRepo) GetByID(id uint) (*models.User, error) {
	if u, ok := f.users[id]; ok {
		return u, nil
	}
	return nil, errors.New("not found")
}

type tsRepo struct {
	repository.TestimonialRepository
	created     *models.Testimonial
	byToken     map[string]*models.Testimonial
	byID        map[uint]*models.Testimonial
	lastUpdates map[string]interface{}
	hasPending  bool
}

func (f *tsRepo) Create(t *models.Testimonial) error {
	t.ID = 1
	f.created = t
	if f.byToken == nil {
		f.byToken = map[string]*models.Testimonial{}
	}
	f.byToken[t.Token] = t
	return nil
}

func (f *tsRepo) GetByToken(token string) (*models.Testimonial, error) {
	if t, ok := f.byToken[token]; ok {
		return t, nil
	}
	return nil, errors.New("not found")
}

func (f *tsRepo) GetByID(id uint) (*models.Testimonial, error) {
	if t, ok := f.byID[id]; ok {
		return t, nil
	}
	return nil, errors.New("not found")
}

func (f *tsRepo) Update(_ uint, updates map[string]interface{}) error {
	f.lastUpdates = updates
	return nil
}

func (f *tsRepo) HasPendingForUser(_ uint, _ string) (bool, error) { return f.hasPending, nil }

// tsNotif captura las campanitas emitidas.
type tsNotif struct {
	NotificationService
	sent []tsNotifCall
}

type tsNotifCall struct {
	userID           uint
	kind, title, msg string
	data             map[string]interface{}
}

func (f *tsNotif) CreateNotification(userID uint, kind, title, msg string, data map[string]interface{}) error {
	f.sent = append(f.sent, tsNotifCall{userID: userID, kind: kind, title: title, msg: msg, data: data})
	return nil
}

// tsEmploymentRepo captura las notas archivadas en el expediente del empleo.
type tsEmploymentRepo struct {
	repository.EmploymentRepository
	active []models.Employment
	notes  []models.EmploymentNote
	err    error
}

func (f *tsEmploymentRepo) ListActiveByUser(_ uint) ([]models.Employment, error) {
	return f.active, f.err
}

// ListByUser devuelve todos los empleos, activos y terminados: es lo que mira
// el archivado desde que un empleo cerrado también sirve de expediente.
func (f *tsEmploymentRepo) ListByUser(_ uint) ([]models.Employment, error) {
	return f.active, f.err
}

func (f *tsEmploymentRepo) CreateNote(n *models.EmploymentNote) error {
	f.notes = append(f.notes, *n)
	return nil
}

// tsAdminRepo captura las entradas archivadas en el expediente del tenant.
type tsAdminRepo struct {
	repository.AdminRepository
	events []models.CompanyEvent
}

func (f *tsAdminRepo) CreateCompanyEvent(e *models.CompanyEvent) error {
	f.events = append(f.events, *e)
	return nil
}

// tsDeps agrupa los dobles de los dos expedientes.
type tsDeps struct {
	employment *tsEmploymentRepo
	admin      *tsAdminRepo
	notif      *tsNotif
}

// newTestimonialSvcForTest arma el servicio con dobles y un directorio de
// subidas temporal, que es donde acaba el trazo de la firma.
func newTestimonialSvcForTest(t *testing.T, repo *tsRepo, users map[uint]*models.User) TestimonialService {
	t.Helper()
	svc, _ := newTestimonialSvcFull(t, repo, users)
	return svc
}

// newTestimonialSvcWithNotif es igual pero conserva el doble de notificaciones,
// para poder inspeccionar lo que se envió.
func newTestimonialSvcWithNotif(t *testing.T, repo *tsRepo, users map[uint]*models.User) (TestimonialService, *tsNotif) {
	t.Helper()
	svc, deps := newTestimonialSvcFull(t, repo, users)
	return svc, deps.notif
}

// newTestimonialSvcFull devuelve el servicio junto con TODOS sus dobles.
func newTestimonialSvcFull(t *testing.T, repo *tsRepo, users map[uint]*models.User) (TestimonialService, tsDeps) {
	t.Helper()
	deps := tsDeps{
		employment: &tsEmploymentRepo{},
		admin:      &tsAdminRepo{},
		notif:      &tsNotif{},
	}
	svc := NewTestimonialService(
		repo, &tsUserRepo{users: users}, deps.employment, deps.admin,
		nil, deps.notif, t.TempDir(), "https://obertrack.test",
	)
	return svc, deps
}

// signaturePNG genera un PNG diminuto y lo devuelve como data URL, igual que
// haría canvas.toDataURL en el navegador.
func signaturePNG(t *testing.T) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 2))
	img.Set(1, 1, color.Black)
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("no se pudo generar el PNG de prueba: %v", err)
	}
	return signaturePNGPrefix + base64.StdEncoding.EncodeToString(buf.Bytes())
}

func validSubmission(t *testing.T) TestimonialSubmission {
	t.Helper()
	return TestimonialSubmission{
		Rating:          5,
		Quote:           strings.Repeat("Una experiencia excelente de principio a fin. ", 2),
		ConsentAccepted: true,
		SignatureName:   "Laura Méndez",
		SignatureImage:  signaturePNG(t),
		IP:              "203.0.113.7",
		UserAgent:       "Mozilla/5.0",
	}
}

// --- Pedir ---

// El cargo, la empresa y el texto de consentimiento se copian al emitir. Si
// mañana la persona cambia de puesto, el testimonio ya firmado sigue diciendo
// lo que decía cuando lo firmó: eso es lo que hace válido el permiso.
func TestRequest_CongelaIdentidadYConsentimiento(t *testing.T) {
	user := &models.User{
		ID: 7, Name: "Laura Méndez", Email: "laura@acme.test",
		UserType: models.UserTypeProfessional, JobTitle: "Diseñadora", CompanyName: "Acme S.A.",
	}
	repo := &tsRepo{}
	svc := newTestimonialSvcForTest(t, repo, map[uint]*models.User{7: user})

	got, err := svc.Request(TestimonialRequestInput{UserID: 7, Audience: models.TestimonialFromProfessional}, 99)
	if err != nil {
		t.Fatalf("Request falló: %v", err)
	}

	// El perfil cambia DESPUÉS de emitir la solicitud.
	user.JobTitle = "Directora"
	user.CompanyName = "Otra Empresa"

	if got.RecipientRole != "Diseñadora" || got.RecipientCompany != "Acme S.A." {
		t.Fatalf("la identidad debía quedar congelada; got=%q / %q", got.RecipientRole, got.RecipientCompany)
	}
	tpl, _ := testimonialTemplateFor(models.TestimonialFromProfessional)
	if got.ConsentText != tpl.ConsentText {
		t.Fatal("el texto de consentimiento debe copiarse en la solicitud, no leerse después")
	}
	if got.ConsentVersion != TestimonialConsentVersion {
		t.Fatalf("falta la versión del consentimiento; got=%q", got.ConsentVersion)
	}
	if got.Token == "" || got.Status != models.TestimonialPending {
		t.Fatalf("solicitud mal inicializada: token=%q status=%q", got.Token, got.Status)
	}
}

// Pedirle a un profesional que firme "en representación de la empresa" —o al
// revés— invalidaría el consentimiento, así que se rechaza al emitir.
func TestRequest_RechazaAudienciaQueNoCasaConLaCuenta(t *testing.T) {
	prof := &models.User{ID: 7, Name: "Laura", Email: "laura@acme.test", UserType: models.UserTypeProfessional}
	repo := &tsRepo{}
	svc := newTestimonialSvcForTest(t, repo, map[uint]*models.User{7: prof})

	if _, err := svc.Request(TestimonialRequestInput{UserID: 7, Audience: models.TestimonialFromCompany}, 1); err == nil {
		t.Fatal("un profesional no debe poder firmar el consentimiento de empresa")
	}
}

func TestRequest_NoDuplicaUnaSolicitudViva(t *testing.T) {
	prof := &models.User{ID: 7, Name: "Laura", Email: "laura@acme.test", UserType: models.UserTypeProfessional}
	repo := &tsRepo{hasPending: true}
	svc := newTestimonialSvcForTest(t, repo, map[uint]*models.User{7: prof})

	if _, err := svc.Request(TestimonialRequestInput{UserID: 7, Audience: models.TestimonialFromProfessional}, 1); err == nil {
		t.Fatal("con una solicitud pendiente hay que reenviar, no crear otra")
	}
}

// A quien le pedimos el testimonio SIEMPRE es usuario de la plataforma, así que
// el aviso no puede quedarse solo en el correo: tiene que llegarle también a la
// campanita, y con un enlace que lleve directo a la página de firma.
func TestRequest_AvisaPorCampanitaAQuienDebeEscribir(t *testing.T) {
	user := &models.User{
		ID: 7, Name: "Laura Méndez", Email: "laura@acme.test",
		UserType: models.UserTypeProfessional,
	}
	repo := &tsRepo{}
	svc, notif := newTestimonialSvcWithNotif(t, repo, map[uint]*models.User{7: user})

	got, err := svc.Request(TestimonialRequestInput{UserID: 7, Audience: models.TestimonialFromProfessional}, 99)
	if err != nil {
		t.Fatalf("Request falló: %v", err)
	}
	if len(notif.sent) != 1 {
		t.Fatalf("se esperaba una campanita al destinatario; got=%d", len(notif.sent))
	}
	sent := notif.sent[0]
	if sent.userID != 7 {
		t.Fatalf("la campanita debía ir a quien escribe (7); got=%d", sent.userID)
	}
	// Sin enlace el aviso se lee pero no lleva a ninguna parte, y el enlace solo
	// sirve si trae el token: es la misma puerta que la del correo.
	if sent.data["link"] != "/testimonio/"+got.Token {
		t.Fatalf("el enlace debe apuntar a la página de firma con el token; got=%v", sent.data["link"])
	}
}

// El reenvío es un recordatorio: vuelve a sonar la campanita, y con el token
// NUEVO (el anterior queda muerto al reenviar).
func TestResend_VuelveAAvisarConElTokenNuevo(t *testing.T) {
	repo, tok := pendingTestimonial()
	svc, notif := newTestimonialSvcWithNotif(t, repo, nil)

	if err := svc.Resend(1); err != nil {
		t.Fatalf("Resend falló: %v", err)
	}
	if len(notif.sent) != 1 {
		t.Fatalf("se esperaba un recordatorio; got=%d", len(notif.sent))
	}
	link, _ := notif.sent[0].data["link"].(string)
	if link == "/testimonio/"+tok {
		t.Fatal("el recordatorio no debe llevar el token viejo: al reenviar se emite uno nuevo")
	}
	newToken, _ := repo.lastUpdates["token"].(string)
	if link != "/testimonio/"+newToken {
		t.Fatalf("el enlace debe traer el token recién emitido; got=%q", link)
	}
}

// La otra mitad: cuando la persona firma, quien lo pidió se entera por la
// campanita y aterriza en la bandeja de revisión.
func TestSubmit_AvisaAQuienLoPidio(t *testing.T) {
	repo, tok := pendingTestimonial()
	svc, notif := newTestimonialSvcWithNotif(t, repo, nil)

	if err := svc.Submit(tok, validSubmission(t)); err != nil {
		t.Fatalf("Submit falló: %v", err)
	}
	if len(notif.sent) != 1 {
		t.Fatalf("se esperaba una campanita al revisor; got=%d", len(notif.sent))
	}
	sent := notif.sent[0]
	if sent.userID != 99 {
		t.Fatalf("la campanita debía ir a quien pidió el testimonio (99); got=%d", sent.userID)
	}
	if sent.data["link"] != "/testimonios" {
		t.Fatalf("el revisor debe aterrizar en la bandeja; got=%v", sent.data["link"])
	}
}

// --- Pedir a varias personas ---

// tsRepoMulti recuerda TODOS los testimonios creados (el doble normal solo
// guarda el último) y permite decidir quién tiene ya una solicitud viva.
type tsRepoMulti struct {
	tsRepo
	creados    []models.Testimonial
	conPendien map[uint]bool
}

func (f *tsRepoMulti) Create(t *models.Testimonial) error {
	t.ID = uint(len(f.creados) + 1)
	f.creados = append(f.creados, *t)
	return nil
}

func (f *tsRepoMulti) HasPendingForUser(userID uint, _ string) (bool, error) {
	return f.conPendien[userID], nil
}

// Un lote es un éxito PARCIAL por naturaleza: siempre habrá alguien con una
// solicitud viva, sin correo o del tipo de cuenta que no toca. Que esas personas
// queden fuera no puede impedir que el resto reciba la suya.
func TestRequestMany_SigueAunqueAlgunasFallen(t *testing.T) {
	repo := &tsRepoMulti{conPendien: map[uint]bool{2: true}}
	users := map[uint]*models.User{
		1: {ID: 1, Name: "Ana", Email: "ana@test.invalid", UserType: models.UserTypeProfessional},
		2: {ID: 2, Name: "Bruno", Email: "bruno@test.invalid", UserType: models.UserTypeProfessional},
		3: {ID: 3, Name: "Cora", Email: "", UserType: models.UserTypeProfessional},
		4: {ID: 4, Name: "Empresa", Email: "emp@test.invalid", UserType: models.UserTypeEmployer},
		5: {ID: 5, Name: "Elena", Email: "elena@test.invalid", UserType: models.UserTypeProfessional},
	}
	svc := NewTestimonialService(repo, &tsUserRepo{users: users}, nil, nil, nil, nil, t.TempDir(), "https://obertrack.test")

	res, err := svc.RequestMany(
		[]uint{1, 2, 3, 4, 5, 99},
		TestimonialRequestInput{Audience: models.TestimonialFromProfessional},
		77,
	)
	if err != nil {
		t.Fatalf("un lote con fallos NO debe fallar entero: %v", err)
	}

	// Ana y Elena sí; Bruno (ya tiene una), Cora (sin correo), la empresa (tipo
	// que no casa) y el 99 (no existe) se quedan fuera.
	if res.Sent != 2 || res.Skipped != 4 {
		t.Fatalf("esperaba 2 enviadas y 4 omitidas; got sent=%d skipped=%d", res.Sent, res.Skipped)
	}
	if len(res.Outcomes) != 6 {
		t.Fatalf("debe haber una línea por persona; got=%d", len(res.Outcomes))
	}
	for _, o := range res.Outcomes {
		if !o.Sent && o.Reason == "" {
			t.Errorf("a %q se le omitió sin decir por qué", o.Name)
		}
		if !o.Sent && o.Name == "" {
			t.Error("una omisión sin nombre es una lista de números")
		}
	}
}

// La misma persona seleccionada dos veces recibiría dos correos, y el segundo
// chocaría con el primero.
func TestRequestMany_IgnoraDuplicados(t *testing.T) {
	repo := &tsRepoMulti{}
	users := map[uint]*models.User{
		1: {ID: 1, Name: "Ana", Email: "ana@test.invalid", UserType: models.UserTypeProfessional},
	}
	svc := NewTestimonialService(repo, &tsUserRepo{users: users}, nil, nil, nil, nil, t.TempDir(), "https://obertrack.test")

	res, err := svc.RequestMany([]uint{1, 1, 1},
		TestimonialRequestInput{Audience: models.TestimonialFromProfessional}, 77)
	if err != nil {
		t.Fatalf("RequestMany falló: %v", err)
	}
	if res.Sent != 1 || len(res.Outcomes) != 1 {
		t.Fatalf("una persona repetida es UNA solicitud; got sent=%d outcomes=%d", res.Sent, len(res.Outcomes))
	}
	if len(repo.creados) != 1 {
		t.Fatalf("no debía crearse más de un testimonio; got=%d", len(repo.creados))
	}
}

// Cada persona recibe su propio enlace: un token compartido dejaría que quien
// firmara primero hablara por los demás.
func TestRequestMany_CadaPersonaConSuPropioEnlace(t *testing.T) {
	repo := &tsRepoMulti{}
	users := map[uint]*models.User{
		1: {ID: 1, Name: "Ana", Email: "ana@test.invalid", UserType: models.UserTypeProfessional},
		2: {ID: 2, Name: "Bruno", Email: "bruno@test.invalid", UserType: models.UserTypeProfessional},
	}
	svc := NewTestimonialService(repo, &tsUserRepo{users: users}, nil, nil, nil, nil, t.TempDir(), "https://obertrack.test")

	if _, err := svc.RequestMany([]uint{1, 2},
		TestimonialRequestInput{Audience: models.TestimonialFromProfessional}, 77); err != nil {
		t.Fatalf("RequestMany falló: %v", err)
	}
	if len(repo.creados) != 2 {
		t.Fatalf("esperaba 2 testimonios; got=%d", len(repo.creados))
	}
	a, b := repo.creados[0], repo.creados[1]
	if a.Token == "" || a.Token == b.Token {
		t.Fatal("cada solicitud necesita su propio token")
	}
	if a.UserID == b.UserID {
		t.Fatal("cada solicitud debe apuntar a su persona")
	}
}

func TestRequestMany_RechazaListaVaciaYLotesEnormes(t *testing.T) {
	svc := NewTestimonialService(&tsRepoMulti{}, &tsUserRepo{}, nil, nil, nil, nil, t.TempDir(), "")

	if _, err := svc.RequestMany(nil, TestimonialRequestInput{Audience: models.TestimonialFromProfessional}, 1); err == nil {
		t.Error("una lista vacía no tiene nada que enviar")
	}
	enorme := make([]uint, maxBulkTestimonials+1)
	for i := range enorme {
		enorme[i] = uint(i + 1)
	}
	if _, err := svc.RequestMany(enorme, TestimonialRequestInput{Audience: models.TestimonialFromProfessional}, 1); err == nil {
		t.Error("un lote por encima del tope debía rechazarse ANTES de mandar nada")
	}
}

// --- Firmar ---

func pendingTestimonial() (*tsRepo, string) {
	tpl, _ := testimonialTemplateFor(models.TestimonialFromProfessional)
	tok := "tok-de-prueba"
	item := &models.Testimonial{
		ID: 1, Token: tok, Audience: models.TestimonialFromProfessional,
		Status: models.TestimonialPending, UserID: 7, RequestedBy: 99,
		RecipientName: "Laura Méndez", RecipientEmail: "laura@acme.test",
		ConsentText: tpl.ConsentText, ConsentVersion: TestimonialConsentVersion,
		ExpiresAt: time.Now().AddDate(0, 0, 30),
	}
	return &tsRepo{
		byToken: map[string]*models.Testimonial{tok: item},
		byID:    map[uint]*models.Testimonial{1: item},
	}, tok
}

func TestSubmit_GuardaLaEvidenciaDelServidor(t *testing.T) {
	repo, tok := pendingTestimonial()
	svc := newTestimonialSvcForTest(t, repo, nil)

	if err := svc.Submit(tok, validSubmission(t)); err != nil {
		t.Fatalf("Submit falló: %v", err)
	}
	u := repo.lastUpdates
	if u["status"] != models.TestimonialSubmitted {
		t.Fatalf("el testimonio debía quedar en revisión; got=%v", u["status"])
	}
	if u["signer_ip"] != "203.0.113.7" {
		t.Fatalf("la IP es evidencia y debe guardarse; got=%v", u["signer_ip"])
	}
	if u["signer_user_agent"] != "Mozilla/5.0" {
		t.Fatalf("el navegador es evidencia y debe guardarse; got=%v", u["signer_user_agent"])
	}
	if u["signed_at"] == nil {
		t.Fatal("falta la marca de tiempo de la firma")
	}
	if name, _ := u["signature_image"].(string); name == "" {
		t.Fatal("el trazo de la firma debía quedar guardado")
	}
}

// Sin la casilla de autorización no hay permiso, y un testimonio sin permiso no
// sirve para nada: se rechaza el envío entero en lugar de guardarlo a medias.
func TestSubmit_ExigeConsentimiento(t *testing.T) {
	repo, tok := pendingTestimonial()
	svc := newTestimonialSvcForTest(t, repo, nil)

	in := validSubmission(t)
	in.ConsentAccepted = false
	if err := svc.Submit(tok, in); err == nil {
		t.Fatal("sin autorización no se debe aceptar el testimonio")
	}
	if repo.lastUpdates != nil {
		t.Fatal("un envío rechazado no debe escribir nada")
	}
}

func TestSubmit_ExigeFirma(t *testing.T) {
	for _, tc := range []struct {
		name  string
		image string
	}{
		{"vacía", ""},
		{"no es un data URL PNG", "data:image/jpeg;base64,AAAA"},
		{"dice PNG pero no lo es", signaturePNGPrefix + base64.StdEncoding.EncodeToString([]byte("no soy un png"))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo, tok := pendingTestimonial()
			svc := newTestimonialSvcForTest(t, repo, nil)

			in := validSubmission(t)
			in.SignatureImage = tc.image
			if err := svc.Submit(tok, in); err == nil {
				t.Fatal("debía rechazarse la firma")
			}
			if repo.lastUpdates != nil {
				t.Fatal("un envío rechazado no debe escribir nada")
			}
		})
	}
}

// NO hay largo mínimo, y es deliberado: un testimonio sincero puede ser una
// sola frase. Poner un suelo obliga a rellenar hasta llegar a la cuenta, que es
// justo lo que hace sonar falso un testimonio. Lo único que se exige es que
// haya texto.
func TestSubmit_AceptaUnTestimonioCorto(t *testing.T) {
	repo, tok := pendingTestimonial()
	svc := newTestimonialSvcForTest(t, repo, nil)

	in := validSubmission(t)
	in.Quote = "Excelente, me encanta la app."
	if err := svc.Submit(tok, in); err != nil {
		t.Fatalf("un testimonio corto es válido: %v", err)
	}
	if repo.lastUpdates["quote"] != "Excelente, me encanta la app." {
		t.Fatalf("la cita no se guardó tal cual; got=%v", repo.lastUpdates["quote"])
	}
}

func TestSubmit_RechazaUnTestimonioVacio(t *testing.T) {
	for _, quote := range []string{"", "   \n\t  "} {
		repo, tok := pendingTestimonial()
		svc := newTestimonialSvcForTest(t, repo, nil)

		in := validSubmission(t)
		in.Quote = quote
		if err := svc.Submit(tok, in); err == nil {
			t.Fatalf("un testimonio vacío (%q) debía rechazarse", quote)
		}
		if repo.lastUpdates != nil {
			t.Fatal("un envío rechazado no debe escribir nada")
		}
	}
}

// El enlace es de un solo uso: quien ya firmó no puede sobrescribir lo que
// dijo, ni un reenvío del correo abre una segunda puerta.
func TestSubmit_NoSeResponderDosVeces(t *testing.T) {
	repo, tok := pendingTestimonial()
	repo.byToken[tok].Status = models.TestimonialSubmitted
	svc := newTestimonialSvcForTest(t, repo, nil)

	if err := svc.Submit(tok, validSubmission(t)); err == nil {
		t.Fatal("un testimonio ya enviado no debe poder reescribirse")
	}
}

func TestSubmit_RechazaEnlaceVencido(t *testing.T) {
	repo, tok := pendingTestimonial()
	repo.byToken[tok].ExpiresAt = time.Now().AddDate(0, 0, -1)
	svc := newTestimonialSvcForTest(t, repo, nil)

	if err := svc.Submit(tok, validSubmission(t)); err == nil {
		t.Fatal("un enlace vencido no debe aceptar firmas")
	}
}

// Cómo se firmó es evidencia: la constancia lo dice y quien la lea tiene que
// poder juzgarlo. Así que la modalidad se valida y se guarda, no se acepta lo
// que venga.
func TestSubmit_GuardaLaModalidadDeFirma(t *testing.T) {
	for _, modo := range []string{models.SignatureDrawn, models.SignatureUploaded, models.SignatureTyped} {
		repo, tok := pendingTestimonial()
		svc := newTestimonialSvcForTest(t, repo, nil)

		in := validSubmission(t)
		in.SignatureMode = modo
		if err := svc.Submit(tok, in); err != nil {
			t.Fatalf("modalidad %q debía aceptarse: %v", modo, err)
		}
		if repo.lastUpdates["signature_mode"] != modo {
			t.Errorf("modalidad %q no se guardó; got=%v", modo, repo.lastUpdates["signature_mode"])
		}
	}
}

// Un navegador anterior a las tres modalidades no manda el campo. Antes solo se
// podía trazar, así que esa es la lectura correcta —y no un error que deje a
// alguien sin poder firmar—.
func TestSubmit_SinModalidadAsumeTrazo(t *testing.T) {
	repo, tok := pendingTestimonial()
	svc := newTestimonialSvcForTest(t, repo, nil)

	in := validSubmission(t)
	in.SignatureMode = ""
	if err := svc.Submit(tok, in); err != nil {
		t.Fatalf("Submit falló: %v", err)
	}
	if repo.lastUpdates["signature_mode"] != models.SignatureDrawn {
		t.Fatalf("debía asumirse trazo; got=%v", repo.lastUpdates["signature_mode"])
	}
}

func TestSubmit_RechazaUnaModalidadInventada(t *testing.T) {
	repo, tok := pendingTestimonial()
	svc := newTestimonialSvcForTest(t, repo, nil)

	in := validSubmission(t)
	in.SignatureMode = "huella-dactilar"
	if err := svc.Submit(tok, in); err == nil {
		t.Fatal("una modalidad desconocida debía rechazarse")
	}
	if repo.lastUpdates != nil {
		t.Fatal("un envío rechazado no debe escribir nada")
	}
}

// La etiqueta es lo que acaba impreso en la constancia.
func TestSignatureModeLabel(t *testing.T) {
	casos := map[string]string{
		models.SignatureDrawn:    "Trazada a mano por el firmante",
		models.SignatureUploaded: "Imagen de firma cargada por el firmante",
		models.SignatureTyped:    "Nombre escrito con una tipografía",
		// Los testimonios anteriores no tienen modalidad: entonces solo se podía
		// trazar, así que se describen como tal.
		"": "Trazada a mano por el firmante",
	}
	for modo, quiero := range casos {
		got := (&models.Testimonial{SignatureMode: modo}).SignatureModeLabel()
		if got != quiero {
			t.Errorf("modo %q -> %q, se esperaba %q", modo, got, quiero)
		}
	}
}

// --- Devolver para corregir ---

// Devolver es distinto de descartar: deja el testimonio abierto esperando a su
// autor, con el enlace reabierto y un plazo nuevo.
func TestRequestChanges_ReabreElEnlaceConMotivo(t *testing.T) {
	repo, tok := pendingTestimonial()
	item := repo.byID[1]
	item.Status = models.TestimonialSubmitted
	svc, notif := newTestimonialSvcWithNotif(t, repo, nil)

	if err := svc.RequestChanges(1, "  La firma salió cortada, vuelve a trazarla  ", 99); err != nil {
		t.Fatalf("RequestChanges falló: %v", err)
	}
	u := repo.lastUpdates
	if u["status"] != models.TestimonialChangesRequested {
		t.Fatalf("debía quedar esperando corrección; got=%v", u["status"])
	}
	if u["change_reason"] != "La firma salió cortada, vuelve a trazarla" {
		t.Fatalf("el motivo debía guardarse sin espacios sobrantes; got=%v", u["change_reason"])
	}
	if u["revisions"] != 1 {
		t.Fatalf("debía contarse la vuelta; got=%v", u["revisions"])
	}
	// Token nuevo: el enlace anterior muere, como en el reenvío.
	newToken, _ := u["token"].(string)
	if newToken == "" || newToken == tok {
		t.Fatalf("debía emitirse un token nuevo; got=%q", newToken)
	}
	// Devolver algo con el plazo a punto de vencer sería pedir una corrección
	// imposible.
	exp, ok := u["expires_at"].(time.Time)
	if !ok || exp.Before(time.Now().AddDate(0, 0, 1)) {
		t.Fatalf("el plazo debía renovarse; got=%v", u["expires_at"])
	}
	if len(notif.sent) != 1 || notif.sent[0].data["link"] != "/testimonio/"+newToken {
		t.Fatalf("debía avisarse a su autor con el enlace nuevo; got=%+v", notif.sent)
	}
}

func TestRequestChanges_ExigeMotivo(t *testing.T) {
	repo, _ := pendingTestimonial()
	repo.byID[1].Status = models.TestimonialSubmitted
	svc := newTestimonialSvcForTest(t, repo, nil)

	if err := svc.RequestChanges(1, "   ", 99); err == nil {
		t.Fatal("sin motivo no hay nada que corregir: debía rechazarse")
	}
	if repo.lastUpdates != nil {
		t.Fatal("una devolución rechazada no debe escribir nada")
	}
}

// Reabrir uno ya aprobado exigiría además retirarlo del expediente donde se
// archivó, así que no se permite: se descarta y se pide de nuevo.
func TestRequestChanges_SoloSobreLoQueEsperaRevision(t *testing.T) {
	for _, estado := range []string{
		models.TestimonialPending,
		models.TestimonialApproved,
		models.TestimonialRejected,
		models.TestimonialChangesRequested,
	} {
		repo, _ := pendingTestimonial()
		repo.byID[1].Status = estado
		svc := newTestimonialSvcForTest(t, repo, nil)

		if err := svc.RequestChanges(1, "Corrige el nombre", 99); err == nil {
			t.Errorf("no debía poder devolverse un testimonio en estado %q", estado)
		}
	}
}

// Al corregir, la página tiene que devolverle lo que ya escribió: nadie debería
// reescribir su testimonio entero por una errata.
func TestLanding_EnCorreccionPrecargaLoEscrito(t *testing.T) {
	repo, tok := pendingTestimonial()
	item := repo.byToken[tok]
	item.Status = models.TestimonialChangesRequested
	item.ChangeReason = "Tu nombre quedó incompleto"
	item.Quote = "Excelente servicio."
	item.Rating = 5
	item.SignatureName = "Laura"
	item.AllowPublicName = true
	svc := newTestimonialSvcForTest(t, repo, nil)

	view, err := svc.Landing(tok)
	if err != nil {
		t.Fatalf("Landing falló: %v", err)
	}
	if view.ChangeReason != "Tu nombre quedó incompleto" {
		t.Fatalf("debía verse el motivo; got=%q", view.ChangeReason)
	}
	if view.Previous == nil {
		t.Fatal("debía venir el borrador para precargar")
	}
	if view.Previous.Quote != "Excelente servicio." || view.Previous.Rating != 5 {
		t.Fatalf("el borrador no trae lo escrito; got=%+v", view.Previous)
	}
	if !view.Previous.AllowPublicName {
		t.Fatal("los permisos ya elegidos también se precargan")
	}
}

// Fuera de una corrección no hay nada que precargar, y devolver el contenido
// sería filtrarlo sin motivo.
func TestLanding_SinCorreccionNoDevuelveBorrador(t *testing.T) {
	for _, estado := range []string{
		models.TestimonialPending,
		models.TestimonialSubmitted,
		models.TestimonialApproved,
	} {
		repo, tok := pendingTestimonial()
		item := repo.byToken[tok]
		item.Status = estado
		item.Quote = "Texto que no debe salir"
		svc := newTestimonialSvcForTest(t, repo, nil)

		view, err := svc.Landing(tok)
		if err != nil {
			t.Fatalf("Landing falló en %q: %v", estado, err)
		}
		if view.Previous != nil {
			t.Errorf("en estado %q no debía devolverse el borrador", estado)
		}
	}
}

// La corrección vuelve a firmar. La firma vieja autorizaba un texto que pudo
// cambiar, así que deja de valer — pero aquel acto de firma ocurrió y no puede
// desaparecer sin rastro.
func TestSubmit_LaCorreccionApartaLaFirmaAnterior(t *testing.T) {
	repo, tok := pendingTestimonial()
	item := repo.byToken[tok]
	item.Status = models.TestimonialChangesRequested
	item.ChangeReason = "La firma salió cortada"
	item.Quote = "Texto viejo."
	item.SignatureName = "Laura M."
	item.SignatureImage = "vieja.png"
	item.SignerIP = "198.51.100.9"
	firmado := time.Now().Add(-48 * time.Hour)
	item.SignedAt = &firmado
	svc := newTestimonialSvcForTest(t, repo, nil)

	in := validSubmission(t)
	in.Quote = "Texto corregido y mucho mejor."
	if err := svc.Submit(tok, in); err != nil {
		t.Fatalf("la corrección debía aceptarse: %v", err)
	}

	u := repo.lastUpdates
	if u["status"] != models.TestimonialSubmitted {
		t.Fatalf("debía volver a revisión; got=%v", u["status"])
	}
	// El motivo ya se atendió: dejarlo haría que la página se lo siguiera
	// enseñando a su autor como si estuviera pendiente.
	if u["change_reason"] != "" {
		t.Fatalf("el motivo debía limpiarse; got=%v", u["change_reason"])
	}
	if u["signature_name"] != "Laura Méndez" {
		t.Fatalf("debía quedar la firma nueva; got=%v", u["signature_name"])
	}

	var trail []signatureRecord
	if err := json.Unmarshal([]byte(u["signature_trail"].(string)), &trail); err != nil {
		t.Fatalf("el rastro de firmas no es JSON válido: %v", err)
	}
	if len(trail) != 1 {
		t.Fatalf("debía apartarse una firma; got=%d", len(trail))
	}
	if trail[0].Name != "Laura M." || trail[0].IP != "198.51.100.9" {
		t.Fatalf("el rastro no conserva la evidencia anterior; got=%+v", trail[0])
	}
	if trail[0].Quote != "Texto viejo." {
		t.Fatalf("el rastro debe decir qué texto autorizaba esa firma; got=%q", trail[0].Quote)
	}
	if trail[0].Reason != "La firma salió cortada" {
		t.Fatalf("el rastro debe decir por qué se devolvió; got=%q", trail[0].Reason)
	}
}

// --- Revisar ---

func TestReview_NoApruebaLoQueNadieRespondio(t *testing.T) {
	repo, _ := pendingTestimonial()
	svc := newTestimonialSvcForTest(t, repo, nil)

	if _, err := svc.Review(1, TestimonialReviewInput{Approve: true}, 99); err == nil {
		t.Fatal("no se puede aprobar un testimonio sin texto ni firma")
	}
}

// La cita original es evidencia: editarla para publicar guarda una copia aparte
// y deja intacto lo que la persona escribió.
func TestReview_NoPisaElOriginalAlEditar(t *testing.T) {
	repo, _ := pendingTestimonial()
	item := repo.byID[1]
	item.Status = models.TestimonialSubmitted
	item.Quote = "Texto original tal como lo escribió."
	svc := newTestimonialSvcForTest(t, repo, nil)

	_, err := svc.Review(1, TestimonialReviewInput{Approve: true, PublishedQuote: "Texto recortado."}, 99)
	if err != nil {
		t.Fatalf("Review falló: %v", err)
	}
	if _, ok := repo.lastUpdates["quote"]; ok {
		t.Fatal("la revisión nunca debe reescribir la cita original")
	}
	if repo.lastUpdates["published_quote"] != "Texto recortado." {
		t.Fatalf("falta la versión editada; got=%v", repo.lastUpdates["published_quote"])
	}
	if repo.lastUpdates["status"] != models.TestimonialApproved {
		t.Fatalf("el estado debía quedar aprobado; got=%v", repo.lastUpdates["status"])
	}
}

// El mejor momento para pedir un testimonio es justo cuando alguien termina su
// etapa —y entonces ya no le queda ningún empleo activo—. Su expediente sigue
// existiendo y es exactamente donde esto pertenece, así que se archiva ahí.
func TestReview_ArchivaAunqueElEmpleoYaTermino(t *testing.T) {
	repo, _ := pendingTestimonial()
	item := repo.byID[1]
	item.Status = models.TestimonialSubmitted
	item.Audience = models.TestimonialFromProfessional
	item.Quote = "Fue una gran etapa."

	svc, deps := newTestimonialSvcFull(t, repo, nil)
	deps.employment.active = []models.Employment{
		{ID: 40, UserID: 7, Status: models.EmploymentEnded, StartedAt: time.Now().AddDate(-2, 0, 0)},
		{ID: 41, UserID: 7, Status: models.EmploymentEnded, StartedAt: time.Now().AddDate(-1, 0, 0)},
	}

	warning, err := svc.Review(1, TestimonialReviewInput{Approve: true}, 99)
	if err != nil {
		t.Fatalf("Review falló: %v", err)
	}
	if warning != "" {
		t.Fatalf("no debía avisar de nada: %q", warning)
	}
	if len(deps.employment.notes) != 1 {
		t.Fatalf("debía archivarse en el expediente; got=%d notas", len(deps.employment.notes))
	}
	// Entre dos terminados gana el más reciente: es donde el equipo va a buscar.
	if deps.employment.notes[0].EmploymentID != 41 {
		t.Fatalf("debía elegirse el empleo más reciente (41); got=%d", deps.employment.notes[0].EmploymentID)
	}
}

// Un empleo ACTIVO le gana a uno terminado aunque el terminado sea más nuevo.
func TestReview_PrefiereElEmpleoActivo(t *testing.T) {
	repo, _ := pendingTestimonial()
	item := repo.byID[1]
	item.Status = models.TestimonialSubmitted
	item.Audience = models.TestimonialFromProfessional
	item.Quote = "Muy bien todo."

	svc, deps := newTestimonialSvcFull(t, repo, nil)
	deps.employment.active = []models.Employment{
		{ID: 50, UserID: 7, Status: models.EmploymentActive, StartedAt: time.Now().AddDate(-3, 0, 0)},
		{ID: 51, UserID: 7, Status: models.EmploymentEnded, StartedAt: time.Now().AddDate(0, -1, 0)},
	}

	if _, err := svc.Review(1, TestimonialReviewInput{Approve: true}, 99); err != nil {
		t.Fatalf("Review falló: %v", err)
	}
	if len(deps.employment.notes) != 1 || deps.employment.notes[0].EmploymentID != 50 {
		t.Fatalf("debía elegirse el empleo activo (50); got=%+v", deps.employment.notes)
	}
}

// Sin ningún empleo no hay expediente donde archivar. La aprobación SÍ se
// aplica —el testimonio es válido—, pero quien aprueba tiene que enterarse en
// vez de quedarse creyendo que quedó archivado.
func TestReview_AvisaCuandoNoHayDondeArchivar(t *testing.T) {
	repo, _ := pendingTestimonial()
	item := repo.byID[1]
	item.Status = models.TestimonialSubmitted
	item.Audience = models.TestimonialFromProfessional
	item.Quote = "Sin empleo registrado."

	svc, deps := newTestimonialSvcFull(t, repo, nil)
	deps.employment.active = nil

	warning, err := svc.Review(1, TestimonialReviewInput{Approve: true}, 99)
	if err != nil {
		t.Fatal("no poder archivar no invalida la aprobación")
	}
	if warning == "" {
		t.Fatal("debía avisarse de que no se pudo archivar")
	}
	if repo.lastUpdates["status"] != models.TestimonialApproved {
		t.Fatalf("el testimonio debía quedar aprobado igualmente; got=%v", repo.lastUpdates["status"])
	}
	if _, marcado := repo.lastUpdates["filed_at"]; marcado {
		t.Fatal("no debe marcarse como archivado si no se archivó")
	}
}

// --- Constancia ---

func TestConsentPDF_ExigeFirmaYSaleUnPDF(t *testing.T) {
	repo, _ := pendingTestimonial()
	svc := newTestimonialSvcForTest(t, repo, nil)

	if _, _, err := svc.ConsentPDF(1); err == nil {
		t.Fatal("sin firma no hay constancia que emitir")
	}

	signed := time.Now()
	item := repo.byID[1]
	item.Status = models.TestimonialSubmitted
	item.Quote = "Una experiencia excelente de principio a fin."
	item.SignatureName = "Laura Méndez"
	item.SignedAt = &signed
	item.SignerIP = "203.0.113.7"

	pdf, filename, err := svc.ConsentPDF(1)
	if err != nil {
		t.Fatalf("ConsentPDF falló: %v", err)
	}
	if !bytes.HasPrefix(pdf, []byte("%PDF")) {
		t.Fatal("la constancia no es un PDF válido")
	}
	if !strings.HasSuffix(filename, ".pdf") {
		t.Fatalf("nombre de archivo inesperado: %q", filename)
	}
}

// --- Archivado en el expediente ---

// Al aprobar, el testimonio de un profesional queda como nota en su empleo
// activo: es donde el equipo mira el historial de esa persona.
func TestReview_ArchivaEnElExpedienteDelProfesional(t *testing.T) {
	repo, _ := pendingTestimonial()
	item := repo.byID[1]
	item.Status = models.TestimonialSubmitted
	item.Quote = "Trabajar con Oberstaff me cambió el año."
	item.Rating = 5
	svc, deps := newTestimonialSvcFull(t, repo, nil)
	deps.employment.active = []models.Employment{{ID: 33, UserID: 7}}

	if _, err := svc.Review(1, TestimonialReviewInput{Approve: true}, 99); err != nil {
		t.Fatalf("Review falló: %v", err)
	}
	if len(deps.employment.notes) != 1 {
		t.Fatalf("se esperaba una nota en el expediente; got=%d", len(deps.employment.notes))
	}
	note := deps.employment.notes[0]
	if note.EmploymentID != 33 {
		t.Fatalf("la nota debía colgar del empleo activo; got=%d", note.EmploymentID)
	}
	if note.Kind != models.NoteKindTestimonial {
		t.Fatalf("la nota debía ir marcada como testimonio; got=%q", note.Kind)
	}
	if note.Content != item.Quote {
		t.Fatalf("la nota debía llevar la cita; got=%q", note.Content)
	}
	if note.Rating == nil || *note.Rating != 5 {
		t.Fatal("la calificación debía viajar con la nota")
	}
	if note.RefID == nil || *note.RefID != item.ID {
		t.Fatalf("la nota debía referenciar el testimonio %d; got=%v", item.ID, note.RefID)
	}
	if repo.lastUpdates["filed_at"] == nil {
		t.Fatal("debía quedar marcado como archivado")
	}
	// Un testimonio de profesional no toca el expediente de la empresa.
	if len(deps.admin.events) != 0 {
		t.Fatal("no debía escribirse nada en el expediente del tenant")
	}
}

// El de una empresa va a la línea de tiempo de su ficha, que es lo que se lee
// al preparar una renovación o material comercial.
func TestReview_ArchivaEnElExpedienteDeLaEmpresa(t *testing.T) {
	repo, _ := pendingTestimonial()
	item := repo.byID[1]
	item.Audience = models.TestimonialFromCompany
	item.Status = models.TestimonialSubmitted
	item.UserID = 12
	item.Quote = "Nos resolvieron la contratación en dos semanas."
	svc, deps := newTestimonialSvcFull(t, repo, nil)

	if _, err := svc.Review(1, TestimonialReviewInput{Approve: true}, 99); err != nil {
		t.Fatalf("Review falló: %v", err)
	}
	if len(deps.admin.events) != 1 {
		t.Fatalf("se esperaba una entrada en el expediente del tenant; got=%d", len(deps.admin.events))
	}
	ev := deps.admin.events[0]
	if ev.CompanyID != 12 {
		t.Fatalf("la entrada debía ir a la empresa que firmó; got=%d", ev.CompanyID)
	}
	if ev.Type != models.CompanyEventTestimonial {
		t.Fatalf("tipo de entrada inesperado; got=%q", ev.Type)
	}
	if ev.Detail != item.Quote {
		t.Fatalf("la entrada debía llevar la cita; got=%q", ev.Detail)
	}
	if ev.ByUserID != 99 {
		t.Fatalf("debía quedar quién aprobó; got=%d", ev.ByUserID)
	}
	// Sin la referencia, la entrada del expediente es una cita sin salida: no
	// se puede abrir el testimonio con su firma ni su constancia.
	if ev.RefID == nil || *ev.RefID != item.ID {
		t.Fatalf("la entrada debía referenciar el testimonio %d; got=%v", item.ID, ev.RefID)
	}
	if len(deps.employment.notes) != 0 {
		t.Fatal("no debía escribirse nada en el expediente de un empleo")
	}
}

// Se archiva la cita APROBADA, no el borrador: si el equipo la editó, el
// expediente tiene que contar lo que se publicó.
func TestReview_ArchivaLaCitaEditada(t *testing.T) {
	repo, _ := pendingTestimonial()
	item := repo.byID[1]
	item.Status = models.TestimonialSubmitted
	item.Quote = "texto original con erratas"
	svc, deps := newTestimonialSvcFull(t, repo, nil)
	deps.employment.active = []models.Employment{{ID: 33, UserID: 7}}

	_, err := svc.Review(1, TestimonialReviewInput{Approve: true, PublishedQuote: "Texto corregido."}, 99)
	if err != nil {
		t.Fatalf("Review falló: %v", err)
	}
	if deps.employment.notes[0].Content != "Texto corregido." {
		t.Fatalf("debía archivarse la versión publicada; got=%q", deps.employment.notes[0].Content)
	}
}

// Aprobar es repetible (se corrige la cita, se descarta y se recupera). Sin la
// marca de archivado, cada pasada dejaría otra entrada y el expediente pasaría
// de historial a eco.
func TestReview_NoArchivaDosVeces(t *testing.T) {
	repo, _ := pendingTestimonial()
	filed := time.Now()
	item := repo.byID[1]
	item.Status = models.TestimonialApproved
	item.Quote = "Excelente."
	item.FiledAt = &filed
	svc, deps := newTestimonialSvcFull(t, repo, nil)
	deps.employment.active = []models.Employment{{ID: 33, UserID: 7}}

	if _, err := svc.Review(1, TestimonialReviewInput{Approve: true}, 99); err != nil {
		t.Fatalf("Review falló: %v", err)
	}
	if len(deps.employment.notes) != 0 {
		t.Fatal("un testimonio ya archivado no debe volver a bajar al expediente")
	}
}

// Descartar no archiva: al expediente solo baja lo aprobado.
func TestReview_DescartarNoArchiva(t *testing.T) {
	repo, _ := pendingTestimonial()
	item := repo.byID[1]
	item.Status = models.TestimonialSubmitted
	item.Quote = "No nos sirve."
	svc, deps := newTestimonialSvcFull(t, repo, nil)
	deps.employment.active = []models.Employment{{ID: 33, UserID: 7}}

	if _, err := svc.Review(1, TestimonialReviewInput{Approve: false}, 99); err != nil {
		t.Fatalf("Review falló: %v", err)
	}
	if len(deps.employment.notes) != 0 || len(deps.admin.events) != 0 {
		t.Fatal("un testimonio descartado no debe dejar rastro en ningún expediente")
	}
	if repo.lastUpdates["filed_at"] != nil {
		t.Fatal("descartar no debe marcar el testimonio como archivado")
	}
}

// Si no hay dónde archivar (un profesional sin empleo activo), la aprobación
// sigue adelante: perderla por no poder anotarla sería peor que no anotarla.
func TestReview_SinEmpleoActivoApruebaIgual(t *testing.T) {
	repo, _ := pendingTestimonial()
	item := repo.byID[1]
	item.Status = models.TestimonialSubmitted
	item.Quote = "Muy buena experiencia."
	svc, deps := newTestimonialSvcFull(t, repo, nil)
	deps.employment.active = nil // sin empleo activo

	if _, err := svc.Review(1, TestimonialReviewInput{Approve: true}, 99); err != nil {
		t.Fatalf("la aprobación no debe depender del expediente: %v", err)
	}
	if repo.lastUpdates["status"] != models.TestimonialApproved {
		t.Fatalf("el testimonio debía quedar aprobado; got=%v", repo.lastUpdates["status"])
	}
	// Sin archivar, no se marca: así un reintento posterior puede archivarlo.
	if repo.lastUpdates["filed_at"] != nil {
		t.Fatal("no debe marcarse como archivado si no se pudo archivar")
	}
}
