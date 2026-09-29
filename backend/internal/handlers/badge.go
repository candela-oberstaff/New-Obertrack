package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/obertrack/backend/internal/middleware"
	"github.com/obertrack/backend/internal/service"
)

// BadgeHandler expone las insignias ganadas en la inducción: las propias
// (perfil) y las de otra persona (expediente en Admin, ficha del empleado
// para su empresa). Quién puede ver a quién lo decide la misma regla que el
// detalle del usuario, para no inventar una segunda.
type BadgeHandler struct {
	induction service.InductionService
	users     service.UserService
}

func NewBadgeHandler(induction service.InductionService, users service.UserService) *BadgeHandler {
	return &BadgeHandler{induction: induction, users: users}
}

// Mine devuelve las insignias del usuario de la sesión, con las pendientes.
func (h *BadgeHandler) Mine(c *gin.Context) {
	overview, err := h.induction.ListBadges(middleware.GetUserID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudieron cargar las insignias"})
		return
	}
	c.JSON(http.StatusOK, overview)
}

// ForUser devuelve las insignias de otra persona. Reusa la visibilidad del
// detalle de usuario: superadmin y soporte ven a cualquiera, la empresa a los
// suyos, y cada quien a sí mismo.
func (h *BadgeHandler) ForUser(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Usuario inválido"})
		return
	}
	if _, err := h.users.GetByID(id, middleware.GetUserID(c), middleware.GetTenantID(c), middleware.GetUserRole(c), middleware.IsSuperadmin(c)); err != nil {
		status := http.StatusNotFound
		if err.Error() == "Access denied" {
			status = http.StatusForbidden
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	overview, err := h.induction.ListBadges(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudieron cargar las insignias"})
		return
	}
	c.JSON(http.StatusOK, overview)
}
