package routes

import (
	"os"

	"github.com/obertrack/backend/internal/config"
	"github.com/obertrack/backend/internal/handlers"
	"github.com/obertrack/backend/internal/middleware"
	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
	"github.com/obertrack/backend/internal/service"
	"github.com/obertrack/backend/internal/websocket"
	"gorm.io/gorm"
)

// deps is the dependency-injection container: it holds every constructed handler
// (plus the few shared services that routes touch directly) so that route
// registration stays declarative. All wiring lives in buildDeps.
type deps struct {
	cfg      *config.Config
	tvGetter middleware.TokenVersionGetter

	auth         *handlers.AuthHandler
	user         *handlers.UserHandler
	admin        *handlers.AdminHandler
	board        *handlers.BoardHandler
	task         *handlers.TaskHandler
	workHour     *handlers.WorkHourHandler
	chat         *handlers.ChatHandler
	channel      *handlers.ChannelHandler
	upload       *handlers.UploadHandler
	notification *handlers.NotificationHandler
	email        *handlers.EmailHandler
	survey       *handlers.SurveyHandler
	metrics      *handlers.MetricsHandler
	tutorial     *handlers.TutorialHandler
	rbac         *handlers.RBACHandler
	ticket       *handlers.TicketHandler
	whatsapp     *handlers.WhatsAppHandler
	waha         *handlers.WahaHandler
	brevoInbound *handlers.BrevoInboundHandler
	audit        *handlers.AuditHandler
	audience     *handlers.AudienceHandler

	// wahaSvc is needed by the /tickets/waha/status inline route.
	wahaSvc *service.WahaService
	// rbacSvc is needed by the per-module RequirePermission route middleware.
	rbacSvc service.RBACService
	// auditSvc is attached as a global middleware in RegisterRoutes.
	auditSvc service.AuditService
	// employmentSvc is needed by the expediente-ownership route middleware.
	employmentSvc service.EmploymentService
}

// buildDeps constructs the full repository → service → handler graph once.
func buildDeps(db *gorm.DB, cfg *config.Config) *deps {
	// Repositories
	userRepo := repository.NewUserRepository(db)
	chatRepo := repository.NewChatRepository(db)
	notifRepo := repository.NewNotificationRepository(db)
	channelRepo := repository.NewChannelRepository(db)
	workHourRepo := repository.NewWorkHourRepository(db)
	emailRepo := repository.NewEmailRepository(db)
	surveyRepo := repository.NewSurveyRepository(db)
	metricsRepo := repository.NewMetricsRepository(db)
	boardRepo := repository.NewBoardRepository(db)
	taskRepo := repository.NewTaskRepository(db)
	adminRepo := repository.NewAdminRepository(db)
	tutorialRepo := repository.NewTutorialRepository(db)
	rbacRepo := repository.NewRBACRepository(db)
	employmentRepo := repository.NewEmploymentRepository(db)
	ticketRepo := repository.NewTicketRepository(db)
	auditRepo := repository.NewAuditRepository(db)
	audienceRepo := repository.NewAudienceRepository(db)

	// Integrations
	brevoSvc := service.NewBrevoService()
	wahaSvc := service.NewWahaService()
	zohoSvc := service.NewZohoService()
	slackSvc := service.NewSlackService()

	// Services
	userSvc := service.NewUserService(userRepo)
	notifSvc := service.NewNotificationService(notifRepo)
	chatSvc := service.NewChatService(chatRepo)
	channelSvc := service.NewChannelService(channelRepo, userRepo, notifSvc)
	ticketSvc := service.NewTicketService(ticketRepo, userRepo, notifSvc, wahaSvc, brevoSvc)
	authSvc := service.NewAuthService(userRepo, cfg.JWTSecret, brevoSvc)
	workHourSvc := service.NewWorkHourService(workHourRepo, userRepo, notifSvc, brevoSvc, ticketSvc)
	uploadSvc := service.NewUploadService(os.Getenv("UPLOAD_PATH"))
	taskSvc := service.NewTaskService(taskRepo, userRepo, boardRepo, notifSvc)
	adminSvc := service.NewAdminService(adminRepo, userRepo, taskRepo, workHourRepo)
	boardSvc := service.NewBoardService(boardRepo, userRepo)
	tutorialSvc := service.NewTutorialService(tutorialRepo)
	rbacSvc := service.NewRBACService(rbacRepo, userRepo)
	employmentSvc := service.NewEmploymentService(employmentRepo, userRepo, workHourRepo, notifSvc)
	auditSvc := service.NewAuditService(auditRepo)

	// WebSocket hubs
	chatHub := websocket.NewChatHub(func(msg websocket.ChatWSMessage) {})
	channelHub := websocket.NewChannelHub()
	// Membership is resolved on every broadcast and every typing frame, so cache
	// it with a short TTL instead of hitting the DB (JOIN users) each time.
	channelMembers := newMemberCache(channelRepo)
	channelHub.MemberResolver = channelMembers.Members
	go chatHub.Run()
	go channelHub.Run()

	// Difusor de mensajes de SISTEMA de soporte: los mensajes de soporte
	// (🛟 tomó / asignó / ✅ resuelto) se generan dentro del servicio y no pasan
	// por el handler HTTP SendMessage (que es quien difunde los mensajes normales
	// de usuario), por lo que sin esto no llegaban en vivo. El callback vive en
	// routes (no en service) para no acoplar service→websocket y evitar el ciclo
	// de imports: construye aquí el ChannelWSMessage con el MISMO formato que usa
	// el handler ("message" + Data: *models.ChannelMessage) para que el cliente
	// los parsee igual que cualquier otro mensaje.
	channelSvc.SetBroadcaster(func(channelID uint, msg *models.ChannelMessage) {
		channelHub.Broadcast(websocket.ChannelWSMessage{
			Type:      "message",
			ChannelID: channelID,
			Data:      msg,
		})
	})

	// Invalidación del caché de miembros tras cada mutación de membresía. El
	// caché vive en routes (newMemberCache) y alimenta al MemberResolver del hub;
	// el channelService lo invalida mediante este callback inyectado (mismo patrón
	// que SetBroadcaster) para no acoplar service→routes. Sin esto, un miembro
	// recién añadido no recibía broadcasts en vivo —y uno removido seguía
	// recibiéndolos— hasta agotarse el TTL de 30s.
	channelSvc.SetMembershipChangeHandler(channelMembers.Invalidate)

	// Watcher diario: alerta al equipo CS (interno + email + Slack) sobre
	// profesionales con 2+ días sin registrar horas.
	service.NewInactivityWatcher(adminRepo, userRepo, notifSvc, brevoSvc, slackSvc).Start()

	// Watcher diario: alerta a la empresa sobre documentos del expediente que
	// están por vencer (contratos, certificados...).
	service.NewDocumentExpiryWatcher(employmentRepo, userRepo, notifSvc).Start()

	return &deps{
		cfg: cfg,
		// Session-revocation lookup used by the auth middleware (audit A-04).
		tvGetter: func(userID uint) (int, error) { return authSvc.GetTokenVersion(userID) },

		auth:         handlers.NewAuthHandler(authSvc, auditSvc, rbacSvc, employmentSvc),
		user:         handlers.NewUserHandler(userSvc),
		admin:        handlers.NewAdminHandler(adminSvc, rbacSvc, employmentSvc),
		board:        handlers.NewBoardHandler(boardSvc),
		task:         handlers.NewTaskHandler(taskSvc),
		workHour:     handlers.NewWorkHourHandler(workHourSvc),
		chat:         handlers.NewChatHandler(chatSvc, chatHub),
		channel:      handlers.NewChannelHandler(channelSvc, channelHub),
		upload:       handlers.NewUploadHandler(uploadSvc, os.Getenv("UPLOAD_PATH"), employmentSvc),
		notification: handlers.NewNotificationHandler(notifSvc),
		email:        handlers.NewEmailHandler(emailRepo, brevoSvc),
		survey:       handlers.NewSurveyHandler(surveyRepo, userRepo, brevoSvc, notifSvc),
		metrics:      handlers.NewMetricsHandler(metricsRepo),
		tutorial:     handlers.NewTutorialHandler(tutorialSvc),
		rbac:         handlers.NewRBACHandler(rbacSvc),
		ticket:       handlers.NewTicketHandler(db, zohoSvc, ticketSvc, channelSvc),
		whatsapp:     handlers.NewWhatsAppHandler(db, zohoSvc),
		waha:         handlers.NewWahaHandler(ticketSvc),
		brevoInbound: handlers.NewBrevoInboundHandler(ticketSvc),
		audit:        handlers.NewAuditHandler(auditSvc),
		audience:     handlers.NewAudienceHandler(audienceRepo),

		wahaSvc:       wahaSvc,
		rbacSvc:       rbacSvc,
		auditSvc:      auditSvc,
		employmentSvc: employmentSvc,
	}
}
