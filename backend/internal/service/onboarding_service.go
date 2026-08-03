package service

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
)

// OnboardingService es el puente Obersuite (captación) → Obertrack (gestión):
// recibe la contratación de un candidato y lo materializa como profesional con
// su empleo activo en la empresa que lo contrató. Es la contraparte del webhook
// que dispara Obersuite al marcar "contratado".
//
// El diseño es IDEMPOTENTE por email: un mismo hire reintentado (reintento del
// webhook, doble clic) no crea profesionales ni empleos duplicados.
type OnboardingService interface {
	// ListCompanies devuelve las empresas [{id,name}] para que el reclutador en
	// Obersuite elija de un dropdown y nos envíe el company_id estable.
	ListCompanies() ([]map[string]interface{}, error)
	// Hire materializa la contratación. Ver HireRequest / HireResult.
	Hire(req HireRequest) (*HireResult, error)
}

// HireCV es el CV del candidato tal como viaja en el webhook: binario en base64.
// Preferimos base64 embebido (no una URL temporal) para que el hire sea atómico
// y resista los reintentos del webhook sin depender de que una URL siga viva.
type HireCV struct {
	FileName      string
	MimeType      string
	ContentBase64 string
}

// HireRequest son los datos que Obersuite envía al contratar. Email + CompanyID
// son obligatorios; el resto enriquece el perfil / expediente.
type HireRequest struct {
	ExternalID       string // id del candidato en Obersuite (trazabilidad)
	Email            string // llave de dedup (se normaliza a minúsculas)
	Name             string
	IdentityDocument string // cédula/documento (opcional)
	PhoneNumber      string
	Country          string
	State            string
	City             string
	Address          string
	JobTitle         string
	CompanyID        uint       // empresa contratante (id de ListCompanies)
	StartedAt        *time.Time // fecha de inicio (opcional; por defecto hoy)
	CV               *HireCV    // opcional
}

// HireResult resume qué pasó, para que Obersuite lo registre.
type HireResult struct {
	UserID       uint `json:"user_id"`
	EmploymentID uint `json:"employment_id"`
	ObersuiteID  string `json:"obersuite_id,omitempty"`
	// Status: created (profesional nuevo), rehired (ya existía, nuevo empleo),
	// already_active (ya tenía empleo activo en esa empresa: no-op idempotente).
	Status     string `json:"status"`
	CVAttached bool   `json:"cv_attached"`
	CVWarning  string `json:"cv_warning,omitempty"`
	// InductionPending indica que el profesional quedó sin acceso hasta aprobar
	// la inducción (se le envió el enlace a la landing en vez de las credenciales).
	InductionPending bool `json:"induction_pending"`
}

type onboardingService struct {
	userRepo       repository.UserRepository
	employmentRepo repository.EmploymentRepository
	employmentSvc  EmploymentService
	uploadSvc      UploadService
	authSvc        AuthService
	inductionSvc   InductionService
}

func NewOnboardingService(
	userRepo repository.UserRepository,
	employmentRepo repository.EmploymentRepository,
	employmentSvc EmploymentService,
	uploadSvc UploadService,
	authSvc AuthService,
	inductionSvc InductionService,
) OnboardingService {
	return &onboardingService{
		userRepo:       userRepo,
		employmentRepo: employmentRepo,
		employmentSvc:  employmentSvc,
		uploadSvc:      uploadSvc,
		authSvc:        authSvc,
		inductionSvc:   inductionSvc,
	}
}

func (s *onboardingService) ListCompanies() ([]map[string]interface{}, error) {
	return s.authSvc.GetPublicCompanies()
}

func (s *onboardingService) Hire(req HireRequest) (*HireResult, error) {
	// 1. Normaliza y valida lo obligatorio.
	email := strings.ToLower(strings.TrimSpace(req.Email))
	name := strings.TrimSpace(req.Name)
	if email == "" || !strings.Contains(email, "@") {
		return nil, errors.New("email inválido o ausente")
	}
	if name == "" {
		return nil, errors.New("el nombre es obligatorio")
	}
	if req.CompanyID == 0 {
		return nil, errors.New("company_id es obligatorio")
	}

	// 2. Valida la empresa ANTES de crear nada (evita profesionales huérfanos si
	//    el company_id es inválido).
	company, err := s.userRepo.GetByID(req.CompanyID)
	if err != nil || company.UserType != models.UserTypeEmployer {
		return nil, errors.New("la empresa (company_id) no es válida")
	}
	if !company.IsActive {
		return nil, errors.New("la empresa está suspendida")
	}

	externalID := strings.TrimSpace(req.ExternalID)
	result := &HireResult{ObersuiteID: externalID}

	// 3. Identidad: PRIMERO por el id de Obersuite, después por email.
	//    El orden importa. El email solo identifica mientras nadie lo cambie;
	//    en cuanto el candidato se registra con uno y el alta llega con otro,
	//    resolver por email crea una segunda cuenta de la misma persona. El id
	//    de Obersuite no tiene ese problema, así que manda cuando viene.
	user := s.resolveProfessional(externalID, email)
	isNew := user == nil

	if isNew {
		// No existe → alta de profesional. Contraseña aleatoria: el profesional
		// la establece con el correo de bienvenida (flujo forgot-password).
		hashed, herr := bcrypt.GenerateFromPassword([]byte(generateRandomPassword()), bcrypt.DefaultCost)
		if herr != nil {
			return nil, errors.New("no se pudo procesar el registro")
		}
		user = &models.User{
			Name:             name,
			Email:            email,
			Password:         string(hashed),
			UserType:         models.UserTypeProfessional,
			IsActive:         true,
			PhoneNumber:      strings.TrimSpace(req.PhoneNumber),
			Country:          strings.TrimSpace(req.Country),
			State:            strings.TrimSpace(req.State),
			City:             strings.TrimSpace(req.City),
			Address:          strings.TrimSpace(req.Address),
			JobTitle:         strings.TrimSpace(req.JobTitle),
			IdentityDocument: strings.TrimSpace(req.IdentityDocument),
			ObersuiteID:      externalID,
		}
		if err := s.userRepo.Create(user); err != nil {
			return nil, err
		}
		result.Status = "created"
	} else {
		// Ya existe. Solo un profesional puede recibir un empleo por esta vía;
		// un email de empresa/superadmin/CS se rechaza para no corromper cuentas.
		if user.UserType != models.UserTypeProfessional {
			return nil, errors.New("ya existe una cuenta con ese email que no es un profesional")
		}
		// Completa datos que falten (no pisa lo que el profesional ya tenga).
		updates := map[string]interface{}{}
		if user.IdentityDocument == "" && strings.TrimSpace(req.IdentityDocument) != "" {
			updates["identity_document"] = strings.TrimSpace(req.IdentityDocument)
		}
		if user.PhoneNumber == "" && strings.TrimSpace(req.PhoneNumber) != "" {
			updates["phone_number"] = strings.TrimSpace(req.PhoneNumber)
		}
		// Vincula hacia atrás a quien ya estaba aquí antes de que existiera el
		// id: la primera contratación que llega por el puente lo estampa y a
		// partir de ahí esa persona queda identificada en los dos sistemas.
		if externalID != "" && user.ObersuiteID == "" {
			updates["obersuite_id"] = externalID
		}
		if len(updates) > 0 {
			_ = s.userRepo.Update(user, updates)
		}
	}
	result.UserID = user.ID

	// 4. Empleo (idempotente): si ya tiene uno activo en esta empresa, no-op.
	//    Aquí se corta el reintento del webhook, y por eso NO se manda ningún
	//    correo desde esta rama: reenviar la inducción en cada reintento le
	//    llenaría la bandeja al profesional con el mismo enlace.
	if existing, gerr := s.employmentRepo.GetActive(user.ID, req.CompanyID); gerr == nil && existing != nil {
		result.EmploymentID = existing.ID
		result.Status = "already_active"
		s.logHire(result, email, req.CompanyID)
		return result, nil
	}

	emp, aerr := s.employmentSvc.AddEmployment(user.ID, req.CompanyID, req.JobTitle, "Contratado vía Obersuite", nil)
	if aerr != nil {
		return nil, aerr
	}
	result.EmploymentID = emp.ID
	if result.Status != "created" {
		result.Status = "rehired"
	}

	// Honra la fecha de inicio si vino (AddEmployment usa "ahora" por defecto)
	// y deja el id de Obersuite en el empleo: identifica ESTA contratación.
	empUpdates := map[string]interface{}{}
	if req.StartedAt != nil {
		empUpdates["started_at"] = *req.StartedAt
	}
	if externalID != "" {
		empUpdates["obersuite_id"] = externalID
	}
	if len(empUpdates) > 0 {
		_ = s.employmentRepo.Update(emp, empUpdates)
	}

	// 5. Inducción / bienvenida. Va DESPUÉS del empleo a propósito: si la
	//    contratación falla a mitad, nadie recibe un correo por un empleo que
	//    no existe.
	s.notifyHired(user, isNew, result)

	// 6. CV (best-effort): un fallo del CV NO revierte la contratación; se avisa.
	if req.CV != nil && strings.TrimSpace(req.CV.ContentBase64) != "" {
		if warn := s.attachCV(emp.ID, req.CompanyID, req.CV); warn != "" {
			result.CVWarning = warn
		} else {
			result.CVAttached = true
		}
	}

	s.logHire(result, email, req.CompanyID)
	return result, nil
}

// resolveProfessional busca al profesional por el id de Obersuite y, si no lo
// encuentra, por email. Devuelve nil si no existe en ninguno de los dos.
//
// Que el id mande sobre el email es justo lo que evita duplicar a una persona
// que cambió de correo entre la postulación y la contratación.
func (s *onboardingService) resolveProfessional(externalID, email string) *models.User {
	if externalID != "" {
		if user, err := s.userRepo.GetByObersuiteID(externalID); err == nil && user != nil {
			return user
		}
	}
	if user, err := s.userRepo.GetByEmail(email); err == nil && user != nil {
		return user
	}
	return nil
}

// notifyHired decide qué correo recibe quien acaba de ser contratado desde
// Obersuite: la inducción (si está encendida) o la bienvenida para establecer
// contraseña.
//
// Se aplica tanto al alta nueva como a la re-contratación de alguien que ya
// estaba: antes solo se emitía en el alta, así que un profesional que volvía a
// ser contratado no recibía nunca la inducción.
//
// Dos casos quedan fuera a propósito, porque invitarlos les QUITARÍA el acceso
// que ya tienen (InviteIfEnabled deja el estado en 'pending'):
//   - quien ya aprobó la inducción;
//   - quien nunca tuvo que hacerla ('not_required'): cuentas anteriores a la
//     inducción y altas por otras vías, que hoy trabajan con normalidad.
//
// Si hace falta que uno de esos repita la inducción, Soporte lo emite a mano
// desde el panel (InductionService.Invite / Reset), que es una decisión
// explícita y no un efecto colateral de una contratación.
func (s *onboardingService) notifyHired(user *models.User, isNew bool, result *HireResult) {
	needsInduction := isNew ||
		user.OnboardingStatus == models.OnboardingPending ||
		user.OnboardingStatus == models.OnboardingBlocked

	if !needsInduction {
		log.Printf("[Onboarding] %s ya tiene acceso (onboarding=%s): no se le reenvía la inducción",
			user.Email, user.OnboardingStatus)
		return
	}

	invited := false
	if s.inductionSvc != nil {
		var ierr error
		invited, ierr = s.inductionSvc.InviteIfEnabled(user)
		if ierr != nil {
			log.Printf("[Onboarding] no se pudo emitir la inducción de %s: %v", user.Email, ierr)
			invited = false
		}
	}
	result.InductionPending = invited

	if invited {
		log.Printf("[Onboarding] inducción emitida para %s", user.Email)
		return
	}

	// La inducción está apagada o sin cuestionario: el profesional entra por el
	// flujo directo de siempre. El puente nunca se rompe por una inducción sin
	// configurar, pero sí se deja constancia de por qué no salió el correo.
	log.Printf("[Onboarding] inducción no emitida para %s (apagada o sin cuestionario)", user.Email)
	if isNew {
		// Correo de bienvenida / establecer contraseña (best-effort). Solo al
		// alta: quien ya existía tiene su contraseña y no necesita reponerla.
		if err := s.authSvc.ForgotPassword(user.Email); err != nil {
			log.Printf("[Onboarding] welcome email failed for %s: %v", user.Email, err)
		}
	}
}

// logHire deja una línea por contratación recibida. Es la bitácora del puente:
// sin ella, un "¿llegó el hire de Fulano?" no se podía responder —el access log
// solo dice que alguien llamó al endpoint, no a quién ni con qué resultado—.
func (s *onboardingService) logHire(result *HireResult, email string, companyID uint) {
	obersuiteID := result.ObersuiteID
	if obersuiteID == "" {
		obersuiteID = "(sin id)"
	}
	log.Printf("[Onboarding] hire obersuite_id=%s email=%s empresa=%d → %s (user=%d empleo=%d inducción_pendiente=%t)",
		obersuiteID, email, companyID, result.Status, result.UserID, result.EmploymentID, result.InductionPending)
}

// attachCV decodifica el CV en base64, lo guarda en disco y lo adjunta al
// expediente del empleo como documento privado (RR.HH.). Devuelve "" si todo
// salió bien, o un mensaje de advertencia (no error fatal) si algo falló.
func (s *onboardingService) attachCV(employmentID, companyID uint, cv *HireCV) string {
	data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(cv.ContentBase64))
	if err != nil {
		return "CV ignorado: base64 inválido"
	}
	const maxCVBytes = 8 << 20 // 8 MB (acordado con Obersuite)
	if len(data) == 0 {
		return "CV ignorado: archivo vacío"
	}
	if len(data) > maxCVBytes {
		return "CV ignorado: excede 8 MB"
	}

	mime := normalizeContentType(cv.MimeType)
	ext, ok := s.uploadSvc.GetAllowedMimeTypes()[mime]
	if !ok {
		return "CV ignorado: tipo de archivo no permitido"
	}

	// Nombre en disco propio (no confiamos en el file_name entrante para la ruta).
	filename := fmt.Sprintf("cv_%d_%d%s", employmentID, time.Now().UnixNano(), ext)
	path := filepath.Join(s.uploadSvc.GetUploadPath(), filename)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		log.Printf("[Onboarding] failed to write CV %q: %v", path, err)
		return "CV no guardado: error de escritura"
	}

	fileURL := "/api/uploads/" + filename
	if _, err := s.employmentSvc.AddDocument(
		employmentID, companyID, "CV", filename, fileURL,
		int64(len(data)), mime, models.ExpedientePrivate, nil,
	); err != nil {
		return "CV no adjuntado: " + err.Error()
	}
	return ""
}

// generateRandomPassword produce una contraseña aleatoria fuerte. No se le
// entrega al profesional: es un relleno hasta que él fije la suya con el correo
// de bienvenida (forgot-password).
func generateRandomPassword() string {
	b := make([]byte, 18)
	if _, err := rand.Read(b); err != nil {
		// Fallback improbable: sigue siendo no adivinable en la práctica.
		return fmt.Sprintf("Ob!%d-fallback", time.Now().UnixNano())
	}
	return "Ob!" + base64.RawURLEncoding.EncodeToString(b)
}
