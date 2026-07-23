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
	UserID       uint   `json:"user_id"`
	EmploymentID uint   `json:"employment_id"`
	// Status: created (profesional nuevo), rehired (ya existía, nuevo empleo),
	// already_active (ya tenía empleo activo en esa empresa: no-op idempotente).
	Status     string `json:"status"`
	CVAttached bool   `json:"cv_attached"`
	CVWarning  string `json:"cv_warning,omitempty"`
}

type onboardingService struct {
	userRepo       repository.UserRepository
	employmentRepo repository.EmploymentRepository
	employmentSvc  EmploymentService
	uploadSvc      UploadService
	authSvc        AuthService
}

func NewOnboardingService(
	userRepo repository.UserRepository,
	employmentRepo repository.EmploymentRepository,
	employmentSvc EmploymentService,
	uploadSvc UploadService,
	authSvc AuthService,
) OnboardingService {
	return &onboardingService{
		userRepo:       userRepo,
		employmentRepo: employmentRepo,
		employmentSvc:  employmentSvc,
		uploadSvc:      uploadSvc,
		authSvc:        authSvc,
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

	result := &HireResult{}

	// 3. Dedup por email: crea el profesional o reutiliza el existente.
	user, err := s.userRepo.GetByEmail(email)
	if err != nil {
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
		}
		if err := s.userRepo.Create(user); err != nil {
			return nil, err
		}
		// Correo de bienvenida / establecer contraseña (best-effort).
		if err := s.authSvc.ForgotPassword(email); err != nil {
			log.Printf("[Onboarding] welcome email failed for %s: %v", email, err)
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
		if len(updates) > 0 {
			_ = s.userRepo.Update(user, updates)
		}
	}
	result.UserID = user.ID

	// 4. Empleo (idempotente): si ya tiene uno activo en esta empresa, no-op.
	if existing, gerr := s.employmentRepo.GetActive(user.ID, req.CompanyID); gerr == nil && existing != nil {
		result.EmploymentID = existing.ID
		result.Status = "already_active"
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

	// Honra la fecha de inicio si vino (AddEmployment usa "ahora" por defecto).
	if req.StartedAt != nil {
		_ = s.employmentRepo.Update(emp, map[string]interface{}{"started_at": *req.StartedAt})
	}

	// 5. CV (best-effort): un fallo del CV NO revierte la contratación; se avisa.
	if req.CV != nil && strings.TrimSpace(req.CV.ContentBase64) != "" {
		if warn := s.attachCV(emp.ID, req.CompanyID, req.CV); warn != "" {
			result.CVWarning = warn
		} else {
			result.CVAttached = true
		}
	}

	return result, nil
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
