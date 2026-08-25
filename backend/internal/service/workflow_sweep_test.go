package service

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/obertrack/backend/internal/models"
)

// El barrido del tiempo. Todo lo demás del motor lo provoca una persona; esto lo
// provoca el calendario, y por eso es lo único que atrapa el trabajo olvidado: una
// tarea que vence y se queda quieta no dispara ningún otro disparador.

// sweepTaskRepo devuelve las tareas que "vencen" en la ventana pedida y guarda con qué
// rango se le preguntó, que es la mitad de lo que hay que comprobar aquí.
type sweepTaskRepo struct {
	histTaskRepo
	tasks        []models.Task
	desde, hasta time.Time
	consultas    int
}

func (r *sweepTaskRepo) ListByDueDate(_ uint, desde, hasta time.Time, _ int) ([]models.Task, error) {
	r.desde, r.hasta = desde, hasta
	r.consultas++
	return r.tasks, nil
}

func venceEl(id uint, fecha string) models.Task {
	d, _ := time.Parse("2006-01-02", fecha)
	return models.Task{
		ID: id, Title: "Informe", BoardID: 1, TenantID: 42,
		Status: models.TaskStatusInProcess, Priority: models.PriorityMedium,
		EndDate: &d,
		Board:   models.Board{ID: 1, Name: "Proyecto", CreatedBy: 9, TenantID: 42},
	}
}

func sweepSvc(rules []models.Workflow, tasks []models.Task) (*WorkflowService, *wfRepo, *sweepTaskRepo) {
	repo := &wfRepo{rules: rules}
	tr := &sweepTaskRepo{tasks: tasks}
	s := NewWorkflowService(
		repo, tr,
		&wfBoardRepo{board: &models.Board{ID: 1, TenantID: 42, CreatedBy: 9}},
		&wfUserRepo{users: map[uint]*models.User{}},
		&wfEmpRepo{}, &fakeNotifSvc{}, nil,
	)
	return s, repo, tr
}

// La ventana es lo que decide qué se considera "vencido hoy". Se mira en días
// enteros porque la fecha de fin es una fecha: lo que vence el 25 no vence a las 00:00.
func TestBarrido_LaVentanaDeCadaDisparador(t *testing.T) {
	ahora := time.Date(2026, 8, 25, 14, 30, 0, 0, time.UTC)

	desde, hasta := sweepWindow(models.TriggerTaskDueSoon, ahora)
	if desde.Format("2006-01-02") != "2026-08-26" || hasta.Format("2006-01-02") != "2026-08-26" {
		t.Fatalf("la víspera es mañana y sólo mañana, got %s..%s", desde, hasta)
	}

	desde, hasta = sweepWindow(models.TriggerTaskOverdue, ahora)
	if hasta.Format("2006-01-02") != "2026-08-24" {
		t.Fatalf("lo vencido llega hasta AYER, no incluye hoy: %s", hasta)
	}
	// Con ventana hacia atrás: encender la receta hoy no puede desenterrar lo que
	// caducó hace un año y ya nadie va a mirar.
	if ahora.Sub(desde) > sweepOverdueWindow+48*time.Hour {
		t.Fatalf("la ventana hacia atrás es demasiado ancha: %s", desde)
	}
}

func TestBarrido_EmiteUnEventoPorTareaVencida(t *testing.T) {
	regla := rule(1, models.TriggerTaskOverdue, 1, "")
	s, repo, tr := sweepSvc([]models.Workflow{regla},
		[]models.Task{venceEl(100, "2026-08-20"), venceEl(101, "2026-08-21")})

	s.sweepOnce(time.Date(2026, 8, 25, 9, 0, 0, 0, time.UTC))

	if len(repo.queued) != 2 {
		t.Fatalf("debería encolarse una ejecución por tarea vencida, got %d", len(repo.queued))
	}
	if tr.consultas == 0 {
		t.Fatal("no se consultaron las tareas")
	}
}

// Sin recetas encendidas para el disparador, el barrido no toca las tareas de nadie:
// una consulta corta por hora y se retira.
func TestBarrido_SinReglasNoMiraTareas(t *testing.T) {
	s, repo, tr := sweepSvc(nil, []models.Task{venceEl(100, "2026-08-20")})

	s.sweepOnce(time.Date(2026, 8, 25, 9, 0, 0, 0, time.UTC))

	if tr.consultas != 0 {
		t.Fatalf("no debería leer tareas sin reglas encendidas, got %d consultas", tr.consultas)
	}
	if len(repo.queued) != 0 {
		t.Fatalf("no debería encolar nada, got %d", len(repo.queued))
	}
}

// Lo más importante del barrido: mirar el reloj dos veces no avisa dos veces. Se corre
// cada hora a propósito —un tick diario que caiga en un despliegue se pierde entero— y
// eso sólo es sostenible si repetir no cuesta nada.
func TestBarrido_RepetirNoAvisaDosVeces(t *testing.T) {
	regla := rule(1, models.TriggerTaskOverdue, 1, "")
	s, repo, _ := sweepSvc([]models.Workflow{regla}, []models.Task{venceEl(100, "2026-08-20")})

	s.sweepOnce(time.Date(2026, 8, 25, 9, 0, 0, 0, time.UTC))
	s.sweepOnce(time.Date(2026, 8, 25, 10, 0, 0, 0, time.UTC))
	s.sweepOnce(time.Date(2026, 8, 26, 9, 0, 0, 0, time.UTC))

	if len(repo.queued) != 1 {
		t.Fatalf("una tarea vencida avisa UNA vez por vencimiento, got %d", len(repo.queued))
	}
}

// Y la deduplicación es por FECHA DE FIN, no por revisión: editar el título de una
// tarea vencida no es un vencimiento nuevo, pero mover la fecha sí.
func TestBarrido_LaDeduplicacionSigueALaFechaNoALaRevision(t *testing.T) {
	tarea := venceEl(100, "2026-08-20")

	base := WorkflowEvent{Type: models.TriggerTaskOverdue, TenantID: 42, Task: &tarea}
	primera := dedupKeyFor(base)

	// Alguien edita la tarea: sube la revisión, pero vence el mismo día.
	tarea.Revision = 7
	if dedupKeyFor(base) != primera {
		t.Fatal("editar una tarea vencida no es un vencimiento nuevo")
	}

	// Le mueven la fecha: eso sí es otro vencimiento.
	nueva, _ := time.Parse("2006-01-02", "2026-09-15")
	tarea.EndDate = &nueva
	if dedupKeyFor(base) == primera {
		t.Fatal("cambiar la fecha de fin abre un vencimiento nuevo")
	}

	// Y un cambio provocado por una persona sigue deduplicándose por revisión.
	humano := WorkflowEvent{Type: models.TriggerTaskStatusChanged, TenantID: 42, Task: &tarea}
	antes := dedupKeyFor(humano)
	tarea.Revision = 8
	if dedupKeyFor(humano) == antes {
		t.Fatal("un cambio nuevo de una persona es un hecho nuevo")
	}
}

// El barrido no lo hizo nadie: lo hizo el calendario. Marcarlo permite que una regla
// diga "cuando lo mueva una persona" sin dispararse también con esto.
func TestBarrido_ElActorEsElSistema(t *testing.T) {
	regla := rule(1, models.TriggerTaskDueSoon, 1, "")
	s, repo, _ := sweepSvc([]models.Workflow{regla}, []models.Task{venceEl(100, "2026-08-26")})

	s.sweepOnce(time.Date(2026, 8, 25, 9, 0, 0, 0, time.UTC))

	if len(repo.queued) != 1 {
		t.Fatalf("debería avisar la víspera, got %d", len(repo.queued))
	}
	var ctx WorkflowContext
	if err := json.Unmarshal([]byte(repo.queued[0].Context), &ctx); err != nil {
		t.Fatal(err)
	}
	if !ctx.Actor.EsSistema {
		t.Fatalf("el barrido tiene que marcarse como sistema: %+v", ctx.Actor)
	}
}

// ---------------------------------------------------------------------------
// Retención del historial
// ---------------------------------------------------------------------------

// La tabla de ejecuciones es a la vez cola y bitácora. Como cola se vacía sola; como
// bitácora crecía para siempre.
func TestLimpieza_ConservaLasFallidasMasTiempo(t *testing.T) {
	repo := &purgeRepo{}
	s, _, _ := sweepSvc(nil, nil)
	s.repo = repo

	ahora := time.Date(2026, 8, 25, 3, 0, 0, 0, time.UTC)
	s.purgeOnce(ahora)

	if repo.llamadas != 1 {
		t.Fatalf("debería haber limpiado una vez, got %d", repo.llamadas)
	}
	// Lo corriente caduca a los 90 días.
	if dias := ahora.Sub(repo.done).Hours() / 24; dias < 89 || dias > 91 {
		t.Fatalf("la retención normal debería rondar los 90 días, got %.0f", dias)
	}
	// Lo fallido dura más: es lo que alguien consulta cuando pregunta por qué no
	// llegó un aviso, y son pocas filas.
	if !repo.failed.Before(repo.done) {
		t.Fatalf("las fallidas tienen que conservarse más tiempo: %s vs %s", repo.failed, repo.done)
	}
}

type purgeRepo struct {
	wfRepo
	done, failed time.Time
	llamadas     int
}

func (r *purgeRepo) PurgeRunsBefore(done, failed time.Time) (int64, error) {
	r.done, r.failed = done, failed
	r.llamadas++
	return 0, nil
}
