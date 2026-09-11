package handlers

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"

	"github.com/obertrack/backend/internal/apperrors"

	"github.com/obertrack/backend/internal/service"
)

// OnboardingHandler expone el puente con Obersuite en los dos sentidos:
// entrada (webhook de contratación) y salida (padrón de empresas). Sus rutas viven
// bajo /api/integrations/obersuite y se protegen con un token de servicio
// estático (middleware.SharedSecretAuth), no con sesión de usuario.
type OnboardingHandler struct {
	svc service.OnboardingService
}

func NewOnboardingHandler(svc service.OnboardingService) *OnboardingHandler {
	return &OnboardingHandler{svc: svc}
}

// ListCompanies devuelve el padrón completo de empresas con su ficha para la
// integración con Obersuite: identidad, responsable, ubicación normalizada
// (país/provincia/ciudad/dirección), contadores de plantilla y de trabajo, y el
// último contacto nuestro.
//
// Lo que NO va, y no por olvido: nada de la operación diaria (horas, jornadas
// por aprobar, tickets, última actividad). Nadie del otro lado lo consumía, y
// eran justo los campos que cambiaban solos cada pocos segundos, con lo que el
// ETag de abajo no acertaba nunca. Ver PayloadSchemaVersion en version.go.
//
// ?updated_since=<RFC3339> acota a lo que cambió después de ese instante, para
// sincronizar en incremental. Sin el parámetro devuelve todas, que es como
// funcionaba antes: Obersuite ya llama a este endpoint y no se le puede cambiar
// el comportamiento por defecto.
//
// Una fecha que no se entiende se rechaza en vez de ignorarse: tratarla como
// "sin filtro" devolvería el padrón entero y el que llama creería estar
// recibiendo solo lo nuevo.
func (h *OnboardingHandler) ListCompanies(c *gin.Context) {
	var updatedSince *time.Time
	if raw := c.Query("updated_since"); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "updated_since debe ser una fecha RFC3339, por ejemplo 2026-09-09T13:00:00Z",
			})
			return
		}
		updatedSince = &parsed
	}

	companies, err := h.svc.ListCompanies(updatedSince)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Se serializa a mano en vez de con c.JSON para poder calcular el ETag sobre
	// el cuerpo EXACTO que se va a devolver. El orden de los campos lo fija el
	// struct, así que el mismo contenido produce siempre el mismo hash.
	body, err := json.Marshal(companies)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "no se pudo serializar el padrón"})
		return
	}

	etag := weakETag(body)
	c.Header("ETag", etag)
	// Obersuite pide este padrón cada vez que alguien abre su pantalla de
	// Empresas. Sin caché eso es traerse el listado entero para, casi siempre,
	// pintar lo mismo.
	c.Header("Cache-Control", "private, max-age=0, must-revalidate")

	// El If-None-Match puede traer varios ETags separados por coma, y el cliente
	// puede devolverlos con el prefijo W/. Se compara contra cada uno.
	if matchesETag(c.GetHeader("If-None-Match"), etag) {
		c.Status(http.StatusNotModified)
		return
	}

	c.Data(http.StatusOK, "application/json; charset=utf-8", body)
}

// weakETag identifica una respuesta por su contenido.
//
// Va sobre el cuerpo y NO sobre un MAX(updated_at) de las empresas, aunque eso
// último sería más barato: la ficha sigue llevando cosas que cambian sin tocar
// la fila de la empresa —los contadores de plantilla y de trabajo, y el propio
// "hace 8 días" del último contacto, que se recalcula al pasar la medianoche—.
// Un validador basado en updated_at contestaría "no ha cambiado nada" mientras
// esos números ya son otros, que es peor que no cachear.
//
// Es débil (W/) a propósito: garantiza que el contenido es equivalente para el
// que lo consume, no que los bytes sean idénticos.
func weakETag(body []byte) string {
	sum := sha256.Sum256(body)
	return fmt.Sprintf(`W/"%s"`, hex.EncodeToString(sum[:16]))
}

// matchesETag compara el If-None-Match entrante contra el ETag actual. Acepta
// la lista separada por comas y el comodín "*" que manda la especificación.
func matchesETag(ifNoneMatch, current string) bool {
	if ifNoneMatch == "" {
		return false
	}
	for _, candidate := range strings.Split(ifNoneMatch, ",") {
		candidate = strings.TrimSpace(candidate)
		if candidate == "*" || candidate == current {
			return true
		}
	}
	return false
}

// hireCVPayload es el CV embebido en base64 (ver HireCV en el servicio).
type hireCVPayload struct {
	FileName      string `json:"file_name"`
	MimeType      string `json:"mime_type"`
	ContentBase64 string `json:"content_base64"`
}

// hirePayload es el cuerpo del webhook de contratación de Obersuite.
type hirePayload struct {
	ExternalID        string         `json:"external_id"`
	Email             string         `json:"email" binding:"required,email"`
	Name              string         `json:"name" binding:"required"`
	IdentityDocument  string         `json:"identity_document"`
	PhoneNumber       string         `json:"phone_number"`
	BirthDate         *string        `json:"birth_date"`
	EmergencyContacts []string       `json:"emergency_contacts"`
	Country           string         `json:"country"`
	State             string         `json:"state"`
	City              string         `json:"city"`
	Address           string         `json:"address"`
	JobTitle          string         `json:"job_title"`
	CompanyID         uint           `json:"company_id" binding:"required"`
	StartedAt         string         `json:"started_at"` // YYYY-MM-DD (opcional)
	CV                *hireCVPayload `json:"cv"`
}

// Hire recibe la contratación desde Obersuite y materializa al profesional con
// su empleo activo. Idempotente por email (ver OnboardingService.Hire).
func (h *OnboardingHandler) Hire(c *gin.Context) {
	var req hirePayload
	if err := c.ShouldBindJSON(&req); err != nil {
		// El error del validador de Go es ilegible ("Key: 'hirePayload.Email'
		// Error:Field validation for 'Email' failed on the 'email' tag") y
		// Obersuite enseña nuestro mensaje TAL CUAL al reclutador. Se traduce a
		// una frase que diga qué campo arreglar.
		c.JSON(http.StatusBadRequest, gin.H{"error": hireBindMessage(err)})
		return
	}

	var birthDate *time.Time
	if req.BirthDate != nil && *req.BirthDate != "" {
		birthDate = parseDatePtr(*req.BirthDate)
	}

	in := service.HireRequest{
		ExternalID:        req.ExternalID,
		Email:             req.Email,
		Name:              req.Name,
		IdentityDocument:  req.IdentityDocument,
		PhoneNumber:       req.PhoneNumber,
		BirthDate:         birthDate,
		EmergencyContacts: req.EmergencyContacts,
		Country:           req.Country,
		State:             req.State,
		City:              req.City,
		Address:           req.Address,
		JobTitle:          req.JobTitle,
		CompanyID:         req.CompanyID,
		StartedAt:         parseDatePtr(req.StartedAt),
	}
	if req.CV != nil {
		in.CV = &service.HireCV{
			FileName:      req.CV.FileName,
			MimeType:      req.CV.MimeType,
			ContentBase64: req.CV.ContentBase64,
		}
	}

	result, err := h.svc.Hire(in)
	if err != nil {
		status, msg := hireStatus(err)
		if status >= 500 {
			// El detalle técnico se queda en el log: Obersuite enseña nuestro
			// mensaje TAL CUAL al reclutador, y un error de base de datos en la
			// cara de quien contrata no le dice nada y encima filtra cómo
			// estamos hechos por dentro.
			//
			// El request_id viaja en el cuerpo Y en la línea de log: con él,
			// Obersuite cita un fallo concreto y aquí se encuentra en un grep,
			// sin reconstruirlo por hora y empresa como hasta ahora.
			rid := hireRequestID()
			log.Printf("[Onboarding] hire falló para %s (empresa %d) request_id=%s: %v", in.Email, in.CompanyID, rid, err)
			c.JSON(status, gin.H{"error": msg, "request_id": rid})
			return
		}
		c.JSON(status, gin.H{"error": msg})
		return
	}
	c.JSON(http.StatusOK, result)
}

// hireBindMessage convierte el fallo de validación del cuerpo en algo que una
// persona pueda accionar. Nombra el campo en los términos del contrato
// (email, name, company_id), no los del struct de Go.
func hireBindMessage(err error) string {
	var invalid validator.ValidationErrors
	if errors.As(err, &invalid) && len(invalid) > 0 {
		switch campo := invalid[0].Field(); campo {
		case "Email":
			if invalid[0].Tag() == "email" {
				return "el email no tiene un formato válido"
			}
			return "falta el email"
		case "Name":
			return "falta el nombre del profesional"
		case "CompanyID":
			return "falta company_id (el id de la empresa que contrata)"
		default:
			return "falta el campo " + campo + " o su valor no es válido"
		}
	}
	// Un JSON que ni siquiera se puede leer: no hay campo que nombrar.
	return "el cuerpo de la petición no es un JSON válido"
}

// hireStatus traduce el fallo al código que Obersuite espera, y devuelve el
// texto que le va a leer una persona.
//
// El contrato está acordado con ellos y su lógica de reintentos depende de él:
// los 4xx NO se reintentan, el 500 SÍ. Antes salía todo como 400, así que
// reintentaban tres veces cosas que no podían funcionar —un email mal escrito—
// mientras mantenían abierta una transacción con la fila de la candidatura
// bloqueada.
//
// Un error que no reconocemos cae a 500 a propósito: si es un fallo nuestro,
// que lo reintenten es lo correcto; darlo por 400 les haría descartar en firme
// una contratación que sí podía salir.
// hireRequestID es un identificador corto y único por fallo, legible en un
// mensaje ("hire-3f9a1c2b"). No es criptográfico ni lo necesita: solo tiene que
// no repetirse en el log.
func hireRequestID() string {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("hire-%d", time.Now().UnixNano())
	}
	return "hire-" + hex.EncodeToString(b[:])
}

func hireStatus(err error) (int, string) {
	switch {
	case errors.Is(err, apperrors.ErrInvalidInput):
		return http.StatusBadRequest, err.Error()
	case errors.Is(err, apperrors.ErrNotFound):
		return http.StatusNotFound, err.Error()
	case errors.Is(err, apperrors.ErrConflict), errors.Is(err, apperrors.ErrEmailTaken):
		return http.StatusConflict, err.Error()
	case errors.Is(err, apperrors.ErrCompanySuspended):
		return http.StatusUnprocessableEntity, err.Error()
	default:
		return http.StatusInternalServerError,
			"no se pudo completar la contratación por un problema en Obertrack. Vuelve a intentarlo en un momento."
	}
}
