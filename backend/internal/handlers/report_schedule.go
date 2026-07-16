package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/obertrack/backend/internal/middleware"
	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
	"github.com/obertrack/backend/internal/service"
)

// ReportScheduleHandler expone la configuración del envío automático de
// reportes. Todas sus rutas son solo-superadmin.
type ReportScheduleHandler struct {
	repo    repository.ReportScheduleRepository
	watcher *service.ReportMailWatcher
}

func NewReportScheduleHandler(repo repository.ReportScheduleRepository, watcher *service.ReportMailWatcher) *ReportScheduleHandler {
	return &ReportScheduleHandler{repo: repo, watcher: watcher}
}

func (h *ReportScheduleHandler) Get(c *gin.Context) {
	cfg, err := h.repo.Get()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo cargar la configuración"})
		return
	}
	c.JSON(http.StatusOK, cfg)
}

type updateReportScheduleRequest struct {
	Enabled    *bool   `json:"enabled"`
	Frequency  *string `json:"frequency"`
	Hour       *int    `json:"hour"`
	Minute     *int    `json:"minute"`
	Timezone   *string `json:"timezone"`
	Weekday    *int    `json:"weekday"`
	DayOfMonth *int    `json:"day_of_month"`
}

func (h *ReportScheduleHandler) Update(c *gin.Context) {
	var req updateReportScheduleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updates := map[string]interface{}{}

	if req.Enabled != nil {
		updates["enabled"] = *req.Enabled
	}
	if req.Frequency != nil {
		if !models.IsValidReportFrequency(*req.Frequency) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Frecuencia inválida: usa daily, weekly o monthly"})
			return
		}
		updates["frequency"] = *req.Frequency
	}
	if req.Hour != nil {
		if *req.Hour < 0 || *req.Hour > 23 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "La hora debe estar entre 0 y 23"})
			return
		}
		updates["hour"] = *req.Hour
	}
	if req.Minute != nil {
		if *req.Minute < 0 || *req.Minute > 59 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Los minutos deben estar entre 0 y 59"})
			return
		}
		updates["minute"] = *req.Minute
	}
	if req.Timezone != nil {
		// Validar contra la base IANA embebida (ver import _ "time/tzdata").
		if _, err := time.LoadLocation(*req.Timezone); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Zona horaria inválida: " + *req.Timezone})
			return
		}
		updates["timezone"] = *req.Timezone
	}
	if req.Weekday != nil {
		if *req.Weekday < 0 || *req.Weekday > 6 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "El día de la semana debe estar entre 0 (domingo) y 6 (sábado)"})
			return
		}
		updates["weekday"] = *req.Weekday
	}
	if req.DayOfMonth != nil {
		// Tope en 28 para que exista en todos los meses (incluido febrero).
		if *req.DayOfMonth < 1 || *req.DayOfMonth > 28 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "El día del mes debe estar entre 1 y 28"})
			return
		}
		updates["day_of_month"] = *req.DayOfMonth
	}

	if len(updates) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No hay cambios que guardar"})
		return
	}
	updates["updated_by"] = middleware.GetUserID(c)

	cfg, err := h.repo.Update(updates)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo guardar la configuración"})
		return
	}
	c.JSON(http.StatusOK, cfg)
}

// RunNow dispara una corrida manual: ignora `enabled` y la hora programada, pero
// respeta la deduplicación (no reenvía un período ya entregado).
func (h *ReportScheduleHandler) RunNow(c *gin.Context) {
	sent, skipped, failed, err := h.watcher.RunOnce(true)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "La corrida falló: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"sent": sent, "skipped": skipped, "failed": failed})
}

func (h *ReportScheduleHandler) ListRuns(c *gin.Context) {
	limit := 50
	if v, err := strconv.Atoi(c.Query("limit")); err == nil && v > 0 {
		limit = v
	}
	runs, err := h.repo.ListRuns(limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo cargar la bitácora"})
		return
	}
	c.JSON(http.StatusOK, runs)
}
