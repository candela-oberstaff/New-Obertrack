package handlers

import (
	"fmt"
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

// Update enciende/apaga o actualiza la fecha y destinatarios de un tipo de correo.
func (h *EmailSettingsHandler) Update(c *gin.Context) {
	key := c.Param("key")
	var req service.UpdateEmailSettingReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cuerpo de solicitud inválido"})
		return
	}

	if err := h.svc.UpdateSetting(key, req, middleware.GetUserID(c)); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	action := "email_settings.updated"
	if req.Enabled != nil {
		if *req.Enabled {
			action = "email_settings.enabled"
		} else {
			action = "email_settings.disabled"
		}
	}
	middleware.SetAudit(c, action, key, fmt.Sprintf(`{"key":%q}`, key))

	c.JSON(http.StatusOK, gin.H{"key": key, "message": "Configuración actualizada correctamente"})
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

// GetPreview devuelve el asunto y cuerpo de la vista previa de un correo.
func (h *EmailSettingsHandler) GetPreview(c *gin.Context) {
	key := c.Param("key")
	userName := "Lorena Moujalli"
	if v, ok := c.Get("name"); ok {
		if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
			userName = s
		}
	}
	subject, body, err := h.svc.GetPreview(key, userName)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"key": key, "subject": subject, "body": body})
}
