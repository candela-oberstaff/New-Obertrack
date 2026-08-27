package service

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
)

const (
	// staleCompanyDays: días sin que NADIE de la empresa abra la app. Dos
	// semanas es el umbral que distingue unas vacaciones de un abandono; con
	// menos, el aviso saltaría cada puente y el equipo aprendería a ignorarlo.
	staleCompanyDays = 14
	// No repetir el aviso de la misma empresa en este plazo. Coincide con el
	// umbral: si sigue muerta dentro de dos semanas, vuelve a avisar una vez.
	staleCompanyCooldown = 14 * 24 * time.Hour
	staleCheckInterval   = 24 * time.Hour
	// Espera tras el arranque: deja migrar y, sobre todo, deja que el contador
	// de uso acumule algo antes del primer chequeo.
	staleFirstRunDelay = 5 * time.Minute
)

// StaleCompanyWatcher avisa a Customer Success de las empresas donde nadie ha
// abierto la app en las últimas dos semanas.
//
// Complementa a InactivityWatcher sin solaparse: aquel mira PERSONAS que dejan
// de registrar horas; este mira EMPRESAS que dejan de aparecer, que es la señal
// que llega antes. Una cuenta se enfría dejando de entrar mucho antes de dejar
// de cargar jornadas, y para cuando falta la jornada la conversación de
// retención ya llega tarde.
type StaleCompanyWatcher struct {
	usageRepo repository.UsageRepository
	adminRepo repository.AdminRepository
	userRepo  repository.UserRepository
	notifSvc  NotificationService
	brevoSvc  *BrevoService
	slackSvc  *SlackService
}

func NewStaleCompanyWatcher(
	usageRepo repository.UsageRepository,
	adminRepo repository.AdminRepository,
	userRepo repository.UserRepository,
	notifSvc NotificationService,
	brevoSvc *BrevoService,
	slackSvc *SlackService,
) *StaleCompanyWatcher {
	return &StaleCompanyWatcher{
		usageRepo: usageRepo,
		adminRepo: adminRepo,
		userRepo:  userRepo,
		notifSvc:  notifSvc,
		brevoSvc:  brevoSvc,
		slackSvc:  slackSvc,
	}
}

func (w *StaleCompanyWatcher) Start() {
	go func() {
		time.Sleep(staleFirstRunDelay)
		for {
			if err := w.RunOnce(); err != nil {
				log.Printf("[stale-company-watcher] chequeo fallido: %v", err)
			}
			time.Sleep(staleCheckInterval)
		}
	}()
}

func (w *StaleCompanyWatcher) RunOnce() error {
	// Sin nada medido no hay nada que decir. Sin esta guarda, el día que se
	// estrena el contador TODAS las empresas saldrían como abandonadas y el
	// primer correo que recibiría el equipo sería una falsa alarma masiva —la
	// forma más rápida de que aprendan a borrarlo sin leerlo—.
	overview, err := w.usageRepo.Overview(repository.UsageScope{Days: staleCompanyDays, ClientsOnly: true})
	if err != nil {
		return fmt.Errorf("leyendo el resumen de uso: %w", err)
	}
	if overview.TrackingSince == nil {
		return nil
	}
	if time.Since(*overview.TrackingSince) < staleCompanyDays*24*time.Hour {
		log.Printf("[stale-company-watcher] el contador lleva menos de %d días midiendo; se omite el chequeo", staleCompanyDays)
		return nil
	}

	stale, err := w.usageRepo.StaleCompanies(staleCompanyDays)
	if err != nil {
		return fmt.Errorf("listando empresas sin uso: %w", err)
	}
	if len(stale) == 0 {
		return nil
	}

	alerted, err := w.adminRepo.GetRecentlyAlertedCompanyIDs(time.Now().Add(-staleCompanyCooldown))
	if err != nil {
		return fmt.Errorf("leyendo avisos recientes: %w", err)
	}
	recent := make(map[uint]bool, len(alerted))
	for _, id := range alerted {
		recent[id] = true
	}

	var fresh []repository.CompanyUsage
	for _, c := range stale {
		if !recent[c.CompanyID] {
			fresh = append(fresh, c)
		}
	}
	if len(fresh) == 0 {
		return nil
	}

	w.notifySupportTeam(fresh)

	now := time.Now()
	marks := make([]models.CompanyUsageAlert, 0, len(fresh))
	for _, c := range fresh {
		marks = append(marks, models.CompanyUsageAlert{
			CompanyID:     c.CompanyID,
			DaysStale:     staleCompanyDays,
			LastAlertedAt: now,
		})
	}
	if err := w.adminRepo.MarkCompaniesAlerted(marks); err != nil {
		return fmt.Errorf("marcando empresas avisadas: %w", err)
	}

	log.Printf("[stale-company-watcher] avisadas %d empresas sin uso en %d días", len(fresh), staleCompanyDays)
	return nil
}

// notifySupportTeam manda un solo aviso por analista con TODAS sus empresas
// frías, no uno por empresa: cinco correos seguidos se leen como spam y el
// quinto no se abre.
func (w *StaleCompanyWatcher) notifySupportTeam(companies []repository.CompanyUsage) {
	csUsers, _, err := w.userRepo.GetAll(string(models.UserTypeCustomerSuccess), "", "", 0, 0, 1000)
	if err != nil {
		log.Printf("[stale-company-watcher] no se pudo listar al equipo CS: %v", err)
	}

	line := func(c repository.CompanyUsage) string {
		last := "nunca desde que medimos"
		if c.LastActive != nil {
			last = fmt.Sprintf("última señal el %s", c.LastActive.Format("02/01/2006"))
		}
		return fmt.Sprintf("• %s — %d personas, nadie entró en %d días (%s)",
			c.CompanyName, c.TotalUsers, staleCompanyDays, last)
	}

	// Mismo reparto que la alerta de inactividad: cada analista recibe SUS
	// empresas, los managers el panorama completo, y lo que no tiene analista
	// asignado va a todo el equipo para que no se quede sin dueño.
	linesByRecipient := map[uint][]string{}
	recipientByID := map[uint]models.User{}
	for _, cs := range csUsers {
		if cs.IsActive {
			recipientByID[cs.ID] = cs
		}
	}

	for _, c := range companies {
		assigned := false
		for _, cs := range recipientByID {
			isAssignedAnalyst := cs.EmpleadorID != nil && *cs.EmpleadorID == c.CompanyID
			if cs.IsManager || isAssignedAnalyst {
				linesByRecipient[cs.ID] = append(linesByRecipient[cs.ID], line(c))
				if isAssignedAnalyst {
					assigned = true
				}
			}
		}
		if !assigned {
			for _, cs := range recipientByID {
				if !cs.IsManager {
					linesByRecipient[cs.ID] = append(linesByRecipient[cs.ID], line(c))
				}
			}
		}
	}

	for csID, lines := range linesByRecipient {
		cs := recipientByID[csID]
		seen := map[string]bool{}
		unique := lines[:0]
		for _, l := range lines {
			if !seen[l] {
				seen[l] = true
				unique = append(unique, l)
			}
		}
		detail := strings.Join(unique, "\n")
		title := fmt.Sprintf("🧊 %d empresa(s) sin abrir la app en %d días", len(unique), staleCompanyDays)

		if err := w.notifSvc.CreateNotification(cs.ID, "stale_company", title, detail, map[string]interface{}{"kind": "stale_company"}); err != nil {
			log.Printf("[stale-company-watcher] notificación interna a %s falló: %v", cs.Email, err)
		}
		html := fmt.Sprintf(
			"<p>%s</p><p>%s</p><p>Míralas en <b>Métricas → Uso</b> para ver desde cuándo y qué módulos dejaron de tocar.</p>",
			title, strings.ReplaceAll(detail, "\n", "<br>"))
		if err := w.brevoSvc.SendEmailKind(EmailKindStaleCompany, cs.Email, cs.Name, title, html); err != nil {
			if !errors.Is(err, ErrEmailKindDisabled) {
				log.Printf("[stale-company-watcher] email a %s falló: %v", cs.Email, err)
			}
		}
	}

	all := make([]string, 0, len(companies))
	for _, c := range companies {
		all = append(all, line(c))
	}
	title := fmt.Sprintf("🧊 %d empresa(s) sin abrir la app en %d días", len(companies), staleCompanyDays)
	if err := w.slackSvc.Notify(fmt.Sprintf("*%s*\n%s", title, strings.Join(all, "\n"))); err != nil {
		log.Printf("[stale-company-watcher] aviso a Slack falló: %v", err)
	}
}
