package service

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
)

// InductionService gobierna la inducción del profesional recién contratado:
// emite la invitación a la landing pública, califica el cuestionario y decide
// si se le habilita el acceso a Obertrack o se escala a Soporte.
//
// Reutiliza dos módulos existentes: el video sale de Novedades/Tutoriales y las
// preguntas del módulo de Encuestas (extendido con respuesta correcta y
// ponderación). Lo único propio es el portero.
type InductionService interface {
	// Enabled indica si la inducción está configurada y encendida.
	Enabled() bool
	GetConfig() (*models.InductionConfig, error)
	SaveConfig(cfg *models.InductionConfig) error

	// InviteIfEnabled emite la invitación y envía el correo con el enlace.
	// Devuelve false —sin error— si la inducción no está configurada, para que
	// el alta del profesional siga el flujo directo de acceso de siempre.
	InviteIfEnabled(user *models.User) (bool, error)

	// Landing devuelve el contenido público de la invitación (video +
	// preguntas SIN las respuestas correctas).
	Landing(token string) (*LandingView, error)
	// Submit califica un intento y aplica la decisión de acceso.
	Submit(token string, answers []SubmittedAnswer) (*SubmitResult, error)

	// Reset (acción de Soporte) desbloquea al profesional y le devuelve sus
	// intentos, reenviando el enlace.
	Reset(userID uint) error
	// Status devuelve el estado de inducción de un usuario, para Soporte.
	Status(userID uint) (*InductionStatusView, error)
}

// LandingQuestion es una pregunta tal como la ve el navegador: sin la respuesta
// correcta ni la ponderación.
type LandingQuestion struct {
	ID         uint     `json:"id"`
	Text       string   `json:"text"`
	Type       string   `json:"type"`
	Options    []string `json:"options"`
	IsRequired bool     `json:"is_required"`
}

// LandingView es todo lo que la landing pública necesita para renderizarse.
type LandingView struct {
	ProfessionalName string            `json:"professional_name"`
	Status           string            `json:"status"`
	VideoTitle       string            `json:"video_title,omitempty"`
	VideoURL         string            `json:"video_url,omitempty"`
	VideoDurationMin int               `json:"video_duration_min,omitempty"`
	SurveyTitle      string            `json:"survey_title"`
	Description      string            `json:"description,omitempty"`
	Questions        []LandingQuestion `json:"questions"`
	PassingScore     int               `json:"passing_score"`
	AttemptsLeft     int               `json:"attempts_left"`
	MaxAttempts      int               `json:"max_attempts"`
	BestScore        float64           `json:"best_score"`
}

// SubmittedAnswer es una respuesta enviada desde la landing.
type SubmittedAnswer struct {
	QuestionID uint   `json:"question_id"`
	Value      string `json:"value"`
}

// SubmitResult es el veredicto de un intento.
type SubmitResult struct {
	Score        float64 `json:"score"`
	PassingScore int     `json:"passing_score"`
	Passed       bool    `json:"passed"`
	Status       string  `json:"status"`
	AttemptsLeft int     `json:"attempts_left"`
	Message      string  `json:"message"`
}

// InductionStatusView resume la inducción de un profesional para Soporte.
type InductionStatusView struct {
	UserID       uint                      `json:"user_id"`
	Name         string                    `json:"name"`
	Email        string                    `json:"email"`
	Status       string                    `json:"status"`
	Attempts     int                       `json:"attempts"`
	MaxAttempts  int                       `json:"max_attempts"`
	BestScore    float64                   `json:"best_score"`
	PassingScore int                       `json:"passing_score"`
	AttemptLog   []models.InductionAttempt `json:"attempt_log"`
}

type inductionService struct {
	repo        repository.InductionRepository
	userRepo    repository.UserRepository
	brevoSvc    *BrevoService
	authSvc     AuthService
	ticketSvc   TicketService
	frontendURL string
}

func NewInductionService(
	repo repository.InductionRepository,
	userRepo repository.UserRepository,
	brevoSvc *BrevoService,
	authSvc AuthService,
	ticketSvc TicketService,
	frontendURL string,
) InductionService {
	return &inductionService{
		repo:        repo,
		userRepo:    userRepo,
		brevoSvc:    brevoSvc,
		authSvc:     authSvc,
		ticketSvc:   ticketSvc,
		frontendURL: strings.TrimRight(frontendURL, "/"),
	}
}

func (s *inductionService) Enabled() bool {
	cfg, err := s.repo.GetConfig()
	if err != nil {
		return false
	}
	return cfg.Ready()
}

func (s *inductionService) GetConfig() (*models.InductionConfig, error) {
	return s.repo.GetConfig()
}

func (s *inductionService) SaveConfig(cfg *models.InductionConfig) error {
	if cfg == nil {
		return errors.New("configuración vacía")
	}
	if cfg.PassingScore < 0 || cfg.PassingScore > 100 {
		return errors.New("el mínimo aprobatorio debe estar entre 0 y 100")
	}
	if cfg.MaxAttempts < 1 {
		return errors.New("los intentos permitidos deben ser al menos 1")
	}
	if cfg.InviteTTLDays < 1 {
		cfg.InviteTTLDays = 30
	}
	// Encender la inducción sin cuestionario dejaría a todo profesional nuevo
	// sin poder entrar nunca: se rechaza explícitamente.
	if cfg.IsActive && (cfg.SurveyID == nil || *cfg.SurveyID == 0) {
		return errors.New("para activar la inducción debes elegir un cuestionario")
	}
	return s.repo.SaveConfig(cfg)
}

func (s *inductionService) InviteIfEnabled(user *models.User) (bool, error) {
	if user == nil {
		return false, errors.New("usuario inválido")
	}
	cfg, err := s.repo.GetConfig()
	if err != nil {
		return false, err
	}
	if !cfg.Ready() {
		// Inducción apagada o incompleta: el profesional entra por el flujo normal.
		return false, nil
	}

	token, err := generateInductionToken()
	if err != nil {
		return false, err
	}

	// Una invitación viva por usuario: si se re-contrata, se reemplaza.
	_ = s.repo.DeleteInviteByUser(user.ID)

	invite := &models.InductionInvite{
		UserID:       user.ID,
		Token:        token,
		Status:       models.InductionPending,
		MaxAttempts:  cfg.MaxAttempts,
		PassingScore: cfg.PassingScore,
		TutorialID:   cfg.TutorialID,
		SurveyID:     cfg.SurveyID,
		ExpiresAt:    time.Now().AddDate(0, 0, cfg.InviteTTLDays),
	}
	if err := s.repo.CreateInvite(invite); err != nil {
		return false, err
	}

	// El profesional queda SIN acceso hasta aprobar.
	if err := s.userRepo.Update(user, map[string]interface{}{
		"onboarding_status": models.OnboardingPending,
	}); err != nil {
		return false, err
	}

	s.sendInviteEmail(user, token)
	return true, nil
}

func (s *inductionService) Landing(token string) (*LandingView, error) {
	invite, err := s.loadInvite(token)
	if err != nil {
		return nil, err
	}
	user, err := s.userRepo.GetByID(invite.UserID)
	if err != nil {
		return nil, errors.New("invitación inválida")
	}

	view := &LandingView{
		ProfessionalName: user.Name,
		Status:           invite.Status,
		PassingScore:     invite.PassingScore,
		AttemptsLeft:     invite.AttemptsLeft(),
		MaxAttempts:      invite.MaxAttempts,
		BestScore:        invite.BestScore,
		Questions:        []LandingQuestion{},
	}

	// Video (Novedades/Tutoriales). Es opcional: la inducción puede ser solo
	// cuestionario.
	if invite.TutorialID != nil && *invite.TutorialID > 0 {
		if t, err := s.repo.GetTutorial(*invite.TutorialID); err == nil {
			view.VideoTitle = t.Title
			view.VideoURL = t.GoogleDriveURL
			view.VideoDurationMin = t.DurationMin
		}
	}

	// Ya resuelto: no se devuelven preguntas.
	if invite.Status != models.InductionPending {
		return view, nil
	}

	survey, err := s.loadSurvey(invite)
	if err != nil {
		return nil, err
	}
	view.SurveyTitle = survey.Title
	view.Description = survey.Description
	for _, q := range survey.Questions {
		view.Questions = append(view.Questions, LandingQuestion{
			ID:         q.ID,
			Text:       q.Text,
			Type:       string(q.Type),
			Options:    parseOptions(q.Options),
			IsRequired: q.IsRequired,
		})
	}
	return view, nil
}

func (s *inductionService) Submit(token string, answers []SubmittedAnswer) (*SubmitResult, error) {
	invite, err := s.loadInvite(token)
	if err != nil {
		return nil, err
	}
	if invite.Status == models.InductionPassed {
		return nil, errors.New("ya completaste la inducción")
	}
	if invite.Status == models.InductionBlocked || invite.AttemptsLeft() <= 0 {
		return nil, errors.New("agotaste tus intentos. Nuestro equipo de soporte se pondrá en contacto contigo")
	}

	user, err := s.userRepo.GetByID(invite.UserID)
	if err != nil {
		return nil, errors.New("invitación inválida")
	}
	survey, err := s.loadSurvey(invite)
	if err != nil {
		return nil, err
	}

	score := scoreAnswers(survey.Questions, answers)
	passed := score >= float64(invite.PassingScore)

	// Deja evidencia del intento para Soporte.
	blob, _ := json.Marshal(answers)
	_ = s.repo.CreateAttempt(&models.InductionAttempt{
		InviteID:    invite.ID,
		UserID:      user.ID,
		Score:       score,
		Passed:      passed,
		AnswersJSON: string(blob),
	})

	attempts := invite.Attempts + 1
	best := invite.BestScore
	if score > best {
		best = score
	}
	updates := map[string]interface{}{"attempts": attempts, "best_score": best}

	result := &SubmitResult{Score: score, PassingScore: invite.PassingScore, Passed: passed}

	switch {
	case passed:
		now := time.Now()
		updates["status"] = models.InductionPassed
		updates["completed_at"] = now
		_ = s.repo.UpdateInvite(invite, updates)
		s.grantAccess(user)
		result.Status = models.InductionPassed
		result.AttemptsLeft = invite.MaxAttempts - attempts
		result.Message = "¡Aprobaste! Te enviamos un correo para que establezcas tu contraseña y entres a Obertrack."

	case attempts >= invite.MaxAttempts:
		updates["status"] = models.InductionBlocked
		_ = s.repo.UpdateInvite(invite, updates)
		_ = s.userRepo.Update(user, map[string]interface{}{
			"onboarding_status": models.OnboardingBlocked,
		})
		s.alertSupport(user, score, attempts, invite.PassingScore)
		result.Status = models.InductionBlocked
		result.AttemptsLeft = 0
		result.Message = "No alcanzaste el mínimo aprobatorio y agotaste tus intentos. Nuestro equipo se pondrá en contacto contigo."

	default:
		_ = s.repo.UpdateInvite(invite, updates)
		result.Status = models.InductionPending
		result.AttemptsLeft = invite.MaxAttempts - attempts
		result.Message = fmt.Sprintf("No alcanzaste el mínimo de %d%%. Puedes intentarlo de nuevo (te quedan %d intentos).",
			invite.PassingScore, result.AttemptsLeft)
	}

	return result, nil
}

func (s *inductionService) Reset(userID uint) error {
	invite, err := s.repo.GetInviteByUser(userID)
	if err != nil {
		return errors.New("este profesional no tiene una inducción pendiente")
	}
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return errors.New("usuario no encontrado")
	}
	if err := s.repo.UpdateInvite(invite, map[string]interface{}{
		"attempts":     0,
		"status":       models.InductionPending,
		"completed_at": nil,
	}); err != nil {
		return err
	}
	if err := s.userRepo.Update(user, map[string]interface{}{
		"onboarding_status": models.OnboardingPending,
	}); err != nil {
		return err
	}
	s.sendInviteEmail(user, invite.Token)
	return nil
}

func (s *inductionService) Status(userID uint) (*InductionStatusView, error) {
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return nil, errors.New("usuario no encontrado")
	}
	invite, err := s.repo.GetInviteByUser(userID)
	if err != nil {
		return nil, errors.New("este profesional no tiene inducción registrada")
	}
	attempts, _ := s.repo.ListAttempts(invite.ID)
	if attempts == nil {
		attempts = []models.InductionAttempt{}
	}
	return &InductionStatusView{
		UserID:       user.ID,
		Name:         user.Name,
		Email:        user.Email,
		Status:       invite.Status,
		Attempts:     invite.Attempts,
		MaxAttempts:  invite.MaxAttempts,
		BestScore:    invite.BestScore,
		PassingScore: invite.PassingScore,
		AttemptLog:   attempts,
	}, nil
}

// --- Internos ---

// loadInvite resuelve el token y valida vigencia. Devuelve siempre el mismo
// mensaje ante token inexistente o vencido para no filtrar cuáles existen.
func (s *inductionService) loadInvite(token string) (*models.InductionInvite, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, errors.New("invitación inválida")
	}
	invite, err := s.repo.GetInviteByToken(token)
	if err != nil {
		return nil, errors.New("invitación inválida o vencida")
	}
	if !invite.ExpiresAt.IsZero() && time.Now().After(invite.ExpiresAt) {
		return nil, errors.New("invitación inválida o vencida")
	}
	return invite, nil
}

func (s *inductionService) loadSurvey(invite *models.InductionInvite) (*models.Survey, error) {
	if invite.SurveyID == nil || *invite.SurveyID == 0 {
		return nil, errors.New("la inducción no tiene un cuestionario configurado")
	}
	survey, err := s.repo.GetSurveyWithQuestions(*invite.SurveyID)
	if err != nil {
		return nil, errors.New("la inducción no tiene un cuestionario configurado")
	}
	return survey, nil
}

// grantAccess habilita al profesional y le manda el enlace para establecer su
// contraseña (reusa el flujo de recuperación: nunca viaja una clave por correo).
func (s *inductionService) grantAccess(user *models.User) {
	if err := s.userRepo.Update(user, map[string]interface{}{
		"onboarding_status": models.OnboardingPassed,
	}); err != nil {
		log.Printf("[Induction] no se pudo habilitar el acceso de %s: %v", user.Email, err)
		return
	}
	if s.authSvc != nil {
		// Correo de PRIMERA vez ("crea tu contraseña"), no de recuperación:
		// quien lo recibe nunca tuvo una.
		if err := s.authSvc.SendPasswordSetupEmail(user.Email); err != nil {
			log.Printf("[Induction] no se pudo enviar el correo de acceso a %s: %v", user.Email, err)
		}
	}
}

// alertSupport abre una alerta interna en el módulo de Soporte para que
// contacten al profesional que no aprobó.
func (s *inductionService) alertSupport(user *models.User, score float64, attempts, passing int) {
	if s.ticketSvc == nil {
		return
	}
	companyName := ""
	if user.EmpleadorID != nil {
		if company, err := s.userRepo.GetByID(*user.EmpleadorID); err == nil {
			companyName = company.CompanyDisplayName()
		}
	}
	err := s.ticketSvc.CreateInductionFailureAlert(InductionAlertInput{
		ProfessionalID:    user.ID,
		ProfessionalName:  user.Name,
		ProfessionalEmail: user.Email,
		ProfessionalPhone: user.PhoneNumber,
		CompanyName:       companyName,
		Score:             score,
		PassingScore:      passing,
		Attempts:          attempts,
	})
	if err != nil {
		log.Printf("[Induction] no se pudo abrir la alerta de soporte para %s: %v", user.Email, err)
	}
}

func (s *inductionService) sendInviteEmail(user *models.User, token string) {
	if s.brevoSvc == nil {
		return
	}
	link := s.frontendURL + "/induccion/" + token
	subject := "Bienvenido a Obertrack — completa tu inducción"
	html := BuildInductionInviteHTML(user.Name, link)

	go func() {
		if err := s.brevoSvc.SendEmail(user.Email, user.Name, subject, html); err != nil {
			log.Printf("[Induction] no se pudo enviar la invitación a %s: %v", user.Email, err)
		}
	}()
}

// scoreAnswers calcula el puntaje ponderado (0-100). Solo puntúan las preguntas
// con respuesta correcta definida y peso > 0.
func scoreAnswers(questions []models.SurveyQuestion, answers []SubmittedAnswer) float64 {
	given := make(map[uint]string, len(answers))
	for _, a := range answers {
		given[a.QuestionID] = a.Value
	}

	totalWeight, earned := 0, 0
	for i := range questions {
		q := &questions[i]
		if !q.IsScorable() {
			continue
		}
		totalWeight += q.Weight
		if normalizeAnswer(given[q.ID]) == normalizeAnswer(q.CorrectAnswer) {
			earned += q.Weight
		}
	}

	if totalWeight == 0 {
		// Cuestionario sin preguntas calificables: es un error de configuración.
		// Se aprueba en lugar de bloquear a todo el mundo, y se avisa fuerte.
		log.Printf("[Induction] WARN: el cuestionario no tiene preguntas calificables; se aprueba por defecto")
		return 100
	}
	return float64(earned) / float64(totalWeight) * 100
}

func normalizeAnswer(v string) string {
	return strings.ToLower(strings.TrimSpace(v))
}

// parseOptions decodifica el JSON de opciones de una pregunta de selección.
func parseOptions(raw string) []string {
	out := []string{}
	if strings.TrimSpace(raw) == "" {
		return out
	}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return []string{}
	}
	return out
}

func generateInductionToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", errors.New("no se pudo generar la invitación")
	}
	return hex.EncodeToString(b), nil
}
