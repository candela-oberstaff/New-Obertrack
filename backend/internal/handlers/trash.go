package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/obertrack/backend/internal/service"
)

type TrashHandler struct {
	svc service.TrashService
}

func NewTrashHandler(svc service.TrashService) *TrashHandler {
	return &TrashHandler{svc: svc}
}

func (h *TrashHandler) List(c *gin.Context) {
	var types []string
	if raw := strings.TrimSpace(c.Query("types")); raw != "" {
		types = strings.Split(raw, ",")
	}
	items, err := h.svc.List(types)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo cargar la papelera: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "types": h.svc.Types()})
}

type trashRef struct {
	Type string `json:"type"`
	ID   uint   `json:"id"`
}

type trashFailure struct {
	Type  string `json:"type"`
	ID    uint   `json:"id"`
	Error string `json:"error"`
}

func (h *TrashHandler) Restore(c *gin.Context) {
	h.apply(c, "restored", h.svc.Restore)
}

func (h *TrashHandler) Purge(c *gin.Context) {
	h.apply(c, "purged", h.svc.Purge)
}

func (h *TrashHandler) apply(c *gin.Context, okKey string, op func(typeKey string, id uint) error) {
	var req struct {
		Items []trashRef `json:"items"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if len(req.Items) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No hay elementos seleccionados"})
		return
	}
	done := 0
	failed := []trashFailure{}
	for _, it := range req.Items {
		if err := op(it.Type, it.ID); err != nil {
			failed = append(failed, trashFailure{Type: it.Type, ID: it.ID, Error: err.Error()})
			continue
		}
		done++
	}
	c.JSON(http.StatusOK, gin.H{okKey: done, "failed": failed})
}
