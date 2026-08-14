package routes

import (
	"os"
	"time"

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

	auth          *handlers.AuthHandler
	user          *handlers.UserHandler
	admin         *handlers.AdminHandler
	board         *handlers.BoardHandler
	task          *handlers.TaskHandler
	workHour      *handlers.WorkHourHandler
	chat          *handlers.ChatHandler
	channel       *handlers.ChannelHandler
	upload        *handlers.UploadHandler
	companyThread *handlers.CompanyThreadHandler
	notification  *handlers.NotificationHandler
	email         *handlers.EmailHandler
	survey        *handlers.SurveyHandler
	metrics       *handlers.MetricsHandler
	tutorial      *handlers.TutorialHandler
	rbac          *handlers.RBACHandler
	ticket        *handlers.TicketHandler
	whatsapp      *handlers.WhatsAppHandler
	waha          *handlers.WahaHandler
	brevoInbound  *handlers.BrevoInboundHandler
	audit         *handlers.AuditHandler
	audience      *handlers.AudienceHandler
	incident      *handlers.IncidentHandler
	wallet        *handlers.WalletHandler
	emergencyTpl  *handlers.EmergencyTemplateHandler
	profileChange *handlers.ProfileChangeHandler
	trash         *handlers.TrashHandler
	reportSched   *handlers.ReportScheduleHandler
	emailSettings *handlers.EmailSettingsHandler
	onboarding    *handlers.OnboardingHandler
	induction     *handlers.InductionHandler
	emailPreview  *handlers.EmailPreviewHandler
	googleCal     *handlers.GoogleCalendarHandler
	meeting       *handlers.MeetingHandler

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
	// Feature flag Fase 2: conmuta las lecturas de manager a la tabla N-a-N
	// (employment_managers). Se fija una sola vez antes de construir services.
	service.SetMultiManagerReads(cfg.MultiManagerReads)
	service.SetSupervisorScope(cfg.SupervisorScope)

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
	incidentRepo := repository.NewIncidentRepository(db)
	emergencyTplRepo := repository.NewEmergencyTemplateRepository(db)
	profileChangeRepo := repository.NewProfileChangeRequestRepository(db)
	inductionRepo := repository.NewInductionRepository(db)
	googleCalRepo := repository.NewGoogleCalendarRepository(db)

	// Integrations
	brevoSvc := service.NewBrevoService()
	// Interruptores de correo por tipo (Configuración → Correos). Se cablea
	// dentro de Brevo para que TODO envío hecho con SendEmailKind los respete,
	// sin que cada emisor tenga que consultarlos.
	emailSettingsSvc := service.NewEmailSettingsService(repository.NewEmailSettingRepository(db), brevoSvc)
	brevoSvc.SetKindGate(emailSettingsSvc.Enabled)
	wahaSvc := service.NewWahaService()
	zohoSvc := service.NewZohoService()
	slackSvc := service.NewSlackService()

	supportNtfy := service.NewSupportNotifier(brevoSvc, userRepo, cfg.SupportEmail)

	// Services
	userSvc := service.NewUserService(userRepo, employmentRepo)
	// Web Push del navegador: las campanitas también llegan con la pestaña
	// cerrada, pero SOLO cuando el usuario no tiene la app abierta.
	webPushSvc := service.NewWebPushService(repository.NewPushRepository(db))
	notifSvc := service.NewNotificationService(notifRepo, webPushSvc)
	chatSvc := service.NewChatService(chatRepo)
	channelSvc := service.NewChannelService(channelRepo, userRepo, notifSvc)
	channelSvc.SetSupportNotifier(supportNtfy)
	// Bandeja de salida de WhatsApp: el envío a WAHA ocurre en un worker, no en el
	// request, para que un reinicio o una caída de WAHA no pierda la respuesta.
	whatsappOutbox := service.NewWhatsAppOutbox(ticketRepo, wahaSvc)
	whatsappOutbox.Start()

	ticketSvc := service.NewTicketService(ticketRepo, userRepo, notifSvc, wahaSvc, brevoSvc, supportNtfy, whatsappOutbox)
	authSvc := service.NewAuthService(userRepo, cfg.JWTSecret, brevoSvc)
	workHourSvc := service.NewWorkHourService(workHourRepo, userRepo, notifSvc, brevoSvc, ticketSvc, employmentRepo)
	uploadSvc := service.NewUploadService(os.Getenv("UPLOAD_PATH"))
	// Hilo del expediente de empresa (comentarios y adjuntos por entrada).
	companyThreadSvc := service.NewCompanyThreadService(repository.NewCompanyThreadRepository(db))
	taskSvc := service.NewTaskService(taskRepo, userRepo, boardRepo, notifSvc)
	// Al asignar/cambiar/completar una tarea, además de la campanita se publica un
	// DM del bot "Obertrack" en el chat interno (como Slack). channelSvc ya está
	// construido arriba, así que se apunta directamente su PostSystemDM.
	taskSvc.SetSystemDM(channelSvc.PostSystemDM)
	// Google Calendar. Fase 1: vinculación de la cuenta personal (servicio inerte
	// si el flag está apagado). Fase 2: worker que sincroniza tareas → eventos;
	// se construye aquí para poder cablear su enganche al taskService.
	googleCalSvc := service.NewGoogleCalendarService(googleCalRepo, cfg)
	calendarSyncRepo := repository.NewCalendarSyncRepository(db)
	calendarSyncSvc := service.NewCalendarSyncService(calendarSyncRepo, googleCalRepo, taskRepo, googleCalSvc)
	// Enganche de Google Calendar: al crear/editar/completar/borrar una tarea se
	// encola la sincronización (el worker la aplica en segundo plano).
	taskSvc.SetCalendarSync(calendarSyncSvc.OnTaskChanged, calendarSyncSvc.OnTaskDeleted)
	// Enganche inverso: al desvincular la cuenta hay que soltar los enlaces
	// tarea↔evento, que sin credencial detrás ya no se pueden mantener.
	googleCalSvc.SetDisconnectHook(calendarSyncSvc.OnAccountDisconnected)
	// Sesiones: reuniones con Google Meet sobre el calendario del organizador.
	// A diferencia de la sincronización de tareas, habla con Google DENTRO del
	// request (quien convoca necesita el enlace en el momento).
	meetingSvc := service.NewMeetingService(
		repository.NewMeetingRepository(db), userRepo, googleCalRepo, googleCalSvc, notifSvc,
	)
	meetingSvc.SetSystemDM(channelSvc.PostSystemDM)
	adminSvc := service.NewAdminService(adminRepo, userRepo, taskRepo, workHourRepo, employmentRepo, brevoSvc, authSvc)
	boardSvc := service.NewBoardService(boardRepo, userRepo, notifSvc)
	tutorialSvc := service.NewTutorialService(tutorialRepo)
	rbacSvc := service.NewRBACService(rbacRepo, userRepo)
	employmentSvc := service.NewEmploymentService(employmentRepo, userRepo, workHourRepo, notifSvc)
	employmentSvc.SetChannelCleaner(channelSvc.RemoveUserFromCompanyChannels)
	auditSvc := service.NewAuditService(auditRepo)
	incidentSvc := service.NewIncidentService(incidentRepo, userRepo, brevoSvc)
	ontopSvc := service.NewOntopService(cfg)
	walletSvc := service.NewWalletService(ontopSvc)
	emergencyTplSvc := service.NewEmergencyTemplateService(emergencyTplRepo)
	profileChangeSvc := service.NewProfileChangeService(profileChangeRepo, userRepo, channelRepo, channelSvc, notifSvc)
	// Inducción del profesional recién contratado: video (Novedades) +
	// cuestionario calificado (Encuestas) en una landing pública que decide su
	// acceso. Si no está configurada, no interfiere con el alta.
	inductionSvc := service.NewInductionService(inductionRepo, userRepo, brevoSvc, authSvc, ticketSvc, cfg.FrontendURL)
	// Puente Obersuite (captación) → Obertrack (gestión): materializa la
	// contratación de un candidato como profesional + empleo activo.
	onboardingSvc := service.NewOnboardingService(userRepo, employmentRepo, employmentSvc, uploadSvc, authSvc, inductionSvc, ticketSvc)

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

	// Watcher del chat: correo de respaldo a quien acumula mensajes sin leer y
	// no se conecta — el mensaje interno deja de depender de que se le ocurra
	// entrar a la plataforma.
	service.NewChatDigestWatcher(channelRepo, brevoSvc).Start()

	// Watcher del supervisor: si en su árbol quedan jornadas sin aprobar hace
	// demasiado, se le avisa a él. El aviso normal sigue yendo solo al manager
	// directo; esto es la red por si no lo resuelve. No-op con el flag apagado.
	service.NewSupervisorEscalationWatcher(userRepo, employmentRepo, workHourRepo, notifSvc).Start()


	// Worker de reportes automáticos. A diferencia de los otros, se conserva la
	// instancia: el panel de configuración la usa para "Enviar ahora".
	reportScheduleRepo := repository.NewReportScheduleRepository(db)
	// Además de enviar el reporte, en la frecuencia MENSUAL este watcher cierra
	// el mes: aprueba las jornadas pendientes de cada empresa antes del envío
	// (tarjeta "Aprobación de horas automática al final del mes"). Todo se
	// gobierna desde /admin/settings (toggle, día, hora, zona).
	reportWatcher := service.NewReportMailWatcher(reportScheduleRepo, userRepo, workHourSvc, workHourRepo, notifSvc)
	reportWatcher.Start()

	// Worker de sincronización con Google Calendar (Fase 2): procesa la cola de
	// jobs que dejan las mutaciones de tareas. No-op si la integración está
	// apagada.
	calendarSyncSvc.Start()

	// Watcher de WhatsApp: cuando la sesión WAHA queda conectada (WORKING),
	// importa las conversaciones existentes del número como tickets (una vez por
	// conexión; la re-importación es idempotente por el índice de external_id).
	service.NewChatImportWatcher(wahaSvc, ticketSvc).Start(60 * time.Second)

	return &deps{
		cfg: cfg,
		// Session-revocation lookup used by the auth middleware (audit A-04).
		tvGetter: func(userID uint) (int, error) { return authSvc.GetTokenVersion(userID) },

		auth:          handlers.NewAuthHandler(authSvc, auditSvc, rbacSvc, employmentSvc),
		user:          handlers.NewUserHandler(userSvc),
		admin:         handlers.NewAdminHandler(adminSvc, rbacSvc, employmentSvc, companyThreadSvc),
		board:         handlers.NewBoardHandler(boardSvc),
		task:          handlers.NewTaskHandler(taskSvc),
		workHour:      handlers.NewWorkHourHandler(workHourSvc),
		chat:          handlers.NewChatHandler(chatSvc, chatHub),
		channel:       handlers.NewChannelHandler(channelSvc, channelHub),
		upload:        handlers.NewUploadHandler(uploadSvc, os.Getenv("UPLOAD_PATH"), employmentSvc),
		companyThread: handlers.NewCompanyThreadHandler(companyThreadSvc, os.Getenv("UPLOAD_PATH")),
		notification:  handlers.NewNotificationHandler(notifSvc, webPushSvc),
		email:         handlers.NewEmailHandler(emailRepo, brevoSvc),
		survey:        handlers.NewSurveyHandler(surveyRepo, userRepo, brevoSvc, notifSvc),
		metrics:       handlers.NewMetricsHandler(metricsRepo),
		tutorial:      handlers.NewTutorialHandler(tutorialSvc),
		rbac:          handlers.NewRBACHandler(rbacSvc),
		ticket:        handlers.NewTicketHandler(db, zohoSvc, ticketSvc, channelSvc),
		whatsapp:      handlers.NewWhatsAppHandler(db, zohoSvc),
		waha:          handlers.NewWahaHandler(ticketSvc, wahaSvc),
		brevoInbound:  handlers.NewBrevoInboundHandler(ticketSvc),
		audit:         handlers.NewAuditHandler(auditSvc),
		audience:      handlers.NewAudienceHandler(audienceRepo),
		incident:      handlers.NewIncidentHandler(incidentSvc),
		wallet:        handlers.NewWalletHandler(walletSvc),
		emergencyTpl:  handlers.NewEmergencyTemplateHandler(emergencyTplSvc),
		profileChange: handlers.NewProfileChangeHandler(profileChangeSvc),
		trash:         handlers.NewTrashHandler(service.NewTrashService(db)),
		reportSched:   handlers.NewReportScheduleHandler(reportScheduleRepo, reportWatcher),
		emailSettings: handlers.NewEmailSettingsHandler(emailSettingsSvc),
		onboarding:    handlers.NewOnboardingHandler(onboardingSvc),
		induction:     handlers.NewInductionHandler(inductionSvc),
		emailPreview:  handlers.NewEmailPreviewHandler(),
		googleCal:     handlers.NewGoogleCalendarHandler(googleCalSvc, cfg.FrontendURL),
		meeting:       handlers.NewMeetingHandler(meetingSvc),

		wahaSvc:       wahaSvc,
		rbacSvc:       rbacSvc,
		auditSvc:      auditSvc,
		employmentSvc: employmentSvc,
	}
}
