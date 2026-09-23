package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/service"
	"github.com/obertrack/backend/internal/utils"
)

// Encuesta SIN SESIÓN: la superficie que abre quien recibió la invitación por
// correo y no tiene por qué iniciar sesión para contestarla.
//
// Existe SOLO para las cuentas empresa (models.UserTypeEmployer). Es una
// decisión de producto, no un descuido: a la empresa se le pide opinión como
// cliente, y obligarla a recordar una contraseña costaba respuestas. Cualquier
// otro rol —profesional, soporte, superadmin— sigue respondiendo dentro de la
// aplicación, con sesión. Por eso el rol se comprueba aquí en el servidor y no
// se da por bueno lo que diga el enlace.
//
// La credencial es el token del enlace, igual que en la inducción y en los
// testimonios. A diferencia de aquellos no se guarda en base de datos: va
// firmado con HMAC y lleva dentro a quién pertenece y hasta cuándo vale, así
// que no hace falta migración ni limpieza de tokens vencidos.

// surveyLinkTTL es cuánto vale el enlace del correo. Un mes da margen para
// responder sin dejar vivo para siempre un correo reenviado.
const surveyLinkTTL = 30 * 24 * time.Hour

var (
	errSurveyLinkInvalid = errors.New("El enlace no es válido.")
	errSurveyLinkExpired = errors.New("Este enlace ya venció. Pídele al equipo que te lo reenvíe.")
	errSurveyLinkClosed  = errors.New("Esta encuesta ya no está activa.")
	// errSurveyLinkNotCompany se devuelve cuando la firma es correcta pero el
	// destinatario no es una cuenta empresa. No debería pasar —los enlaces solo
	// se generan para empresas—, y si pasa es que alguien reutilizó uno:
	// responder sin sesión no está disponible para ese rol.
	errSurveyLinkNotCompany = errors.New("Para responder esta encuesta necesitas entrar a Obertrack con tu cuenta.")
)

// surveyHMACSecret es la llave con la que se firman los enlaces. Prefiere una
// dedicada (SURVEY_TOKEN_SECRET) y si no usa JWT_SECRET, que el arranque ya
// valida como fuerte, en vez de un valor por defecto que volvería falsificable
// cualquier enlace.
func surveyHMACSecret() string {
	if secret := os.Getenv("SURVEY_TOKEN_SECRET"); secret != "" {
		return secret
	}
	return os.Getenv("JWT_SECRET")
}

func surveyLinkMAC(surveyID, userID uint, exp int64) string {
	mac := hmac.New(sha256.New, []byte(surveyHMACSecret()))
	fmt.Fprintf(mac, "%d:%d:%d", surveyID, userID, exp)
	return hex.EncodeToString(mac.Sum(nil))
}

// newSurveyLinkToken arma el token del enlace: "encuesta.usuario.vence.firma".
// Se lee de un vistazo en un registro y no necesita codificación extra —solo
// dígitos, puntos e hexadecimal—, así que viaja igual en la ruta que en la
// query.
func newSurveyLinkToken(surveyID, userID uint, expiresAt time.Time) string {
	exp := expiresAt.Unix()
	return fmt.Sprintf("%d.%d.%d.%s", surveyID, userID, exp, surveyLinkMAC(surveyID, userID, exp))
}

// parseSurveyLinkToken comprueba la firma y el vencimiento, y devuelve a quién
// pertenece el enlace. Distingue "inválido" de "vencido" a propósito: son dos
// mensajes muy distintos para quien lo abre.
func parseSurveyLinkToken(raw string) (surveyID, userID uint, err error) {
	parts := strings.Split(raw, ".")
	if len(parts) != 4 {
		return 0, 0, errSurveyLinkInvalid
	}
	sid, err1 := strconv.ParseUint(parts[0], 10, 32)
	uid, err2 := strconv.ParseUint(parts[1], 10, 32)
	exp, err3 := strconv.ParseInt(parts[2], 10, 64)
	if err1 != nil || err2 != nil || err3 != nil {
		return 0, 0, errSurveyLinkInvalid
	}

	// La firma se comprueba SIEMPRE antes que la fecha: al revés, alargar el
	// vencimiento a mano respondería "vencido" en lugar de "inválido" y con eso
	// confirmaría que el resto del token era bueno.
	if !hmac.Equal([]byte(surveyLinkMAC(uint(sid), uint(uid), exp)), []byte(parts[3])) {
		return 0, 0, errSurveyLinkInvalid
	}
	if time.Now().After(time.Unix(exp, 0)) {
		return 0, 0, errSurveyLinkExpired
	}
	return uint(sid), uint(uid), nil
}

// resolveSurveyLink valida el token de punta a punta: firma, vencimiento,
// encuesta activa, destinatario de la lista y —lo importante— que sea una
// cuenta empresa.
func (h *SurveyHandler) resolveSurveyLink(raw string) (*models.Survey, *models.User, error) {
	surveyID, userID, err := parseSurveyLinkToken(raw)
	if err != nil {
		return nil, nil, err
	}

	survey, err := h.repo.GetSurveyByID(surveyID)
	if err != nil || survey == nil {
		return nil, nil, errSurveyLinkInvalid
	}
	if survey.Status != models.SurveyStatusActive {
		return nil, nil, errSurveyLinkClosed
	}
	if !surveyHasRecipient(survey.RecipientList, userID) {
		return nil, nil, errSurveyLinkInvalid
	}

	user, err := h.userRepo.GetByID(userID)
	if err != nil || user == nil {
		return nil, nil, errSurveyLinkInvalid
	}
	if user.UserType != models.UserTypeEmployer {
		return nil, nil, errSurveyLinkNotCompany
	}
	return survey, user, nil
}

// surveyLinkStatus traduce el motivo del rechazo al código HTTP que le toca.
func surveyLinkStatus(err error) int {
	switch {
	case errors.Is(err, errSurveyLinkExpired), errors.Is(err, errSurveyLinkClosed):
		return http.StatusGone
	case errors.Is(err, errSurveyLinkNotCompany):
		return http.StatusForbidden
	default:
		return http.StatusNotFound
	}
}

// surveyLinkQuestion es una pregunta tal como la ve el navegador de quien
// responde. Es un DTO propio y no el modelo: así la clave de respuesta y la
// ponderación de un cuestionario calificado no pueden escaparse por olvido.
type surveyLinkQuestion struct {
	ID         uint   `json:"id"`
	Text       string `json:"text"`
	Type       string `json:"type"`
	Options    string `json:"options"`
	IsRequired bool   `json:"is_required"`
	OrderIndex int    `json:"order_index"`
}

// surveyLinkView es todo lo que la página pública necesita para dibujarse.
type surveyLinkView struct {
	SurveyID      uint                 `json:"survey_id"`
	Title         string               `json:"title"`
	Description   string               `json:"description"`
	RecipientName string               `json:"recipient_name"`
	Questions     []surveyLinkQuestion `json:"questions"`
	// AlreadyAnswered avisa de que ya hay algo respondido —normalmente el clic
	// que dio en el propio correo—. No bloquea: puede completar el resto, y lo
	// que mande reemplaza a lo anterior.
	AlreadyAnswered bool `json:"already_answered"`
}

// PublicLanding devuelve la encuesta para responderla sin sesión.
func (h *SurveyHandler) PublicLanding(c *gin.Context) {
	survey, user, err := h.resolveSurveyLink(c.Param("token"))
	if err != nil {
		c.JSON(surveyLinkStatus(err), gin.H{"error": err.Error()})
		return
	}

	view := surveyLinkView{
		SurveyID:      survey.ID,
		Title:         survey.Title,
		Description:   survey.Description,
		RecipientName: user.Name,
		Questions:     make([]surveyLinkQuestion, 0, len(survey.Questions)),
	}
	for _, q := range survey.Questions {
		view.Questions = append(view.Questions, surveyLinkQuestion{
			ID:         q.ID,
			Text:       q.Text,
			Type:       string(q.Type),
			Options:    q.Options,
			IsRequired: q.IsRequired,
			OrderIndex: q.OrderIndex,
		})
	}
	if existing, err := h.repo.GetResponseByUser(survey.ID, user.ID); err == nil && existing != nil {
		view.AlreadyAnswered = len(existing.Answers) > 0
	}

	c.JSON(http.StatusOK, view)
}

type surveyLinkSubmitPayload struct {
	Answers []models.SurveyAnswer `json:"answers"`
}

// PublicSubmit guarda las respuestas enviadas desde la página pública.
func (h *SurveyHandler) PublicSubmit(c *gin.Context) {
	survey, user, err := h.resolveSurveyLink(c.Param("token"))
	if err != nil {
		c.JSON(surveyLinkStatus(err), gin.H{"error": err.Error()})
		return
	}

	var payload surveyLinkSubmitPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No se entendieron las respuestas."})
		return
	}
	if len(payload.Answers) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No respondiste ninguna pregunta."})
		return
	}

	// Cada respuesta tiene que apuntar a una pregunta DE ESTA encuesta. Sin
	// esto, el token de una encuesta serviría para escribir en otra.
	answers := make([]models.SurveyAnswer, 0, len(payload.Answers))
	for _, a := range payload.Answers {
		if !surveyHasQuestion(survey, a.QuestionID) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Alguna respuesta no corresponde a esta encuesta."})
			return
		}
		answers = append(answers, models.SurveyAnswer{
			QuestionID:  a.QuestionID,
			TextValue:   a.TextValue,
			NumberValue: a.NumberValue,
		})
	}

	if err := h.saveOneResponsePerUser(survey.ID, user.ID, answers); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudieron guardar tus respuestas. Vuelve a intentarlo."})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Respuestas registradas"})
}

func surveyHasQuestion(survey *models.Survey, questionID uint) bool {
	for _, q := range survey.Questions {
		if q.ID == questionID {
			return true
		}
	}
	return false
}

// QuickResponse registra la valoración que se pulsa DENTRO del correo: un clic
// en "4" y esa pregunta queda contestada. Responde con una página de gracias
// que ofrece completar el resto, también sin sesión.
func (h *SurveyHandler) QuickResponse(c *gin.Context) {
	token := c.Query("t")
	survey, user, err := h.resolveSurveyLink(token)
	if err != nil {
		surveyLinkPage(c, surveyLinkStatus(err), "No pudimos registrar tu respuesta", err.Error())
		return
	}

	questionID, err1 := strconv.ParseUint(c.Query("q_id"), 10, 32)
	score, err2 := strconv.Atoi(c.Query("score"))
	if err1 != nil || err2 != nil || !surveyHasQuestion(survey, uint(questionID)) {
		surveyLinkPage(c, http.StatusBadRequest, "No pudimos registrar tu respuesta",
			"El enlace que pulsaste no corresponde a una pregunta de esta encuesta.")
		return
	}
	if score < 1 || score > 10 {
		surveyLinkPage(c, http.StatusBadRequest, "No pudimos registrar tu respuesta",
			"La valoración está fuera de rango.")
		return
	}

	// Un clic contesta UNA pregunta y lo demás que hubiera respondido se
	// conserva. Por eso aquí se guarda solo esa respuesta en vez de reemplazar
	// la participación entera como hace el envío del formulario.
	answer := models.SurveyAnswer{QuestionID: uint(questionID), NumberValue: score}
	now := time.Now()
	if existing, err := h.repo.GetResponseByUser(survey.ID, user.ID); err == nil && existing != nil {
		_ = h.repo.SaveAnswer(existing.ID, answer, now)
	} else if err := h.repo.CreateResponse(&models.SurveyResponse{
		SurveyID:    survey.ID,
		UserID:      user.ID,
		CompletedAt: &now,
		Answers:     []models.SurveyAnswer{answer},
	}); err != nil {
		// Pulsó dos puntuaciones casi a la vez (pasa: el correo tiene cinco
		// botones juntos). La participación ya existe, así que esta se guarda
		// encima en lugar de perderse.
		if existing, e := h.repo.GetResponseByUser(survey.ID, user.ID); e == nil && existing != nil {
			_ = h.repo.SaveAnswer(existing.ID, answer, now)
		}
	}

	body := fmt.Sprintf(`
		<div style="text-align:center;padding:16px 4px;">
			<div style="font-size:44px;line-height:1;margin-bottom:12px;">&#10003;</div>
			<h2 style="margin:0 0 8px 0;color:#1e293b;font-family:sans-serif;">¡Gracias por tu respuesta!</h2>
			<p style="color:#64748b;font-size:16px;font-family:sans-serif;margin:0;">Registramos tu valoración de <strong>%d</strong>.</p>
			<p style="color:#64748b;font-size:15px;font-family:sans-serif;margin:16px 0 0 0;">Si quieres, puedes contestar el resto de la encuesta. No hace falta iniciar sesión.</p>
			<div style="margin-top:24px;">
				<a href="%s" style="display:inline-block;padding:12px 24px;background-color:#cc33cc;color:#ffffff;text-decoration:none;border-radius:8px;font-weight:600;font-family:sans-serif;">Completar la encuesta</a>
			</div>
		</div>
	`, score, surveyPublicLink(token))

	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, utils.WrapInPremiumTemplate("Respuesta registrada", body))
}

// surveyLinkPage responde una página de aviso. Quien pulsa desde el correo está
// en un navegador, no en una aplicación: un JSON de error ahí no se entiende.
func surveyLinkPage(c *gin.Context, status int, title, message string) {
	body := fmt.Sprintf(`
		<div style="text-align:center;padding:16px 4px;">
			<h2 style="margin:0 0 8px 0;color:#1e293b;font-family:sans-serif;">%s</h2>
			<p style="color:#64748b;font-size:16px;font-family:sans-serif;margin:0;">%s</p>
		</div>
	`, title, message)
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(status, utils.WrapInPremiumTemplate(title, body))
}

// surveyFrontendURL es la base pública del sitio.
func surveyFrontendURL() string {
	if url := strings.TrimRight(os.Getenv("FRONTEND_URL"), "/"); url != "" {
		return url
	}
	if url := strings.TrimRight(os.Getenv("SERVICE_URL_FRONTEND"), "/"); url != "" {
		return url
	}
	return "https://obertrack.com"
}

// surveyPublicLink es la página donde se responde sin sesión.
func surveyPublicLink(token string) string {
	return fmt.Sprintf("%s/encuesta/%s", surveyFrontendURL(), token)
}

// surveyQuickOptions arma los botones que van DENTRO del correo a partir de la
// primera pregunta de valoración de la encuesta: devuelve su enunciado y las
// opciones. Si la encuesta no tiene ninguna pregunta puntuable no hay nada que
// pulsar y se devuelve vacío: el correo queda solo con el enlace.
func surveyQuickOptions(survey *models.Survey, backendURL, token string) (string, []service.SurveyQuickOption) {
	question := firstScorableQuestion(survey)
	if question == nil {
		return "", nil
	}

	low, high := 1, 5
	if question.Type == models.QuestionTypeLinearScale {
		low, high = linearScaleRange(question.Options)
		// El endpoint solo acepta de 1 a 10. Una escala fuera de ese rango
		// (0-10, 1-100) no se puede contestar de un clic sin mentir sobre lo
		// que se guarda, así que esa encuesta se responde en la página.
		if low < 1 || high > 10 || high <= low {
			return "", nil
		}
	}

	options := make([]service.SurveyQuickOption, 0, high-low+1)
	for score := low; score <= high; score++ {
		options = append(options, service.SurveyQuickOption{
			Label: strconv.Itoa(score),
			Href: fmt.Sprintf("%s/api/surveys/%d/quick-response?t=%s&q_id=%d&score=%d",
				strings.TrimRight(backendURL, "/"), survey.ID, token, question.ID, score),
		})
	}
	return question.Text, options
}

// firstScorableQuestion es la primera pregunta, por orden, que se contesta con
// un número.
func firstScorableQuestion(survey *models.Survey) *models.SurveyQuestion {
	var found *models.SurveyQuestion
	for i := range survey.Questions {
		q := &survey.Questions[i]
		if q.Type != models.QuestionTypeRating && q.Type != models.QuestionTypeLinearScale {
			continue
		}
		if found == nil || q.OrderIndex < found.OrderIndex {
			found = q
		}
	}
	return found
}

// linearScaleRange lee el mínimo y el máximo de una escala lineal. El editor
// los guarda como JSON en Options; si no se entiende, se asume la escala de 1 a
// 5 que trae por defecto.
func linearScaleRange(options string) (int, int) {
	cfg := struct {
		Min *int `json:"min"`
		Max *int `json:"max"`
	}{}
	if err := json.Unmarshal([]byte(options), &cfg); err != nil {
		return 1, 5
	}
	low, high := 1, 5
	if cfg.Min != nil {
		low = *cfg.Min
	}
	if cfg.Max != nil {
		high = *cfg.Max
	}
	return low, high
}
