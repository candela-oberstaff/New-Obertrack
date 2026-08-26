package service

import (
	"testing"

	"github.com/obertrack/backend/internal/models"
	"github.com/obertrack/backend/internal/repository"
)

// Reparto por carga.
//
// La regla "asignar sola si empieza sin responsable" daba la tarea al líder del
// proyecto, y el líder es siempre el mismo: quien más trabajo tiene acaba
// recibiendo más. Ahora la recibe quien esté más libre, midiendo la carga en tareas
// SIN TERMINAR. Que lo finalizado no cuente es el centro del asunto: acabar el
// trabajo tiene que devolverte a la cola, no sacarte de ella.

type cargaTaskRepo struct {
	histTaskRepo
	cargas map[uint]repository.UserLoad
	// preguntadoPor guarda a quién se preguntó, para comprobar a quién se descarta
	// ANTES de contar.
	preguntadoPor []uint
	err           error
}

func (r *cargaTaskRepo) OpenLoadByUser(_ uint, userIDs []uint) (map[uint]repository.UserLoad, error) {
	r.preguntadoPor = append([]uint{}, userIDs...)
	if r.err != nil {
		return nil, r.err
	}
	return r.cargas, nil
}

// equipo arma un tablero con esos miembros, todos activos, y sus cargas.
func equipo(t *testing.T, miembros []uint, cargas map[uint]repository.UserLoad) (*WorkflowService, *cargaTaskRepo) {
	t.Helper()
	users := map[uint]*models.User{}
	board := &models.Board{ID: 1, TenantID: 42, CreatedBy: 9}
	for _, id := range miembros {
		users[id] = &models.User{ID: id, Name: "Persona", IsActive: true}
		board.Members = append(board.Members, models.User{ID: id})
	}
	repoTareas := &cargaTaskRepo{cargas: cargas}
	s := NewWorkflowService(
		&wfRepo{}, repoTareas,
		&wfBoardRepo{board: board},
		&wfUserRepo{users: users},
		&wfEmpRepo{}, &fakeNotifSvc{}, nil,
	)
	return s, repoTareas
}

func cargaCtx() WorkflowContext {
	return WorkflowContext{
		Trigger: models.TriggerTaskStatusChanged,
		Task:    workflowTaskCtx{ID: 100, Titulo: "Revisar informe", Estado: "en_proceso"},
		Board:   workflowBoardCtx{ID: 1, CreadorID: 9},
	}
}

func TestCarga_LaRecibeElMasLibre(t *testing.T) {
	s, _ := equipo(t, []uint{5, 6, 7}, map[uint]repository.UserLoad{
		5: {UserID: 5, Abiertas: 9},
		6: {UserID: 6, Abiertas: 2},
		7: {UserID: 7, Abiertas: 6},
	})

	ids, why := s.leastLoadedMember(cargaCtx(), 42)

	if len(ids) != 1 || ids[0] != 6 {
		t.Fatalf("debía tocarle al que tiene 2 tareas, got %v (%s)", ids, why)
	}
}

// El caso que pidió el cliente: alguien con mucho en finalizado y poco vivo está
// LIBRE. La consulta ya excluye lo terminado, así que aquí se fija que el criterio
// es el número de tareas vivas y nada más: quien no aparece en el mapa no tiene
// ninguna, y ese gana aunque haya cerrado cincuenta esta semana.
func TestCarga_LoTerminadoNoPesa(t *testing.T) {
	s, _ := equipo(t, []uint{5, 6}, map[uint]repository.UserLoad{
		5: {UserID: 5, Abiertas: 3},
		// 6 no aparece: cerró todo lo suyo y no le queda nada vivo.
	})

	ids, _ := s.leastLoadedMember(cargaCtx(), 42)

	if len(ids) != 1 || ids[0] != 6 {
		t.Fatalf("el que no tiene nada vivo debía ganar, got %v", ids)
	}
}

// Empate en número: pesa más ir atrasado. Tres tareas todas vencidas no es lo mismo
// que tres tranquilas.
func TestCarga_ADosIgualesGanaElQueNoVaAtrasado(t *testing.T) {
	s, _ := equipo(t, []uint{5, 6}, map[uint]repository.UserLoad{
		5: {UserID: 5, Abiertas: 3, Vencidas: 3},
		6: {UserID: 6, Abiertas: 3, Vencidas: 0},
	})

	ids, _ := s.leastLoadedMember(cargaCtx(), 42)

	if len(ids) != 1 || ids[0] != 6 {
		t.Fatalf("debía ganar el que no va atrasado, got %v", ids)
	}
}

// Empate total: la elección tiene que ser SIEMPRE la misma. Un reintento del mismo
// paso no puede asignar a una persona distinta de la del primer intento.
func TestCarga_ElEmpateSeRompeIgualTodasLasVeces(t *testing.T) {
	cargas := map[uint]repository.UserLoad{
		5: {UserID: 5, Abiertas: 4},
		6: {UserID: 6, Abiertas: 4},
		7: {UserID: 7, Abiertas: 4},
	}
	for intento := 0; intento < 5; intento++ {
		s, _ := equipo(t, []uint{7, 5, 6}, cargas)
		ids, _ := s.leastLoadedMember(cargaCtx(), 42)
		if len(ids) != 1 || ids[0] != 5 {
			t.Fatalf("intento %d eligió %v: el desempate no es estable", intento, ids)
		}
	}
}

// La cuenta de la empresa es miembro de sus tableros pero no ejecuta tareas. Se
// descarta ANTES de contar: si no, con carga cero se llevaría siempre todo.
func TestCarga_LaCuentaDeLaEmpresaNoEntraEnElReparto(t *testing.T) {
	s, repoTareas := equipo(t, []uint{42, 5}, map[uint]repository.UserLoad{
		5: {UserID: 5, Abiertas: 8},
	})

	ids, _ := s.leastLoadedMember(cargaCtx(), 42)

	if len(ids) != 1 || ids[0] != 5 {
		t.Fatalf("la empresa no debía llevarse la tarea, got %v", ids)
	}
	for _, id := range repoTareas.preguntadoPor {
		if id == 42 {
			t.Fatal("ni siquiera debía preguntarse por la carga de la cuenta de empresa")
		}
	}
}

func TestCarga_QuienYaNoTrabajaAquiNoRecibeNada(t *testing.T) {
	users := map[uint]*models.User{
		5: {ID: 5, IsActive: false}, // se fue
		6: {ID: 6, IsActive: true},
	}
	board := &models.Board{ID: 1, TenantID: 42, Members: []models.User{{ID: 5}, {ID: 6}}}
	repoTareas := &cargaTaskRepo{cargas: map[uint]repository.UserLoad{6: {UserID: 6, Abiertas: 7}}}
	s := NewWorkflowService(&wfRepo{}, repoTareas, &wfBoardRepo{board: board},
		&wfUserRepo{users: users}, &wfEmpRepo{}, &fakeNotifSvc{}, nil)

	ids, _ := s.leastLoadedMember(cargaCtx(), 42)

	if len(ids) != 1 || ids[0] != 6 {
		t.Fatalf("la baja tenía carga cero y aun así no le toca: got %v", ids)
	}
}

// Sin nadie a quien repartir el paso se salta CON MOTIVO. Un salto mudo en la
// actividad no se distingue de una regla que no llegó a correr.
func TestCarga_SinMiembrosLoDiceEnVezDeCallar(t *testing.T) {
	s, _ := equipo(t, nil, nil)

	ids, why := s.leastLoadedMember(cargaCtx(), 42)

	if len(ids) != 0 || why == "" {
		t.Fatalf("debía saltarse explicando el motivo: ids=%v why=%q", ids, why)
	}
}

func TestCarga_SiLaConsultaFallaNoAsignaAlAzar(t *testing.T) {
	s, repoTareas := equipo(t, []uint{5, 6}, nil)
	repoTareas.err = gormNotFound

	ids, why := s.leastLoadedMember(cargaCtx(), 42)

	if len(ids) != 0 {
		t.Fatalf("sin saber la carga no se reparte: got %v", ids)
	}
	if why == "" {
		t.Fatal("y se explica por qué")
	}
}

// La receta usa esta clase de destinatario a través de resolveRecipients: si el
// case se cayera, el paso quedaría "clase de destinatario desconocida" y la regla
// no asignaría nunca.
func TestCarga_ElMotorReconoceLaClaseDeDestinatario(t *testing.T) {
	s, _ := equipo(t, []uint{5, 6}, map[uint]repository.UserLoad{5: {UserID: 5, Abiertas: 4}})

	ids, why := s.resolveRecipients(
		stepConfig{Recipient: models.RecipientLeastLoaded},
		&models.WorkflowRun{ID: 7, TenantID: 42},
		cargaCtx(),
	)

	if len(ids) != 1 || ids[0] != 6 {
		t.Fatalf("resolveRecipients no resolvió el reparto por carga: %v (%s)", ids, why)
	}
}

// ---------------------------------------------------------------------------
// Lo que un paso de asignar deja para los siguientes
// ---------------------------------------------------------------------------

// El aviso tiene que llegar a QUIEN ACABA de recibir la tarea. Resolver la carga
// otra vez en el paso del aviso avisaría a otra persona si la carga cambió entre un
// paso y el siguiente; por eso el aviso apunta a "nuevos asignados", y eso sólo
// funciona si el paso de asignar los deja puestos en el contexto.
func TestPasoAsignar_DejaALosRecienAsignadosParaElAviso(t *testing.T) {
	ctx := cargaCtx()

	applyStepEffect(&ctx, map[string]any{"asignados": []uint{6}})

	if len(ctx.Task.NuevosAsignados) != 1 || ctx.Task.NuevosAsignados[0] != 6 {
		t.Fatalf("el aviso no encontraría a quien avisar: %v", ctx.Task.NuevosAsignados)
	}
}

// Al reintentar, la salida del paso ya hecho se repone desde el JSON guardado y los
// ids vuelven como float64. Si no se leyeran, el reintento avisaría a nadie.
func TestPasoAsignar_TambienAlReponerseDesdeLoGuardado(t *testing.T) {
	ctx := cargaCtx()

	applyStepEffect(&ctx, map[string]any{"asignados": []any{float64(6)}})

	if len(ctx.Task.NuevosAsignados) != 1 || ctx.Task.NuevosAsignados[0] != 6 {
		t.Fatalf("un reintento perdería a quien avisar: %v", ctx.Task.NuevosAsignados)
	}
}

// Un segundo paso de asignar reconstruye la lista final a partir del contexto. Si
// el primero no se hubiera anotado ahí, el segundo mandaría una lista sin él y
// SyncAssignees borraría al primero.
func TestPasoAsignar_NoBorraLoQueAsignoElPasoAnterior(t *testing.T) {
	ctx := cargaCtx()
	ctx.Task.AsignadosIDs = []uint{3}

	applyStepEffect(&ctx, map[string]any{"asignados": []uint{6}})

	if len(ctx.Task.AsignadosIDs) != 2 {
		t.Fatalf("la lista debía quedar con los dos, got %v", ctx.Task.AsignadosIDs)
	}
}

func TestPasoAsignar_UnPasoSinAsignacionNoTocaNada(t *testing.T) {
	ctx := cargaCtx()
	ctx.Task.AsignadosIDs = []uint{3}

	applyStepEffect(&ctx, map[string]any{"notificados": []uint{9}})

	if len(ctx.Task.NuevosAsignados) != 0 || len(ctx.Task.AsignadosIDs) != 1 {
		t.Fatalf("un paso de aviso no cambia asignaciones: %v / %v",
			ctx.Task.NuevosAsignados, ctx.Task.AsignadosIDs)
	}
}
