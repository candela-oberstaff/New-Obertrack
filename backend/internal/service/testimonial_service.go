package service

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
	"github.com/obertrack/backend/internal/utils"
)

// Límites de la firma. El trazo de un canvas de ~600x200 pesa unas decenas de
// KB; medio mega deja margen de sobra para pantallas retina y corta de raíz el
// envío de una imagen cualquiera disfrazada de firma.
const (
	maxSignatureBytes = 512 * 1024
	// testimonialTTLDays es la vigencia por omisión del enlace, en días.
	testimonialTTLDays = 45
)

// signaturePNGPrefix es el único encabezado de data URL aceptado. El canvas del
// navegador siempre produce PNG (toDataURL sin argumentos).
const signaturePNGPrefix = "data:image/png;base64,"

// TestimonialRequestInput es lo que el equipo completa al pedir un testimonio.
type TestimonialRequestInput struct {
	UserID   uint   `json:"user_id"`
	Audience string `json:"audience"`
	// IntroMessage es la nota personal. Vacía = la sugerida por la plantilla.
	IntroMessage string `json:"intro_message"`
	// Prompts reemplaza las preguntas guía de la plantilla. Vacío = las de la
	// plantilla.
	Prompts []string `json:"prompts"`
	// TTLDays es la vigencia del enlace. 0 = el valor por omisión.
	TTLDays int `json:"ttl_days"`
}

// maxBulkTestimonials es el tope de personas por lote. No es una limitación
// técnica: es un freno para que un clic desafortunado sobre una selección amplia
// no dispare cientos de correos que ya no se pueden recoger.
const maxBulkTestimonials = 50

// TestimonialBulkOutcome es qué pasó con UNA persona del lote.
type TestimonialBulkOutcome struct {
	UserID uint   `json:"user_id"`
	Name   string `json:"name"`
	Sent   bool   `json:"sent"`
	// Reason explica por qué NO se envió, en palabras que se puedan enseñar.
	Reason string `json:"reason,omitempty"`
}

// TestimonialBulkResult es el resultado del lote completo.
//
// Existe —en vez de devolver un error— porque un lote es un éxito PARCIAL por
// naturaleza: casi siempre habrá alguien con una solicitud viva o sin correo.
// Que dos personas queden fuera no puede impedir que las otras ocho reciban la
// suya, y quien lo lanzó necesita saber exactamente quién se quedó atrás.
type TestimonialBulkResult struct {
	Sent     int                      `json:"sent"`
	Skipped  int                      `json:"skipped"`
	Outcomes []TestimonialBulkOutcome `json:"outcomes"`
}

// TestimonialAnswer es una pregunta guía con su respuesta.
type TestimonialAnswer struct {
	Prompt string `json:"prompt"`
	Answer string `json:"answer"`
}

// TestimonialLandingView es todo lo que la página pública necesita. Se arma con
// los datos CONGELADOS de la solicitud, nunca releyendo el perfil: quien firma
// debe ver exactamente lo que se le pidió firmar.
type TestimonialLandingView struct {
	Status           string   `json:"status"`
	Audience         string   `json:"audience"`
	Headline         string   `json:"headline"`
	RecipientName    string   `json:"recipient_name"`
	RecipientRole    string   `json:"recipient_role,omitempty"`
	RecipientCompany string   `json:"recipient_company,omitempty"`
	IntroMessage     string   `json:"intro_message"`
	Prompts          []string `json:"prompts"`
	ConsentText      string   `json:"consent_text"`
	ConsentVersion   string   `json:"consent_version"`
	// Expired distingue "el enlace venció" de "ya lo respondiste": son dos
	// pantallas distintas y la segunda no es un error.
	Expired bool `json:"expired"`

	// --- Solo cuando se devolvió para corregir ---
	// ChangeReason es lo que hay que arreglar, tal como se lo escribió el equipo.
	ChangeReason string `json:"change_reason,omitempty"`
	// Previous es lo que la persona ya había escrito. Se devuelve para precargar
	// el formulario: quien vuelve a corregir una errata no debería tener que
	// reescribir su testimonio entero.
	//
	// La firma NO viaja aquí a propósito: al corregir hay que volver a firmar,
	// porque la firma anterior autorizaba un texto que puede haber cambiado.
	Previous *TestimonialDraft `json:"previous,omitempty"`
}

// TestimonialDraft es lo que ya se había enviado, para repoblar el formulario
// en una corrección.
type TestimonialDraft struct {
	Rating          int                 `json:"rating"`
	Quote           string              `json:"quote"`
	Answers         []TestimonialAnswer `json:"answers"`
	AllowPublicName bool                `json:"allow_public_name"`
	AllowRole       bool                `json:"allow_role"`
	AllowPhoto      bool                `json:"allow_photo"`
	AllowLogo       bool                `json:"allow_logo"`
	SignatureName   string              `json:"signature_name"`
}

// TestimonialSubmission es lo que envía la página pública al firmar.
type TestimonialSubmission struct {
	Rating          int                 `json:"rating"`
	Quote           string              `json:"quote"`
	Answers         []TestimonialAnswer `json:"answers"`
	AllowPublicName bool                `json:"allow_public_name"`
	AllowRole       bool                `json:"allow_role"`
	AllowPhoto      bool                `json:"allow_photo"`
	AllowLogo       bool                `json:"allow_logo"`
	// ConsentAccepted es la casilla de autorización. Sin ella no hay permiso y
	// el testimonio no sirve para nada, así que se rechaza el envío.
	ConsentAccepted bool `json:"consent_accepted"`
	// SignatureName es el nombre completo tipeado.
	SignatureName string `json:"signature_name"`
	// SignatureImage es la firma como data URL PNG. Las tres modalidades
	// (trazo, imagen cargada, nombre tipografiado) llegan ya normalizadas a PNG
	// por el navegador, así que aquí no hay tres caminos que validar: uno solo.
	SignatureImage string `json:"signature_image"`
	// SignatureMode dice CÓMO se firmó. Es evidencia, no adorno.
	SignatureMode string `json:"signature_mode"`

	// Evidencia que pone el servidor, no el cliente. El handler la completa
	// desde el request; lo que venga del navegador en estos campos se ignora.
	IP        string `json:"-"`
	UserAgent string `json:"-"`
}

// TestimonialReviewInput es la decisión del equipo sobre un testimonio recibido.
type TestimonialReviewInput struct {
	Approve bool   `json:"approve"`
	Note    string `json:"note"`
	// PublishedQuote es la cita editada para publicar. Vacía = se usa la
	// original. La original nunca se sobrescribe: es parte de la evidencia.
	PublishedQuote string `json:"published_quote"`
}

// TestimonialService gobierna el ciclo completo de un testimonio: pedirlo por
// correo, recogerlo firmado en una página pública y aprobarlo para su uso.
//
// La firma es electrónica simple con evidencia: no hay proveedor externo. Lo
// que la sostiene es el conjunto que se guarda —el texto exacto autorizado, el
// trazo, el nombre tipeado, la fecha, la IP y el navegador— más el hecho de que
// el enlace se envió al correo registrado de esa persona. De ahí se emite una
// constancia en PDF (ver testimonial_pdf.go).
type TestimonialService interface {
	Templates() []TestimonialTemplate

	// Request emite la solicitud y envía el correo con el enlace.
	Request(in TestimonialRequestInput, requestedBy uint) (*models.Testimonial, error)
	// RequestMany emite la misma solicitud a varias personas. Nunca falla en
	// bloque: devuelve qué pasó con cada una.
	RequestMany(userIDs []uint, in TestimonialRequestInput, requestedBy uint) (*TestimonialBulkResult, error)
	// Resend renueva el token y vuelve a enviar el correo.
	Resend(id uint) error

	// Landing y Submit son la mitad PÚBLICA: sin sesión, autenticadas por el
	// token del enlace.
	Landing(token string) (*TestimonialLandingView, error)
	Submit(token string, in TestimonialSubmission) error

	List(f repository.TestimonialFilter) ([]models.Testimonial, map[string]int64, error)
	Get(id uint) (*models.Testimonial, error)
	// Review aprueba o descarta. Devuelve un aviso (no un error) cuando la
	// decisión se aplicó pero algo secundario no salió, como no haber podido
	// archivarlo en el expediente.
	Review(id uint, in TestimonialReviewInput, reviewerID uint) (warning string, err error)
	// RequestChanges devuelve el testimonio a su autor con un motivo, reabriendo
	// el enlace para que lo corrija.
	RequestChanges(id uint, reason string, reviewerID uint) error
	Delete(id uint) error

	// ConsentPDF genera la constancia firmada. Se genera al vuelo desde la
	// evidencia guardada: no hay archivo que mantener sincronizado.
	ConsentPDF(id uint) ([]byte, string, error)
	// SignatureImage devuelve el PNG del trazo, para mostrarlo en el panel.
	SignatureImage(id uint) ([]byte, error)
}

type testimonialService struct {
	repo     repository.TestimonialRepository
	userRepo repository.UserRepository
	// employmentRepo y adminRepo son los dos expedientes donde aterriza un
	// testimonio aprobado: el del empleo (profesional) y el del tenant
	// (empresa). Se inyectan como repositorios y no como servicios para no
	// acoplar este módulo al ciclo de vida de empleos ni al panel de admin:
	// aquí solo se escribe una entrada.
	employmentRepo repository.EmploymentRepository
	adminRepo      repository.AdminRepository
	brevoSvc       *BrevoService
	notifSvc       NotificationService
	uploadPath     string
	frontendURL    string
}

func NewTestimonialService(
	repo repository.TestimonialRepository,
	userRepo repository.UserRepository,
	employmentRepo repository.EmploymentRepository,
	adminRepo repository.AdminRepository,
	brevoSvc *BrevoService,
	notifSvc NotificationService,
	uploadPath string,
	frontendURL string,
) TestimonialService {
	if uploadPath == "" {
		uploadPath = "./uploads"
	}
	return &testimonialService{
		repo:           repo,
		userRepo:       userRepo,
		employmentRepo: employmentRepo,
		adminRepo:      adminRepo,
		brevoSvc:       brevoSvc,
		notifSvc:       notifSvc,
		uploadPath:     uploadPath,
		frontendURL:    strings.TrimRight(frontendURL, "/"),
	}
}

func (s *testimonialService) baseURL() string {
	if s.frontendURL == "" {
		return FrontendBaseURL()
	}
	return s.frontendURL
}

func (s *testimonialService) Templates() []TestimonialTemplate {
	return TestimonialTemplates()
}

// --- Pedir ---

func (s *testimonialService) Request(in TestimonialRequestInput, requestedBy uint) (*models.Testimonial, error) {
	tpl, ok := testimonialTemplateFor(in.Audience)
	if !ok {
		return nil, errors.New("audiencia de testimonio inválida")
	}

	user, err := s.userRepo.GetByID(in.UserID)
	if err != nil {
		return nil, errors.New("no encontramos a esa persona")
	}
	if strings.TrimSpace(user.Email) == "" {
		return nil, errors.New("esa cuenta no tiene correo: no hay a dónde enviar la solicitud")
	}
	// La audiencia tiene que casar con el tipo de cuenta. Pedirle a un
	// profesional que hable "en representación de la empresa" —y al revés—
	// invalida el consentimiento que firma.
	if err := checkTestimonialAudience(user, in.Audience); err != nil {
		return nil, err
	}

	pending, err := s.repo.HasPendingForUser(user.ID, in.Audience)
	if err != nil {
		return nil, err
	}
	if pending {
		return nil, errors.New("esa persona ya tiene una solicitud pendiente; reenvíala desde el panel en lugar de crear otra")
	}

	token, err := generateTestimonialToken()
	if err != nil {
		return nil, err
	}

	prompts := in.Prompts
	if len(prompts) == 0 {
		prompts = tpl.Prompts
	}
	promptsJSON, err := json.Marshal(prompts)
	if err != nil {
		return nil, errors.New("no se pudieron guardar las preguntas")
	}

	intro := strings.TrimSpace(in.IntroMessage)
	if intro == "" {
		intro = tpl.Intro
	}

	ttl := in.TTLDays
	if ttl < 1 {
		ttl = testimonialTTLDays
	}

	t := &models.Testimonial{
		Token:            token,
		Audience:         in.Audience,
		Status:           models.TestimonialPending,
		UserID:           user.ID,
		RequestedBy:      requestedBy,
		RecipientName:    user.Name,
		RecipientEmail:   user.Email,
		RecipientRole:    user.JobTitle,
		RecipientCompany: testimonialCompanyName(user),
		Prompts:          string(promptsJSON),
		IntroMessage:     intro,
		ConsentText:      tpl.ConsentText,
		ConsentVersion:   TestimonialConsentVersion,
		ExpiresAt:        time.Now().AddDate(0, 0, ttl),
	}
	if err := s.repo.Create(t); err != nil {
		return nil, err
	}

	s.sendRequestEmail(t)
	s.notifyRecipient(t, false)
	return t, nil
}

func (s *testimonialService) RequestMany(userIDs []uint, in TestimonialRequestInput, requestedBy uint) (*TestimonialBulkResult, error) {
	// Duplicados fuera: la misma persona seleccionada dos veces recibiría dos
	// correos y el segundo chocaría con el primero. Se resuelve aquí y no en la
	// interfaz porque el tope y el resultado cuentan personas, no clics.
	seen := make(map[uint]bool, len(userIDs))
	unique := make([]uint, 0, len(userIDs))
	for _, id := range userIDs {
		if id == 0 || seen[id] {
			continue
		}
		seen[id] = true
		unique = append(unique, id)
	}
	if len(unique) == 0 {
		return nil, errors.New("elige al menos una persona")
	}
	if len(unique) > maxBulkTestimonials {
		return nil, fmt.Errorf("son demasiadas personas de una vez (máximo %d)", maxBulkTestimonials)
	}

	result := &TestimonialBulkResult{Outcomes: make([]TestimonialBulkOutcome, 0, len(unique))}
	for _, id := range unique {
		outcome := TestimonialBulkOutcome{UserID: id, Name: s.nameOf(id)}

		single := in
		single.UserID = id
		if _, err := s.Request(single, requestedBy); err != nil {
			// El fallo de una persona NO corta el lote: se anota y se sigue.
			outcome.Reason = err.Error()
			result.Skipped++
		} else {
			outcome.Sent = true
			result.Sent++
		}
		result.Outcomes = append(result.Outcomes, outcome)
	}
	return result, nil
}

// nameOf resuelve un nombre para poder nombrar a quien se quedó fuera. Sin él,
// el resultado del lote sería una lista de números.
func (s *testimonialService) nameOf(userID uint) string {
	if u, err := s.userRepo.GetByID(userID); err == nil && u.Name != "" {
		return u.Name
	}
	return fmt.Sprintf("Usuario %d", userID)
}

func (s *testimonialService) Resend(id uint) error {
	t, err := s.repo.GetByID(id)
	if err != nil {
		return errors.New("solicitud no encontrada")
	}
	if t.Status != models.TestimonialPending {
		return errors.New("esa solicitud ya fue respondida")
	}

	// Token nuevo: el enlace anterior queda muerto en cuanto se reenvía, así
	// que un correo reenviado a otra dirección no deja dos puertas abiertas.
	token, err := generateTestimonialToken()
	if err != nil {
		return err
	}
	now := time.Now()
	if err := s.repo.Update(t.ID, map[string]interface{}{
		"token":       token,
		"expires_at":  now.AddDate(0, 0, testimonialTTLDays),
		"reminded_at": now,
	}); err != nil {
		return err
	}

	t.Token = token
	s.sendRequestEmail(t)
	s.notifyRecipient(t, true)
	return nil
}

// --- Página pública ---

func (s *testimonialService) Landing(token string) (*TestimonialLandingView, error) {
	t, err := s.repo.GetByToken(strings.TrimSpace(token))
	if err != nil {
		return nil, errors.New("no encontramos esta solicitud. Verifica el enlace de tu correo")
	}
	tpl, _ := testimonialTemplateFor(t.Audience)

	var prompts []string
	if t.Prompts != "" {
		_ = json.Unmarshal([]byte(t.Prompts), &prompts)
	}

	// En una corrección se devuelve lo ya escrito para repoblar el formulario.
	// Solo en ese estado: en cualquier otro no hay nada que corregir y sería
	// filtrar contenido sin motivo.
	var previous *TestimonialDraft
	if t.Status == models.TestimonialChangesRequested {
		previous = &TestimonialDraft{
			Rating:          t.Rating,
			Quote:           t.Quote,
			Answers:         decodeTestimonialAnswers(t.Answers),
			AllowPublicName: t.AllowPublicName,
			AllowRole:       t.AllowRole,
			AllowPhoto:      t.AllowPhoto,
			AllowLogo:       t.AllowLogo,
			SignatureName:   t.SignatureName,
		}
	}

	return &TestimonialLandingView{
		Status:           t.Status,
		Audience:         t.Audience,
		Headline:         tpl.Headline,
		RecipientName:    t.RecipientName,
		RecipientRole:    t.RecipientRole,
		RecipientCompany: t.RecipientCompany,
		IntroMessage:     t.IntroMessage,
		Prompts:          prompts,
		ConsentText:      t.ConsentText,
		ConsentVersion:   t.ConsentVersion,
		Expired:          t.Expired(),
		ChangeReason:     t.ChangeReason,
		Previous:         previous,
	}, nil
}

func (s *testimonialService) Submit(token string, in TestimonialSubmission) error {
	t, err := s.repo.GetByToken(strings.TrimSpace(token))
	if err != nil {
		return errors.New("no encontramos esta solicitud. Verifica el enlace de tu correo")
	}
	// Se admite tanto un envío nuevo como la corrección de uno devuelto. Lo que
	// no se admite es reescribir un testimonio que ya está en revisión o
	// resuelto: para eso el equipo tiene que devolverlo primero.
	if !t.Open() {
		return errors.New("este testimonio ya fue enviado. Gracias")
	}
	if t.Expired() {
		return errors.New("el enlace venció. Escríbenos y te enviamos uno nuevo")
	}

	// Se exige que haya texto, pero NO un largo mínimo: un testimonio sincero
	// puede ser una sola frase ("Excelente, me encanta la app"), y poner un
	// suelo obliga a rellenar hasta llegar a la cuenta, que es justo lo que
	// hace sonar falso un testimonio. Recortar es trabajo del equipo al
	// revisar; alargar no debería serlo de quien lo escribe.
	quote := strings.TrimSpace(in.Quote)
	if quote == "" {
		return errors.New("escribe tu testimonio antes de enviarlo")
	}
	if !in.ConsentAccepted {
		return errors.New("necesitamos tu autorización para poder usar el testimonio")
	}
	signer := strings.TrimSpace(in.SignatureName)
	if len([]rune(signer)) < 3 {
		return errors.New("escribe tu nombre completo para firmar")
	}
	mode, err := normalizeSignatureMode(in.SignatureMode)
	if err != nil {
		return err
	}
	if in.Rating < 0 || in.Rating > 5 {
		return errors.New("calificación inválida")
	}

	// La firma se guarda ANTES de marcar el testimonio como enviado: si el
	// archivo no se puede escribir, la solicitud queda intacta y la persona
	// puede reintentar. Al revés quedaría un testimonio firmado sin firma.
	sigPath, err := s.storeSignature(t.ID, in.SignatureImage)
	if err != nil {
		return err
	}

	answersJSON, err := json.Marshal(in.Answers)
	if err != nil {
		answersJSON = []byte("[]")
	}

	// Al corregir se vuelve a firmar, y la firma anterior deja de valer: pudo
	// autorizar un texto que ya no existe. Se aparta con su evidencia en lugar
	// de sobrescribirla sin dejar rastro, porque aquel acto de firma ocurrió.
	trail := t.SignatureTrail
	if t.Signed() {
		trail = appendSignatureTrail(t)
	}

	now := time.Now()
	updates := map[string]interface{}{
		"status":          models.TestimonialSubmitted,
		"signature_trail": trail,
		// Se limpia el motivo: ya se atendió, y dejarlo haría que la página se
		// lo siguiera enseñando a su autor como si estuviera pendiente.
		"change_reason":     "",
		"rating":            in.Rating,
		"quote":             quote,
		"answers":           string(answersJSON),
		"submitted_at":      now,
		"allow_public_name": in.AllowPublicName,
		"allow_role":        in.AllowRole,
		"allow_photo":       in.AllowPhoto,
		"allow_logo":        in.AllowLogo,
		"signature_name":    signer,
		"signature_mode":    mode,
		"signature_image":   sigPath,
		"signed_at":         now,
		"signer_ip":         truncateRunes(in.IP, 64),
		"signer_user_agent": truncateRunes(in.UserAgent, 500),
	}
	if err := s.repo.Update(t.ID, updates); err != nil {
		return err
	}

	s.notifyReviewer(t)
	return nil
}

// --- Panel interno ---

func (s *testimonialService) List(f repository.TestimonialFilter) ([]models.Testimonial, map[string]int64, error) {
	items, err := s.repo.List(f)
	if err != nil {
		return nil, nil, err
	}
	counts, err := s.repo.CountByStatus()
	if err != nil {
		return nil, nil, err
	}
	return items, counts, nil
}

func (s *testimonialService) Get(id uint) (*models.Testimonial, error) {
	t, err := s.repo.GetByID(id)
	if err != nil {
		return nil, errors.New("testimonio no encontrado")
	}
	return t, nil
}

func (s *testimonialService) Review(id uint, in TestimonialReviewInput, reviewerID uint) (warning string, err error) {
	t, err := s.repo.GetByID(id)
	if err != nil {
		return "", errors.New("testimonio no encontrado")
	}
	// Solo se revisa lo que ya llegó. Aprobar una solicitud sin responder
	// crearía un testimonio aprobado sin texto ni firma. Sí se permite volver
	// sobre uno ya revisado: cambiar de opinión es parte del trabajo.
	if t.Status == models.TestimonialPending {
		return "", errors.New("este testimonio todavía no fue respondido")
	}

	status := models.TestimonialRejected
	if in.Approve {
		status = models.TestimonialApproved
	}
	now := time.Now()
	updates := map[string]interface{}{
		"status":          status,
		"review_note":     strings.TrimSpace(in.Note),
		"published_quote": strings.TrimSpace(in.PublishedQuote),
		"reviewed_by":     reviewerID,
		"reviewed_at":     now,
	}

	// Al aprobar, el testimonio baja al expediente de quien lo escribió, para
	// que el equipo lo encuentre donde mira el historial de esa cuenta y no solo
	// en el módulo de Testimonios.
	//
	// Solo al aprobar y solo una vez: aprobar es una acción repetible (se
	// corrige la cita, se descarta y se recupera) y sin la marca cada pasada
	// dejaría una entrada más.
	//
	// El archivado NO puede tumbar la aprobación: si el expediente falla —un
	// profesional sin empleo activo, por ejemplo—, se registra y la revisión
	// sigue adelante. Perder la aprobación por no poder anotarla sería peor que
	// no anotarla.
	if in.Approve && t.FiledAt == nil {
		// La cita que se archiva es la que se va a publicar: si el equipo la
		// editó, el expediente debe reflejar lo editado.
		quote := strings.TrimSpace(in.PublishedQuote)
		if quote == "" {
			quote = t.DisplayQuote()
		}
		if err := s.fileInExpediente(t, quote, reviewerID); err != nil {
			// Aprobar SÍ funcionó; lo que falló es dejar constancia. Antes esto
			// solo iba a un log y quien aprobaba se quedaba creyendo que había
			// quedado archivado. Ahora se devuelve para poder decírselo.
			log.Printf("[Testimonios] no se pudo archivar el testimonio %d en el expediente: %v", t.ID, err)
			warning = "El testimonio quedó aprobado, pero no se pudo archivar en el expediente: " + err.Error()
		} else {
			updates["filed_at"] = now
		}
	}

	return warning, s.repo.Update(t.ID, updates)
}

// betterExpedienteTarget indica si `cand` es mejor sitio que `best` para
// archivar: gana un empleo activo sobre uno terminado, y a igualdad el más
// reciente.
func betterExpedienteTarget(cand, best models.Employment) bool {
	candActive := cand.Status == models.EmploymentActive
	bestActive := best.Status == models.EmploymentActive
	if candActive != bestActive {
		return candActive
	}
	return cand.StartedAt.After(best.StartedAt)
}

// fileInExpediente deja constancia del testimonio aprobado en el expediente que
// le corresponde según quién lo escribió:
//
//   - Empresa  → una entrada en el expediente del tenant (company_events), que
//     es la línea de tiempo que el equipo lee al abrir la ficha del cliente.
//   - Profesional → una nota en su empleo activo, que es lo que la aplicación
//     llama "expediente" de una persona.
//
// Se archiva la cita ya aprobada, no el borrador: el expediente cuenta lo que
// se publicó. La evidencia de la firma sigue viviendo en el módulo de
// Testimonios, que es su sitio; aquí va el rastro para el seguimiento.
func (s *testimonialService) fileInExpediente(t *models.Testimonial, quote string, byUserID uint) error {
	quote = utils.SanitizeHTML(quote)
	if quote == "" {
		return errors.New("el testimonio no tiene texto que archivar")
	}

	if t.Audience == models.TestimonialFromCompany {
		if s.adminRepo == nil {
			return errors.New("expediente de empresa no disponible")
		}
		// En una cuenta empleador, el id del usuario ES el de la empresa.
		return s.adminRepo.CreateCompanyEvent(&models.CompanyEvent{
			CompanyID: t.UserID,
			Type:      models.CompanyEventTestimonial,
			Detail:    quote,
			ByUserID:  byUserID,
			// La referencia es lo que convierte la entrada en una puerta: desde
			// el expediente se abre el testimonio completo, con su firma y su
			// constancia, en vez de quedarse en una cita suelta.
			RefID: &t.ID,
		})
	}

	if s.employmentRepo == nil {
		return errors.New("expediente de profesional no disponible")
	}
	// El expediente de un profesional cuelga de un empleo, así que hay que
	// elegir uno. Se miran TODOS y no solo los activos: el mejor momento para
	// pedir un testimonio es justo cuando alguien termina su etapa, y entonces
	// ya no le queda ninguno activo. Su expediente sigue existiendo y es
	// exactamente donde esto pertenece.
	//
	// Preferencia: un empleo activo sobre uno terminado, y entre iguales el más
	// reciente, que es donde el equipo va a buscarlo.
	employments, err := s.employmentRepo.ListByUser(t.UserID)
	if err != nil {
		return err
	}
	if len(employments) == 0 {
		return errors.New("esta persona no tiene ningún empleo registrado, así que no hay expediente donde archivarlo")
	}
	target := employments[0]
	for _, e := range employments[1:] {
		if betterExpedienteTarget(e, target) {
			target = e
		}
	}

	note := &models.EmploymentNote{
		EmploymentID: target.ID,
		AuthorID:     byUserID,
		Kind:         models.NoteKindTestimonial,
		Content:      quote,
		// Privada: es una anotación de seguimiento del equipo. La persona ya
		// tiene su copia —lo escribió ella— y no necesita verlo repetido en su
		// CV, ni recibir un aviso de "te compartieron una nota" por algo suyo.
		Visibility: models.ExpedientePrivate,
		// Igual que en el expediente de empresa: la referencia permite volver
		// al testimonio completo desde la nota.
		RefID: &t.ID,
	}
	if t.Rating > 0 {
		rating := t.Rating
		note.Rating = &rating
	}
	return s.employmentRepo.CreateNote(note)
}

// RequestChanges devuelve el testimonio a su autor para que lo corrija.
//
// Es distinto de descartar: descartar cierra el asunto, esto lo deja abierto
// esperando a la persona. Reabre el enlace con lo que ya escribió precargado, y
// le manda el motivo por correo y por campanita.
func (s *testimonialService) RequestChanges(id uint, reason string, reviewerID uint) error {
	t, err := s.repo.GetByID(id)
	if err != nil {
		return errors.New("testimonio no encontrado")
	}
	// Solo se devuelve lo que llegó y está sin resolver. Reabrir uno aprobado
	// exigiría además retirarlo del expediente donde ya se archivó; si hace
	// falta rehacerlo, se descarta y se pide de nuevo.
	if t.Status != models.TestimonialSubmitted {
		return errors.New("solo se puede pedir una corrección de un testimonio que está esperando revisión")
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return errors.New("explica qué debe corregir: es lo único que va a leer")
	}

	// Token nuevo, como en el reenvío: el enlace anterior queda muerto y no
	// quedan dos puertas abiertas al mismo testimonio.
	token, err := generateTestimonialToken()
	if err != nil {
		return err
	}

	now := time.Now()
	if err := s.repo.Update(t.ID, map[string]interface{}{
		"status":              models.TestimonialChangesRequested,
		"change_reason":       reason,
		"change_requested_at": now,
		"revisions":           t.Revisions + 1,
		"reviewed_by":         reviewerID,
		"reviewed_at":         now,
		"token":               token,
		// El plazo se renueva: devolver algo con el enlace a punto de vencer
		// sería pedir una corrección imposible.
		"expires_at": now.AddDate(0, 0, testimonialTTLDays),
	}); err != nil {
		return err
	}

	t.Token = token
	t.ChangeReason = reason
	s.sendChangesEmail(t)
	s.notifyChangesRequested(t)
	return nil
}

func (s *testimonialService) Delete(id uint) error {
	return s.repo.Delete(id)
}

func (s *testimonialService) SignatureImage(id uint) ([]byte, error) {
	t, err := s.repo.GetByID(id)
	if err != nil {
		return nil, errors.New("testimonio no encontrado")
	}
	if t.SignatureImage == "" {
		return nil, errors.New("este testimonio no tiene firma")
	}
	return os.ReadFile(s.signaturePath(t.SignatureImage))
}

// --- Interno ---

// checkTestimonialAudience valida que el tipo de cuenta corresponda con la
// audiencia elegida.
func checkTestimonialAudience(user *models.User, audience string) error {
	switch audience {
	case models.TestimonialFromCompany:
		if user.UserType != models.UserTypeEmployer {
			return errors.New("el testimonio de empresa se le pide a una cuenta empleador")
		}
	case models.TestimonialFromProfessional:
		if user.UserType != models.UserTypeProfessional {
			return errors.New("el testimonio de profesional se le pide a una cuenta profesional")
		}
	}
	return nil
}

// testimonialCompanyName resuelve el nombre de empresa que acompaña a la cita.
// En una cuenta empleador es su propio nombre comercial; en un profesional, el
// de la empresa donde trabaja.
func testimonialCompanyName(user *models.User) string {
	if user.CompanyName != "" {
		return user.CompanyName
	}
	if user.Empleador != nil {
		return user.Empleador.CompanyName
	}
	return ""
}

// storeSignature decodifica el trazo del canvas y lo escribe como PNG. Devuelve
// el NOMBRE del archivo, no la ruta completa: el directorio de subidas se
// resuelve al leer, para que un cambio de volumen no invalide lo ya guardado.
func (s *testimonialService) storeSignature(id uint, dataURL string) (string, error) {
	dataURL = strings.TrimSpace(dataURL)
	if dataURL == "" {
		return "", errors.New("falta tu firma: dibújala en el recuadro")
	}
	if !strings.HasPrefix(dataURL, signaturePNGPrefix) {
		return "", errors.New("no pudimos leer la firma; vuelve a dibujarla")
	}

	raw, err := base64.StdEncoding.DecodeString(dataURL[len(signaturePNGPrefix):])
	if err != nil {
		return "", errors.New("no pudimos leer la firma; vuelve a dibujarla")
	}
	if len(raw) > maxSignatureBytes {
		return "", errors.New("la firma es demasiado pesada; vuelve a dibujarla")
	}
	// Comprobación del encabezado real del archivo: que el data URL diga PNG no
	// garantiza que lo sea.
	if len(raw) < 8 || string(raw[1:4]) != "PNG" {
		return "", errors.New("no pudimos leer la firma; vuelve a dibujarla")
	}

	suffix := make([]byte, 8)
	if _, err := rand.Read(suffix); err != nil {
		return "", errors.New("no se pudo guardar la firma")
	}
	name := fmt.Sprintf("testimonial_%d_%s.png", id, hex.EncodeToString(suffix))

	if err := os.MkdirAll(s.uploadPath, 0755); err != nil {
		return "", errors.New("no se pudo guardar la firma")
	}
	if err := os.WriteFile(s.signaturePath(name), raw, 0644); err != nil {
		log.Printf("[Testimonios] no se pudo escribir la firma %s: %v", name, err)
		return "", errors.New("no se pudo guardar la firma")
	}
	return name, nil
}

// signaturePath resuelve el archivo dentro del directorio de subidas.
// filepath.Base corta cualquier intento de salirse con "../".
func (s *testimonialService) signaturePath(name string) string {
	return filepath.Join(s.uploadPath, filepath.Base(name))
}

func (s *testimonialService) sendRequestEmail(t *models.Testimonial) {
	if s.brevoSvc == nil {
		return
	}
	link := s.baseURL() + "/testimonio/" + t.Token
	subject := "Nos gustaría conocer tu experiencia"
	html := BuildTestimonialRequestHTML(t.RecipientName, t.IntroMessage, link)

	go func() {
		if err := s.brevoSvc.SendEmailKind(EmailKindTestimonialRequest, t.RecipientEmail, t.RecipientName, subject, html); err != nil {
			if !errors.Is(err, ErrEmailKindDisabled) {
				log.Printf("[Testimonios] no se pudo enviar la solicitud a %s: %v", t.RecipientEmail, err)
			}
		}
	}()
}

// notifyRecipient avisa por campanita a quien tiene que escribir el testimonio.
//
// El correo por sí solo es un canal frágil —se pierde entre otros, se va a spam,
// se lee desde el móvil y se olvida—, y a quien le pedimos el testimonio SIEMPRE
// es usuario de la plataforma (la audiencia se valida contra el tipo de cuenta).
// Así que el aviso llega también por la campanita, que además viaja por Web Push
// cuando no tiene la pestaña abierta.
//
// El enlace lleva el token porque es la misma puerta que la del correo: la
// notificación es privada de esa persona, así que no expone nada que no tenga ya
// en su bandeja de entrada.
func (s *testimonialService) notifyRecipient(t *models.Testimonial, resend bool) {
	if s.notifSvc == nil || t.UserID == 0 {
		return
	}
	title := "Nos gustaría conocer tu experiencia"
	message := "Te pedimos unas líneas sobre tu experiencia. Puedes escribirlas y firmar en un par de minutos."
	if resend {
		title = "Recordatorio: tu testimonio"
		message = "Te reenviamos el enlace para escribir tu testimonio. Sigue disponible."
	}

	err := s.notifSvc.CreateNotification(
		t.UserID,
		"testimonial_request",
		title,
		message,
		map[string]interface{}{"link": "/testimonio/" + t.Token, "testimonial_id": t.ID},
	)
	if err != nil {
		log.Printf("[Testimonios] no se pudo notificar la solicitud a %d: %v", t.UserID, err)
	}
}

// notifyReviewer avisa por campanita a quien pidió el testimonio de que ya
// llegó y espera su revisión.
func (s *testimonialService) notifyReviewer(t *models.Testimonial) {
	if s.notifSvc == nil || t.RequestedBy == 0 {
		return
	}
	err := s.notifSvc.CreateNotification(
		t.RequestedBy,
		"testimonial",
		"Testimonio recibido",
		fmt.Sprintf("%s firmó su testimonio y está esperando revisión.", t.RecipientName),
		// "link" es lo que sigue la campanita al pulsarla: sin él el aviso
		// se lee pero no lleva a ninguna parte.
		map[string]interface{}{"link": "/testimonios", "testimonial_id": t.ID},
	)
	if err != nil {
		log.Printf("[Testimonios] no se pudo notificar la recepción: %v", err)
	}
}

func (s *testimonialService) sendChangesEmail(t *models.Testimonial) {
	if s.brevoSvc == nil {
		return
	}
	link := s.baseURL() + "/testimonio/" + t.Token
	subject := "Un detalle por corregir en tu testimonio"
	html := BuildTestimonialChangesHTML(t.RecipientName, t.ChangeReason, link)

	go func() {
		if err := s.brevoSvc.SendEmailKind(EmailKindTestimonialRequest, t.RecipientEmail, t.RecipientName, subject, html); err != nil {
			if !errors.Is(err, ErrEmailKindDisabled) {
				log.Printf("[Testimonios] no se pudo enviar la corrección a %s: %v", t.RecipientEmail, err)
			}
		}
	}()
}

func (s *testimonialService) notifyChangesRequested(t *models.Testimonial) {
	if s.notifSvc == nil || t.UserID == 0 {
		return
	}
	err := s.notifSvc.CreateNotification(
		t.UserID,
		"testimonial_request",
		"Un detalle por corregir en tu testimonio",
		t.ChangeReason,
		map[string]interface{}{"link": "/testimonio/" + t.Token, "testimonial_id": t.ID},
	)
	if err != nil {
		log.Printf("[Testimonios] no se pudo notificar la corrección a %d: %v", t.UserID, err)
	}
}

// signatureRecord es una firma apartada: la que había antes de una corrección.
type signatureRecord struct {
	Name      string     `json:"name"`
	Mode      string     `json:"mode"`
	Image     string     `json:"image"`
	SignedAt  *time.Time `json:"signed_at"`
	IP        string     `json:"ip"`
	UserAgent string     `json:"user_agent"`
	Quote     string     `json:"quote"`
	// Reason es por qué se devolvió, para poder leer el rastro entero.
	Reason string `json:"reason,omitempty"`
}

// appendSignatureTrail aparta la firma vigente al historial y devuelve el JSON
// resultante. Un rastro ilegible no debe impedir firmar de nuevo: si no se
// puede leer, se empieza uno nuevo en lugar de fallar el envío.
func appendSignatureTrail(t *models.Testimonial) string {
	var trail []signatureRecord
	if t.SignatureTrail != "" {
		_ = json.Unmarshal([]byte(t.SignatureTrail), &trail)
	}
	trail = append(trail, signatureRecord{
		Name:      t.SignatureName,
		Mode:      t.SignatureMode,
		Image:     t.SignatureImage,
		SignedAt:  t.SignedAt,
		IP:        t.SignerIP,
		UserAgent: t.SignerUserAgent,
		Quote:     t.Quote,
		Reason:    t.ChangeReason,
	})
	raw, err := json.Marshal(trail)
	if err != nil {
		return t.SignatureTrail
	}
	return string(raw)
}

// normalizeSignatureMode valida la modalidad declarada. Vacía se toma como
// trazo, que es lo que hacían todos los clientes antes de existir las tres
// opciones: así una versión antigua del navegador sigue pudiendo firmar.
func normalizeSignatureMode(mode string) (string, error) {
	switch strings.TrimSpace(mode) {
	case "", models.SignatureDrawn:
		return models.SignatureDrawn, nil
	case models.SignatureUploaded:
		return models.SignatureUploaded, nil
	case models.SignatureTyped:
		return models.SignatureTyped, nil
	default:
		return "", errors.New("no reconocemos esa forma de firmar")
	}
}

func generateTestimonialToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", errors.New("no se pudo generar la solicitud")
	}
	return hex.EncodeToString(b), nil
}

// truncateRunes recorta a n caracteres para que la evidencia quepa en su
// columna. Un User-Agent exótico no debe hacer fallar el envío.
func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}
