package service

import (
	"log"
	"time"

	"github.com/obertrack/backend/internal/models"
)

// Barrido del tiempo.
//
// Todos los demás disparadores los provoca una persona: alguien crea, mueve, prioriza
// o asigna. El paso del tiempo no lo provoca nadie, y era el hueco más grande del
// motor en un producto que va de fechas: una tarea que vence el martes y se queda
// quieta no producía nada. La receta "marcar urgente lo que va con retraso" sólo
// saltaba si además alguien la movía, que es exactamente lo que no pasa con el trabajo
// olvidado.
//
// El barrido emite dos hechos y se retira. Quién avisa, a quién y con qué texto lo
// deciden las recetas, igual que con cualquier otro disparador.

const (
	// Cada cuánto se mira el reloj. Se mira más de una vez al día a propósito: un
	// tick diario que caiga durante un despliegue se pierde entero, y la
	// deduplicación por fecha de fin hace que mirar de más no cueste nada.
	sweepInterval = time.Hour
	// Tope de tareas por empresa y pasada. Una empresa con mil tareas vencidas tiene
	// un problema que no se arregla con mil avisos.
	sweepMaxTasks = 200
	// Cuánto hacia atrás se mira lo vencido. Sin ventana, encender la receta hoy
	// desenterraría tareas caducadas hace un año que ya nadie va a mirar.
	sweepOverdueWindow = 30 * 24 * time.Hour
	// Retención del historial de ejecuciones. La tabla es a la vez cola y bitácora:
	// como cola se vacía sola, como bitácora crecía para siempre. Las fallidas duran
	// el triple porque son las que alguien consulta cuando pregunta por qué no llegó
	// un aviso, y son pocas.
	runRetention       = 90 * 24 * time.Hour
	runRetentionFailed = 270 * 24 * time.Hour
	// La limpieza no necesita mirarse cada hora como el calendario.
	purgeInterval = 24 * time.Hour
)

// StartPurge arranca la limpieza del historial de ejecuciones.
func (s *WorkflowService) StartPurge() {
	if s == nil {
		return
	}
	go func() {
		ticker := time.NewTicker(purgeInterval)
		defer ticker.Stop()
		for range ticker.C {
			s.purgeOnce(time.Now())
		}
	}()
}

// purgeOnce borra lo caducado una vez. Separada del bucle para poder probarla.
//
// No corre al arrancar a propósito: un reinicio en bucle dispararía un borrado masivo
// una y otra vez, y esto no tiene ninguna prisa.
func (s *WorkflowService) purgeOnce(now time.Time) {
	n, err := s.repo.PurgeRunsBefore(now.Add(-runRetention), now.Add(-runRetentionFailed))
	if err != nil {
		log.Printf("[workflow] no se pudo limpiar el historial de ejecuciones: %v", err)
		return
	}
	if n > 0 {
		log.Printf("[workflow] historial limpiado: %d ejecuciones caducadas", n)
	}
}

// StartSweep arranca el vigilante del tiempo. Sin recetas encendidas para estos
// disparadores no hace nada más que una consulta corta por hora.
func (s *WorkflowService) StartSweep() {
	if s == nil {
		return
	}
	go func() {
		// Una primera pasada al arrancar: si el proceso estuvo caído toda la noche,
		// lo vencido de ayer sigue vencido y merece salir ahora, no mañana.
		s.sweepOnce(time.Now())
		ticker := time.NewTicker(sweepInterval)
		defer ticker.Stop()
		for range ticker.C {
			s.sweepOnce(time.Now())
		}
	}()
}

// sweepOnce mira el reloj una vez. Se separa del bucle para poder probarla con una
// fecha fija en vez de esperar a que pase el tiempo de verdad.
func (s *WorkflowService) sweepOnce(now time.Time) {
	s.sweepTrigger(models.TriggerTaskOverdue, now)
	s.sweepTrigger(models.TriggerTaskDueSoon, now)
}

func (s *WorkflowService) sweepTrigger(trigger string, now time.Time) {
	// Sólo se recorren las empresas que tienen algo encendido para este disparador.
	// Sin esto, cada hora se leerían las tareas vencidas de toda la plataforma para
	// descartarlas casi siempre.
	tenants, err := s.repo.TenantsWithTrigger(trigger)
	if err != nil {
		log.Printf("[workflow] barrido %s: no se pudieron leer las empresas con reglas: %v", trigger, err)
		return
	}
	if len(tenants) == 0 {
		return
	}

	desde, hasta := sweepWindow(trigger, now)
	for _, tenantID := range tenants {
		if tenantID == 0 {
			continue
		}
		tasks, terr := s.taskRepo.ListByDueDate(tenantID, desde, hasta, sweepMaxTasks)
		if terr != nil {
			log.Printf("[workflow] barrido %s: no se pudieron leer las tareas de la empresa %d: %v",
				trigger, tenantID, terr)
			continue
		}
		for i := range tasks {
			task := tasks[i]
			s.OnEvent(WorkflowEvent{
				Type:     trigger,
				TenantID: tenantID,
				Task:     &task,
				// No lo hizo nadie: lo hizo el calendario. Marcarlo como sistema es
				// lo que permite que una regla diga "cuando lo mueva una persona" sin
				// dispararse también con esto.
				ActorIsSystem: true,
			})
		}
	}
}

// sweepWindow traduce el disparador a un rango de fechas de fin.
//
//	vencida     → [hoy - ventana, ayer]      lo que ya pasó y sigue sin terminarse
//	vence pronto → [mañana, mañana]          el aviso de la víspera
//
// Se trabaja con días enteros porque end_date es una fecha, no un instante: una tarea
// que vence "el 25" no vence a las 00:00 del 25.
func sweepWindow(trigger string, now time.Time) (desde, hasta time.Time) {
	hoy := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	switch trigger {
	case models.TriggerTaskDueSoon:
		manana := hoy.AddDate(0, 0, 1)
		return manana, manana
	default:
		return hoy.Add(-sweepOverdueWindow), hoy.AddDate(0, 0, -1)
	}
}
