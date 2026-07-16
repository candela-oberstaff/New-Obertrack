package service

import (
	"fmt"
	"log"
	"time"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
)

const (
	reportFirstRunDelay = 1 * time.Minute
	// Tick corto: los watchers existentes duermen 24h, pero acá hay que apuntar a
	// una hora de reloj configurable, así que se chequea seguido y se decide.
	reportTickInterval = 5 * time.Minute
	// Ventana de tolerancia alrededor de la hora objetivo. Debe ser mayor que el
	// tick para no perder la ejecución si el chequeo cae justo antes.
	reportSendWindow = 10 * time.Minute
	// Tope de empresas por corrida (mismo criterio que los otros watchers).
	reportEmployerLimit = 1000
)

// ReportMailWatcher envía automáticamente el reporte de jornadas a cada empresa
// según la configuración global (diario / semanal / mensual, hora y zona).
//
// La deduplicación NO vive en memoria sino en la tabla report_runs: un índice
// único parcial sobre (tenant_id, period_key) WHERE status='sent' garantiza que
// reiniciar el backend nunca reenvíe un período ya entregado. Los intentos
// fallidos sí se pueden reintentar mientras dure la ventana.
type ReportMailWatcher struct {
	scheduleRepo repository.ReportScheduleRepository
	userRepo     repository.UserRepository
	workHourSvc  WorkHourService
}

func NewReportMailWatcher(
	scheduleRepo repository.ReportScheduleRepository,
	userRepo repository.UserRepository,
	workHourSvc WorkHourService,
) *ReportMailWatcher {
	return &ReportMailWatcher{scheduleRepo: scheduleRepo, userRepo: userRepo, workHourSvc: workHourSvc}
}

func (w *ReportMailWatcher) Start() {
	go func() {
		time.Sleep(reportFirstRunDelay)
		for {
			if _, _, _, err := w.RunOnce(false); err != nil {
				log.Printf("[report-mail-watcher] corrida fallida: %v", err)
			}
			time.Sleep(reportTickInterval)
		}
	}()
}

// RunOnce evalúa si toca enviar y, en ese caso, manda el reporte a cada empresa.
// Con force=true ignora `enabled` y la hora programada (lo usa el botón "Enviar
// ahora" del panel), pero SIEMPRE respeta la deduplicación.
func (w *ReportMailWatcher) RunOnce(force bool) (sent, skipped, failed int, err error) {
	cfg, err := w.scheduleRepo.Get()
	if err != nil {
		return 0, 0, 0, err
	}
	if !force && !cfg.Enabled {
		return 0, 0, 0, nil
	}

	// Fail-safe: una zona inválida nunca debe tumbar el worker.
	loc, lerr := time.LoadLocation(cfg.Timezone)
	if lerr != nil {
		log.Printf("[report-mail-watcher] zona horaria inválida %q, usando UTC: %v", cfg.Timezone, lerr)
		loc = time.UTC
	}
	now := time.Now().In(loc)

	if !force && !isDue(cfg, now) {
		return 0, 0, 0, nil
	}

	start, end, periodKey, title, label := periodFor(cfg.Frequency, now, loc)

	employers, _, err := w.userRepo.GetAll(string(models.UserTypeEmployer), "", "", 0, 0, reportEmployerLimit)
	if err != nil {
		return 0, 0, 0, err
	}

	for i := range employers {
		emp := employers[i]
		if !emp.IsActive || emp.Email == "" {
			continue
		}

		already, herr := w.scheduleRepo.HasSuccessfulRun(emp.ID, periodKey)
		if herr != nil {
			log.Printf("[report-mail-watcher] no se pudo verificar la empresa %d: %v", emp.ID, herr)
			continue
		}
		if already {
			skipped++
			continue
		}

		run := &models.ReportRun{
			TenantID:       emp.ID,
			PeriodKey:      periodKey,
			Frequency:      cfg.Frequency,
			RecipientEmail: emp.Email,
			RecipientName:  emp.Name,
			Status:         models.ReportRunSent,
		}

		if serr := w.workHourSvc.SendPeriodReport(&emp, emp.ID, title, label, start, end); serr != nil {
			run.Status = models.ReportRunFailed
			run.Error = serr.Error()
			failed++
			log.Printf("[report-mail-watcher] falló el envío a %s: %v", emp.Email, serr)
		} else {
			sent++
		}

		if rerr := w.scheduleRepo.RecordRun(run); rerr != nil {
			log.Printf("[report-mail-watcher] no se pudo registrar la corrida de %d: %v", emp.ID, rerr)
		}
	}

	log.Printf("[report-mail-watcher] período=%s enviados=%d omitidos=%d fallidos=%d", periodKey, sent, skipped, failed)
	return sent, skipped, failed, nil
}

// isDue decide si `now` cae dentro de la ventana de envío configurada.
func isDue(cfg *models.ReportSchedule, now time.Time) bool {
	target := time.Date(now.Year(), now.Month(), now.Day(), cfg.Hour, cfg.Minute, 0, 0, now.Location())
	diff := now.Sub(target)
	if diff < 0 || diff >= reportSendWindow {
		return false
	}

	switch cfg.Frequency {
	case models.ReportFreqWeekly:
		return int(now.Weekday()) == cfg.Weekday
	case models.ReportFreqMonthly:
		return now.Day() == cfg.DayOfMonth
	default: // diaria
		return true
	}
}

var monthsEsReport = []string{
	"Enero", "Febrero", "Marzo", "Abril", "Mayo", "Junio",
	"Julio", "Agosto", "Septiembre", "Octubre", "Noviembre", "Diciembre",
}

// periodFor devuelve el rango del período YA CERRADO que corresponde reportar,
// más una clave estable para deduplicar y los textos del correo.
func periodFor(freq string, now time.Time, loc *time.Location) (start, end time.Time, periodKey, title, label string) {
	day := func(t time.Time) time.Time {
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
	}

	switch freq {
	case models.ReportFreqWeekly:
		// Semana anterior completa, de lunes a domingo.
		weekday := (int(now.Weekday()) + 6) % 7 // 0 = lunes
		thisMonday := day(now).AddDate(0, 0, -weekday)
		start = thisMonday.AddDate(0, 0, -7)
		end = thisMonday.AddDate(0, 0, -1)
		isoYear, isoWeek := start.ISOWeek()
		periodKey = fmt.Sprintf("w:%d-W%02d", isoYear, isoWeek)
		title = "Reporte Semanal de Jornadas"
		label = fmt.Sprintf("%s al %s", start.Format("02/01/2006"), end.Format("02/01/2006"))

	case models.ReportFreqMonthly:
		// Mes anterior completo.
		firstThisMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc)
		end = firstThisMonth.AddDate(0, 0, -1)
		start = time.Date(end.Year(), end.Month(), 1, 0, 0, 0, 0, loc)
		periodKey = fmt.Sprintf("m:%d-%02d", start.Year(), int(start.Month()))
		title = "Reporte Mensual de Jornadas"
		label = fmt.Sprintf("%s %d", monthsEsReport[int(start.Month())-1], start.Year())

	default:
		// Diaria: el día de ayer.
		start = day(now).AddDate(0, 0, -1)
		end = start
		periodKey = fmt.Sprintf("d:%s", start.Format("2006-01-02"))
		title = "Reporte Diario de Jornadas"
		label = start.Format("02/01/2006")
	}
	return start, end, periodKey, title, label
}
