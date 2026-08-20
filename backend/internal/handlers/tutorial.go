package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/obertrack/backend/internal/middleware"
	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/service"
)

type TutorialHandler struct {
	service service.TutorialService
}

func NewTutorialHandler(service service.TutorialService) *TutorialHandler {
	return &TutorialHandler{service: service}
}

type CreateTutorialRequest struct {
	Title          string `json:"title"`
	Description    string `json:"description"`
	ContentType    string `json:"content_type"`
	GoogleDriveURL string `json:"google_drive_url"`
	ImageURL       string `json:"image_url"`
	Body           string `json:"body"`
	IconName       string `json:"icon_name"`
	Category       string `json:"category"`
	Audience       string `json:"audience"`
	DurationMin    int    `json:"duration_min"`
	OrderIndex     int    `json:"order_index"`
	// AnnounceDays son los días que el aviso emergente insiste con la novedad.
	// Ausente = el default del servicio; 0 = solo notificación, sin emergente.
	AnnounceDays *int `json:"announce_days"`
	// AnnounceMaxShows limita cuantas veces aparece el aviso. 0 = sin limite.
	AnnounceMaxShows *int `json:"announce_max_shows"`
	// Target acota el publico por encima del tipo de cuenta. Ausente = sin acotar.
	Target *models.TutorialTarget `json:"target"`
	// Boton de accion opcional.
	CTALabel string `json:"cta_label"`
	CTAURL   string `json:"cta_url"`
	// Programacion.
	PublishAt  *time.Time `json:"publish_at"`
	ExpiresAt  *time.Time `json:"expires_at"`
	RequireAck bool       `json:"require_ack"`
	IsActive   *bool      `json:"is_active"`
}

type UpdateTutorialRequest struct {
	Title            *string                `json:"title"`
	Description      *string                `json:"description"`
	ContentType      *string                `json:"content_type"`
	GoogleDriveURL   *string                `json:"google_drive_url"`
	ImageURL         *string                `json:"image_url"`
	Body             *string                `json:"body"`
	IconName         *string                `json:"icon_name"`
	Category         *string                `json:"category"`
	Audience         *string                `json:"audience"`
	DurationMin      *int                   `json:"duration_min"`
	OrderIndex       *int                   `json:"order_index"`
	AnnounceDays     *int                   `json:"announce_days"`
	AnnounceMaxShows *int                   `json:"announce_max_shows"`
	Target           *models.TutorialTarget `json:"target"`
	CTALabel         *string                `json:"cta_label"`
	CTAURL           *string                `json:"cta_url"`
	PublishAt        *time.Time             `json:"publish_at"`
	ExpiresAt        *time.Time             `json:"expires_at"`
	RequireAck       *bool                  `json:"require_ack"`
	IsActive         *bool                  `json:"is_active"`
}

type ReorderTutorialsRequest struct {
	IDs []uint `json:"ids" binding:"required"`
}

// audienceForRequest maps the authenticated user's type to the tutorial audience
// they're allowed to see. Empty string means no filter: superadmins and platform
// staff (customer_success) see tutorials for every audience.
// audienceForRequest traduce quién pide a qué audiencias le alcanzan. Vacío =
// sin filtro: superadmin y personal de plataforma ven todas. Un manager recibe
// dos audiencias, la de profesional y la suya, porque el rol de manager va
// dentro de los profesionales y no en lugar de ellos.
func audienceForRequest(c *gin.Context) []string {
	if middleware.IsSuperadmin(c) {
		return nil
	}
	return models.AudiencesForUser(
		middleware.GetUserRole(c),
		middleware.IsManager(c) || middleware.IsSupervisor(c),
	)
}

func (h *TutorialHandler) GetAll(c *gin.Context) {
	onlyActive := !middleware.IsSuperadmin(c)

	tutorials, err := h.service.GetAll(onlyActive, audienceForRequest(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch tutorials", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": tutorials})
}

func (h *TutorialHandler) GetByID(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid tutorial ID"})
		return
	}

	tutorial, err := h.service.GetByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Tutorial no encontrado"})
		return
	}

	if audiences := audienceForRequest(c); len(audiences) > 0 {
		allowed := false
		for _, audience := range audiences {
			if tutorial.Audience == audience {
				allowed = true
				break
			}
		}
		if !allowed {
			c.JSON(http.StatusNotFound, gin.H{"error": "Tutorial no encontrado"})
			return
		}
	}

	c.JSON(http.StatusOK, tutorial)
}

func (h *TutorialHandler) Create(c *gin.Context) {
	var req CreateTutorialRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := middleware.GetUserID(c)
	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}
	announceDays := -1 // El servicio traduce el negativo a su default.
	if req.AnnounceDays != nil {
		announceDays = *req.AnnounceDays
	}
	announceMaxShows := 0 // Sin tope, que es como se comportaba siempre.
	if req.AnnounceMaxShows != nil {
		announceMaxShows = *req.AnnounceMaxShows
	}

	tutorial, err := h.service.Create(userID, service.TutorialInput{
		Title:            req.Title,
		Description:      req.Description,
		ContentType:      req.ContentType,
		VideoURL:         req.GoogleDriveURL,
		ImageURL:         req.ImageURL,
		Body:             req.Body,
		IconName:         req.IconName,
		Category:         req.Category,
		Audience:         req.Audience,
		DurationMin:      req.DurationMin,
		OrderIndex:       req.OrderIndex,
		AnnounceDays:     announceDays,
		AnnounceMaxShows: announceMaxShows,
		IsActive:         isActive,
		Target:           targetOrEmpty(req.Target),
		CTALabel:         req.CTALabel,
		CTAURL:           req.CTAURL,
		PublishAt:        req.PublishAt,
		ExpiresAt:        req.ExpiresAt,
		RequireAck:       req.RequireAck,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, tutorial)
}

func (h *TutorialHandler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid tutorial ID"})
		return
	}

	var req UpdateTutorialRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updates := map[string]interface{}{}
	if req.Title != nil {
		updates["title"] = *req.Title
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.ContentType != nil {
		updates["content_type"] = *req.ContentType
	}
	if req.GoogleDriveURL != nil {
		updates["google_drive_url"] = *req.GoogleDriveURL
	}
	if req.ImageURL != nil {
		updates["image_url"] = *req.ImageURL
	}
	if req.Body != nil {
		updates["body"] = *req.Body
	}
	if req.IconName != nil {
		updates["icon_name"] = *req.IconName
	}
	if req.Category != nil {
		updates["category"] = *req.Category
	}
	if req.Audience != nil {
		updates["audience"] = *req.Audience
	}
	if req.DurationMin != nil {
		updates["duration_min"] = *req.DurationMin
	}
	if req.OrderIndex != nil {
		updates["order_index"] = *req.OrderIndex
	}
	if req.AnnounceDays != nil {
		updates["announce_days"] = *req.AnnounceDays
	}
	if req.AnnounceMaxShows != nil {
		updates["announce_max_shows"] = *req.AnnounceMaxShows
	}
	if req.Target != nil {
		// Viaja como struct: el servicio decide como se guarda.
		updates["target"] = *req.Target
	}
	if req.CTALabel != nil {
		updates["cta_label"] = *req.CTALabel
	}
	if req.CTAURL != nil {
		updates["cta_url"] = *req.CTAURL
	}
	if req.PublishAt != nil {
		updates["publish_at"] = *req.PublishAt
	}
	if req.ExpiresAt != nil {
		updates["expires_at"] = *req.ExpiresAt
	}
	if req.RequireAck != nil {
		updates["require_ack"] = *req.RequireAck
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}

	if len(updates) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No fields to update"})
		return
	}

	tutorial, err := h.service.Update(middleware.GetUserID(c), uint(id), updates)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, tutorial)
}

func (h *TutorialHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid tutorial ID"})
		return
	}

	if err := h.service.Delete(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete tutorial"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Tutorial eliminado"})
}

func (h *TutorialHandler) Reorder(c *gin.Context) {
	var req ReorderTutorialsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.Reorder(req.IDs); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Orden actualizado"})
}

type RecordViewRequest struct {
	// Source distingue si la vista salió del aviso a pantalla completa o de la
	// sección. Cuerpo opcional: sin él se asume la sección.
	Source string `json:"source"`
	// Acknowledged marca que la persona confirmó haber leído la novedad, en
	// las que lo exigen. Se sella aparte de la vista porque es evidencia.
	Acknowledged bool `json:"acknowledged"`
}

func (h *TutorialHandler) RecordView(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid tutorial ID"})
		return
	}

	var req RecordViewRequest
	_ = c.ShouldBindJSON(&req)

	userID := middleware.GetUserID(c)
	if err := h.service.RecordView(uint(id), userID, req.Source, req.Acknowledged); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Vista registrada"})
}

// GetPending devuelve las novedades que el usuario todavía no ha visto. Es lo
// que la app consulta al entrar para mostrarlas en una ventana emergente: la
// audiencia se resuelve igual que en el listado, así que nadie ve por sorpresa
// una novedad que no le tocaba.
func (h *TutorialHandler) GetPending(c *gin.Context) {
	tutorials, err := h.service.GetPendingAnnouncements(middleware.GetUserID(c), audienceForRequest(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch pending tutorials", "details": err.Error()})
		return
	}
	if tutorials == nil {
		tutorials = []models.Tutorial{}
	}
	c.JSON(http.StatusOK, gin.H{"data": tutorials})
}

// targetOrEmpty normaliza el publico ausente a "sin acotar".
func targetOrEmpty(target *models.TutorialTarget) models.TutorialTarget {
	if target == nil {
		return models.TutorialTarget{}
	}
	return *target
}

type PreviewAudienceRequest struct {
	Audience string                 `json:"audience"`
	Target   *models.TutorialTarget `json:"target"`
}

// PreviewAudience responde a cuanta gente llegaria la novedad con la audiencia
// y el publico elegidos. Es lo que deja acotar sin publicar a ciegas.
func (h *TutorialHandler) PreviewAudience(c *gin.Context) {
	var req PreviewAudienceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	preview, err := h.service.PreviewAudience(req.Audience, targetOrEmpty(req.Target))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, preview)
}

// GetAudienceOptions lista empresas, paises y grupos elegibles como publico.
func (h *TutorialHandler) GetAudienceOptions(c *gin.Context) {
	options, err := h.service.GetAudienceOptions()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch audience options", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, options)
}

// RecordShow anota que el aviso a pantalla completa se mostró una vez más.
func (h *TutorialHandler) RecordShow(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid tutorial ID"})
		return
	}

	if err := h.service.RecordShow(uint(id), middleware.GetUserID(c)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Aparición registrada"})
}

// RecordClick anota que la persona pulsó el botón de acción de la novedad.
func (h *TutorialHandler) RecordClick(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid tutorial ID"})
		return
	}

	if err := h.service.RecordClick(uint(id), middleware.GetUserID(c)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Clic registrado"})
}

// RemindPending vuelve a avisar a quienes todavía no han visto la novedad.
func (h *TutorialHandler) RemindPending(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid tutorial ID"})
		return
	}

	reminded, err := h.service.RemindPending(middleware.GetUserID(c), uint(id))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"reminded": reminded})
}

// GetMetrics devuelve el desempeño de una novedad. Solo para superadmin: es
// quien publica y el único que ve la sección completa.
func (h *TutorialHandler) GetMetrics(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid tutorial ID"})
		return
	}

	metrics, err := h.service.GetMetrics(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, metrics)
}

func (h *TutorialHandler) GetMyViews(c *gin.Context) {
	userID := middleware.GetUserID(c)
	ids, err := h.service.GetUserViewedIDs(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if ids == nil {
		ids = []uint{}
	}
	c.JSON(http.StatusOK, gin.H{"data": ids})
}
