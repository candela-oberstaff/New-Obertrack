package handlers

import (
	"errors"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/obertrack/backend/internal/middleware"
	"github.com/obertrack/backend/internal/repository"
	"github.com/obertrack/backend/internal/service"
)

// defaultGoogleReturnTo es la pantalla a la que vuelve el navegador tras el
// consentimiento si el frontend no pidió otra. Debe ser una ruta REAL del SPA
// (react-router); si no coincide, el usuario cae en una pantalla en blanco con
// "No routes matched" pese a que la vinculación sí ocurrió.
const defaultGoogleReturnTo = "/profile"

// GoogleCalendarHandler expone la vinculación PERSONAL con Google Calendar.
// Todas las rutas autenticadas operan sobre el usuario de la sesión y nunca
// reciben un user_id por parámetro: no hay forma de tocar el vínculo de otro.
type GoogleCalendarHandler struct {
	service     service.GoogleCalendarService
	frontendURL string
}

func NewGoogleCalendarHandler(s service.GoogleCalendarService, frontendURL string) *GoogleCalendarHandler {
	return &GoogleCalendarHandler{service: s, frontendURL: strings.TrimRight(frontendURL, "/")}
}

// safeReturnTo acota el destino post-consentimiento a una ruta interna. Sin
// esto, el returnTo (que viaja por la URL de Google y vuelve en el state) sería
// un redirect abierto: bastaría un enlace preparado para que el usuario acabe
// en un dominio ajeno justo después de autenticarse. Se exige que empiece por
// "/" y no por "//" (que el navegador interpreta como protocolo-relativo).
func safeReturnTo(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || !strings.HasPrefix(raw, "/") || strings.HasPrefix(raw, "//") {
		return defaultGoogleReturnTo
	}
	if parsed, err := url.Parse(raw); err != nil || parsed.Host != "" || parsed.Scheme != "" {
		return defaultGoogleReturnTo
	}
	return raw
}

type connectGoogleRequest struct {
	// ReturnTo es la ruta del frontend a la que volver al terminar (opcional).
	ReturnTo string `json:"return_to"`
}

// Connect devuelve la URL de consentimiento para que el frontend navegue a
// ella. Se devuelve en JSON en lugar de responder con un 302 porque la petición
// sale por XHR: un redirect ahí lo seguiría axios, no el navegador.
func (h *GoogleCalendarHandler) Connect(c *gin.Context) {
	if !h.service.Enabled() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "La integración con Google Calendar no está disponible"})
		return
	}

	var req connectGoogleRequest
	// El cuerpo es opcional: sin él se usa el destino por defecto.
	_ = c.ShouldBindJSON(&req)

	authURL, err := h.service.AuthURL(middleware.GetUserID(c), safeReturnTo(req.ReturnTo))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo iniciar la conexión con Google"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"auth_url": authURL})
}

// Callback recibe la vuelta de Google. Es una ruta PÚBLICA: llega como
// navegación del navegador desde accounts.google.com, y la identidad del
// usuario sale del state firmado, no de la sesión. Así el flujo funciona aunque
// el access token haya expirado durante el consentimiento (y servirá igual para
// el deep link de la app móvil).
//
// Siempre responde con un 302 al frontend, nunca con JSON: quien está mirando
// es una persona en una pestaña del navegador.
func (h *GoogleCalendarHandler) Callback(c *gin.Context) {
	if !h.service.Enabled() {
		h.redirect(c, defaultGoogleReturnTo, "error", "disabled")
		return
	}

	// Google devuelve 'error' cuando el usuario cancela en la pantalla de
	// consentimiento. Es un caso normal, no un fallo.
	if reason := c.Query("error"); reason != "" {
		h.redirect(c, defaultGoogleReturnTo, "error", reason)
		return
	}

	code, state := c.Query("code"), c.Query("state")
	if code == "" || state == "" {
		h.redirect(c, defaultGoogleReturnTo, "error", "invalid_request")
		return
	}

	account, returnTo, err := h.service.HandleCallback(code, state)
	if err != nil {
		log.Printf("Google Calendar: falló el callback: %v", err)
		reason := "exchange_failed"
		if errors.Is(err, service.ErrInvalidState) {
			reason = "expired"
		}
		h.redirect(c, safeReturnTo(returnTo), "error", reason)
		return
	}

	log.Printf("Google Calendar: vínculo completado para el usuario %d", account.UserID)
	h.redirect(c, safeReturnTo(returnTo), "ok", "")
}

// redirect construye el destino final. El resultado de la vinculación viaja
// como query param para que el frontend muestre el aviso correspondiente.
func (h *GoogleCalendarHandler) redirect(c *gin.Context, path, result, reason string) {
	target := h.frontendURL + path
	sep := "?"
	if strings.Contains(path, "?") {
		sep = "&"
	}
	target += sep + "google=" + url.QueryEscape(result)
	if reason != "" {
		target += "&reason=" + url.QueryEscape(reason)
	}
	c.Redirect(http.StatusFound, target)
}

// Status describe el vínculo del usuario autenticado. Responde 200 con
// connected:false cuando no hay ninguno: no tener cuenta vinculada es el estado
// normal de la mayoría de usuarios, no un 404.
func (h *GoogleCalendarHandler) Status(c *gin.Context) {
	if !h.service.Enabled() {
		c.JSON(http.StatusOK, gin.H{"enabled": false, "connected": false})
		return
	}

	account, err := h.service.Status(middleware.GetUserID(c))
	if errors.Is(err, repository.ErrGoogleAccountNotFound) {
		c.JSON(http.StatusOK, gin.H{"enabled": true, "connected": false})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo consultar la conexión"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"enabled": true, "connected": true, "account": account})
}

func (h *GoogleCalendarHandler) Disconnect(c *gin.Context) {
	err := h.service.Disconnect(middleware.GetUserID(c))
	// Desconectar algo que ya no está vinculado es un éxito, no un error: deja
	// al usuario en el estado que pidió.
	if err != nil && !errors.Is(err, repository.ErrGoogleAccountNotFound) {
		h.respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"connected": false})
}

// respondError traduce los errores del servicio a códigos HTTP. El caso que
// importa es ErrNeedsReauth: es 409 y no 500 porque no es un fallo del sistema
// sino un estado que el usuario resuelve reconectando, y el frontend lo
// distingue para mostrar el botón en vez de un error genérico.
func (h *GoogleCalendarHandler) respondError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrGoogleDisabled):
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "La integración con Google Calendar no está disponible"})
	case errors.Is(err, repository.ErrGoogleAccountNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "No tienes una cuenta de Google vinculada"})
	case errors.Is(err, service.ErrNeedsReauth):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error(), "needs_reauth": true})
	default:
		log.Printf("Google Calendar: error inesperado: %v", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
	}
}
