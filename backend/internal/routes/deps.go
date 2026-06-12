package routes

import (
	"os"

	"github.com/obertrack/backend/internal/config"
	"github.com/obertrack/backend/internal/handlers"
	"github.com/obertrack/backend/internal/middleware"
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

	// wahaSvc is needed by the /tickets/waha/status inline route.
	wahaSvc *service.WahaService
	// rbacSvc is needed by the per-module RequirePermission route middleware.
	rbacSvc service.RBACService
	// auditSvc is attached as a global middleware in RegisterRoutes.
	auditSvc service.AuditService
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
	ticketRepo := repository.NewTicketRepository(db)
	auditRepo := repository.NewAuditRepository(db)

	// Integrations
	brevoSvc := service.NewBrevoService()
	wahaSvc := service.NewWahaService()
	zohoSvc := service.NewZohoService()

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
	auditSvc := service.NewAuditService(auditRepo)

	// WebSocket hubs
	chatHub := websocket.NewChatHub(func(msg websocket.ChatWSMessage) {})
	channelHub := websocket.NewChannelHub(func(msg websocket.ChannelWSMessage) {})
	channelHub.MemberResolver = func(channelID uint) map[uint]bool {
		members, err := channelRepo.GetMembers(channelID)
		if err != nil {
			return map[uint]bool{}
		}
		result := make(map[uint]bool, len(members))
		for _, m := range members {
			result[m.ID] = true
		}
		return result
	}
	go chatHub.Run()
	go channelHub.Run()

	return &deps{
		cfg: cfg,
		// Session-revocation lookup used by the auth middleware (audit A-04).
		tvGetter: func(userID uint) (int, error) { return authSvc.GetTokenVersion(userID) },

		auth:         handlers.NewAuthHandler(authSvc, auditSvc, rbacSvc),
		user:         handlers.NewUserHandler(userSvc),
		admin:        handlers.NewAdminHandler(adminSvc, rbacSvc),
		board:        handlers.NewBoardHandler(boardSvc),
		task:         handlers.NewTaskHandler(taskSvc),
		workHour:     handlers.NewWorkHourHandler(workHourSvc),
		chat:         handlers.NewChatHandler(chatSvc, chatHub),
		channel:      handlers.NewChannelHandler(channelSvc, channelHub),
		upload:       handlers.NewUploadHandler(uploadSvc, os.Getenv("UPLOAD_PATH")),
		notification: handlers.NewNotificationHandler(notifSvc),
		email:        handlers.NewEmailHandler(emailRepo, brevoSvc),
		survey:       handlers.NewSurveyHandler(surveyRepo, userRepo, brevoSvc, notifSvc),
		metrics:      handlers.NewMetricsHandler(metricsRepo),
		tutorial:     handlers.NewTutorialHandler(tutorialSvc),
		rbac:         handlers.NewRBACHandler(rbacSvc),
		ticket:       handlers.NewTicketHandler(db, zohoSvc, ticketSvc),
		whatsapp:     handlers.NewWhatsAppHandler(db, zohoSvc),
		waha:         handlers.NewWahaHandler(ticketSvc),
		brevoInbound: handlers.NewBrevoInboundHandler(ticketSvc),
		audit:        handlers.NewAuditHandler(auditSvc),

		wahaSvc:  wahaSvc,
		rbacSvc:  rbacSvc,
		auditSvc: auditSvc,
	}
}
