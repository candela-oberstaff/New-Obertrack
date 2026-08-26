package service

import (
	"strings"
	"testing"
	"time"

	"github.com/obertrack/backend/internal/models"
)

// Consecuencia según la respuesta (G3). Una puerta deja de ser un peaje y pasa a ser
// una decisión: quien revisa aprueba —y la tarjeta se cierra sola— o la devuelve, y
// vuelve al principio con el motivo escrito. Lo que se prueba aquí es que la decisión
// llegue entera desde el formulario hasta la consecuencia, y que ninguna de las dos
// ramas se ejecute cuando le toca a la otra.

// ---------------------------------------------------------------------------
// Columnas por su papel en el tablero
// ---------------------------------------------------------------------------

// Una receta no puede llevar escrito "finalizado": cada tablero nombra sus columnas
// como quiere, y mover una tarjeta a una columna inexistente es peor que no moverla.
func TestVeredicto_ColumnasPorSuPapel(t *testing.T) {
	casos := []struct {
		nombre         string
		fases          []models.Phase
		inicial, final string
	}{
		{
			"tablero por defecto",
			[]models.Phase{
				{ID: 1, Name: "Por hacer", Status: "por_hacer"},
				{ID: 2, Name: "En proceso", Status: "en_proceso"},
				{ID: 3, Name: "Finalizado", Status: "finalizado"},
			},
			"por_hacer", "finalizado",
		},
		{
			// Renombradas: se cae a la primera y la última, que es como se lee un
			// kanban de izquierda a derecha.
			"columnas propias",
			[]models.Phase{
				{ID: 1, Name: "Backlog"},
				{ID: 2, Name: "Haciendo"},
				{ID: 3, Name: "Entregado"},
			},
			"backlog", "entregado",
		},
		{
			// Sin destino posible. Vale más no mover que mover a donde no toca.
			"una sola columna",
			[]models.Phase{{ID: 1, Name: "Todo"}},
			"todo", "",
		},
		{"sin columnas", nil, "", ""},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			inicial, final := boardColumns(models.Board{Phases: c.fases})
			if inicial != c.inicial || final != c.final {
				t.Fatalf("got (%q, %q), want (%q, %q)", inicial, final, c.inicial, c.final)
			}
		})
	}
}

func TestVeredicto_SinColumnaDeCierreNoMueveNada(t *testing.T) {
	actual := &models.Task{ID: 100, Status: models.TaskStatusInProcess, Priority: models.PriorityMedium}
	s, m := mutSvc(actual, nil)

	ctx := snapshotCtx("en_proceso", "medium")
	ctx.Trigger = models.TriggerTaskEnteringPhase
	// Tablero de una sola columna: no hay "final" distinto del principio.
	ctx.Board.ColumnaFinal = ""

	_, skip, err := s.runAction(mutStep(models.ActionSetStatus, stepConfig{Status: statusBoardEnd}), mutRun(), ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(skip, "columna de cierre") {
		t.Fatalf("el motivo debería explicar que falta la columna, got %q", skip)
	}
	if len(m.applied) != 0 {
		t.Fatalf("no debería haber movido nada, got %+v", m.applied)
	}
}

func TestVeredicto_MueveALaColumnaDeCierreDelTablero(t *testing.T) {
	actual := &models.Task{ID: 100, Status: models.TaskStatusInProcess, Priority: models.PriorityMedium}
	s, m := mutSvc(actual, nil)

	ctx := snapshotCtx("en_proceso", "medium")
	ctx.Trigger = models.TriggerTaskEnteringPhase
	ctx.Board.ColumnaFinal = "entregado"

	out, skip, err := s.runAction(mutStep(models.ActionSetStatus, stepConfig{Status: statusBoardEnd}), mutRun(), ctx)
	if err != nil || skip != "" {
		t.Fatalf("debería mover, got %q / %v", skip, err)
	}
	if out["estado"] != "entregado" {
		t.Fatalf("destino equivocado: %+v", out)
	}
	if len(m.applied) != 1 || m.applied[0].updates["status"] != "entregado" {
		t.Fatalf("no se aplicó el movimiento esperado: %+v", m.applied)
	}
	// Y hereda la justificación de la puerta: si la columna de cierre tuviera su
	// propia puerta, este movimiento puede atravesarla porque una persona ya
	// respondió el formulario que lo provocó.
	if !m.applied[0].cause.GateJustified {
		t.Fatal("la consecuencia de una puerta debe viajar justificada")
	}
}

// ---------------------------------------------------------------------------
// El formulario llega hasta la condición y hasta el texto
// ---------------------------------------------------------------------------

func TestVeredicto_LoRespondidoViajaEnElSnapshot(t *testing.T) {
	ev := WorkflowEvent{
		Type:     models.TriggerTaskEnteringPhase,
		TenantID: 42,
		Task:     taskFor(3, models.TaskStatusInProcess),
		ActorID:  7,
		GateAnswers: map[string]any{
			"veredicto":  "rechazado",
			"comentario": "Faltan las pruebas del endpoint nuevo.",
		},
	}
	ctx := buildContext(ev, "Ana")

	// Preguntable por una condición…
	fields := conditionFields(ctx)
	if fields["respuesta.veredicto"] != "rechazado" {
		t.Fatalf("la respuesta debería poder preguntarse, got %v", fields["respuesta.veredicto"])
	}
	// …y escribible en un texto, que es como llega a la persona.
	texto := interpolate(`{{actor.nombre}} devolvió "{{tarea.titulo}}": {{respuestas.comentario}}`, ctx)
	if !strings.Contains(texto, "Ana devolvió") || !strings.Contains(texto, "Faltan las pruebas") {
		t.Fatalf("el comentario no se compuso con lo respondido: %q", texto)
	}
}

// Dos puertas en columnas distintas del mismo tablero pasan igual el filtro de
// ámbito. Sin acotar por la puerta CRUZADA, la que nadie tocó acabaría decidiendo
// sobre respuestas que no son suyas.
func TestVeredicto_SoloReaccionaLaPuertaQueSeCruzo(t *testing.T) {
	cruzada := rule(1, models.TriggerTaskEnteringPhase, 1, "")
	otra := rule(2, models.TriggerTaskEnteringPhase, 1, "")
	repo := &wfRepo{rules: []models.Workflow{cruzada, otra}}
	s := newWfService(repo, &models.Board{ID: 1, TenantID: 42, CreatedBy: 9}, nil, nil)

	s.OnEvent(WorkflowEvent{
		Type:           models.TriggerTaskEnteringPhase,
		TenantID:       42,
		Task:           taskFor(4, models.TaskStatusInProcess),
		GateWorkflowID: 1,
		GateAnswers:    map[string]any{"veredicto": "aprobado"},
	})

	if len(repo.queued) != 1 {
		t.Fatalf("sólo la puerta cruzada debería ejecutarse, got %d", len(repo.queued))
	}
	if repo.queued[0].WorkflowID != 1 {
		t.Fatalf("se encoló la puerta equivocada: %+v", repo.queued[0])
	}
}

// ---------------------------------------------------------------------------
// Una ejecución no se estorba a sí misma
// ---------------------------------------------------------------------------

// La comprobación de obsolescencia existe para no pisar el trabajo de una PERSONA.
// Aplicada a lo que la propia ejecución acaba de hacer, dejaba a la regla que mueve y
// luego comenta moviendo sin comentar: justo el caso del veredicto.
func TestVeredicto_MoverNoDejaObsoletoElComentarioQueVieneDetras(t *testing.T) {
	ctx := snapshotCtx("en_revisión", "medium")

	applyStepEffect(&ctx, map[string]any{"estado": "finalizado", "estado_anterior": "en_revisión"})

	if ctx.Task.Estado != "finalizado" || ctx.Task.EstadoAnterior != "en_revisión" {
		t.Fatalf("el snapshot debería seguir a su propia consecuencia: %+v", ctx.Task)
	}

	// Lo que no tocó el paso sigue congelado: el snapshot no se recarga, sólo se
	// pone al día con lo que esta misma ejecución cambió.
	if ctx.Task.Prioridad != "medium" || ctx.Task.Titulo != "Revisar informe" {
		t.Fatalf("no debería haberse tocado nada más: %+v", ctx.Task)
	}

	applyStepEffect(&ctx, map[string]any{"comentario_id": float64(3)})
	if ctx.Task.Estado != "finalizado" {
		t.Fatal("un paso que no cambia estado no debe alterarlo")
	}
}

// ---------------------------------------------------------------------------
// La receta, tal como se sirve
// ---------------------------------------------------------------------------

// La receta es la única forma de encender esto hoy, así que su definición es parte
// del comportamiento: un paso sin condición se ejecutaría en las DOS ramas, y una
// revisión aprobada acabaría además devuelta al principio.
func TestVeredicto_LaRecetaSeparaLasDosRamas(t *testing.T) {
	r, ok := findRecipe("puerta_veredicto")
	if !ok {
		t.Fatal("la receta del veredicto debería estar en el catálogo")
	}
	if err := ValidateGateForm(r.FormSchema); err != nil {
		t.Fatalf("el formulario de la receta tiene que ser válido: %v", err)
	}
	if !r.NeedsPhase || !r.Mutates {
		t.Fatal("una puerta que mueve tarjetas debe pedir columna y anunciarse como que modifica")
	}

	aprobados, rechazados := 0, 0
	for i, st := range r.Steps {
		if st.Conditions == "" {
			t.Fatalf("el paso %d se ejecutaría en las dos ramas", i)
		}
		switch {
		case strings.Contains(st.Conditions, `"aprobado"`):
			aprobados++
		case strings.Contains(st.Conditions, `"rechazado"`):
			rechazados++
		default:
			t.Fatalf("el paso %d no pertenece a ninguna rama: %s", i, st.Conditions)
		}
	}
	if aprobados == 0 || rechazados == 0 {
		t.Fatalf("faltan pasos en alguna rama: %d aprobados, %d rechazados", aprobados, rechazados)
	}

	// Y las dos ramas responden a lo que el formulario deja elegir: una opción sin
	// consecuencia sería una promesa incumplida en la pantalla.
	form, err := parseGateForm(r.FormSchema)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range form.Fields {
		if f.Key != "veredicto" {
			continue
		}
		if len(f.Options) != 2 {
			t.Fatalf("el veredicto debería tener exactamente las dos opciones que el motor sabe ejecutar, got %d", len(f.Options))
		}
	}
}

// ---------------------------------------------------------------------------
// El catálogo manda sobre la copia guardada
// ---------------------------------------------------------------------------

// Al encender una receta sus pasos se copian a la fila, y esa copia envejece:
// corregir el mensaje de una receta no llegaba a quien ya la tenía encendida.
func TestReceta_ElTextoDeLosPasosSigueAlCatalogo(t *testing.T) {
	r, _ := findRecipe("puerta_veredicto")
	wf := &models.Workflow{
		ID: 5, RecipeKey: "puerta_veredicto",
		Steps: make([]models.WorkflowStep, len(r.Steps)),
	}
	for i, st := range r.Steps {
		// Lo que había guardado: los mismos pasos, con texto y condiciones viejos.
		wf.Steps[i] = models.WorkflowStep{
			ID: uint(i + 1), Order: i, ActionType: st.ActionType,
			Config: `{"content":"texto viejo"}`, Conditions: "{}",
		}
	}

	got := refreshedSteps(wf)
	if len(got) != len(r.Steps) {
		t.Fatalf("deberían seguir siendo %d pasos, got %d", len(r.Steps), len(got))
	}
	for i, st := range got {
		if st.ID != uint(i+1) {
			t.Fatalf("el paso %d perdió su id: %+v", i, st)
		}
		if st.Config != r.Steps[i].Config || st.Conditions != r.Steps[i].Conditions {
			t.Fatalf("el paso %d no se refrescó desde el catálogo: %+v", i, st)
		}
	}
}

// Un cambio de FORMA no es un retoque de redacción: manda lo guardado, que es lo que
// la gente encendió, y el cambio pide una migración explícita.
func TestReceta_UnCambioDeFormaRespetaLoGuardado(t *testing.T) {
	guardados := []models.WorkflowStep{
		{ID: 1, Order: 0, ActionType: models.ActionNotify, Config: `{"title":"guardado"}`},
	}

	// Menos pasos que la receta actual.
	corta := &models.Workflow{ID: 5, RecipeKey: "puerta_veredicto", Steps: guardados}
	if got := refreshedSteps(corta); got[0].Config != `{"title":"guardado"}` {
		t.Fatalf("con otro número de pasos debe respetarse lo guardado: %+v", got[0])
	}

	// Y una regla sin receta manda siempre con lo suyo.
	propia := &models.Workflow{ID: 6, Steps: guardados}
	if got := refreshedSteps(propia); got[0].Config != `{"title":"guardado"}` {
		t.Fatalf("una regla sin receta usa sus propios pasos: %+v", got[0])
	}
}

// La receta retirada no puede reaparecer: encenderla otra vez devolvería el aviso
// duplicado que motivó quitarla.
func TestReceta_LaDeChatAlAsignarYaNoSeOfrece(t *testing.T) {
	if _, ok := findRecipe("asignacion_por_chat"); ok {
		t.Fatal("asignacion_por_chat duplica el DM nativo de Tareas: no debe estar en el catálogo")
	}
}

// ---------------------------------------------------------------------------
// Borrar una columna vigilada
// ---------------------------------------------------------------------------

// Borrar la columna que vigila una puerta encendida deja la regla apuntando al vacío:
// viva en la pantalla y muda en el motor. El tablero pregunta antes de borrar, y esto
// es lo que responde.
func TestPuerta_DiceSiVigilaUnaColumna(t *testing.T) {
	puerta := models.Workflow{
		ID: 5, TenantID: 42, BoardID: 1, Enabled: true,
		TriggerType:   models.TriggerTaskEnteringPhase,
		TriggerConfig: `{"phase_id":12}`, Name: "Exigir entrega",
	}
	apagada := models.Workflow{
		ID: 6, TenantID: 42, BoardID: 1, Enabled: false,
		TriggerType:   models.TriggerTaskEnteringPhase,
		TriggerConfig: `{"phase_id":13}`, Name: "Exigir reporte",
	}
	reactiva := models.Workflow{
		ID: 7, TenantID: 42, BoardID: 1, Enabled: true,
		TriggerType: models.TriggerTaskStatusChanged, Name: "Aviso",
	}
	s := newWfService(&wfRepo{rules: []models.Workflow{puerta, apagada, reactiva}}, nil, nil, nil)

	if nombre, enUso := s.PhaseInUse(42, 1, 12); !enUso || nombre != "Exigir entrega" {
		t.Fatalf("debería frenar el borrado nombrando la regla, got %q / %v", nombre, enUso)
	}
	// Una puerta APAGADA no frena nada: no está impidiendo trabajar a nadie, y su
	// tarjeta ya avisa cuando la columna desaparece.
	if _, enUso := s.PhaseInUse(42, 1, 13); enUso {
		t.Fatal("una puerta apagada no debería impedir borrar la columna")
	}
	// Y una columna que nadie vigila se borra sin más.
	if _, enUso := s.PhaseInUse(42, 1, 99); enUso {
		t.Fatal("ninguna regla vigila esa columna")
	}
	// El tenant tiene que ser el del tablero: con otro, no se ve nada.
	if _, enUso := s.PhaseInUse(77, 1, 12); enUso {
		t.Fatal("no debería alcanzar reglas de otra empresa")
	}
}

// ---------------------------------------------------------------------------
// Cupo por empresa
// ---------------------------------------------------------------------------

// El antibucle impide que las reglas se llamen entre sí. El cupo es para el otro
// lado: una persona que importa doscientas tareas dispararía un aviso por cada una.
func TestCupo_CortaLaAvalanchaYNoTocaALasDemasEmpresas(t *testing.T) {
	q := newRunQuota(2)
	ahora := time.Date(2026, 8, 25, 10, 30, 0, 0, time.UTC)

	if !q.allow(ahora, 42) || !q.allow(ahora, 42) {
		t.Fatal("las dos primeras entran en el cupo")
	}
	if q.allow(ahora, 42) {
		t.Fatal("la tercera debería quedarse fuera")
	}
	// El cupo es POR empresa: la avalancha de una no puede dejar sin avisos a otra.
	if !q.allow(ahora, 77) {
		t.Fatal("otra empresa tiene su propio cupo")
	}
	// Y se renueva con la hora.
	if !q.allow(ahora.Add(time.Hour), 42) {
		t.Fatal("a la hora siguiente vuelve a haber cupo")
	}
}

func TestCupo_SinLimiteNoFrenaNada(t *testing.T) {
	q := newRunQuota(0)
	ahora := time.Date(2026, 8, 25, 10, 0, 0, 0, time.UTC)
	for i := 0; i < 1000; i++ {
		if !q.allow(ahora, 42) {
			t.Fatalf("sin límite no debería frenar (iteración %d)", i)
		}
	}
	// Y un motor sin cupo cableado tampoco: no todas las instancias lo configuran.
	var sinCupo *runQuota
	if !sinCupo.allow(ahora, 42) {
		t.Fatal("sin cupo cableado se deja pasar")
	}
}

// ---------------------------------------------------------------------------
// El enlace de los avisos
// ---------------------------------------------------------------------------

// A un aviso de automatización se llega sin contexto: nadie estaba mirando esa tarjeta
// cuando saltó. Un enlace a "/tasks" deja al lector buscando a mano la tarea de la que
// le acaban de hablar.
func TestAviso_ElEnlaceLlevaALaTarjetaYNoALaPantalla(t *testing.T) {
	ev := WorkflowEvent{
		Type:     models.TriggerTaskStatusChanged,
		TenantID: 42,
		Task:     taskFor(3, models.TaskStatusInProcess),
	}
	ctx := buildContext(ev, "Ana")

	if ctx.Empresa != 42 {
		t.Fatalf("el snapshot tiene que llevar la empresa para poder enlazar: %+v", ctx.Empresa)
	}
	// Los tres parámetros: la tarea, su tablero y su empresa. Sin tablero el enlace
	// se queda a medias si quien lee tenía otro abierto; sin empresa, un superadmin
	// ni siquiera ve ese tablero hasta cambiar de foco.
	for _, quiero := range []string{"task=100", "board=1", "company=42"} {
		if !strings.Contains(ctx.Task.Enlace, quiero) {
			t.Fatalf("al enlace le falta %q: %q", quiero, ctx.Task.Enlace)
		}
	}
}

func TestEnlace_OmiteLoQueNoSabe(t *testing.T) {
	// Sin tablero ni empresa conocidos, el enlace no inventa parámetros vacíos: eso
	// dejaría a la pantalla buscando un tablero 0 que no existe.
	if got := taskDeepLink(7, 0, 0); got != "/tasks?task=7" {
		t.Fatalf("got %q", got)
	}
}

// ---------------------------------------------------------------------------
// Avisos omitidos por deduplicación
// ---------------------------------------------------------------------------

// La deduplicación existe para que el aviso nativo de Tareas y el de una regla no
// lleguen los dos por el mismo cambio. Cuando se lo traga, el paso NO entregó nada: si
// lo apunta como "notificado", quien revise el registro buscando por qué no le llegó
// el aviso encuentra una mentira.
func TestAviso_ElOmitidoPorDuplicadoNoSeApuntaComoEntregado(t *testing.T) {
	actual := &models.Task{ID: 100, Status: models.TaskStatusInProcess, Priority: models.PriorityMedium}
	s, _ := mutSvc(actual, map[uint]*models.User{7: activeUser(7, "Ana")})
	s.notifSvc = &notifSuprimido{}

	ctx := snapshotCtx("en_proceso", "medium", 7)
	out, skip, err := s.runAction(
		mutStep(models.ActionNotify, stepConfig{Recipient: models.RecipientAssignees, Title: "X", Message: "Y"}),
		mutRun(), ctx)

	if err != nil {
		t.Fatalf("omitir no es fallar: %v", err)
	}
	if skip == "" {
		t.Fatal("si no se entregó nada, el paso tiene que decir por qué")
	}
	if notificados, _ := out["notificados"].([]uint); len(notificados) != 0 {
		t.Fatalf("nadie recibió el aviso: %+v", out)
	}
	if omitidos, _ := out["omitidos_por_duplicado"].([]uint); len(omitidos) != 1 {
		t.Fatalf("el registro tiene que decir a quién se omitió: %+v", out)
	}
}

// notifSuprimido simula la deduplicación tragándose todos los avisos.
type notifSuprimido struct{ fakeNotifSvc }

func (n *notifSuprimido) CreateNotificationChecked(_ uint, _, _, _ string, _ map[string]interface{}) (bool, error) {
	return false, ErrNotificationSuppressed
}
