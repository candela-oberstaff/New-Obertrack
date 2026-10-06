package service

import (
	"errors"
	"fmt"
	"log"
	"sort"
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
	// tenantID 0: la alerta automática mira a todo el mundo, no a una empresa.
	inactive, err := w.adminRepo.GetInactiveUsersList(0, inactivityAlertDays)
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

	// Asignación de casos por destinatario (un CS puede cubrir varias empresas).
	casesByRecipient := map[uint][]repository.InactiveUser{}
	recipientByID := map[uint]models.User{}
	for _, cs := range csUsers {
		if cs.IsActive {
			recipientByID[cs.ID] = cs
		}
	}

	// El CS de cada empresa es el que tiene asignado (users.assigned_cs_id).
	// Puede ser un superadmin, que no está en la lista de CS: se suma.
	assignedCS := map[uint]uint{}
	for _, u := range users {
		if u.TenantID == 0 {
			continue
		}
		if _, done := assignedCS[u.TenantID]; done {
			continue
		}
		assignedCS[u.TenantID] = 0
		company, err := w.userRepo.GetByID(u.TenantID)
		if err != nil || company == nil || company.AssignedCSID == nil {
			continue
		}
		cs, err := w.userRepo.GetByID(*company.AssignedCSID)
		if err != nil || cs == nil || !cs.IsActive {
			continue
		}
		assignedCS[u.TenantID] = cs.ID
		if _, ok := recipientByID[cs.ID]; !ok {
			recipientByID[cs.ID] = *cs
		}
	}

	for _, u := range users {
		assignedToSomeone := false
		if csID := assignedCS[u.TenantID]; csID != 0 {
			casesByRecipient[csID] = append(casesByRecipient[csID], u)
			assignedToSomeone = true
		}
		for _, cs := range recipientByID {
			// Asignación antigua: el analista cuya empresa (empleador_id) es la
			// del profesional. Se respeta mientras haya datos así.
			legacyAnalyst := cs.EmpleadorID != nil && *cs.EmpleadorID == u.TenantID && u.TenantID != 0
			if cs.IsManager || legacyAnalyst {
				casesByRecipient[cs.ID] = append(casesByRecipient[cs.ID], u)
				if legacyAnalyst {
					assignedToSomeone = true
				}
			}
		}
		if !assignedToSomeone {
			// Empresa sin CS asignado: el caso va a todo el equipo.
			for _, cs := range recipientByID {
				if !cs.IsManager && cs.UserType == models.UserTypeCustomerSuccess {
					casesByRecipient[cs.ID] = append(casesByRecipient[cs.ID], u)
				}
			}
		}
	}

	// Best-effort por destinatario: un canal caído no detiene los demás.
	for csID, cases := range casesByRecipient {
		cs := recipientByID[csID]
		// Dedup por si un caso entró por más de una vía.
		seen := map[uint]bool{}
		unique := cases[:0]
		for _, c := range cases {
			if !seen[c.ID] {
				seen[c.ID] = true
				unique = append(unique, c)
			}
		}
		lines := make([]string, 0, len(unique))
		for _, c := range unique {
			lines = append(lines, line(c))
		}
		detail := strings.Join(lines, "\n")
		title := inactivityTitle(len(unique))

		// En la campanita, un resumen: la lista completa va por correo y está
		// en la pestaña Actividad, adonde lleva el aviso.
		if err := w.notifSvc.CreateNotification(cs.ID, "inactivity_alert", title, inactivitySummary(unique),
			map[string]interface{}{"kind": "inactivity", "link": "/admin?tab=activity"}); err != nil {
			log.Printf("[inactivity-watcher] notificación interna a %s falló: %v", cs.Email, err)
		}
		html := fmt.Sprintf("<p>%s</p><p>%s</p><p>Revisa la pestaña <b>Actividad</b> del panel de administración de Obertrack para contactarlos.</p>",
			title, strings.ReplaceAll(detail, "\n", "<br>"))
		if err := w.brevoSvc.SendEmailKind(EmailKindInactivityAlert, cs.Email, cs.Name, title, html); err != nil {
			if !errors.Is(err, ErrEmailKindDisabled) {
				log.Printf("[inactivity-watcher] email a %s falló: %v", cs.Email, err)
			}
		}
	}

	// Slack: resumen global único al canal de customer success.
	allLines := make([]string, 0, len(users))
	for _, u := range users {
		allLines = append(allLines, line(u))
	}
	globalTitle := inactivityTitle(len(users))
	if err := w.slackSvc.Notify(fmt.Sprintf("*%s*\n%s", globalTitle, strings.Join(allLines, "\n"))); err != nil {
		log.Printf("[inactivity-watcher] aviso a Slack falló: %v", err)
	}
}

// inactivityTitle: «69 profesionales llevan 2+ días hábiles sin registrar horas».
func inactivityTitle(n int) string {
	if n == 1 {
		return fmt.Sprintf("1 profesional lleva %d+ días hábiles sin registrar horas", inactivityAlertDays)
	}
	return fmt.Sprintf("%d profesionales llevan %d+ días hábiles sin registrar horas", n, inactivityAlertDays)
}

// inactivitySummary nombra a los más atrasados y cuenta el resto, para que el
// aviso quepa en la campanita: «Carlos Pérez (81 días), Daniela López (60
// días), Johelys Bocanegra (60 días) y 66 más.»
func inactivitySummary(cases []repository.InactiveUser) string {
	sorted := append([]repository.InactiveUser(nil), cases...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].DaysInactive > sorted[j].DaysInactive })
	const shown = 3
	names := make([]string, 0, shown)
	for i, c := range sorted {
		if i == shown {
			break
		}
		names = append(names, fmt.Sprintf("%s (%d días)", c.Name, c.DaysInactive))
	}
	summary := strings.Join(names, ", ")
	if rest := len(sorted) - shown; rest > 0 {
		summary += fmt.Sprintf(" y %d más", rest)
	}
	return summary + ". Revísalos en Actividad."
}
