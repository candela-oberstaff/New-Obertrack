package service

import (
	"encoding/json"
	"errors"
	"math"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
	"github.com/obertrack/backend/internal/utils"
)

// TutorialInput es lo que llega del formulario de una novedad. Es una struct y
// no una lista de parámetros porque son doce campos y tres tipos de contenido:
// posicionalmente ya nadie acertaba el orden.
type TutorialInput struct {
	Title       string
	Description string
	ContentType string
	// Boton de accion opcional.
	CTALabel string
	CTAURL   string
	// Programacion: publicar y retirar solos.
	PublishAt *time.Time
	ExpiresAt *time.Time
	// RequireAck exige confirmar la lectura.
	RequireAck bool
	// Contenido: manda el que corresponda a ContentType.
	VideoURL string
	ImageURL string
	Body     string

	IconName     string
	Category     string
	Audience     string
	DurationMin  int
	OrderIndex   int
	AnnounceDays int
	// AnnounceMaxShows limita cuantas veces aparece el aviso. 0 = sin limite.
	AnnounceMaxShows int
	IsActive         bool
	// Target acota el público por encima del tipo de cuenta.
	Target models.TutorialTarget
}

type TutorialService interface {
	GetAll(onlyActive bool, audiences []string) ([]models.Tutorial, error)
	GetByID(id uint) (*models.Tutorial, error)
	Create(userID uint, in TutorialInput) (*models.Tutorial, error)
	Update(actorID, id uint, updates map[string]interface{}) (*models.Tutorial, error)
	Delete(id uint) error
	Reorder(ids []uint) error
	RecordView(tutorialID, userID uint, source string, acknowledged bool) error
	// RecordClick anota que alguien pulso el boton de accion de la novedad.
	RecordClick(tutorialID, userID uint) error
	// RecordShow anota que el aviso se le mostro una vez mas a esa persona.
	RecordShow(tutorialID, userID uint) error
	// RemindPending vuelve a avisar SOLO a quienes no la han visto y reabre la
	// ventana del aviso. Devuelve a cuanta gente se le recordo.
	RemindPending(actorID, tutorialID uint) (int, error)
	// RunSchedule publica lo programado y retira lo caducado. Lo llama el reloj.
	RunSchedule() (published int, expired int, err error)
	GetUserViewedIDs(userID uint) ([]uint, error)
	// GetPendingAnnouncements son las novedades que este usuario todavía no ha
	// visto y que emergen al iniciar sesión.
	GetPendingAnnouncements(userID uint, audiences []string) ([]models.Tutorial, error)
	// GetMetrics es el desempeño de una novedad: a cuántos llegó, cuántos la
	// vieron y por dónde.
	GetMetrics(tutorialID uint) (*models.TutorialMetrics, error)
	// PreviewAudience responde a cuánta gente llegaría una novedad con esa
	// audiencia y ese público objetivo, sin publicar nada.
	PreviewAudience(audience string, target models.TutorialTarget) (*models.TutorialAudiencePreview, error)
	// GetAudienceOptions son las empresas, países y grupos entre los que se
	// puede elegir al acotar el público.
	GetAudienceOptions() (*models.TutorialAudienceOptions, error)
}

type tutorialService struct {
	repo     repository.TutorialRepository
	userRepo repository.UserRepository
	notifSvc NotificationService
}

func NewTutorialService(repo repository.TutorialRepository, userRepo repository.UserRepository, notifSvc NotificationService) TutorialService {
	return &tutorialService{repo: repo, userRepo: userRepo, notifSvc: notifSvc}
}

// Ventana del aviso emergente, en días, cuando la novedad no trae una propia.
// Dos días alcanzan para cubrir a quien no entró ayer sin volverse molesto.
const defaultAnnounceDays = 2

// Tope de la ventana. Más de tres meses insistiendo con lo mismo ya no es un
// anuncio, y el aviso emergente es la interrupción más cara que tiene la app.
const maxAnnounceDays = 90

// Cuántas novedades se muestran de una sentada al iniciar sesión. Si se
// publicaron más, el resto queda en la página (y en la campanita).
const maxPendingAnnouncements = 3

// encodeTarget serializa el publico objetivo. Un publico vacio se guarda como
// cadena vacia y no como "{}": asi "sin acotar" se lee igual en las novedades
// anteriores a esta funcion y en las nuevas.
func encodeTarget(target models.TutorialTarget) string {
	if target.IsEmpty() {
		return ""
	}
	encoded, err := json.Marshal(target)
	if err != nil {
		return ""
	}
	return string(encoded)
}

// normalizeAnnounceDays acota la ventana del aviso. El 0 es intencional y se
// respeta: significa "avisa por la campanita, pero no interrumpas a nadie".
// Tope de apariciones del aviso. Más de diez veces delante de la misma persona
// deja de ser un aviso y pasa a ser un castigo.
const maxAnnounceShows = 10

// normalizeAnnounceShows acota el número de apariciones. El 0 se respeta:
// significa "sin límite, manda solo el plazo en días".
func normalizeAnnounceShows(shows int) int {
	if shows < 0 {
		return 0
	}
	if shows > maxAnnounceShows {
		return maxAnnounceShows
	}
	return shows
}

func normalizeAnnounceDays(days int) int {
	if days < 0 {
		return defaultAnnounceDays
	}
	if days > maxAnnounceDays {
		return maxAnnounceDays
	}
	return days
}

// announceRecipientTypes traduce la audiencia de la novedad a los tipos de
// usuario que reciben el aviso. El superadmin va siempre: es quien publica y
// quien ve la página completa, sin filtro de audiencia (quien la publicó queda
// excluido aparte). Analistas IT y customer_success no entran: para ellos el
// módulo de Novedades ni siquiera existe en el menú.
func announceRecipientTypes(audience string) []models.UserType {
	types := []models.UserType{models.UserTypeSuperadmin}
	switch audience {
	case models.TutorialAudienceEmployer:
		types = append(types, models.UserTypeEmployer)
	case models.TutorialAudienceProfessional, models.TutorialAudienceManager:
		// Los managers son profesionales; el filtro de "con equipo a cargo" lo
		// aplica resolveAudience, que es quien tiene las fichas delante.
		types = append(types, models.UserTypeProfessional)
	default:
		types = append(types, models.UserTypeEmployer, models.UserTypeProfessional)
	}
	return types
}

// hasTeam responde si esa persona tiene gente a cargo. El supervisor cuenta
// como manager (todo supervisor lo es).
func hasTeam(user *models.User) bool {
	return user != nil && (user.IsManager || user.IsSupervisor)
}

// announcementSummary es el cuerpo del aviso: la descripción de la novedad,
// recortada para que la campanita no se convierta en un muro de texto.
func announcementSummary(description string) string {
	summary := strings.TrimSpace(description)
	if summary == "" {
		return "Entra a Novedades para verla."
	}
	if runes := []rune(summary); len(runes) > 180 {
		return strings.TrimSpace(string(runes[:180])) + "…"
	}
	return summary
}

// resolveAudience devuelve las personas a las que alcanza una novedad: los
// usuarios activos de su tipo de cuenta que además pasan el público objetivo.
//
// Se resuelve en Go y no en SQL a propósito: la regla vive en
// TutorialTarget.Matches y la usan el reparto, el aviso emergente y las
// métricas. Con tres consultas distintas, tarde o temprano dirían cosas
// distintas sobre la misma novedad.
func (s *tutorialService) resolveAudience(audience string, target models.TutorialTarget) ([]models.User, error) {
	if s.userRepo == nil {
		return nil, errors.New("Repositorio de usuarios no disponible")
	}
	candidates, err := s.userRepo.ListActiveByTypes(announceRecipientTypes(audience))
	if err != nil {
		return nil, err
	}

	// Audiencia "manager": dentro de los profesionales, solo quienes tienen
	// equipo a cargo. El superadmin sigue recibiendo el aviso, como en el
	// resto de audiencias.
	if audience == models.TutorialAudienceManager {
		managers := make([]models.User, 0, len(candidates))
		for i := range candidates {
			user := candidates[i]
			if user.UserType == models.UserTypeSuperadmin || hasTeam(&user) {
				managers = append(managers, user)
			}
		}
		candidates = managers
	}

	if target.IsEmpty() {
		return candidates, nil
	}

	// La pertenencia a grupos es una consulta aparte, y solo hace falta cuando
	// el público acota por grupos.
	inGroup := map[uint]bool{}
	if len(target.GroupIDs) > 0 {
		inGroup, err = s.repo.UsersInGroups(target.GroupIDs)
		if err != nil {
			return nil, err
		}
	}

	matched := make([]models.User, 0, len(candidates))
	for i := range candidates {
		user := candidates[i]
		if target.Matches(&user, inGroup[user.ID]) {
			matched = append(matched, user)
		}
	}
	return matched, nil
}

// announce reparte la notificación de una novedad recién publicada entre su
// audiencia. Es best-effort y en segundo plano: publicar no puede quedarse
// esperando a que se escriba una fila por cada persona de la plataforma, y que
// falle un aviso suelto no debe tumbar la publicación.
func (s *tutorialService) announce(tutorial *models.Tutorial, actorID uint) {
	if tutorial == nil || s.notifSvc == nil || s.userRepo == nil {
		return
	}
	recipients, err := s.resolveAudience(tutorial.Audience, tutorial.Target)
	if err != nil {
		return
	}
	title := "Novedades: " + tutorial.Title
	message := announcementSummary(tutorial.Description)
	data := map[string]interface{}{"link": "/novedades", "tutorial_id": tutorial.ID}
	for _, user := range recipients {
		if user.ID == actorID {
			continue
		}
		_ = s.notifSvc.CreateNotification(user.ID, "novedad", title, message, data)
	}
}

var (
	driveFileIDRegex = regexp.MustCompile(`/file/d/([a-zA-Z0-9_-]+)`)
	youtubeIDRegex   = regexp.MustCompile(`(?:youtube\.com/(?:watch\?(?:[^&]*&)*v=|embed/|v/|shorts/)|youtu\.be/)([a-zA-Z0-9_-]{11})`)
)

func validateVideoURL(url string) error {
	url = strings.TrimSpace(url)
	if url == "" {
		return errors.New("El link del video es obligatorio")
	}
	if strings.Contains(url, "drive.google.com") {
		if !driveFileIDRegex.MatchString(url) {
			return errors.New("El link de Google Drive debe tener el formato /file/d/{ID}/...")
		}
		return nil
	}
	if strings.Contains(url, "youtube.com") || strings.Contains(url, "youtu.be") {
		if !youtubeIDRegex.MatchString(url) {
			return errors.New("El link de YouTube no tiene un ID de video válido")
		}
		return nil
	}
	return errors.New("Solo se aceptan links de Google Drive o YouTube")
}

func normalizeTutorialContentType(contentType string) (string, error) {
	contentType = strings.TrimSpace(contentType)
	if contentType == "" {
		return models.TutorialContentVideo, nil
	}
	if !models.IsValidTutorialContentType(contentType) {
		return "", errors.New("Tipo de contenido inválido: usa 'video', 'imagen' o 'texto'")
	}
	return contentType, nil
}

var htmlTagRegex = regexp.MustCompile(`<[^>]*>`)

// hasVisibleText responde si un HTML tiene algo que leer, más allá de las
// etiquetas y los espacios que deja atrás cualquier editor enriquecido.
func hasVisibleText(html string) bool {
	stripped := htmlTagRegex.ReplaceAllString(html, "")
	stripped = strings.ReplaceAll(stripped, "&nbsp;", " ")
	return strings.TrimSpace(stripped) != ""
}

// validateImageURL acepta lo que sube el propio sistema (rutas /uploads/...) y
// enlaces http(s). Cualquier otro esquema queda fuera: 'javascript:' y 'data:'
// en un <img> del anuncio serían un agujero abierto a todo el equipo.
func validateImageURL(url string) error {
	url = strings.TrimSpace(url)
	if url == "" {
		return errors.New("La imagen es obligatoria")
	}
	if strings.HasPrefix(url, "/") || strings.HasPrefix(url, "https://") || strings.HasPrefix(url, "http://") {
		return nil
	}
	return errors.New("La imagen debe ser un archivo subido o un enlace http(s)")
}

// validateContent comprueba que la novedad traiga el contenido que su tipo
// exige. Se aplica igual al crear y al editar, sobre el resultado final.
func validateContent(contentType, videoURL, imageURL, body string) error {
	switch contentType {
	case models.TutorialContentImage:
		return validateImageURL(imageURL)
	case models.TutorialContentText:
		if !hasVisibleText(body) {
			return errors.New("El contenido de la novedad es obligatorio")
		}
		return nil
	default:
		return validateVideoURL(videoURL)
	}
}

// validateCTA comprueba el boton de accion. Las dos mitades van juntas: un
// texto sin destino es un boton muerto y un destino sin texto no se ve.
// El destino se limita a rutas internas y enlaces http(s): un 'javascript:' en
// un boton que se le muestra a toda la empresa no es una opcion.
func validateCTA(label, url string) (string, string, error) {
	label = strings.TrimSpace(label)
	url = strings.TrimSpace(url)
	if label == "" && url == "" {
		return "", "", nil
	}
	if label == "" {
		return "", "", errors.New("El botón necesita un texto")
	}
	if url == "" {
		return "", "", errors.New("El botón necesita un destino")
	}
	if !strings.HasPrefix(url, "/") && !strings.HasPrefix(url, "https://") && !strings.HasPrefix(url, "http://") {
		return "", "", errors.New("El destino debe ser una ruta interna (/tareas) o un enlace http(s)")
	}
	return label, url, nil
}

// validateSchedule revisa la programacion. Publicar en el pasado es casi
// siempre un dedazo con el calendario, y caducar antes de publicar dejaria una
// novedad que nace muerta.
func validateSchedule(publishAt, expiresAt *time.Time) error {
	if expiresAt != nil {
		start := time.Now()
		if publishAt != nil {
			start = *publishAt
		}
		if !expiresAt.After(start) {
			return errors.New("La fecha de retiro debe ser posterior a la de publicación")
		}
	}
	return nil
}

func normalizeAudience(audience string) (string, error) {
	audience = strings.TrimSpace(audience)
	if audience == "" {
		return models.TutorialAudienceAll, nil
	}
	if !models.IsValidTutorialAudience(audience) {
		return "", errors.New("Audiencia inválida: usa 'all', 'empleador' o 'profesional'")
	}
	return audience, nil
}

func (s *tutorialService) GetAll(onlyActive bool, audiences []string) ([]models.Tutorial, error) {
	return s.repo.FindAll(onlyActive, audiences)
}

func (s *tutorialService) GetByID(id uint) (*models.Tutorial, error) {
	return s.repo.GetByID(id)
}

func (s *tutorialService) Create(userID uint, in TutorialInput) (*models.Tutorial, error) {
	if strings.TrimSpace(in.Title) == "" {
		return nil, errors.New("El título es obligatorio")
	}
	contentType, err := normalizeTutorialContentType(in.ContentType)
	if err != nil {
		return nil, err
	}
	if err := validateContent(contentType, in.VideoURL, in.ImageURL, in.Body); err != nil {
		return nil, err
	}
	iconName := in.IconName
	if iconName == "" {
		iconName = "PlayCircle"
	}
	category := strings.TrimSpace(in.Category)
	if category == "" {
		category = "General"
	}
	audience, err := normalizeAudience(in.Audience)
	if err != nil {
		return nil, err
	}
	ctaLabel, ctaURL, err := validateCTA(in.CTALabel, in.CTAURL)
	if err != nil {
		return nil, err
	}
	if err := validateSchedule(in.PublishAt, in.ExpiresAt); err != nil {
		return nil, err
	}

	tutorial := &models.Tutorial{
		Title:            utils.SanitizeHTML(in.Title),
		Description:      utils.SanitizeHTML(in.Description),
		ContentType:      contentType,
		GoogleDriveURL:   strings.TrimSpace(in.VideoURL),
		ImageURL:         strings.TrimSpace(in.ImageURL),
		Body:             utils.SanitizeHTML(in.Body),
		IconName:         iconName,
		Category:         utils.SanitizeHTML(category),
		Audience:         audience,
		DurationMin:      in.DurationMin,
		OrderIndex:       in.OrderIndex,
		AnnounceDays:     normalizeAnnounceDays(in.AnnounceDays),
		AnnounceMaxShows: normalizeAnnounceShows(in.AnnounceMaxShows),
		TargetSpec:       encodeTarget(in.Target),
		Target:           in.Target,
		CTALabel:         utils.SanitizeHTML(ctaLabel),
		CTAURL:           ctaURL,
		PublishAt:        in.PublishAt,
		ExpiresAt:        in.ExpiresAt,
		RequireAck:       in.RequireAck,
		IsActive:         in.IsActive,
		CreatedBy:        userID,
	}
	// Publicar es anunciar: la novedad nace con su marca de anuncio y desde ese
	// momento emerge al iniciar sesión. Un borrador (is_active=false) no se
	// anuncia; lo hará cuando se active.
	//
	// Con fecha programada se queda esperando: aunque se marque como visible,
	// el reloj es quien la publica y la anuncia a su hora.
	scheduled := in.PublishAt != nil && in.PublishAt.After(time.Now())
	if scheduled {
		tutorial.IsActive = false
	} else if in.IsActive {
		now := time.Now()
		tutorial.AnnouncedAt = &now
	}

	if err := s.repo.Create(tutorial); err != nil {
		return nil, err
	}

	if tutorial.AnnouncedAt != nil {
		go s.announce(tutorial, userID)
	}

	return s.repo.GetByID(tutorial.ID)
}

func (s *tutorialService) Update(actorID, id uint, updates map[string]interface{}) (*models.Tutorial, error) {
	tutorial, err := s.repo.GetByID(id)
	if err != nil {
		return nil, errors.New("Tutorial no encontrado")
	}

	if title, ok := updates["title"].(string); ok {
		if strings.TrimSpace(title) == "" {
			return nil, errors.New("El título es obligatorio")
		}
		updates["title"] = utils.SanitizeHTML(title)
	}
	if description, ok := updates["description"].(string); ok {
		updates["description"] = utils.SanitizeHTML(description)
	}
	// El contenido se valida sobre el RESULTADO de la edición, no sobre lo que
	// vino en el cuerpo: cambiar el tipo sin mandar el campo nuevo (o al revés)
	// dejaría una novedad publicada y vacía.
	contentType := tutorial.ContentType
	if raw, ok := updates["content_type"].(string); ok {
		normalized, err := normalizeTutorialContentType(raw)
		if err != nil {
			return nil, err
		}
		contentType = normalized
		updates["content_type"] = normalized
	}
	videoURL, imageURL, body := tutorial.GoogleDriveURL, tutorial.ImageURL, tutorial.Body
	if url, ok := updates["google_drive_url"].(string); ok {
		videoURL = strings.TrimSpace(url)
		updates["google_drive_url"] = videoURL
	}
	if url, ok := updates["image_url"].(string); ok {
		imageURL = strings.TrimSpace(url)
		updates["image_url"] = imageURL
	}
	if raw, ok := updates["body"].(string); ok {
		body = utils.SanitizeHTML(raw)
		updates["body"] = body
	}
	if err := validateContent(contentType, videoURL, imageURL, body); err != nil {
		return nil, err
	}
	// El publico llega ya desempaquetado desde el handler y se vuelve a
	// serializar aqui, que es donde vive la forma de guardarlo.
	if target, ok := updates["target"].(models.TutorialTarget); ok {
		delete(updates, "target")
		updates["target_spec"] = encodeTarget(target)
	}

	// El boton se valida entero aunque llegue a medias: cambiar solo el texto
	// no puede dejar un boton sin destino.
	if _, hasLabel := updates["cta_label"]; hasLabel {
		label, _ := updates["cta_label"].(string)
		url := tutorial.CTAURL
		if raw, ok := updates["cta_url"].(string); ok {
			url = raw
		}
		label, url, err := validateCTA(label, url)
		if err != nil {
			return nil, err
		}
		updates["cta_label"] = utils.SanitizeHTML(label)
		updates["cta_url"] = url
	} else if raw, ok := updates["cta_url"].(string); ok {
		label, url, err := validateCTA(tutorial.CTALabel, raw)
		if err != nil {
			return nil, err
		}
		updates["cta_label"] = utils.SanitizeHTML(label)
		updates["cta_url"] = url
	}
	if category, ok := updates["category"].(string); ok {
		trimmed := strings.TrimSpace(category)
		if trimmed == "" {
			trimmed = "General"
		}
		updates["category"] = utils.SanitizeHTML(trimmed)
	}
	if audience, ok := updates["audience"].(string); ok {
		normalized, err := normalizeAudience(audience)
		if err != nil {
			return nil, err
		}
		updates["audience"] = normalized
	}
	if days, ok := updates["announce_days"].(int); ok {
		updates["announce_days"] = normalizeAnnounceDays(days)
	}
	if shows, ok := updates["announce_max_shows"].(int); ok {
		updates["announce_max_shows"] = normalizeAnnounceShows(shows)
	}

	if len(updates) == 0 {
		return tutorial, nil
	}

	// Activar por primera vez una novedad que estaba en borrador equivale a
	// publicarla: ahí es cuando se anuncia. Editar una ya anunciada no vuelve a
	// avisar a nadie —corregir una falta de ortografía no es una novedad—.
	activating := false
	if active, ok := updates["is_active"].(bool); ok && active && tutorial.AnnouncedAt == nil {
		activating = true
		updates["announced_at"] = time.Now()
	}

	if err := s.repo.Update(tutorial, updates); err != nil {
		return nil, err
	}

	updated, err := s.repo.GetByID(id)
	if err != nil {
		return nil, err
	}

	if activating {
		go s.announce(updated, actorID)
	}

	return updated, nil
}

func (s *tutorialService) Delete(id uint) error {
	return s.repo.Delete(id)
}

func (s *tutorialService) Reorder(ids []uint) error {
	if len(ids) == 0 {
		return errors.New("La lista de IDs no puede estar vacía")
	}
	return s.repo.Reorder(ids)
}

func (s *tutorialService) RecordView(tutorialID, userID uint, source string, acknowledged bool) error {
	if tutorialID == 0 || userID == 0 {
		return errors.New("IDs inválidos")
	}
	if source != models.TutorialViewFromAnnouncement {
		source = models.TutorialViewFromSection
	}
	return s.repo.RecordView(tutorialID, userID, source, acknowledged)
}

func (s *tutorialService) RecordShow(tutorialID, userID uint) error {
	if tutorialID == 0 || userID == 0 {
		return errors.New("IDs inválidos")
	}
	return s.repo.RecordShow(tutorialID, userID)
}

func (s *tutorialService) RecordClick(tutorialID, userID uint) error {
	if tutorialID == 0 || userID == 0 {
		return errors.New("IDs inválidos")
	}
	// Pulsar el boton implica haberla visto: si alguien llego al CTA desde un
	// enlace directo, la vista tambien cuenta.
	_ = s.repo.RecordView(tutorialID, userID, models.TutorialViewFromSection, false)
	return s.repo.RecordClick(tutorialID, userID)
}

// Freno del recordatorio: dos pulsaciones seguidas mandarian dos avisos a media
// empresa. Seis horas es suficiente para que sea un acto deliberado.
const remindCooldown = 6 * time.Hour

func (s *tutorialService) RemindPending(actorID, tutorialID uint) (int, error) {
	tutorial, err := s.repo.GetByID(tutorialID)
	if err != nil {
		return 0, errors.New("Novedad no encontrada")
	}
	if !tutorial.IsActive || tutorial.AnnouncedAt == nil {
		return 0, errors.New("La novedad todavía no se ha publicado")
	}
	if tutorial.RemindedAt != nil && time.Since(*tutorial.RemindedAt) < remindCooldown {
		return 0, errors.New("Ya se envió un recordatorio hace poco. Espera unas horas.")
	}

	people, err := s.resolveAudience(tutorial.Audience, tutorial.Target)
	if err != nil {
		return 0, err
	}
	viewed, err := s.repo.ViewsFor(tutorialID)
	if err != nil {
		return 0, err
	}
	seen := make(map[uint]bool, len(viewed))
	for _, view := range viewed {
		seen[view.UserID] = true
	}

	now := time.Now()
	// Reabrir la ventana del aviso es la mitad del recordatorio: sin esto solo
	// llegaria una campanita mas, y a quien no entra a mirarla no le sirve.
	fields := map[string]interface{}{"reminded_at": now, "announced_at": now}
	if err := s.repo.SetFields(tutorialID, fields); err != nil {
		return 0, err
	}

	title := "Recordatorio: " + tutorial.Title
	message := announcementSummary(tutorial.Description)
	data := map[string]interface{}{"link": "/novedades", "tutorial_id": tutorial.ID}
	reminded := 0
	for _, user := range people {
		if user.ID == actorID || seen[user.ID] || user.UserType == models.UserTypeSuperadmin {
			continue
		}
		if s.notifSvc != nil {
			_ = s.notifSvc.CreateNotification(user.ID, "novedad", title, message, data)
		}
		reminded++
	}
	return reminded, nil
}

// RunSchedule publica lo que le toca y retira lo caducado. Es idempotente: si
// se ejecuta dos veces seguidas, la segunda no encuentra nada que hacer.
func (s *tutorialService) RunSchedule() (int, int, error) {
	now := time.Now()

	due, err := s.repo.ListDuePublications(now)
	if err != nil {
		return 0, 0, err
	}
	published := 0
	for i := range due {
		tutorial := due[i]
		if err := s.repo.SetFields(tutorial.ID, map[string]interface{}{
			"is_active":    true,
			"announced_at": now,
		}); err != nil {
			continue
		}
		tutorial.IsActive = true
		tutorial.AnnouncedAt = &now
		// Publicada por el reloj: no hay actor al que excluir del reparto.
		s.announce(&tutorial, 0)
		published++
	}

	expired, err := s.repo.ListDueExpirations(now)
	if err != nil {
		return published, 0, err
	}
	retired := 0
	for _, tutorial := range expired {
		if err := s.repo.SetFields(tutorial.ID, map[string]interface{}{"is_active": false}); err != nil {
			continue
		}
		retired++
	}

	return published, retired, nil
}

// Cuántas personas se listan en "últimos que la vieron". Es una muestra para
// reconocer caras, no un censo: para eso está el conteo.
const metricsViewerLimit = 25

// countableAudience deja fuera al superadmin. Es quien publica: sumarlo al
// denominador ensuciaria el porcentaje de lectura del equipo (y su propia
// vista podria empujarlo por encima del 100%).
func countableAudience(people []models.User) []models.User {
	countable := make([]models.User, 0, len(people))
	for _, user := range people {
		if user.UserType == models.UserTypeSuperadmin {
			continue
		}
		countable = append(countable, user)
	}
	return countable
}

// audienceStats agrupa alcance y vistas por tipo de cuenta.
func audienceStats(people []models.User, viewers map[uint]bool) []models.TutorialAudienceStat {
	order := []models.UserType{models.UserTypeEmployer, models.UserTypeProfessional}
	reach := map[models.UserType]int64{}
	views := map[models.UserType]int64{}
	for _, user := range people {
		reach[user.UserType]++
		if viewers[user.ID] {
			views[user.UserType]++
		}
	}
	stats := make([]models.TutorialAudienceStat, 0, len(order))
	for _, userType := range order {
		if reach[userType] == 0 {
			continue
		}
		stats = append(stats, models.TutorialAudienceStat{
			UserType: string(userType),
			Reach:    reach[userType],
			Views:    views[userType],
		})
	}
	return stats
}

func (s *tutorialService) GetMetrics(tutorialID uint) (*models.TutorialMetrics, error) {
	tutorial, err := s.repo.GetByID(tutorialID)
	if err != nil {
		return nil, errors.New("Novedad no encontrada")
	}

	metrics := &models.TutorialMetrics{
		TutorialID:  tutorialID,
		Audience:    tutorial.Audience,
		AnnouncedAt: tutorial.AnnouncedAt,
		RemindedAt:  tutorial.RemindedAt,
		RequireAck:  tutorial.RequireAck,
	}
	if tutorial.AnnouncedAt != nil && tutorial.AnnounceDays > 0 {
		metrics.AnnounceOpen = tutorial.AnnouncedAt.AddDate(0, 0, tutorial.AnnounceDays).After(time.Now())
	}

	// Alcance: el mismo publico al que se le repartio el anuncio.
	people, err := s.resolveAudience(tutorial.Audience, tutorial.Target)
	if err != nil {
		return nil, err
	}
	people = countableAudience(people)
	metrics.Reach = int64(len(people))

	byID := make(map[uint]models.User, len(people))
	for _, user := range people {
		byID[user.ID] = user
	}

	views, err := s.repo.ViewsFor(tutorialID)
	if err != nil {
		return nil, err
	}

	// Solo cuentan las vistas de gente que sigue dentro del publico: si alguien
	// se dio de baja o la novedad se reoriento, su vista ya no dice nada del
	// alcance actual.
	clickers, err := s.repo.ClickersFor(tutorialID)
	if err != nil {
		return nil, err
	}

	viewers := make(map[uint]bool, len(views))
	acknowledged := make(map[uint]bool, len(views))
	relevant := make([]models.TutorialView, 0, len(views))
	for _, view := range views {
		if _, ok := byID[view.UserID]; !ok {
			continue
		}
		viewers[view.UserID] = true
		relevant = append(relevant, view)
		metrics.Views++
		if view.Source == models.TutorialViewFromAnnouncement {
			metrics.FromAnnouncement++
		} else {
			metrics.FromSection++
		}
		if view.AcknowledgedAt != nil {
			acknowledged[view.UserID] = true
			metrics.Acknowledged++
		}
		if clickers[view.UserID] {
			metrics.Clicks++
		}
	}
	// El clic se mide sobre quienes la vieron: contra el alcance castigaria a
	// la novedad por gente que ni siquiera la abrio.
	if metrics.Views > 0 {
		metrics.ClickRate = math.Round(float64(metrics.Clicks)/float64(metrics.Views)*1000) / 10
	}

	metrics.Pending = metrics.Reach - metrics.Views
	if metrics.Pending < 0 {
		metrics.Pending = 0
	}
	if metrics.Reach > 0 {
		metrics.ViewRate = math.Round(float64(metrics.Views)/float64(metrics.Reach)*1000) / 10
	}
	metrics.ByAudience = audienceStats(people, viewers)

	sort.Slice(relevant, func(i, j int) bool { return relevant[i].ViewedAt.After(relevant[j].ViewedAt) })
	if len(relevant) > metricsViewerLimit {
		relevant = relevant[:metricsViewerLimit]
	}
	for _, view := range relevant {
		user := byID[view.UserID]
		metrics.RecentViewers = append(metrics.RecentViewers, models.TutorialViewer{
			UserID:       user.ID,
			Name:         user.Name,
			Email:        user.Email,
			UserType:     string(user.UserType),
			Source:       view.Source,
			ViewedAt:     view.ViewedAt,
			Acknowledged: acknowledged[user.ID],
			Clicked:      clickers[user.ID],
		})
	}

	return metrics, nil
}

func (s *tutorialService) GetUserViewedIDs(userID uint) ([]uint, error) {
	return s.repo.GetUserViewedIDs(userID)
}

func (s *tutorialService) GetPendingAnnouncements(userID uint, audiences []string) ([]models.Tutorial, error) {
	if userID == 0 {
		return []models.Tutorial{}, nil
	}
	pending, err := s.repo.FindPendingAnnouncements(userID, audiences, time.Now(), maxPendingAnnouncements)
	if err != nil {
		return nil, err
	}

	// El publico objetivo no se puede filtrar en la consulta (vive en JSON),
	// asi que se aplica aqui sobre un punado de filas, con la MISMA regla que
	// uso el reparto. Sin esto, alguien fuera del publico veria emerger una
	// novedad que nunca le fue anunciada.
	targeted := make([]models.Tutorial, 0, len(pending))
	var user *models.User
	for _, tutorial := range pending {
		if tutorial.Target.IsEmpty() {
			targeted = append(targeted, tutorial)
			continue
		}
		if user == nil {
			user, err = s.userRepo.GetByID(userID)
			if err != nil {
				return nil, err
			}
		}
		inGroup := false
		if len(tutorial.Target.GroupIDs) > 0 {
			members, err := s.repo.UsersInGroups(tutorial.Target.GroupIDs)
			if err != nil {
				return nil, err
			}
			inGroup = members[userID]
		}
		if tutorial.Target.Matches(user, inGroup) {
			targeted = append(targeted, tutorial)
		}
	}
	return targeted, nil
}

func (s *tutorialService) PreviewAudience(audience string, target models.TutorialTarget) (*models.TutorialAudiencePreview, error) {
	normalized, err := normalizeAudience(audience)
	if err != nil {
		return nil, err
	}
	people, err := s.resolveAudience(normalized, target)
	if err != nil {
		return nil, err
	}
	countable := countableAudience(people)
	return &models.TutorialAudiencePreview{
		Reach:      int64(len(countable)),
		ByAudience: audienceStats(countable, nil),
	}, nil
}

func (s *tutorialService) GetAudienceOptions() (*models.TutorialAudienceOptions, error) {
	people, err := s.userRepo.ListActiveByTypes([]models.UserType{models.UserTypeEmployer, models.UserTypeProfessional})
	if err != nil {
		return nil, err
	}

	// Las empresas y los paises salen de la gente activa, no de una lista
	// aparte: lo que se ofrece elegir es exactamente lo que existe hoy.
	headcount := map[uint]int64{}
	countries := map[string]bool{}
	for _, user := range people {
		if user.EmpleadorID != nil {
			headcount[*user.EmpleadorID]++
		}
		if country := strings.TrimSpace(user.Country); country != "" {
			countries[country] = true
		}
	}

	options := &models.TutorialAudienceOptions{
		Companies: []models.TutorialAudienceOption{},
		Countries: []string{},
		Groups:    []models.TutorialAudienceOption{},
	}
	for _, user := range people {
		if user.UserType != models.UserTypeEmployer {
			continue
		}
		options.Companies = append(options.Companies, models.TutorialAudienceOption{
			ID:   user.ID,
			Name: user.Name,
			// La cuenta de la empresa cuenta ademas de sus profesionales.
			Count: headcount[user.ID] + 1,
		})
	}
	sort.Slice(options.Companies, func(i, j int) bool {
		return strings.ToLower(options.Companies[i].Name) < strings.ToLower(options.Companies[j].Name)
	})

	for country := range countries {
		options.Countries = append(options.Countries, country)
	}
	sort.Slice(options.Countries, func(i, j int) bool {
		return strings.ToLower(options.Countries[i]) < strings.ToLower(options.Countries[j])
	})

	groups, err := s.repo.ListAudienceGroups()
	if err != nil {
		return nil, err
	}
	if groups != nil {
		options.Groups = groups
	}

	return options, nil
}
