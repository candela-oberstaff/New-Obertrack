package service

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
)

const (
	// Umbral de alerta: profesionales con 2+ días sin registrar horas.
	inactivityAlertDays = 2
	// No repetir la alerta del mismo profesional durante esta ventana.
	inactivityAlertCooldown = 7 * 24 * time.Hour
	// Cadencia del chequeo.
	inactivityCheckInterval = 24 * time.Hour
	// Espera tras el arranque antes del primer chequeo (deja migrar/estabilizar).
	inactivityFirstRunDelay = 2 * time.Minute
)

// InactivityWatcher revisa a diario los profesionales con 2+ días sin
// registrar horas y alerta al equipo de customer success (managers y
// analistas) por notificación interna, email y Slack.
type InactivityWatcher struct {
	adminRepo repository.AdminRepository
	userRepo  repository.UserRepository
	notifSvc  NotificationService
	brevoSvc  *BrevoService
	slackSvc  *SlackService
}

func NewInactivityWatcher(adminRepo repository.AdminRepository, userRepo repository.UserRepository, notifSvc NotificationService, brevoSvc *BrevoService, slackSvc *SlackService) *InactivityWatcher {
	return &InactivityWatcher{
		adminRepo: adminRepo,
		userRepo:  userRepo,
		notifSvc:  notifSvc,
		brevoSvc:  brevoSvc,
		slackSvc:  slackSvc,
	}
}

// Start lanza el chequeo periódico en segundo plano.
func (w *InactivityWatcher) Start() {
	go func() {
		time.Sleep(inactivityFirstRunDelay)
		for {
			if err := w.RunOnce(); err != nil {
				log.Printf("[inactivity-watcher] chequeo fallido: %v", err)
			}
			time.Sleep(inactivityCheckInterval)
		}
	}()
}

// RunOnce ejecuta un chequeo: detecta rojos nuevos (sin alerta reciente),
// notifica al equipo CS y los marca como alertados.
func (w *InactivityWatcher) RunOnce() error {
	inactive, err := w.adminRepo.GetInactiveUsersList(inactivityAlertDays)
	if err != nil {
		return fmt.Errorf("listando inactivos: %w", err)
	}

	var red []repository.InactiveUser
	for _, u := range inactive {
		if u.DaysInactive >= inactivityAlertDays {
			red = append(red, u)
		}
	}
	if len(red) == 0 {
		return nil
	}

	alertedIDs, err := w.adminRepo.GetRecentlyAlertedUserIDs(time.Now().Add(-inactivityAlertCooldown))
	if err != nil {
		return fmt.Errorf("leyendo alertas recientes: %w", err)
	}
	recentlyAlerted := make(map[uint]bool, len(alertedIDs))
	for _, id := range alertedIDs {
		recentlyAlerted[id] = true
	}

	var fresh []repository.InactiveUser
	for _, u := range red {
		if !recentlyAlerted[u.ID] {
			fresh = append(fresh, u)
		}
	}
	if len(fresh) == 0 {
		return nil
	}

	w.notifySupportTeam(fresh)

	now := time.Now()
	alerts := make([]models.InactivityAlert, 0, len(fresh))
	for _, u := range fresh {
		alerts = append(alerts, models.InactivityAlert{
			UserID:        u.ID,
			DaysInactive:  u.DaysInactive,
			LastAlertedAt: now,
		})
	}
	if err := w.adminRepo.MarkUsersAlerted(alerts); err != nil {
		return fmt.Errorf("marcando alertados: %w", err)
	}

	log.Printf("[inactivity-watcher] alertados %d profesionales con %d+ días de inactividad", len(fresh), inactivityAlertDays)
	return nil
}

// notifySupportTeam alerta de forma DIRIGIDA: cada profesional inactivo se
// notifica al CS vinculado a su empresa; los CS managers reciben siempre el
// panorama completo. Si una empresa no tiene CS asignado, esos casos van a
// todo el equipo. Slack recibe un único resumen global.
func (w *InactivityWatcher) notifySupportTeam(users []repository.InactiveUser) {
	csUsers, _, err := w.userRepo.GetAll(string(models.UserTypeCustomerSuccess), "", "", 0, 0, 1000)
	if err != nil {
		log.Printf("[inactivity-watcher] no se pudo listar al equipo CS: %v", err)
	}

	line := func(u repository.InactiveUser) string {
		return fmt.Sprintf("• %s (%s) — %d días hábiles sin registrar horas", u.Name, u.Company, u.DaysInactive)
	}

	// Asignación de líneas por destinatario (un CS puede cubrir varias empresas).
	linesByRecipient := map[uint][]string{}
	recipientByID := map[uint]models.User{}
	for _, cs := range csUsers {
		if cs.IsActive {
			recipientByID[cs.ID] = cs
		}
	}

	for _, u := range users {
		assignedToSomeone := false
		for _, cs := range recipientByID {
			isAssignedAnalyst := cs.EmpleadorID != nil && *cs.EmpleadorID == u.TenantID && u.TenantID != 0
			if cs.IsManager || isAssignedAnalyst {
				linesByRecipient[cs.ID] = append(linesByRecipient[cs.ID], line(u))
				if isAssignedAnalyst {
					assignedToSomeone = true
				}
			}
		}
		if !assignedToSomeone {
			// Empresa sin CS asignado: el caso va a todo el equipo.
			for _, cs := range recipientByID {
				if !cs.IsManager {
					linesByRecipient[cs.ID] = append(linesByRecipient[cs.ID], line(u))
				}
			}
		}
	}

	// Best-effort por destinatario: un canal caído no detiene los demás.
	for csID, lines := range linesByRecipient {
		cs := recipientByID[csID]
		// Dedup por si un caso entró por más de una vía.
		seen := map[string]bool{}
		unique := lines[:0]
		for _, l := range lines {
			if !seen[l] {
				seen[l] = true
				unique = append(unique, l)
			}
		}
		detail := strings.Join(unique, "\n")
		title := fmt.Sprintf("⚠️ %d profesional(es) con %d+ días hábiles de inactividad", len(unique), inactivityAlertDays)

		if err := w.notifSvc.CreateNotification(cs.ID, "inactivity_alert", title, detail, map[string]interface{}{"kind": "inactivity"}); err != nil {
			log.Printf("[inactivity-watcher] notificación interna a %s falló: %v", cs.Email, err)
		}
		html := fmt.Sprintf("<p>%s</p><p>%s</p><p>Revisa la pestaña <b>Actividad</b> del panel de administración de Obertrack para contactarlos.</p>",
			title, strings.ReplaceAll(detail, "\n", "<br>"))
		if err := w.brevoSvc.SendEmail(cs.Email, cs.Name, title, html); err != nil {
			log.Printf("[inactivity-watcher] email a %s falló: %v", cs.Email, err)
		}
	}

	// Slack: resumen global único al canal de customer success.
	allLines := make([]string, 0, len(users))
	for _, u := range users {
		allLines = append(allLines, line(u))
	}
	globalTitle := fmt.Sprintf("⚠️ %d profesional(es) con %d+ días hábiles de inactividad", len(users), inactivityAlertDays)
	if err := w.slackSvc.Notify(fmt.Sprintf("*%s*\n%s", globalTitle, strings.Join(allLines, "\n"))); err != nil {
		log.Printf("[inactivity-watcher] aviso a Slack falló: %v", err)
	}
}
