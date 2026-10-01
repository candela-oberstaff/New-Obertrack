package handlers

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"

	"github.com/obertrack/backend/internal/middleware"
	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/service"
)

// CrmSSOHandler es la puerta del acceso embebido a Tareas desde el CRM
// (docs/integracion-crm-tareas.md). No emite cookies: la sesión viaja en el
// fragmento de la redirección y el frontend la guarda solo en memoria.
type CrmSSOHandler struct {
	svc   *service.CrmSSOService
	audit service.AuditService
}

func NewCrmSSOHandler(svc *service.CrmSSOService, audit service.AuditService) *CrmSSOHandler {
	return &CrmSSOHandler{svc: svc, audit: audit}
}

// Exchange es GET /api/auth/crm?token=<jwt> (§4). Responde siempre con una
// redirección 303, a /embed/tareas#s=<access> o a /embed/error?code=<código>,
// salvo cuando el canje está deshabilitado o falla algo interno (503): ahí no
// hay código de contrato que dar, y el CRM lo cubre con su tiempo límite.
func (h *CrmSSOHandler) Exchange(c *gin.Context) {
	c.Header("Referrer-Policy", "no-referrer")
	c.Header("Cache-Control", "no-store")

	if !h.svc.Enabled() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "El acceso desde el CRM no está configurado"})
		return
	}

	res, err := h.svc.Exchange(c.Query("token"))
	if err != nil {
		var ssoErr *service.CrmSSOError
		if !errors.As(err, &ssoErr) {
			log.Printf("[CRM SSO] fallo interno en el canje: %v", err)
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "No se pudo completar el acceso. Inténtalo de nuevo."})
			return
		}
		h.record(c, false, nil, "", ssoErr.Email, ssoErr.JTI, map[string]interface{}{
			"code": ssoErr.Code, "reason": ssoErr.Reason, "kid": ssoErr.Kid, "company_id": ssoErr.CompanyID,
		})
		redirect303(c, "/embed/error?code="+url.QueryEscape(ssoErr.Code))
		return
	}

	h.record(c, true, &res.User.ID, string(res.User.UserType), res.User.Email, res.JTI, map[string]interface{}{
		"kid": res.Kid, "company_id": res.CompanyID, "scope": res.Scope,
	})
	redirect303(c, embedPathFor(res.Scope)+"#s="+res.AccessToken)
}

// embedPathFor es la vista embebida de cada alcance (contrato CRM, §4).
func embedPathFor(scope string) string {
	if scope == middleware.ScopeHours {
		return "/embed/horas"
	}
	return "/embed/tareas"
}

// redirect303 escribe la redirección a mano: http.Redirect pasa la ruta por
// path.Clean y no conviene que toque un fragmento que lleva un token.
func redirect303(c *gin.Context, location string) {
	c.Header("Location", location)
	c.Status(http.StatusSeeOther)
}

// record deja la auditoría del canje con el motivo real y el kid (§7). Nunca
// el token: el jti basta para cruzarlo con la auditoría del CRM.
func (h *CrmSSOHandler) record(c *gin.Context, success bool, actorID *uint, role, email, jti string, detail map[string]interface{}) {
	action := "auth.crm_login"
	if !success {
		action = "auth.crm_login_failed"
	}
	changes, _ := json.Marshal(detail)
	h.audit.Record(models.AuditLog{
		Kind:       "activity",
		ActorID:    actorID,
		ActorEmail: email,
		ActorRole:  role,
		Action:     action,
		Module:     "auth",
		EntityType: "crm_sso",
		EntityID:   jti,
		Changes:    string(changes),
		Method:     http.MethodGet,
		Path:       "/api/auth/crm",
		Status:     http.StatusSeeOther,
		Success:    success,
		IP:         c.ClientIP(),
		UserAgent:  c.Request.UserAgent(),
	})
}
