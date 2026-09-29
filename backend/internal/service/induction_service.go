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
	"github.com/obertrack/backend/internal/utils"
)

// InductionService gobierna la inducción del profesional recién contratado:
// emite la invitación a la landing pública, califica los cuestionarios bloque
// por bloque y decide si se le habilita el acceso a Obertrack o se escala a
// Soporte.
//
// Reutiliza dos módulos existentes: el video de cada bloque sale de
// Novedades/Tutoriales y sus preguntas del módulo de Encuestas (extendido con
// respuesta correcta y ponderación). Lo propio es el portero y la secuencia:
// bloques reutilizables armados en programas, asignables por empresa.
type InductionService interface {
	// Enabled indica si la inducción está encendida y hay un programa por
	// defecto usable.
	Enabled() bool
	GetConfig() (*models.InductionConfig, error)
	SaveConfig(cfg *models.InductionConfig) error

	// --- Biblioteca de bloques ---
	ListBlocks() ([]models.InductionBlock, error)
	CreateBlock(actorID uint, in BlockInput) (*models.InductionBlock, error)
	UpdateBlock(id uint, in BlockInput) (*models.InductionBlock, error)
	DeleteBlock(id uint) error

	// --- Programas ---
	ListPrograms() ([]models.InductionProgram, error)
	GetProgram(id uint) (*models.InductionProgram, error)
	CreateProgram(actorID uint, in ProgramInput) (*models.InductionProgram, error)
	UpdateProgram(id uint, in ProgramInput) (*models.InductionProgram, error)
	DeleteProgram(id uint) error
	SetProgramBlocks(id uint, blockIDs []uint) (*models.InductionProgram, error)
	SetProgramCompanies(id uint, companyIDs []uint) (*models.InductionProgram, error)

	// InviteIfEnabled emite la invitación con el programa que le toca a la
	// empresa del profesional (o el por defecto) y envía el correo con el
	// enlace. Devuelve false —sin error— si la inducción no está lista, para
	// que el alta del profesional siga el flujo directo de acceso de siempre.
	InviteIfEnabled(user *models.User) (bool, error)

	// Invite emite la inducción a un profesional que YA existe, a petición de
	// Soporte. Es la puerta para los que no llegaron por el puente de Obersuite
	// (alta manual, alta desde la empresa, importación), que hasta ahora no
	// tenían forma de pasar por la inducción. A diferencia de InviteIfEnabled,
	// aquí una inducción apagada sí es un error: es una acción explícita.
	// programID 0 = resolver por empresa / por defecto. gatesAccess decide si
	// la persona queda sin acceso hasta aprobar; si nunca lo tuvo, se fuerza.
	Invite(userID uint, programID uint, gatesAccess bool) error

	// MyInduction devuelve la capacitación en curso del usuario de la sesión
	// (nil si no tiene ninguna pendiente), para que la app la ofrezca.
	MyInduction(userID uint) (*MyInductionView, error)

	// Landing devuelve el contenido público de la invitación: el progreso por
	// bloque y el bloque actual completo (video + preguntas SIN respuestas).
	Landing(token string) (*LandingView, error)
	// Submit califica un intento sobre el bloque actual y aplica la decisión:
	// avanza al siguiente, repite, aprueba el programa o bloquea.
	Submit(token string, blockID uint, answers []SubmittedAnswer) (*SubmitResult, error)

	// Reset (acción de Soporte) reinicia la capacitación ACTUAL del
	// profesional (la pendiente o la última bloqueada): le devuelve sus
	// intentos en todos los bloques y le reenvía un enlace nuevo.
	Reset(userID uint) error
	// ResetInvite reinicia una invitación concreta del historial.
	ResetInvite(inviteID uint) error
	// Status devuelve el estado de inducción de un usuario, para Soporte.
	Status(userID uint) (*InductionStatusView, error)

	// ListBadges devuelve las insignias ganadas por una persona y las que
	// todavía puede ganar en su inducción en curso.
	ListBadges(userID uint) (*models.BadgeOverview, error)
}

// BlockInput es lo que llega del formulario de un bloque.
type BlockInput struct {
	Name         string `json:"name"`
	Description  string `json:"description"`
	TutorialID   *uint  `json:"tutorial_id"`
	SurveyID     uint   `json:"survey_id"`
	PassingScore *int   `json:"passing_score"`
	BadgeTitle   string `json:"badge_title"`
	BadgeIcon    string `json:"badge_icon"`
	BadgeColor   string `json:"badge_color"`
}

// ProgramInput es lo que llega del formulario de un programa. Los bloques y
// las empresas se mandan por sus propias operaciones.
type ProgramInput struct {
	Name                string `json:"name"`
	Description         string `json:"description"`
	DefaultPassingScore int    `json:"default_passing_score"`
	MaxAttempts         int    `json:"max_attempts"`
	IsDefault           bool   `json:"is_default"`
	IsActive            bool   `json:"is_active"`
	BadgeTitle          string `json:"badge_title"`
	BadgeIcon           string `json:"badge_icon"`
	BadgeColor          string `json:"badge_color"`
	// CertificateTemplateID 0 = sin certificado.
	CertificateTemplateID uint `json:"certificate_template_id"`
}

// CertificateIssuer es lo que la inducción necesita de los certificados:
// emitir al completar y saber si una capacitación ya tiene el suyo. Es una
// interfaz pequeña para que las pruebas no arrastren el renderizador.
type CertificateIssuer interface {
	IssueForCompletion(user *models.User, invite *models.InductionInvite) (*models.Certificate, error)
	GetForInvite(inviteID uint) (*models.Certificate, error)
}

// CertificateSummary es lo que viaja a la landing y al panel sobre un
// certificado emitido.
type CertificateSummary struct {
	ID          uint      `json:"id"`
	Code        string    `json:"code"`
	ProgramName string    `json:"program_name"`
	IssuedAt    time.Time `json:"issued_at"`
	DownloadURL string    `json:"download_url"`
}

func summarizeCertificate(c *models.Certificate) *CertificateSummary {
	if c == nil {
		return nil
	}
	return &CertificateSummary{ID: c.ID, Code: c.Code, ProgramName: c.ProgramName, IssuedAt: c.IssuedAt, DownloadURL: c.DownloadURL()}
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

// LandingBlock es un bloque en la barra de progreso de la landing.
type LandingBlock struct {
	BlockID      uint    `json:"block_id"`
	OrderIndex   int     `json:"order_index"`
	Name         string  `json:"name"`
	Status       string  `json:"status"`
	Attempts     int     `json:"attempts"`
	AttemptsLeft int     `json:"attempts_left"`
	BestScore    float64 `json:"best_score"`
	PassingScore int     `json:"passing_score"`
	HasVideo     bool    `json:"has_video"`
}

// LandingCurrentBlock es el bloque que le toca responder, completo.
type LandingCurrentBlock struct {
	BlockID          uint              `json:"block_id"`
	OrderIndex       int               `json:"order_index"`
	Name             string            `json:"name"`
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

// MyInductionView es la capacitación pendiente vista desde dentro de la app.
type MyInductionView struct {
	ProgramName     string    `json:"program_name"`
	Token           string    `json:"token"`
	Status          string    `json:"status"`
	TotalBlocks     int       `json:"total_blocks"`
	CompletedBlocks int       `json:"completed_blocks"`
	ExpiresAt       time.Time `json:"expires_at"`
	GatesAccess     bool      `json:"gates_access"`
}

// LandingView es todo lo que la landing pública necesita para renderizarse.
type LandingView struct {
	ProfessionalName string `json:"professional_name"`
	Status           string `json:"status"`
	ProgramName      string `json:"program_name"`
	// GatesAccess: la landing adapta sus textos según si aprobar habilita el
	// acceso (ingreso) o solo cierra una capacitación.
	GatesAccess bool `json:"gates_access"`
	MaxAttempts      int    `json:"max_attempts"`
	TotalBlocks      int    `json:"total_blocks"`
	CompletedBlocks  int    `json:"completed_blocks"`
	Blocks           []LandingBlock `json:"blocks"`
	// Current es nil cuando la inducción ya se resolvió (aprobada o bloqueada).
	Current *LandingCurrentBlock `json:"current,omitempty"`
	// Badges son las insignias que ya ganó, para mostrarlas en el panel.
	Badges []models.UserBadge `json:"badges"`
	// Certificate es el certificado de esta capacitación, si ya se emitió.
	Certificate *CertificateSummary `json:"certificate,omitempty"`
}

// SubmittedAnswer es una respuesta enviada desde la landing.
type SubmittedAnswer struct {
	QuestionID uint   `json:"question_id"`
	Value      string `json:"value"`
}

// SubmitResult es el veredicto de un intento sobre un bloque.
type SubmitResult struct {
	BlockID      uint    `json:"block_id"`
	BlockIndex   int     `json:"block_index"` // 1-based
	BlockName    string  `json:"block_name"`
	Score        float64 `json:"score"`
	PassingScore int     `json:"passing_score"`
	Passed       bool    `json:"passed"`
	// BlockStatus es el estado del bloque tras el intento.
	BlockStatus string `json:"block_status"`
	// Status es el estado de la inducción completa tras el intento.
	Status string `json:"status"`
	// Completed indica que con este intento se aprobó el programa entero.
	Completed    bool   `json:"completed"`
	AttemptsLeft int    `json:"attempts_left"`
	NextBlockID  *uint  `json:"next_block_id,omitempty"`
	NextBlock    string `json:"next_block_name,omitempty"`
	Message      string `json:"message"`
	// BadgesEarned son las insignias que se ganaron con este envío, para que
	// la landing las celebre.
	BadgesEarned []models.UserBadge `json:"badges_earned"`
	// Certificate se emite al completar el programa, si tiene plantilla.
	Certificate *CertificateSummary `json:"certificate,omitempty"`
}

// InductionBlockStatus resume un bloque de la invitación para Soporte.
type InductionBlockStatus struct {
	BlockID      uint       `json:"block_id"`
	OrderIndex   int        `json:"order_index"`
	Name         string     `json:"name"`
	Status       string     `json:"status"`
	Attempts     int        `json:"attempts"`
	BestScore    float64    `json:"best_score"`
	PassingScore int        `json:"passing_score"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
}

// InductionStatusView resume la inducción de un profesional para Soporte.
type InductionStatusView struct {
	UserID      uint   `json:"user_id"`
	Name        string `json:"name"`
	Email       string `json:"email"`
	Status      string `json:"status"`
	ProgramName string `json:"program_name"`
	GatesAccess bool   `json:"gates_access"`
	// Attempts es el total de intentos sumando todos los bloques; MaxAttempts
	// es el tope por bloque.
	Attempts        int                       `json:"attempts"`
	MaxAttempts     int                       `json:"max_attempts"`
	TotalBlocks     int                       `json:"total_blocks"`
	CompletedBlocks int                       `json:"completed_blocks"`
	ExpiresAt       time.Time                 `json:"expires_at"`
	Blocks          []InductionBlockStatus    `json:"blocks"`
	AttemptLog      []models.InductionAttempt `json:"attempt_log"`
	// InviteID es la invitación que describe esta vista (la actual).
	InviteID uint `json:"invite_id"`
	// History son todas las capacitaciones de la persona, la actual incluida.
	History []InductionHistoryItem `json:"history"`
}

// InductionHistoryItem resume una capacitación pasada o en curso.
type InductionHistoryItem struct {
	ID              uint       `json:"id"`
	ProgramName     string     `json:"program_name"`
	Status          string     `json:"status"`
	GatesAccess     bool       `json:"gates_access"`
	TotalBlocks     int        `json:"total_blocks"`
	CompletedBlocks int        `json:"completed_blocks"`
	CreatedAt       time.Time  `json:"created_at"`
	CompletedAt     *time.Time `json:"completed_at,omitempty"`
	Current         bool       `json:"current"`
	// Certificate es el emitido para esta capacitación, si lo hay.
	Certificate *CertificateSummary `json:"certificate,omitempty"`
}

type inductionService struct {
	repo        repository.InductionRepository
	userRepo    repository.UserRepository
	badgeRepo   repository.BadgeRepository
	brevoSvc    *BrevoService
	authSvc     AuthService
	ticketSvc   TicketService
	notifSvc    NotificationService
	certSvc     CertificateIssuer
	frontendURL string
}

func NewInductionService(
	repo repository.InductionRepository,
	userRepo repository.UserRepository,
	badgeRepo repository.BadgeRepository,
	brevoSvc *BrevoService,
	authSvc AuthService,
	ticketSvc TicketService,
	notifSvc NotificationService,
	certSvc CertificateIssuer,
	frontendURL string,
) InductionService {
	return &inductionService{
		repo:        repo,
		userRepo:    userRepo,
		badgeRepo:   badgeRepo,
		brevoSvc:    brevoSvc,
		authSvc:     authSvc,
		ticketSvc:   ticketSvc,
		notifSvc:    notifSvc,
		certSvc:     certSvc,
		frontendURL: strings.TrimRight(frontendURL, "/"),
	}
}

// certificateFor devuelve el resumen del certificado de una invitación, si
// hay servicio y si existe.
func (s *inductionService) certificateFor(inviteID uint) *CertificateSummary {
	if s.certSvc == nil {
		return nil
	}
	cert, err := s.certSvc.GetForInvite(inviteID)
	if err != nil || cert == nil {
		return nil
	}
	return summarizeCertificate(cert)
}

// baseURL es el dominio para los enlaces del correo. Sin el respaldo, un
// FRONTEND_URL vacío mandaba "/induccion/<token>": una ruta relativa que en un
// correo no lleva a ninguna parte (el rastreo de clics de Brevo responde 404).
func (s *inductionService) baseURL() string {
	if s.frontendURL == "" {
		return FrontendBaseURL()
	}
	return s.frontendURL
}

// --- Configuración -----------------------------------------------------------

func (s *inductionService) Enabled() bool {
	cfg, err := s.repo.GetConfig()
	if err != nil || cfg == nil || !cfg.IsActive {
		return false
	}
	program, err := s.repo.GetDefaultProgram()
	return err == nil && program.Usable()
}

func (s *inductionService) GetConfig() (*models.InductionConfig, error) {
	return s.repo.GetConfig()
}

func (s *inductionService) SaveConfig(cfg *models.InductionConfig) error {
	if cfg == nil {
		return errors.New("configuración vacía")
	}
	if cfg.InviteTTLDays < 1 {
		cfg.InviteTTLDays = 30
	}
	// Encender la inducción sin un programa por defecto usable dejaría a todo
	// profesional nuevo sin poder entrar nunca: se rechaza explícitamente.
	if cfg.IsActive {
		program, err := s.repo.GetDefaultProgram()
		if err != nil {
			return err
		}
		if !program.Usable() {
			return errors.New("para activar la inducción debes tener un programa por defecto activo con al menos un bloque")
		}
	}
	return s.repo.SaveConfig(cfg)
}

// --- Bloques -----------------------------------------------------------------

func (s *inductionService) ListBlocks() ([]models.InductionBlock, error) {
	blocks, err := s.repo.ListBlocks()
	if err != nil {
		return nil, err
	}
	if blocks == nil {
		blocks = []models.InductionBlock{}
	}
	return blocks, nil
}

func validateBlockInput(in *BlockInput) error {
	in.Name = strings.TrimSpace(utils.SanitizeHTML(in.Name))
	in.Description = strings.TrimSpace(utils.SanitizeHTML(in.Description))
	if in.Name == "" {
		return errors.New("el nombre del bloque es obligatorio")
	}
	if in.SurveyID == 0 {
		return errors.New("el bloque necesita un cuestionario")
	}
	if in.TutorialID != nil && *in.TutorialID == 0 {
		in.TutorialID = nil
	}
	if in.PassingScore != nil && (*in.PassingScore < 0 || *in.PassingScore > 100) {
		return errors.New("el mínimo aprobatorio debe estar entre 0 y 100")
	}
	in.BadgeTitle = strings.TrimSpace(utils.SanitizeHTML(in.BadgeTitle))
	in.BadgeIcon = normalizeBadgeIcon(in.BadgeIcon, DefaultBlockBadgeIcon)
	in.BadgeColor = normalizeBadgeColor(in.BadgeColor, DefaultBlockBadgeColor)
	return nil
}

func (s *inductionService) CreateBlock(actorID uint, in BlockInput) (*models.InductionBlock, error) {
	if err := validateBlockInput(&in); err != nil {
		return nil, err
	}
	if _, err := s.repo.GetSurveyWithQuestions(in.SurveyID); err != nil {
		return nil, errors.New("el cuestionario elegido no existe")
	}
	if in.TutorialID != nil {
		if _, err := s.repo.GetTutorial(*in.TutorialID); err != nil {
			return nil, errors.New("el video elegido no existe")
		}
	}
	block := &models.InductionBlock{
		Name:         in.Name,
		Description:  in.Description,
		TutorialID:   in.TutorialID,
		SurveyID:     in.SurveyID,
		PassingScore: in.PassingScore,
		BadgeTitle:   in.BadgeTitle,
		BadgeIcon:    in.BadgeIcon,
		BadgeColor:   in.BadgeColor,
		CreatedBy:    actorID,
	}
	if err := s.repo.CreateBlock(block); err != nil {
		return nil, err
	}
	return s.repo.GetBlock(block.ID)
}

func (s *inductionService) UpdateBlock(id uint, in BlockInput) (*models.InductionBlock, error) {
	if _, err := s.repo.GetBlock(id); err != nil {
		return nil, errors.New("bloque no encontrado")
	}
	if err := validateBlockInput(&in); err != nil {
		return nil, err
	}
	if _, err := s.repo.GetSurveyWithQuestions(in.SurveyID); err != nil {
		return nil, errors.New("el cuestionario elegido no existe")
	}
	if in.TutorialID != nil {
		if _, err := s.repo.GetTutorial(*in.TutorialID); err != nil {
			return nil, errors.New("el video elegido no existe")
		}
	}
	// Los opcionales van como nil explícito (no como puntero nulo tipado)
	// para que el UPDATE escriba NULL sin depender del driver.
	var tutorialID interface{}
	if in.TutorialID != nil {
		tutorialID = *in.TutorialID
	}
	var passingScore interface{}
	if in.PassingScore != nil {
		passingScore = *in.PassingScore
	}
	updates := map[string]interface{}{
		"name":          in.Name,
		"description":   in.Description,
		"tutorial_id":   tutorialID,
		"survey_id":     in.SurveyID,
		"passing_score": passingScore,
		"badge_title":   in.BadgeTitle,
		"badge_icon":    in.BadgeIcon,
		"badge_color":   in.BadgeColor,
	}
	if err := s.repo.UpdateBlock(id, updates); err != nil {
		return nil, err
	}
	return s.repo.GetBlock(id)
}

func (s *inductionService) DeleteBlock(id uint) error {
	if _, err := s.repo.GetBlock(id); err != nil {
		return errors.New("bloque no encontrado")
	}
	// Un bloque en uso no se borra: dejaría un hueco en la secuencia de un
	// programa que alguien puede estar recibiendo ahora mismo.
	used, err := s.repo.CountProgramsUsingBlock(id)
	if err != nil {
		return err
	}
	if used > 0 {
		return errors.New("este bloque está en uso en un programa; quítalo del programa antes de borrarlo")
	}
	return s.repo.DeleteBlock(id)
}

// --- Programas ---------------------------------------------------------------

func (s *inductionService) ListPrograms() ([]models.InductionProgram, error) {
	programs, err := s.repo.ListPrograms()
	if err != nil {
		return nil, err
	}
	if programs == nil {
		programs = []models.InductionProgram{}
	}
	return programs, nil
}

func (s *inductionService) GetProgram(id uint) (*models.InductionProgram, error) {
	program, err := s.repo.GetProgram(id)
	if err != nil {
		return nil, errors.New("programa no encontrado")
	}
	return program, nil
}

func validateProgramInput(in *ProgramInput) error {
	in.Name = strings.TrimSpace(utils.SanitizeHTML(in.Name))
	in.Description = strings.TrimSpace(utils.SanitizeHTML(in.Description))
	if in.Name == "" {
		return errors.New("el nombre del programa es obligatorio")
	}
	if in.DefaultPassingScore < 0 || in.DefaultPassingScore > 100 {
		return errors.New("el mínimo aprobatorio debe estar entre 0 y 100")
	}
	if in.MaxAttempts < 1 {
		return errors.New("los intentos permitidos deben ser al menos 1")
	}
	in.BadgeTitle = strings.TrimSpace(utils.SanitizeHTML(in.BadgeTitle))
	in.BadgeIcon = normalizeBadgeIcon(in.BadgeIcon, DefaultProgramBadgeIcon)
	in.BadgeColor = normalizeBadgeColor(in.BadgeColor, DefaultProgramBadgeColor)
	return nil
}

func (s *inductionService) CreateProgram(actorID uint, in ProgramInput) (*models.InductionProgram, error) {
	if err := validateProgramInput(&in); err != nil {
		return nil, err
	}
	program := &models.InductionProgram{
		Name:                in.Name,
		Description:         in.Description,
		DefaultPassingScore: in.DefaultPassingScore,
		MaxAttempts:         in.MaxAttempts,
		IsActive:            in.IsActive,
		BadgeTitle:          in.BadgeTitle,
		BadgeIcon:           in.BadgeIcon,
		BadgeColor:          in.BadgeColor,
		CertificateTemplateID: optionalID(in.CertificateTemplateID),
		CreatedBy:             actorID,
	}
	if err := s.repo.CreateProgram(program); err != nil {
		return nil, err
	}
	// El primer programa que se crea es el por defecto aunque no lo pidan:
	// sin uno, la inducción no tiene a quién mandar a nadie.
	makeDefault := in.IsDefault
	if !makeDefault {
		current, err := s.repo.GetDefaultProgram()
		if err == nil && current == nil {
			makeDefault = true
		}
	}
	if makeDefault {
		if err := s.repo.SetDefaultProgram(program.ID); err != nil {
			return nil, err
		}
	}
	return s.repo.GetProgram(program.ID)
}

func (s *inductionService) UpdateProgram(id uint, in ProgramInput) (*models.InductionProgram, error) {
	current, err := s.repo.GetProgram(id)
	if err != nil {
		return nil, errors.New("programa no encontrado")
	}
	if err := validateProgramInput(&in); err != nil {
		return nil, err
	}
	// El programa por defecto no se apaga ni se desmarca: primero hay que
	// elegir otro. Si no, la inducción encendida quedaría sin destino.
	if current.IsDefault && !in.IsDefault {
		return nil, errors.New("elige otro programa por defecto antes de quitarle la marca a este")
	}
	if current.IsDefault && !in.IsActive {
		return nil, errors.New("el programa por defecto no se puede apagar; elige otro por defecto primero")
	}
	updates := map[string]interface{}{
		"name":                  in.Name,
		"description":           in.Description,
		"default_passing_score": in.DefaultPassingScore,
		"max_attempts":          in.MaxAttempts,
		"is_active":             in.IsActive,
		"badge_title":           in.BadgeTitle,
		"badge_icon":            in.BadgeIcon,
		"badge_color":           in.BadgeColor,
		"certificate_template_id": optionalIDValue(in.CertificateTemplateID),
	}
	if err := s.repo.UpdateProgram(id, updates); err != nil {
		return nil, err
	}
	if in.IsDefault && !current.IsDefault {
		if !in.IsActive {
			return nil, errors.New("el programa por defecto debe estar activo")
		}
		if err := s.repo.SetDefaultProgram(id); err != nil {
			return nil, err
		}
	}
	return s.repo.GetProgram(id)
}

func optionalID(v uint) *uint {
	if v == 0 {
		return nil
	}
	return &v
}

// optionalIDValue es el mismo opcional para un UPDATE por mapa: nil explícito
// escribe NULL sin depender del driver.
func optionalIDValue(v uint) interface{} {
	if v == 0 {
		return nil
	}
	return v
}

func (s *inductionService) DeleteProgram(id uint) error {
	program, err := s.repo.GetProgram(id)
	if err != nil {
		return errors.New("programa no encontrado")
	}
	if program.IsDefault {
		return errors.New("el programa por defecto no se puede borrar; elige otro por defecto primero")
	}
	return s.repo.DeleteProgram(id)
}

func (s *inductionService) SetProgramBlocks(id uint, blockIDs []uint) (*models.InductionProgram, error) {
	program, err := s.repo.GetProgram(id)
	if err != nil {
		return nil, errors.New("programa no encontrado")
	}
	// Sin repetidos y todos existentes: un bloque dos veces en la misma
	// secuencia no tiene sentido, y uno inexistente rompería la landing.
	seen := map[uint]bool{}
	clean := make([]uint, 0, len(blockIDs))
	for _, blockID := range blockIDs {
		if blockID == 0 || seen[blockID] {
			continue
		}
		if _, err := s.repo.GetBlock(blockID); err != nil {
			return nil, fmt.Errorf("el bloque %d no existe", blockID)
		}
		seen[blockID] = true
		clean = append(clean, blockID)
	}
	// El programa por defecto no puede quedarse vacío con la inducción
	// encendida: es lo que recibe todo el que no tiene asignación.
	if program.IsDefault && len(clean) == 0 {
		cfg, err := s.repo.GetConfig()
		if err == nil && cfg.IsActive {
			return nil, errors.New("el programa por defecto necesita al menos un bloque mientras la inducción esté encendida")
		}
	}
	if err := s.repo.ReplaceProgramBlocks(id, clean); err != nil {
		return nil, err
	}
	return s.repo.GetProgram(id)
}

func (s *inductionService) SetProgramCompanies(id uint, companyIDs []uint) (*models.InductionProgram, error) {
	if _, err := s.repo.GetProgram(id); err != nil {
		return nil, errors.New("programa no encontrado")
	}
	seen := map[uint]bool{}
	clean := make([]uint, 0, len(companyIDs))
	for _, companyID := range companyIDs {
		if companyID == 0 || seen[companyID] {
			continue
		}
		user, err := s.userRepo.GetByID(companyID)
		if err != nil || user.UserType != models.UserTypeEmployer {
			return nil, fmt.Errorf("la empresa %d no existe", companyID)
		}
		seen[companyID] = true
		clean = append(clean, companyID)
	}
	if err := s.repo.ReplaceProgramCompanies(id, clean); err != nil {
		return nil, err
	}
	return s.repo.GetProgram(id)
}

// resolveProgram decide qué programa recibe el profesional: el asignado a su
// empresa si es usable; si no, el por defecto. Devuelve nil sin error cuando
// no hay ninguno usable, que es "la inducción no aplica".
func (s *inductionService) resolveProgram(user *models.User) (*models.InductionProgram, error) {
	if user != nil && user.EmpleadorID != nil && *user.EmpleadorID > 0 {
		program, err := s.repo.GetProgramForCompany(*user.EmpleadorID)
		if err != nil {
			return nil, err
		}
		if program.Usable() {
			return program, nil
		}
		if program != nil {
			log.Printf("[Induction] el programa %q de la empresa %d no es usable; se usa el por defecto", program.Name, *user.EmpleadorID)
		}
	}
	program, err := s.repo.GetDefaultProgram()
	if err != nil {
		return nil, err
	}
	if !program.Usable() {
		return nil, nil
	}
	return program, nil
}

// --- Invitaciones ------------------------------------------------------------

func (s *inductionService) InviteIfEnabled(user *models.User) (bool, error) {
	if user == nil {
		return false, errors.New("usuario inválido")
	}
	cfg, err := s.repo.GetConfig()
	if err != nil {
		return false, err
	}
	if !cfg.IsActive {
		// Inducción apagada: el profesional entra por el flujo normal.
		return false, nil
	}
	program, err := s.resolveProgram(user)
	if err != nil {
		return false, err
	}
	if program == nil {
		// Sin programa usable: igual que apagada.
		return false, nil
	}
	// Ingreso desde Obersuite: sin acceso hasta aprobar.
	return true, s.issueInvite(user, program, cfg.InviteTTLDays, true)
}

// issueInvite reemplaza la invitación viva del usuario por una nueva con el
// snapshot del programa, lo deja sin acceso y manda el correo.
func (s *inductionService) issueInvite(user *models.User, program *models.InductionProgram, ttlDays int, gatesAccess bool) error {
	token, err := generateInductionToken()
	if err != nil {
		return err
	}
	if ttlDays < 1 {
		ttlDays = 30
	}

	// Una sola capacitación EN CURSO por persona. Un ingreso (o re-ingreso)
	// reemplaza la que hubiera pendiente; una capacitación espera a que la
	// pendiente termine. Las terminadas se conservan como historial.
	if pending, err := s.repo.GetPendingInviteByUser(user.ID); err == nil && pending != nil {
		if !gatesAccess {
			return errors.New("este profesional ya tiene una capacitación en curso; espera a que la termine o reiníciala")
		}
		_ = s.repo.DeleteInvite(pending.ID)
	}

	invite := &models.InductionInvite{
		UserID:      user.ID,
		Token:       token,
		Status:      models.InductionPending,
		ProgramID:   &program.ID,
		ProgramName: program.Name,
		MaxAttempts: program.MaxAttempts,
		GatesAccess: gatesAccess,
		ExpiresAt:   time.Now().AddDate(0, 0, ttlDays),
	}
	blocks := make([]models.InductionInviteBlock, 0, len(program.Blocks))
	for i, b := range program.Blocks {
		blocks = append(blocks, models.InductionInviteBlock{
			BlockID:      b.ID,
			OrderIndex:   i,
			Name:         b.Name,
			TutorialID:   b.TutorialID,
			SurveyID:     b.SurveyID,
			PassingScore: b.EffectivePassingScore(program.DefaultPassingScore),
			Status:       models.InductionPending,
		})
	}
	if err := s.repo.CreateInvite(invite, blocks); err != nil {
		return err
	}

	if gatesAccess {
		// Ingreso: el profesional queda SIN acceso hasta aprobar.
		if err := s.userRepo.Update(user, map[string]interface{}{
			"onboarding_status": models.OnboardingPending,
		}); err != nil {
			return err
		}
	} else if s.notifSvc != nil {
		// Capacitación: la persona sigue dentro, así que además del correo se
		// le avisa por la campanita con el enlace, que es lo que va a ver.
		_ = s.notifSvc.CreateNotification(user.ID, "capacitacion",
			"Nueva capacitación: "+program.Name,
			fmt.Sprintf("Tienes %d %s por completar. Entra cuando quieras.", len(blocks), pluralBlocks(len(blocks))),
			map[string]interface{}{"link": "/induccion/" + token})
	}

	s.sendInviteEmail(user, token)
	return nil
}

func pluralBlocks(n int) string {
	if n == 1 {
		return "bloque"
	}
	return "bloques"
}

func (s *inductionService) Invite(userID uint, programID uint, gatesAccess bool) error {
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return errors.New("usuario no encontrado")
	}
	// Solo los profesionales pasan por inducción: invitar a una cuenta empresa,
	// a soporte o a un superadmin les cortaría el acceso sin motivo.
	if user.UserType != models.UserTypeProfessional {
		return errors.New("solo los profesionales pasan por la inducción")
	}
	cfg, err := s.repo.GetConfig()
	if err != nil {
		return err
	}
	if !cfg.IsActive {
		return errors.New("la inducción está apagada")
	}
	// Quien nunca ha tenido acceso (ingreso pendiente o bloqueado) sigue sin
	// tenerlo hasta aprobar: aquí no hay opción. A quien ya trabaja se le
	// respeta lo que pidió Soporte.
	if user.OnboardingStatus == models.OnboardingPending || user.OnboardingStatus == models.OnboardingBlocked {
		gatesAccess = true
	}
	// Bloquear el acceso de quien ya aprobó su ingreso le quitaría lo que se
	// ganó: para volver a hacer la inducción de ingreso se reinicia desde el
	// panel. Como capacitación (sin bloqueo) sí se le puede mandar.
	if gatesAccess && user.OnboardingStatus == models.OnboardingPassed {
		return errors.New("este profesional ya aprobó su inducción de ingreso; envíasela como capacitación (sin bloquear el acceso) o reiníciala")
	}

	var program *models.InductionProgram
	if programID > 0 {
		program, err = s.repo.GetProgram(programID)
		if err != nil {
			return errors.New("programa no encontrado")
		}
		if !program.Usable() {
			return errors.New("el programa elegido está apagado o no tiene bloques")
		}
	} else {
		program, err = s.resolveProgram(user)
		if err != nil {
			return err
		}
		if program == nil {
			return errors.New("no hay un programa de inducción usable para este profesional")
		}
	}
	return s.issueInvite(user, program, cfg.InviteTTLDays, gatesAccess)
}

func (s *inductionService) MyInduction(userID uint) (*MyInductionView, error) {
	invite, err := s.repo.GetPendingInviteByUser(userID)
	if err != nil || invite == nil {
		return nil, nil
	}
	if !invite.ExpiresAt.IsZero() && time.Now().After(invite.ExpiresAt) {
		return nil, nil
	}
	blocks, err := s.repo.ListInviteBlocks(invite.ID)
	if err != nil {
		return nil, err
	}
	return &MyInductionView{
		ProgramName:     invite.ProgramName,
		Token:           invite.Token,
		Status:          invite.Status,
		TotalBlocks:     len(blocks),
		CompletedBlocks: countCompleted(blocks),
		ExpiresAt:       invite.ExpiresAt,
		GatesAccess:     invite.GatesAccess,
	}, nil
}

// --- Landing pública ---------------------------------------------------------

// currentBlock es el primer bloque pendiente en orden, o nil si no queda.
func currentBlock(blocks []models.InductionInviteBlock) *models.InductionInviteBlock {
	for i := range blocks {
		if blocks[i].Status == models.InductionPending {
			return &blocks[i]
		}
	}
	return nil
}

func countCompleted(blocks []models.InductionInviteBlock) int {
	n := 0
	for _, b := range blocks {
		if b.Status == models.InductionPassed {
			n++
		}
	}
	return n
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
	blocks, err := s.repo.ListInviteBlocks(invite.ID)
	if err != nil {
		return nil, err
	}

	view := &LandingView{
		ProfessionalName: user.Name,
		Status:           invite.Status,
		ProgramName:      invite.ProgramName,
		GatesAccess:      invite.GatesAccess,
		MaxAttempts:      invite.MaxAttempts,
		TotalBlocks:      len(blocks),
		CompletedBlocks:  countCompleted(blocks),
		Blocks:           make([]LandingBlock, 0, len(blocks)),
		Badges:           []models.UserBadge{},
	}
	if s.badgeRepo != nil {
		if earned, err := s.badgeRepo.ListByUser(user.ID); err == nil && earned != nil {
			view.Badges = earned
		}
	}
	for _, b := range blocks {
		view.Blocks = append(view.Blocks, LandingBlock{
			BlockID:      b.BlockID,
			OrderIndex:   b.OrderIndex,
			Name:         b.Name,
			Status:       b.Status,
			Attempts:     b.Attempts,
			AttemptsLeft: b.AttemptsLeft(invite.MaxAttempts),
			BestScore:    b.BestScore,
			PassingScore: b.PassingScore,
			HasVideo:     b.TutorialID != nil && *b.TutorialID > 0,
		})
	}

	// Ya resuelta: no se devuelve bloque actual ni preguntas, pero sí el
	// certificado si se emitió.
	if invite.Status != models.InductionPending {
		if invite.Status == models.InductionPassed {
			view.Certificate = s.certificateFor(invite.ID)
		}
		return view, nil
	}
	current := currentBlock(blocks)
	if current == nil {
		return view, nil
	}

	cur := &LandingCurrentBlock{
		BlockID:      current.BlockID,
		OrderIndex:   current.OrderIndex,
		Name:         current.Name,
		PassingScore: current.PassingScore,
		AttemptsLeft: current.AttemptsLeft(invite.MaxAttempts),
		MaxAttempts:  invite.MaxAttempts,
		BestScore:    current.BestScore,
		Questions:    []LandingQuestion{},
	}
	// Video (Novedades/Tutoriales). Es opcional: el bloque puede ser solo
	// cuestionario.
	if current.TutorialID != nil && *current.TutorialID > 0 {
		if t, err := s.repo.GetTutorial(*current.TutorialID); err == nil {
			cur.VideoTitle = t.Title
			cur.VideoURL = t.GoogleDriveURL
			cur.VideoDurationMin = t.DurationMin
		}
	}
	survey, err := s.loadSurvey(current.SurveyID)
	if err != nil {
		return nil, err
	}
	cur.SurveyTitle = survey.Title
	cur.Description = survey.Description
	for _, q := range survey.Questions {
		cur.Questions = append(cur.Questions, LandingQuestion{
			ID:         q.ID,
			Text:       q.Text,
			Type:       string(q.Type),
			Options:    parseOptions(q.Options),
			IsRequired: q.IsRequired,
		})
	}
	view.Current = cur
	return view, nil
}

func (s *inductionService) Submit(token string, blockID uint, answers []SubmittedAnswer) (*SubmitResult, error) {
	invite, err := s.loadInvite(token)
	if err != nil {
		return nil, err
	}
	if invite.Status == models.InductionPassed {
		return nil, errors.New("ya completaste la inducción")
	}
	if invite.Status == models.InductionBlocked {
		return nil, errors.New("agotaste tus intentos. Nuestro equipo de soporte se pondrá en contacto contigo")
	}
	blocks, err := s.repo.ListInviteBlocks(invite.ID)
	if err != nil {
		return nil, err
	}
	current := currentBlock(blocks)
	if current == nil {
		return nil, errors.New("ya completaste la inducción")
	}
	// Solo se califica el bloque que toca: ni saltar hacia adelante ni volver
	// a enviar uno aprobado.
	if blockID != 0 && blockID != current.BlockID {
		return nil, errors.New("este no es el bloque que te corresponde ahora; recarga la página")
	}
	if current.AttemptsLeft(invite.MaxAttempts) <= 0 {
		return nil, errors.New("agotaste tus intentos. Nuestro equipo de soporte se pondrá en contacto contigo")
	}

	user, err := s.userRepo.GetByID(invite.UserID)
	if err != nil {
		return nil, errors.New("invitación inválida")
	}
	survey, err := s.loadSurvey(current.SurveyID)
	if err != nil {
		return nil, err
	}

	score := scoreAnswers(survey.Questions, answers)
	passed := score >= float64(current.PassingScore)

	// Deja evidencia del intento para Soporte.
	blob, _ := json.Marshal(answers)
	_ = s.repo.CreateAttempt(&models.InductionAttempt{
		InviteID:    invite.ID,
		UserID:      user.ID,
		BlockID:     current.BlockID,
		Score:       score,
		Passed:      passed,
		AnswersJSON: string(blob),
	})

	attempts := current.Attempts + 1
	best := current.BestScore
	if score > best {
		best = score
	}
	blockUpdates := map[string]interface{}{"attempts": attempts, "best_score": best}

	blockIndex := current.OrderIndex + 1
	result := &SubmitResult{
		BlockID:      current.BlockID,
		BlockIndex:   blockIndex,
		BlockName:    current.Name,
		Score:        score,
		PassingScore: current.PassingScore,
		Passed:       passed,
		AttemptsLeft: invite.MaxAttempts - attempts,
		BadgesEarned: []models.UserBadge{},
	}
	if result.AttemptsLeft < 0 {
		result.AttemptsLeft = 0
	}

	switch {
	case passed:
		now := time.Now()
		blockUpdates["status"] = models.InductionPassed
		blockUpdates["completed_at"] = now
		_ = s.repo.UpdateInviteBlock(invite.ID, current.BlockID, blockUpdates)
		current.Status = models.InductionPassed
		current.Attempts = attempts
		current.BestScore = best
		current.CompletedAt = &now
		result.BlockStatus = models.InductionPassed
		result.BadgesEarned = append(result.BadgesEarned, s.awardBlockBadge(user, invite, current, score, now)...)

		next := currentBlock(blocks)
		if next == nil {
			// Era el último: se aprueba el programa entero.
			_ = s.repo.UpdateInvite(invite, map[string]interface{}{
				"status":       models.InductionPassed,
				"completed_at": now,
			})
			if invite.GatesAccess {
				s.grantAccess(user)
				result.Message = "¡Aprobaste! Te enviamos un correo para que establezcas tu contraseña y entres a Obertrack."
			} else {
				result.Message = "¡Completaste la capacitación! Tus insignias ya están en tu perfil."
			}
			result.BadgesEarned = append(result.BadgesEarned, s.awardProgramBadges(user, invite, blocks, now)...)
			// Certificado, si el programa tiene plantilla. Best-effort: no
			// frena la aprobación.
			if s.certSvc != nil {
				invite.CompletedAt = &now
				if cert, err := s.certSvc.IssueForCompletion(user, invite); err != nil {
					log.Printf("[Induction] no se pudo emitir el certificado de %s: %v", user.Email, err)
				} else if cert != nil {
					result.Certificate = summarizeCertificate(cert)
				}
			}
			result.Status = models.InductionPassed
			result.Completed = true
		} else {
			result.Status = models.InductionPending
			nextID := next.BlockID
			result.NextBlockID = &nextID
			result.NextBlock = next.Name
			result.Message = fmt.Sprintf("¡Aprobaste el bloque %d de %d! Sigue con: %s.", blockIndex, len(blocks), next.Name)
		}

	case attempts >= invite.MaxAttempts:
		blockUpdates["status"] = models.InductionBlocked
		_ = s.repo.UpdateInviteBlock(invite.ID, current.BlockID, blockUpdates)
		_ = s.repo.UpdateInvite(invite, map[string]interface{}{"status": models.InductionBlocked})
		if invite.GatesAccess {
			_ = s.userRepo.Update(user, map[string]interface{}{
				"onboarding_status": models.OnboardingBlocked,
			})
		}
		s.alertSupport(user, invite, score, attempts, current, len(blocks))
		result.BlockStatus = models.InductionBlocked
		result.Status = models.InductionBlocked
		result.AttemptsLeft = 0
		if invite.GatesAccess {
			result.Message = "No alcanzaste el mínimo aprobatorio y agotaste tus intentos. Nuestro equipo se pondrá en contacto contigo."
		} else {
			result.Message = "No alcanzaste el mínimo aprobatorio y agotaste tus intentos. Tu acceso a Obertrack no cambia; nuestro equipo se pondrá en contacto contigo."
		}

	default:
		_ = s.repo.UpdateInviteBlock(invite.ID, current.BlockID, blockUpdates)
		result.BlockStatus = models.InductionPending
		result.Status = models.InductionPending
		result.Message = fmt.Sprintf("No alcanzaste el mínimo de %d%%. Puedes intentarlo de nuevo (te quedan %d intentos).",
			current.PassingScore, result.AttemptsLeft)
	}

	return result, nil
}

func (s *inductionService) Reset(userID uint) error {
	invite, err := s.repo.GetInviteByUser(userID)
	if err != nil {
		return errors.New("este profesional no tiene una inducción pendiente")
	}
	return s.resetInvite(invite)
}

func (s *inductionService) ResetInvite(inviteID uint) error {
	invite, err := s.repo.GetInviteByID(inviteID)
	if err != nil {
		return errors.New("capacitación no encontrada")
	}
	return s.resetInvite(invite)
}

// resetInvite reinicia una invitación concreta. Una aprobada no se reinicia
// (para repetir un programa se envía de nuevo), y si hay otra pendiente, esta
// no puede volver a pendiente: una sola en curso a la vez.
func (s *inductionService) resetInvite(invite *models.InductionInvite) error {
	if invite.Status == models.InductionPassed {
		return errors.New("esta capacitación ya está aprobada; para repetirla, envíala de nuevo")
	}
	if invite.Status != models.InductionPending {
		if pending, err := s.repo.GetPendingInviteByUser(invite.UserID); err == nil && pending != nil && pending.ID != invite.ID {
			return errors.New("este profesional ya tiene otra capacitación en curso; termina o reinicia esa primero")
		}
	}
	user, err := s.userRepo.GetByID(invite.UserID)
	if err != nil {
		return errors.New("usuario no encontrado")
	}

	// Enlace NUEVO, no el de antes. Dos motivos:
	//  1. Sin renovar la vigencia, reiniciar una invitación ya vencida reenviaba
	//     un enlace muerto — y quien lleva semanas parado es justo el caso
	//     típico de este botón.
	//  2. Rotar el token invalida el enlace viejo, que pudo quedar reenviado o
	//     en un correo compartido.
	token, err := generateInductionToken()
	if err != nil {
		return err
	}

	if err := s.repo.UpdateInvite(invite, map[string]interface{}{
		"status":       models.InductionPending,
		"completed_at": nil,
		"token":        token,
		"expires_at":   time.Now().AddDate(0, 0, s.inviteTTLDays()),
	}); err != nil {
		return err
	}
	// Se reinician TODOS los bloques, incluidos los aprobados: el reinicio es
	// "vuelve a hacer la inducción", no "repite donde fallaste". El snapshot
	// (contenido y mínimos) se conserva.
	if err := s.repo.ResetInviteBlocks(invite.ID); err != nil {
		return err
	}
	// El reinicio conserva el modo: una capacitación sin bloqueo sigue sin
	// bloquear; un ingreso vuelve a dejar a la persona sin acceso.
	if invite.GatesAccess {
		if err := s.userRepo.Update(user, map[string]interface{}{
			"onboarding_status": models.OnboardingPending,
		}); err != nil {
			return err
		}
	}
	s.sendInviteEmail(user, token)
	return nil
}

// inviteTTLDays es la vigencia configurada, con un valor de respaldo sensato:
// una configuración a medias no debe emitir un enlace ya vencido.
func (s *inductionService) inviteTTLDays() int {
	cfg, err := s.repo.GetConfig()
	if err != nil || cfg == nil || cfg.InviteTTLDays < 1 {
		return 30
	}
	return cfg.InviteTTLDays
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
	blocks, err := s.repo.ListInviteBlocks(invite.ID)
	if err != nil {
		return nil, err
	}
	attempts, _ := s.repo.ListAttempts(invite.ID)
	if attempts == nil {
		attempts = []models.InductionAttempt{}
	}
	nameOf := map[uint]string{}
	view := &InductionStatusView{
		UserID:          user.ID,
		Name:            user.Name,
		Email:           user.Email,
		Status:          invite.Status,
		ProgramName:     invite.ProgramName,
		GatesAccess:     invite.GatesAccess,
		MaxAttempts:     invite.MaxAttempts,
		TotalBlocks:     len(blocks),
		CompletedBlocks: countCompleted(blocks),
		ExpiresAt:       invite.ExpiresAt,
		Blocks:          make([]InductionBlockStatus, 0, len(blocks)),
	}
	for _, b := range blocks {
		nameOf[b.BlockID] = b.Name
		view.Attempts += b.Attempts
		view.Blocks = append(view.Blocks, InductionBlockStatus{
			BlockID:      b.BlockID,
			OrderIndex:   b.OrderIndex,
			Name:         b.Name,
			Status:       b.Status,
			Attempts:     b.Attempts,
			BestScore:    b.BestScore,
			PassingScore: b.PassingScore,
			CompletedAt:  b.CompletedAt,
		})
	}
	for i := range attempts {
		attempts[i].BlockName = nameOf[attempts[i].BlockID]
	}
	view.AttemptLog = attempts
	view.InviteID = invite.ID

	// Historial: todas las capacitaciones, con su avance.
	all, err := s.repo.ListInvitesByUser(userID)
	if err == nil {
		view.History = make([]InductionHistoryItem, 0, len(all))
		for _, inv := range all {
			item := InductionHistoryItem{
				ID:          inv.ID,
				ProgramName: inv.ProgramName,
				Status:      inv.Status,
				GatesAccess: inv.GatesAccess,
				CreatedAt:   inv.CreatedAt,
				CompletedAt: inv.CompletedAt,
				Current:     inv.ID == invite.ID,
			}
			if inv.Status == models.InductionPassed {
				item.Certificate = s.certificateFor(inv.ID)
			}
			if inv.ID == invite.ID {
				item.TotalBlocks, item.CompletedBlocks = len(blocks), countCompleted(blocks)
			} else if bs, err := s.repo.ListInviteBlocks(inv.ID); err == nil {
				item.TotalBlocks, item.CompletedBlocks = len(bs), countCompleted(bs)
			}
			view.History = append(view.History, item)
		}
	}
	if view.History == nil {
		view.History = []InductionHistoryItem{}
	}
	return view, nil
}

// --- Insignias ---------------------------------------------------------------

// award otorga una insignia y, si es nueva, avisa por la campanita. Best-effort:
// una insignia que no se pudo guardar no puede frenar una aprobación.
func (s *inductionService) award(user *models.User, badge models.UserBadge) *models.UserBadge {
	if s.badgeRepo == nil || user == nil {
		return nil
	}
	created, err := s.badgeRepo.Award(&badge)
	if err != nil {
		log.Printf("[Induction] no se pudo otorgar la insignia %s a %s: %v", badge.SourceKey, user.Email, err)
		return nil
	}
	if !created {
		return nil
	}
	if s.notifSvc != nil {
		_ = s.notifSvc.CreateNotification(user.ID, "insignia",
			"Nueva insignia: "+badge.Title, badge.Description,
			map[string]interface{}{"link": "/profile", "badge_id": badge.ID})
	}
	return &badge
}

// awardBlockBadge otorga la insignia del bloque recién aprobado. El aspecto se
// lee de la definición viva del bloque; si ya no existe, el de respaldo.
func (s *inductionService) awardBlockBadge(user *models.User, invite *models.InductionInvite, block *models.InductionInviteBlock, score float64, at time.Time) []models.UserBadge {
	icon, color, title := DefaultBlockBadgeIcon, DefaultBlockBadgeColor, block.Name
	if def, err := s.repo.GetBlock(block.BlockID); err == nil && def != nil {
		icon = normalizeBadgeIcon(def.BadgeIcon, icon)
		color = normalizeBadgeColor(def.BadgeColor, color)
		if strings.TrimSpace(def.BadgeTitle) != "" {
			title = def.BadgeTitle
		}
	}
	badge := models.UserBadge{
		UserID:      user.ID,
		Kind:        models.BadgeKindBlock,
		SourceKey:   blockBadgeKey(block.BlockID),
		Title:       title,
		Description: fmt.Sprintf("Aprobaste el bloque «%s» de tu inducción.", block.Name),
		Icon:        icon,
		Color:       color,
		Score:       score,
		ProgramName: invite.ProgramName,
		EarnedAt:    at,
	}
	if got := s.award(user, badge); got != nil {
		return []models.UserBadge{*got}
	}
	return nil
}

// awardProgramBadges otorga la insignia del programa completado y los méritos
// que correspondan por cómo se hizo.
func (s *inductionService) awardProgramBadges(user *models.User, invite *models.InductionInvite, blocks []models.InductionInviteBlock, at time.Time) []models.UserBadge {
	if invite.ProgramID == nil || *invite.ProgramID == 0 {
		return nil
	}
	programID := *invite.ProgramID
	icon, color, title := DefaultProgramBadgeIcon, DefaultProgramBadgeColor, invite.ProgramName
	if def, err := s.repo.GetProgram(programID); err == nil && def != nil {
		icon = normalizeBadgeIcon(def.BadgeIcon, icon)
		color = normalizeBadgeColor(def.BadgeColor, color)
		if strings.TrimSpace(def.BadgeTitle) != "" {
			title = def.BadgeTitle
		}
	}
	avg := 0.0
	for _, b := range blocks {
		avg += b.BestScore
	}
	if len(blocks) > 0 {
		avg /= float64(len(blocks))
	}
	var out []models.UserBadge
	if got := s.award(user, models.UserBadge{
		UserID:      user.ID,
		Kind:        models.BadgeKindProgram,
		SourceKey:   programBadgeKey(programID),
		Title:       title,
		Description: fmt.Sprintf("Completaste el programa de inducción «%s».", invite.ProgramName),
		Icon:        icon,
		Color:       color,
		Score:       avg,
		ProgramName: invite.ProgramName,
		EarnedAt:    at,
	}); got != nil {
		out = append(out, *got)
	}
	for _, merit := range earnedMerits(blocks) {
		def := meritCatalog[merit]
		if got := s.award(user, models.UserBadge{
			UserID:      user.ID,
			Kind:        models.BadgeKindMerit,
			SourceKey:   meritBadgeKey(merit, programID),
			Title:       def.Title,
			Description: def.describe(invite.ProgramName),
			Icon:        def.Icon,
			Color:       def.Color,
			Score:       avg,
			ProgramName: invite.ProgramName,
			EarnedAt:    at,
		}); got != nil {
			out = append(out, *got)
		}
	}
	return out
}

func (s *inductionService) ListBadges(userID uint) (*models.BadgeOverview, error) {
	overview := &models.BadgeOverview{Earned: []models.UserBadge{}, Pending: []models.PendingBadge{}}
	if s.badgeRepo != nil {
		earned, err := s.badgeRepo.ListByUser(userID)
		if err != nil {
			return nil, err
		}
		if earned != nil {
			overview.Earned = earned
		}
	}
	// Lo que falta: los bloques y el programa de la capacitación en curso.
	invite, err := s.repo.GetPendingInviteByUser(userID)
	if err != nil || invite == nil {
		return overview, nil
	}
	blocks, err := s.repo.ListInviteBlocks(invite.ID)
	if err != nil {
		return overview, nil
	}
	for _, b := range blocks {
		if b.Status == models.InductionPassed {
			continue
		}
		icon, color, title := DefaultBlockBadgeIcon, DefaultBlockBadgeColor, b.Name
		if def, err := s.repo.GetBlock(b.BlockID); err == nil && def != nil {
			icon = normalizeBadgeIcon(def.BadgeIcon, icon)
			color = normalizeBadgeColor(def.BadgeColor, color)
			if strings.TrimSpace(def.BadgeTitle) != "" {
				title = def.BadgeTitle
			}
		}
		overview.Pending = append(overview.Pending, models.PendingBadge{
			Kind: models.BadgeKindBlock, Title: title, Icon: icon, Color: color,
		})
	}
	if invite.ProgramID != nil && *invite.ProgramID > 0 {
		icon, color, title := DefaultProgramBadgeIcon, DefaultProgramBadgeColor, invite.ProgramName
		if def, err := s.repo.GetProgram(*invite.ProgramID); err == nil && def != nil {
			icon = normalizeBadgeIcon(def.BadgeIcon, icon)
			color = normalizeBadgeColor(def.BadgeColor, color)
			if strings.TrimSpace(def.BadgeTitle) != "" {
				title = def.BadgeTitle
			}
		}
		overview.Pending = append(overview.Pending, models.PendingBadge{
			Kind: models.BadgeKindProgram, Title: title, Icon: icon, Color: color,
		})
	}
	return overview, nil
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

func (s *inductionService) loadSurvey(surveyID uint) (*models.Survey, error) {
	if surveyID == 0 {
		return nil, errors.New("el bloque no tiene un cuestionario configurado")
	}
	survey, err := s.repo.GetSurveyWithQuestions(surveyID)
	if err != nil {
		return nil, errors.New("el bloque no tiene un cuestionario configurado")
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
	// Aprobar la capacitación es lo que cierra la incorporación: si llegó desde
	// Obersuite, su ticket de soporte ya no tiene nada pendiente. Best-effort:
	// no dejar el acceso a medias por un ticket.
	if s.ticketSvc != nil {
		if err := s.ticketSvc.CloseObersuiteHireAlert(user.ID); err != nil {
			log.Printf("[Induction] no se pudo cerrar el ticket de incorporación de %s: %v", user.Email, err)
		}
	}
}

// alertSupport abre una alerta interna en el módulo de Soporte para que
// contacten al profesional que no aprobó.
func (s *inductionService) alertSupport(user *models.User, invite *models.InductionInvite, score float64, attempts int, block *models.InductionInviteBlock, totalBlocks int) {
	if s.ticketSvc == nil {
		return
	}
	companyName := ""
	if user.EmpleadorID != nil {
		if company, err := s.userRepo.GetByID(*user.EmpleadorID); err == nil {
			companyName = company.CompanyDisplayName()
		}
	}
	in := InductionAlertInput{
		ProfessionalID:    user.ID,
		ProfessionalName:  user.Name,
		ProfessionalEmail: user.Email,
		ProfessionalPhone: user.PhoneNumber,
		CompanyName:       companyName,
		Score:             score,
		Attempts:          attempts,
		BlockCount:        totalBlocks,
		GatesAccess:       invite != nil && invite.GatesAccess,
		ProgramName:       invite.ProgramName,
	}
	if block != nil {
		in.PassingScore = block.PassingScore
		in.BlockName = block.Name
		in.BlockIndex = block.OrderIndex + 1
	}
	if err := s.ticketSvc.CreateInductionFailureAlert(in); err != nil {
		log.Printf("[Induction] no se pudo abrir la alerta de soporte para %s: %v", user.Email, err)
	}
}

func (s *inductionService) sendInviteEmail(user *models.User, token string) {
	if s.brevoSvc == nil {
		return
	}
	link := s.baseURL() + "/induccion/" + token
	subject := "Bienvenido a Obertrack — completa tu inducción"
	html := BuildInductionInviteHTML(user.Name, link)

	go func() {
		if err := s.brevoSvc.SendEmailKind(EmailKindInductionInvite, user.Email, user.Name, subject, html); err != nil {
			if !errors.Is(err, ErrEmailKindDisabled) {
				log.Printf("[Induction] no se pudo enviar la invitación a %s: %v", user.Email, err)
			}
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
