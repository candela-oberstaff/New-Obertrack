package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/obertrack/backend/internal/middleware"
	"github.com/obertrack/backend/internal/repository"
	"github.com/obertrack/backend/internal/service"
)

// TestimonialHandler expone el módulo de testimonios. Tiene dos superficies muy
// distintas, igual que la inducción:
//   - Landing / Submit: PÚBLICAS. Quien firma puede no tener sesión (una empresa
//     que delega en su gerente, un profesional cuyo empleo ya terminó); su
//     credencial es el token del enlace que recibió por correo.
//   - El resto: interno, para el equipo que pide y aprueba.
type TestimonialHandler struct {
	svc service.TestimonialService
}

func NewTestimonialHandler(svc service.TestimonialService) *TestimonialHandler {
	return &TestimonialHandler{svc: svc}
}

// --- Público (sin sesión, autenticado por el token del enlace) ---

// Landing devuelve lo que hay que mostrar en la página del testimonio.
func (h *TestimonialHandler) Landing(c *gin.Context) {
	view, err := h.svc.Landing(c.Param("token"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, view)
}

// Submit guarda el testimonio con su firma.
func (h *TestimonialHandler) Submit(c *gin.Context) {
	var req service.TestimonialSubmission
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No pudimos leer tu respuesta"})
		return
	}

	// La evidencia la pone el servidor, nunca el cliente: si viniera en el
	// cuerpo, cualquiera podría firmar declarando la IP que se le antoje.
	req.IP = c.ClientIP()
	req.UserAgent = c.Request.UserAgent()

	if err := h.svc.Submit(c.Param("token"), req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "¡Gracias! Recibimos tu testimonio."})
}

// --- Interno ---

// Templates devuelve las plantillas por audiencia, para armar la solicitud.
func (h *TestimonialHandler) Templates(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"templates": h.svc.Templates()})
}

// List devuelve la bandeja con sus contadores por estado.
func (h *TestimonialHandler) List(c *gin.Context) {
	items, counts, err := h.svc.List(repository.TestimonialFilter{
		Status:   c.Query("status"),
		Audience: c.Query("audience"),
		Search:   c.Query("search"),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "counts": counts})
}

// Get devuelve el detalle de un testimonio.
func (h *TestimonialHandler) Get(c *gin.Context) {
	id, ok := testimonialID(c)
	if !ok {
		return
	}
	t, err := h.svc.Get(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, t)
}

// Request emite una solicitud y envía el correo.
func (h *TestimonialHandler) Request(c *gin.Context) {
	var req service.TestimonialRequestInput
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Solicitud inválida"})
		return
	}
	t, err := h.svc.Request(req, middleware.GetUserID(c))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, t)
}

type bulkRequestPayload struct {
	service.TestimonialRequestInput
	UserIDs []uint `json:"user_ids"`
}

// RequestMany emite la misma solicitud a varias personas de una vez.
//
// Responde 200 aunque parte del lote no salga: un lote es un éxito parcial por
// naturaleza y el cuerpo dice qué pasó con cada persona. Solo es 400 cuando no
// se pudo intentar nada (lista vacía o demasiada gente).
func (h *TestimonialHandler) RequestMany(c *gin.Context) {
	var req bulkRequestPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Solicitud inválida"})
		return
	}
	result, err := h.svc.RequestMany(req.UserIDs, req.TestimonialRequestInput, middleware.GetUserID(c))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

// Resend renueva el enlace y lo vuelve a enviar.
func (h *TestimonialHandler) Resend(c *gin.Context) {
	id, ok := testimonialID(c)
	if !ok {
		return
	}
	if err := h.svc.Resend(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Solicitud reenviada"})
}

// Review aprueba o descarta un testimonio recibido.
func (h *TestimonialHandler) Review(c *gin.Context) {
	id, ok := testimonialID(c)
	if !ok {
		return
	}
	var req service.TestimonialReviewInput
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Revisión inválida"})
		return
	}
	warning, err := h.svc.Review(id, req, middleware.GetUserID(c))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// El aviso NO es un error: la decisión se aplicó. Viaja aparte para que el
	// panel pueda contarlo sin teñir la operación de fallida.
	c.JSON(http.StatusOK, gin.H{"message": "Testimonio actualizado", "warning": warning})
}

type changesPayload struct {
	Reason string `json:"reason"`
}

// RequestChanges devuelve el testimonio a su autor para que lo corrija.
func (h *TestimonialHandler) RequestChanges(c *gin.Context) {
	id, ok := testimonialID(c)
	if !ok {
		return
	}
	var req changesPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Solicitud inválida"})
		return
	}
	if err := h.svc.RequestChanges(id, req.Reason, middleware.GetUserID(c)); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Se lo devolvimos para que lo corrija"})
}

// Delete borra la solicitud (borrado lógico).
func (h *TestimonialHandler) Delete(c *gin.Context) {
	id, ok := testimonialID(c)
	if !ok {
		return
	}
	if err := h.svc.Delete(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Testimonio eliminado"})
}

// ConsentPDF descarga la constancia firmada.
func (h *TestimonialHandler) ConsentPDF(c *gin.Context) {
	id, ok := testimonialID(c)
	if !ok {
		return
	}
	pdf, filename, err := h.svc.ConsentPDF(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	c.Data(http.StatusOK, "application/pdf", pdf)
}

// Signature sirve el trazo de la firma. Va por aquí y no por el servidor
// estático de subidas para que solo lo vea quien puede entrar al panel: es un
// dato personal, no un adjunto cualquiera.
func (h *TestimonialHandler) Signature(c *gin.Context) {
	id, ok := testimonialID(c)
	if !ok {
		return
	}
	img, err := h.svc.SignatureImage(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.Data(http.StatusOK, "image/png", img)
}

// testimonialID lee el :id de la ruta y responde 400 si no es válido.
func testimonialID(c *gin.Context) (uint, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Identificador inválido"})
		return 0, false
	}
	return uint(id), true
}
