package middleware

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/obertrack/backend/internal/models"
)

// ActivityTracker es la superficie mínima que necesita el middleware de uso
// (la implementa service.ActivityService). Se declara aquí, como AuditRecorder,
// para no crear un ciclo service→middleware.
type ActivityTracker interface {
	Track(userID, tenantID uint, module string, at time.Time)
}

// activityModules traduce el primer segmento de la ruta al módulo de PRODUCTO.
// Lo que se mide es la pantalla que la persona cree estar usando, no el grupo
// de rutas: /api/tasks y /api/boards son las dos "Tareas", y /api/chat y
// /api/channels son las dos "Chat". Un segmento que no esté aquí no cuenta
// como módulo —sí como uso de la app— porque un nombre de ruta suelto en el
// panel de Customer Success no le dice nada a nadie.
var activityModules = map[string]string{
	"chat":              models.ModuleChat,
	"chats":             models.ModuleChat,
	"channels":          models.ModuleChat,
	"tasks":             models.ModuleTasks,
	"boards":            models.ModuleTasks,
	"board-invitations": models.ModuleTasks,
	"work-hours":        models.ModuleHours,
	"meetings":          models.ModuleMeetings,
	"tickets":           models.ModuleSupport,
	"follow-ups":        models.ModuleSupport,
	"surveys":           models.ModuleSurveys,
	"tutorials":         models.ModuleNews,
	"admin":             models.ModuleAdmin,
	"employer":          models.ModuleCompany,
	"users":             models.ModulePeople,
	"email":             models.ModuleEmail,
	"audiences":         models.ModuleEmail,
	"testimonial":       models.ModuleTestimony,
	"testimonials":      models.ModuleTestimony,
	"workflows":         models.ModuleWorkflows,
	"profile":           models.ModuleProfile,
	"induction":         models.ModuleInduction,
	"inductions":        models.ModuleInduction,
}

// activityPollPaths son las llamadas que el layout dispara SOLO por estar la
// app abierta, en cualquier pantalla: el badge de mensajes sin leer, el
// contador de la campanita y la cola de Novedades.
//
// Sin esta lista el "% de uso del chat" saldría 100% siempre —el sidebar
// pregunta por los no leídos cada 30 segundos aunque nadie entre al chat— y la
// métrica que más le importa a Customer Success sería exactamente la que
// miente. Siguen contando como uso de la app (ModuleApp): que el sondeo llegue
// prueba que la pestaña está viva, que es justo lo que ModuleApp mide.
var activityPollPaths = map[string]bool{
	"/api/channels/unread/total":      true,
	"/api/notifications/unread-count": true,
	"/api/notifications":              true,
	"/api/tutorials/pending":          true,
	"/api/auth/me":                    true,
	"/api/auth/refresh":               true,
}

// ActivityMiddleware anota el uso real de la app: una marca por petición
// autenticada que salió bien. Corre DESPUÉS del handler para poder mirar el
// código de respuesta —un 403 contra una pantalla que no te toca no es uso— y
// no toca la base de datos: solo suma en memoria (ver service.ActivityService),
// que se vacía por lotes cada pocos segundos.
func ActivityMiddleware(tracker ActivityTracker) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if tracker == nil {
			return
		}
		userID := GetUserID(c)
		if userID == 0 {
			return
		}
		// Peticiones fallidas fuera: un 401 por token vencido o un 404 no son
		// uso, y contarlos inflaría justo a quien tiene la sesión rota.
		if c.Writer.Status() >= 400 {
			return
		}

		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}
		now := time.Now()
		tenantID := GetTenantID(c)

		tracker.Track(userID, tenantID, models.ModuleApp, now)

		if activityPollPaths[path] {
			return
		}
		if module := activityModuleFor(path); module != "" {
			tracker.Track(userID, tenantID, module, now)
		}
	}
}

// activityModuleFor devuelve el módulo de producto de una ruta, o "" si no
// corresponde a ninguno (rutas técnicas: subidas, métricas, auth, semillas).
func activityModuleFor(path string) string {
	segment := moduleFromPath(path)
	// El wallet cuelga de /me, que como segmento no distingue nada; se
	// reconoce por la ruta completa antes de mirar el mapa.
	if strings.HasPrefix(path, "/api/me/wallet") {
		return models.ModuleWallet
	}
	return activityModules[segment]
}
