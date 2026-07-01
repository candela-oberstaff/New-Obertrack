package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/obertrack/backend/internal/middleware"
	"github.com/obertrack/backend/internal/service"
)

type ProfileChangeHandler struct {
	svc service.ProfileChangeService
}

func NewProfileChangeHandler(svc service.ProfileChangeService) *ProfileChangeHandler {
	return &ProfileChangeHandler{svc: svc}
}

func (h *ProfileChangeHandler) Create(c *gin.Context) {
	var req struct {
		Changes map[string]string `json:"changes"`
		Note    string            `json:"note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	result, err := h.svc.CreateRequest(middleware.GetUserID(c), req.Changes, req.Note)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, result)
}

func (h *ProfileChangeHandler) GetMine(c *gin.Context) {
	req, err := h.svc.GetPending(middleware.GetUserID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, req)
}

func (h *ProfileChangeHandler) GetForUser(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	req, err := h.svc.GetPending(uint(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, req)
}

func (h *ProfileChangeHandler) Apply(c *gin.Context) {
	reqID, _ := strconv.ParseUint(c.Param("reqId"), 10, 32)
	var body struct {
		Values map[string]string `json:"values"`
	}
	_ = c.ShouldBindJSON(&body)
	if err := h.svc.Apply(uint(reqID), middleware.GetUserID(c), body.Values); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Cambios aplicados"})
}

func (h *ProfileChangeHandler) Reject(c *gin.Context) {
	reqID, _ := strconv.ParseUint(c.Param("reqId"), 10, 32)
	var body struct {
		Reason string `json:"reason"`
	}
	_ = c.ShouldBindJSON(&body)
	if err := h.svc.Reject(uint(reqID), middleware.GetUserID(c), body.Reason); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Solicitud rechazada"})
}
