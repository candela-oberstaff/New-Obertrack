package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/obertrack/backend/internal/middleware"
	"github.com/obertrack/backend/internal/service"
)

// EmailSettingsHandler expone el panel de Configuración → Correos: qué correos
// salen del sistema, su interruptor y el envío de una muestra.
type EmailSettingsHandler struct {
	svc *service.EmailSettingsService
}

func NewEmailSettingsHandler(svc *service.EmailSettingsService) *EmailSettingsHandler {
	return &EmailSettingsHandler{svc: svc}
}

// List devuelve el catálogo de correos con su estado.
func (h *EmailSettingsHandler) List(c *gin.Context) {
	c.JSON(http.StatusOK, h.svc.List())
}

// Update enciende o apaga un tipo de correo.
func (h *EmailSettingsHandler) Update(c *gin.Context) {
	key := c.Param("key")
	var req struct {
		Enabled *bool `json:"enabled" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Enabled == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Falta el valor de 'enabled'"})
		return
	}
	if err := h.svc.SetEnabled(key, *req.Enabled, middleware.GetUserID(c)); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"key": key, "enabled": *req.Enabled})
}

// UpdateAll enciende o apaga todos los correos del sistema de una vez.
func (h *EmailSettingsHandler) UpdateAll(c *gin.Context) {
	var req struct {
		Enabled *bool `json:"enabled" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Enabled == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Falta el valor de 'enabled'"})
		return
	}
	if err := h.svc.SetAll(*req.Enabled, middleware.GetUserID(c)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo aplicar el cambio"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"enabled": *req.Enabled})
}

// SendTest manda una muestra del correo. Sin destinatario en el cuerpo usa el
// correo de quien lo pide (lo habitual: "quiero ver cómo llega").
func (h *EmailSettingsHandler) SendTest(c *gin.Context) {
	key := c.Param("key")
	var req struct {
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	_ = c.ShouldBindJSON(&req)

	to := strings.TrimSpace(req.Email)
	if to == "" {
		// El correo de la sesión (lo pone el middleware de auth desde el JWT).
		if v, ok := c.Get("email"); ok {
			to, _ = v.(string)
		}
	}
	if to == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Indica un correo de destino para la prueba"})
		return
	}

	if err := h.svc.SendTest(key, to, req.Name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Correo de prueba enviado a " + to, "email": to})
}
