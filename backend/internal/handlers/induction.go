package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/obertrack/backend/internal/middleware"
	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/service"
)

// InductionHandler expone la inducción del profesional recién contratado. Tiene
// dos superficies muy distintas:
//   - Landing / Submit: PÚBLICAS (el profesional aún no tiene cuenta activa;
//     su única credencial es el token del enlace que recibió por correo).
//   - Config / Bloques / Programas / Status / Reset: internas, para
//     configuración (superadmin) y para Soporte.
type InductionHandler struct {
	svc service.InductionService
}

func NewInductionHandler(svc service.InductionService) *InductionHandler {
	return &InductionHandler{svc: svc}
}

func parseIDParam(c *gin.Context, name string) (uint, bool) {
	id, err := strconv.ParseUint(c.Param(name), 10, 32)
	if err != nil || id == 0 {
		return 0, false
	}
	return uint(id), true
}

// --- Público (sin sesión, autenticado por el token del enlace) ---

// Landing devuelve el progreso por bloque y el bloque actual con su video y
// sus preguntas. Nunca incluye las respuestas correctas.
func (h *InductionHandler) Landing(c *gin.Context) {
	view, err := h.svc.Landing(c.Param("token"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, view)
}

type submitPayload struct {
	BlockID uint                      `json:"block_id"`
	Answers []service.SubmittedAnswer `json:"answers"`
}

// Submit califica el intento sobre el bloque actual y aplica la decisión.
func (h *InductionHandler) Submit(c *gin.Context) {
	var req submitPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Respuestas inválidas"})
		return
	}
	result, err := h.svc.Submit(c.Param("token"), req.BlockID, req.Answers)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

// --- Interno: configuración global ---

func (h *InductionHandler) GetConfig(c *gin.Context) {
	cfg, err := h.svc.GetConfig()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, cfg)
}

type configPayload struct {
	InviteTTLDays int  `json:"invite_ttl_days"`
	IsActive      bool `json:"is_active"`
}

func (h *InductionHandler) SaveConfig(c *gin.Context) {
	var req configPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	cfg := &models.InductionConfig{
		InviteTTLDays: req.InviteTTLDays,
		IsActive:      req.IsActive,
	}
	if err := h.svc.SaveConfig(cfg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, cfg)
}

// --- Interno: biblioteca de bloques ---

func (h *InductionHandler) ListBlocks(c *gin.Context) {
	blocks, err := h.svc.ListBlocks()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": blocks})
}

func (h *InductionHandler) CreateBlock(c *gin.Context) {
	var in service.BlockInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Datos inválidos"})
		return
	}
	block, err := h.svc.CreateBlock(middleware.GetUserID(c), in)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, block)
}

func (h *InductionHandler) UpdateBlock(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Bloque inválido"})
		return
	}
	var in service.BlockInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Datos inválidos"})
		return
	}
	block, err := h.svc.UpdateBlock(id, in)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, block)
}

func (h *InductionHandler) DeleteBlock(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Bloque inválido"})
		return
	}
	if err := h.svc.DeleteBlock(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Bloque eliminado"})
}

// --- Interno: programas ---

func (h *InductionHandler) ListPrograms(c *gin.Context) {
	programs, err := h.svc.ListPrograms()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": programs})
}

func (h *InductionHandler) GetProgram(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Programa inválido"})
		return
	}
	program, err := h.svc.GetProgram(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, program)
}

func (h *InductionHandler) CreateProgram(c *gin.Context) {
	var in service.ProgramInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Datos inválidos"})
		return
	}
	program, err := h.svc.CreateProgram(middleware.GetUserID(c), in)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, program)
}

func (h *InductionHandler) UpdateProgram(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Programa inválido"})
		return
	}
	var in service.ProgramInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Datos inválidos"})
		return
	}
	program, err := h.svc.UpdateProgram(id, in)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, program)
}

func (h *InductionHandler) DeleteProgram(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Programa inválido"})
		return
	}
	if err := h.svc.DeleteProgram(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Programa eliminado"})
}

type idListPayload struct {
	BlockIDs   []uint `json:"block_ids"`
	CompanyIDs []uint `json:"company_ids"`
}

// SetProgramBlocks reemplaza la lista ordenada de bloques del programa.
func (h *InductionHandler) SetProgramBlocks(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Programa inválido"})
		return
	}
	var req idListPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Datos inválidos"})
		return
	}
	program, err := h.svc.SetProgramBlocks(id, req.BlockIDs)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, program)
}

// SetProgramCompanies reemplaza las empresas asignadas al programa.
func (h *InductionHandler) SetProgramCompanies(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Programa inválido"})
		return
	}
	var req idListPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Datos inválidos"})
		return
	}
	program, err := h.svc.SetProgramCompanies(id, req.CompanyIDs)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, program)
}

// --- Interno: Soporte ---

// Status devuelve el detalle de la inducción de un profesional (bloques,
// intentos, puntajes) para que Soporte tenga contexto antes de contactarlo.
func (h *InductionHandler) Status(c *gin.Context) {
	userID, ok := parseIDParam(c, "userId")
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Usuario inválido"})
		return
	}
	view, err := h.svc.Status(userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, view)
}

type invitePayload struct {
	// ProgramID es opcional: sin él se resuelve por la empresa del profesional
	// o el programa por defecto.
	ProgramID uint `json:"program_id"`
	// GatesAccess: bloquear el acceso hasta aprobar (ingreso) o no
	// (capacitación a alguien que ya trabaja). Por defecto no bloquea; si la
	// persona nunca tuvo acceso, el servicio lo fuerza.
	GatesAccess bool `json:"gates_access"`
}

// Invite emite la inducción a un profesional que ya existe (alta manual, alta
// desde la empresa o importación), que de otro modo nunca pasaría por ella.
func (h *InductionHandler) Invite(c *gin.Context) {
	userID, ok := parseIDParam(c, "userId")
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Usuario inválido"})
		return
	}
	var req invitePayload
	// El cuerpo es opcional: sin él se resuelve el programa solo.
	_ = c.ShouldBindJSON(&req)
	if err := h.svc.Invite(userID, req.ProgramID, req.GatesAccess); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Inducción enviada. El profesional recibirá el enlace por correo."})
}

// MyInduction devuelve la capacitación pendiente del usuario de la sesión,
// con su enlace, para que la app se la ofrezca desde dentro.
func (h *InductionHandler) MyInduction(c *gin.Context) {
	view, err := h.svc.MyInduction(middleware.GetUserID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo consultar la capacitación"})
		return
	}
	if view == nil {
		c.JSON(http.StatusOK, gin.H{"pending": false})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"pending":          true,
		"program_name":     view.ProgramName,
		"token":            view.Token,
		"status":           view.Status,
		"total_blocks":     view.TotalBlocks,
		"completed_blocks": view.CompletedBlocks,
		"expires_at":       view.ExpiresAt,
		"gates_access":     view.GatesAccess,
	})
}

// ResetInvite reinicia una capacitación concreta del historial.
func (h *InductionHandler) ResetInvite(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Capacitación inválida"})
		return
	}
	if err := h.svc.ResetInvite(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Capacitación reiniciada. Se reenvió el enlace al profesional."})
}

// Reset desbloquea al profesional, le devuelve sus intentos en todos los
// bloques y le reenvía el enlace. Es la acción que ejecuta Soporte tras
// contactarlo.
func (h *InductionHandler) Reset(c *gin.Context) {
	userID, ok := parseIDParam(c, "userId")
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Usuario inválido"})
		return
	}
	if err := h.svc.Reset(userID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Inducción reiniciada. Se reenvió el enlace al profesional."})
}
