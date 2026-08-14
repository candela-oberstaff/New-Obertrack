package service

import (
	"fmt"
	"log"
	"time"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
)

const (
	// escalationPendingFor: cuánto tiempo debe llevar una jornada sin aprobar
	// antes de que suba al supervisor. El aviso normal sigue yendo solo al
	// manager directo, que es quien tiene que resolverlo; esto es la red por si
	// no lo hace, no una copia en paralelo. Corto sería devolverle al supervisor
	// el ruido de toda su estructura, que es justo lo que se decidió evitar.
	escalationPendingFor = 3 * 24 * time.Hour
	// escalationResendAfter: como máximo un aviso por supervisor por ventana,
	// aunque las jornadas sigan pendientes.
	escalationResendAfter = 24 * time.Hour
	escalationInterval    = 6 * time.Hour
	escalationFirstRun    = 4 * time.Minute
	// Tope defensivo por corrida, mismo criterio que los otros watchers.
	escalationSupervisorLimit = 500
)

// SupervisorEscalationWatcher avisa a cada supervisor de las jornadas de SU
// ÁRBOL que llevan demasiado tiempo esperando aprobación.
//
// Es deliberadamente un digest y no un aviso por jornada: al supervisor le
// interesa "en tu estructura hay 7 jornadas atascadas", no siete
// notificaciones. Eso además evita tener que llevar estado por jornada — basta
// con recordar cuándo se le avisó por última vez a cada supervisor.
type SupervisorEscalationWatcher struct {
	userRepo repository.UserRepository
	empRepo  repository.EmploymentRepository
	whRepo   repository.WorkHourRepository
	notifSvc NotificationService
}

func NewSupervisorEscalationWatcher(
	userRepo repository.UserRepository,
	empRepo repository.EmploymentRepository,
	whRepo repository.WorkHourRepository,
	notifSvc NotificationService,
) *SupervisorEscalationWatcher {
	return &SupervisorEscalationWatcher{userRepo: userRepo, empRepo: empRepo, whRepo: whRepo, notifSvc: notifSvc}
}

func (w *SupervisorEscalationWatcher) Start() {
	go func() {
		time.Sleep(escalationFirstRun)
		for {
			if _, err := w.RunOnce(); err != nil {
				log.Printf("[supervisor-escalation] corrida fallida: %v", err)
			}
			time.Sleep(escalationInterval)
		}
	}()
}

// RunOnce recorre los supervisores activos y avisa a los que tengan jornadas
// atascadas en su árbol. Devuelve a cuántos se avisó.
//
// El registro se escribe ANTES de notificar, igual que en el digest del chat:
// si el proceso muere en medio, perder un aviso es más barato que duplicarlo.
func (w *SupervisorEscalationWatcher) RunOnce() (int, error) {
	// El escalado es parte del rol: sin el alcance de supervisor no hay árbol que
	// recorrer y el watcher no tiene nada que hacer.
	if !SupervisorScopeEnabled() {
		return 0, nil
	}

	supervisors, err := w.userRepo.ListActiveSupervisors()
	if err != nil {
		return 0, fmt.Errorf("listando supervisores: %w", err)
	}
	if len(supervisors) > escalationSupervisorLimit {
		supervisors = supervisors[:escalationSupervisorLimit]
	}

	cutoff := time.Now().Add(-escalationPendingFor)
	notified := 0

	for i := range supervisors {
		sup := supervisors[i]
		companyID := models.TenantForUser(&sup)
		if companyID == 0 {
			continue
		}

		// Se pregunta por la ventana ANTES de resolver el árbol y contar: a quien
		// ya se avisó hoy no hace falta calcularle nada.
		can, err := w.whRepo.CanEscalateTo(sup.ID, escalationResendAfter)
		if err != nil {
			log.Printf("[supervisor-escalation] no se pudo comprobar la ventana de %d: %v", sup.ID, err)
			continue
		}
		if !can {
			continue
		}

		team, err := w.empRepo.DescendantIDs(sup.ID, companyID, maxSupervisorDepth)
		if err != nil {
			log.Printf("[supervisor-escalation] no se pudo resolver el árbol de %d: %v", sup.ID, err)
			continue
		}
		if len(team) == 0 {
			continue
		}

		// Pendientes (ni aprobadas ni rechazadas) con fecha anterior al corte.
		// end_date filtra por work_date, que es la fecha de la jornada: lo que se
		// mide es cuánto lleva el trabajo sin revisar, no cuándo se cargó.
		_, total, err := w.whRepo.FindAll(map[string]interface{}{
			"tenant_id": companyID,
			"user_ids":  team,
			"approved":  false,
			"rejected":  false,
			"end_date":  cutoff,
		}, 0, 1)
		if err != nil {
			log.Printf("[supervisor-escalation] no se pudieron contar las pendientes de %d: %v", sup.ID, err)
			continue
		}
		if total == 0 {
			continue
		}

		if err := w.whRepo.MarkEscalationSent(sup.ID); err != nil {
			log.Printf("[supervisor-escalation] no se pudo registrar el aviso a %d: %v", sup.ID, err)
			continue
		}

		dias := int(escalationPendingFor.Hours() / 24)
		plural := "jornadas llevan"
		if total == 1 {
			plural = "jornada lleva"
		}
		_ = w.notifSvc.CreateNotification(
			sup.ID,
			"work_hour_escalation",
			"Jornadas pendientes en tu equipo",
			fmt.Sprintf("⏳ %d %s más de %d días esperando aprobación en tu estructura.", total, plural, dias),
			map[string]interface{}{"count": total, "link": "/work-hours"},
		)
		notified++
	}

	if notified > 0 {
		log.Printf("[supervisor-escalation] avisados %d supervisor(es) con jornadas atascadas", notified)
	}
	return notified, nil
}
