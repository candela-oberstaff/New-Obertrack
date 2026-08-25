package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/obertrack/backend/internal/middleware"
	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/service"
)

type WorkflowHandler struct {
	svc *service.WorkflowService
}

func NewWorkflowHandler(svc *service.WorkflowService) *WorkflowHandler {
	return &WorkflowHandler{svc: svc}
}

// actorFrom arma el actor a partir de los claims del token. El tipo de cuenta y el
// tenant salen del JWT, no de la base: son los mismos que gobiernan el resto de los
// módulos y así una sesión revocada no puede seguir configurando reglas.
func actorFrom(c *gin.Context) service.WorkflowActor {
	role := middleware.GetUserRole(c)
	return service.WorkflowActor{
		UserID:       middleware.GetUserID(c),
		TenantID:     middleware.GetTenantID(c),
		IsSuperadmin: middleware.IsSuperadmin(c),
		IsEmployer:   role == string(models.UserTypeEmployer),
		IsManager:    middleware.IsManager(c),
	}
}

// RequireWorkflowsAccess es el portero del módulo. Fail-closed: sólo pasan
// superadmin, empleador, manager y supervisor. A diferencia de RequirePermission,
// NO deja pasar a quien no tiene roles RBAC asignados — ese default es razonable
// para consultar tareas y no lo es para un módulo que puede disparar correos.
func RequireWorkflowsAccess() gin.HandlerFunc {
	return func(c *gin.Context) {
		if service.CanConfigureWorkflows(actorFrom(c)) {
			c.Next()
			return
		}
		c.JSON(http.StatusForbidden, gin.H{
			"error": "Sólo la empresa, los managers y los supervisores pueden configurar automatizaciones",
		})
		c.Abort()
	}
}

// respondScoped traduce los errores del servicio al código HTTP que les toca. El
// alcance de tablero devuelve 403 y no 404 a propósito: quien pregunta por un
// tablero de su empresa al que no pertenece tiene que saber que existe y que no le
// corresponde, no quedarse pensando que se borró.
func respondScoped(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrWorkflowBoardScope):
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrRecipeNotFound), errors.Is(err, service.ErrWorkflowNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrGateNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrPhaseAlreadyGated), errors.Is(err, service.ErrGateIsRecipe):
		// Conflicto con algo que ya existe, no un dato mal escrito: el cliente no
		// arregla esto reintentando con otro texto, sino eligiendo otra columna o
		// yendo a la pantalla de recetas.
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrPhaseRequired), errors.Is(err, service.ErrPhaseNotInBoard),
		errors.Is(err, service.ErrGateNameRequired), errors.Is(err, service.ErrGateForm):
		// Falta un dato o es incoherente: es la petición la que está mal, no el
		// servidor, y el mensaje ya explica qué corregir.
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo completar la operación"})
	}
}

func boardIDFromQuery(c *gin.Context) (uint, bool) {
	raw := c.Query("board_id")
	id, err := strconv.ParseUint(raw, 10, 32)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Falta el tablero (board_id)"})
		return 0, false
	}
	return uint(id), true
}

// ---------------------------------------------------------------------------
// Constructor de puertas
// ---------------------------------------------------------------------------

// Gates lista las puertas PROPIAS de un tablero (las del catálogo salen en Recipes).
func (h *WorkflowHandler) Gates(c *gin.Context) {
	boardID, ok := boardIDFromQuery(c)
	if !ok {
		return
	}
	gates, err := h.svc.ListGates(actorFrom(c), boardID)
	if err != nil {
		respondScoped(c, err)
		return
	}
	c.JSON(http.StatusOK, gates)
}

func (h *WorkflowHandler) CreateGate(c *gin.Context) {
	var in service.GateInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	wf, err := h.svc.CreateGate(actorFrom(c), in)
	if err != nil {
		respondScoped(c, err)
		return
	}
	middleware.SetAudit(c, "workflows.create_gate", strconv.FormatUint(uint64(wf.ID), 10),
		`{"board_id":`+strconv.FormatUint(uint64(wf.BoardID), 10)+`}`)
	c.JSON(http.StatusCreated, gin.H{"id": wf.ID, "name": wf.Name, "enabled": wf.Enabled})
}

func (h *WorkflowHandler) UpdateGate(c *gin.Context) {
	id, ok := idFromParam(c)
	if !ok {
		return
	}
	var in service.GateInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	wf, err := h.svc.UpdateGate(actorFrom(c), id, in)
	if err != nil {
		respondScoped(c, err)
		return
	}
	middleware.SetAudit(c, "workflows.update_gate", strconv.FormatUint(uint64(wf.ID), 10), "{}")
	c.JSON(http.StatusOK, gin.H{"id": wf.ID, "name": wf.Name, "enabled": wf.Enabled})
}

func (h *WorkflowHandler) DeleteGate(c *gin.Context) {
	id, ok := idFromParam(c)
	if !ok {
		return
	}
	if err := h.svc.DeleteGate(actorFrom(c), id); err != nil {
		respondScoped(c, err)
		return
	}
	middleware.SetAudit(c, "workflows.delete_gate", strconv.FormatUint(uint64(id), 10), "{}")
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

func idFromParam(c *gin.Context) (uint, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Identificador inválido"})
		return 0, false
	}
	return uint(id), true
}

// Recipes lista el catálogo de recetas con su estado en un tablero.
func (h *WorkflowHandler) Recipes(c *gin.Context) {
	boardID, ok := boardIDFromQuery(c)
	if !ok {
		return
	}
	recipes, err := h.svc.Recipes(actorFrom(c), boardID)
	if err != nil {
		respondScoped(c, err)
		return
	}
	c.JSON(http.StatusOK, recipes)
}

// List lista las automatizaciones ya creadas en un tablero.
func (h *WorkflowHandler) List(c *gin.Context) {
	boardID, ok := boardIDFromQuery(c)
	if !ok {
		return
	}
	items, err := h.svc.ListForBoard(actorFrom(c), boardID)
	if err != nil {
		respondScoped(c, err)
		return
	}
	c.JSON(http.StatusOK, items)
}

type setRecipeRequest struct {
	BoardID uint   `json:"board_id" binding:"required"`
	Recipe  string `json:"recipe" binding:"required"`
	Enabled *bool  `json:"enabled" binding:"required"`
	// PhaseID sólo lo llevan las recetas de PUERTA: la columna que se convierte en
	// punto de control.
	PhaseID uint `json:"phase_id,omitempty"`
}

// SetRecipe enciende o apaga una receta en un tablero.
func (h *WorkflowHandler) SetRecipe(c *gin.Context) {
	var req setRecipeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	wf, err := h.svc.SetRecipeEnabled(actorFrom(c), req.BoardID, req.Recipe, *req.Enabled, req.PhaseID)
	if err != nil {
		respondScoped(c, err)
		return
	}
	// Apagar una receta que nunca se activó no crea nada y no es un error.
	if wf == nil {
		c.JSON(http.StatusOK, gin.H{"enabled": false})
		return
	}

	middleware.SetAudit(c, "workflows.set_recipe", strconv.FormatUint(uint64(wf.ID), 10),
		`{"recipe":"`+req.Recipe+`","enabled":`+strconv.FormatBool(*req.Enabled)+`}`)

	c.JSON(http.StatusOK, gin.H{
		"id":      wf.ID,
		"enabled": wf.Enabled,
		"name":    wf.Name,
	})
}

// Runs devuelve el historial de ejecuciones de una automatización.
func (h *WorkflowHandler) Runs(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Identificador inválido"})
		return
	}
	runs, rerr := h.svc.Runs(actorFrom(c), uint(id))
	if rerr != nil {
		respondScoped(c, rerr)
		return
	}
	c.JSON(http.StatusOK, runs)
}
