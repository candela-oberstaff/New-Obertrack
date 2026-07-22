package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/obertrack/backend/internal/service"
)

// WalletHandler expone la vista PERSONAL de la billetera: cada usuario consulta
// solo sus propios pagos. Es de solo lectura y self-service (cualquier usuario
// autenticado ve lo suyo; el filtrado por email ocurre en el servicio).
type WalletHandler struct {
	service service.WalletService
}

func NewWalletHandler(s service.WalletService) *WalletHandler {
	return &WalletHandler{service: s}
}

// MyWallet devuelve las ganancias del usuario autenticado. Si la integración no
// está configurada responde 200 con enabled:false para que el cliente muestre un
// estado neutro (sin filtrar detalles técnicos al usuario final).
func (h *WalletHandler) MyWallet(c *gin.Context) {
	if !h.service.Enabled() {
		c.JSON(http.StatusOK, gin.H{"enabled": false})
		return
	}
	email := c.GetString("email")
	if email == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Sesión inválida"})
		return
	}
	summary, err := h.service.MyEarnings(email)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"enabled": true, "summary": summary})
}
