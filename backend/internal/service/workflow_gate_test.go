package service

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/obertrack/backend/internal/models"
)

// Una puerta de fase es lo único del motor que puede IMPEDIR que alguien trabaje.
// Eso pone el listón en dos sitios opuestos: tiene que bloquear de verdad —si sólo
// vive en el modal, un PUT se la salta— y no puede bloquear de más, porque una
// columna inalcanzable paraliza al equipo.

func gateForm() string {
	return `{
      "title": "Entrega para revisión",
      "fields": [
        {"key":"enlace","label":"Enlace de entrega","type":"url","required":true},
        {"key":"resultado","label":"Resultado","type":"select","required":true,
         "options":[{"value":"aprobado","label":"Aprobar"},{"value":"rechazado","label":"Rechazar"}]},
        {"key":"horas","label":"Horas","type":"number","required":false,"min":0,"max":24},
        {"key":"notas","label":"Notas","type":"text","required":false}
      ]}`
}

// gateSvc monta un motor con UNA puerta sobre la fase 12 del tablero 1.
//
// La fase no declara Status, así que su id de columna se deriva del nombre: "En
// revisión" → "en_revisión", CON acento. Es el comportamiento real de las columnas
// personalizadas (PhaseColumnID) y conviene que la prueba lo ejercite tal cual.
func gateSvc(form string) *WorkflowService {
	wf := models.Workflow{
		ID: 5, TenantID: 42, Enabled: true,
		TriggerType:   models.TriggerTaskEnteringPhase,
		BoardID:       1,
		TriggerConfig: `{"phase_id":12}`,
		FormSchema:    form,
		Name:          "Revisión de entrega",
		CreatedBy:     42,
	}
	board := &models.Board{
		ID: 1, TenantID: 42, CreatedBy: 42,
		Phases: []models.Phase{
			{ID: 11, Name: "Por hacer", Status: "por_hacer"},
			{ID: 12, Name: "En revisión"},
		},
	}
	return newWfService(&wfRepo{rules: []models.Workflow{wf}}, board, nil, nil)
}

// ---------------------------------------------------------------------------
// Validación del esquema (al configurar la puerta)
// ---------------------------------------------------------------------------

// Una puerta mal formada deja una columna inalcanzable. El momento de detectarlo es
// al guardarla, no cuando alguien intente mover una tarjeta y no pueda.
func TestValidateGateForm_RechazaEsquemasQueBloquearianLaColumna(t *testing.T) {
	casos := []struct {
		nombre string
		json   string
		motivo string
	}{
		{"sin campos", `{"fields":[]}`, "no tiene campos"},
		{"vacío", `{}`, "no tiene formulario"},
		{"clave repetida", `{"fields":[
			{"key":"a","label":"A","type":"text"},
			{"key":"a","label":"B","type":"text"}]}`, "repetida"},
		{"sin etiqueta", `{"fields":[{"key":"a","label":"","type":"text"}]}`, "etiqueta"},
		{"tipo desconocido", `{"fields":[{"key":"a","label":"A","type":"firma"}]}`, "tipo desconocido"},
		// El caso más dañino: un select obligatorio sin opciones no se puede
		// responder, así que nadie podría entrar nunca en esa columna.
		{"selección sin opciones", `{"fields":[{"key":"a","label":"A","type":"select","required":true}]}`, "no tiene opciones"},
		{"mínimo mayor que máximo", `{"fields":[{"key":"a","label":"A","type":"number","min":10,"max":2}]}`, "mínimo mayor"},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			err := ValidateGateForm(c.json)
			if err == nil {
				t.Fatal("debería rechazarse")
			}
			if !strings.Contains(err.Error(), c.motivo) {
				t.Fatalf("el motivo debería mencionar %q, got %q", c.motivo, err)
			}
		})
	}
}

func TestValidateGateForm_AceptaUnEsquemaCorrecto(t *testing.T) {
	if err := ValidateGateForm(gateForm()); err != nil {
		t.Fatalf("el esquema de ejemplo debería ser válido: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Validación de lo enviado
// ---------------------------------------------------------------------------

func TestPuerta_ExigeLoObligatorioYValidaCadaTipo(t *testing.T) {
	s := gateSvc(gateForm())

	casos := []struct {
		nombre string
		envio  map[string]any
		campo  string // campo que debe fallar; "" = debe pasar
	}{
		{"falta lo obligatorio", map[string]any{"resultado": "aprobado"}, "enlace"},
		{"enlace sin esquema", map[string]any{"enlace": "entrega.pdf", "resultado": "aprobado"}, "enlace"},
		{"opción inexistente", map[string]any{"enlace": "https://x.test/a", "resultado": "quizás"}, "resultado"},
		{"número fuera de rango", map[string]any{"enlace": "https://x.test/a", "resultado": "aprobado", "horas": 99}, "horas"},
		{"no es número", map[string]any{"enlace": "https://x.test/a", "resultado": "aprobado", "horas": "muchas"}, "horas"},
		{"completo y válido", map[string]any{"enlace": "https://x.test/a", "resultado": "aprobado", "horas": 6}, ""},
		{"lo opcional puede faltar", map[string]any{"enlace": "https://x.test/a", "resultado": "rechazado"}, ""},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			res, err := s.CheckGate(42, 1, "en_revisión", c.envio)
			if c.campo == "" {
				if err != nil {
					t.Fatalf("debería pasar, got %v", err)
				}
				if res == nil || res.WorkflowID != 5 {
					t.Fatalf("debería devolver la puerta cruzada, got %+v", res)
				}
				return
			}
			var gate *GateRequiredError
			if !errors.As(err, &gate) {
				t.Fatalf("se esperaba un rechazo de puerta, got %v", err)
			}
			if gate.Errors[c.campo] == "" {
				t.Fatalf("debería señalar el campo %q, got %v", c.campo, gate.Errors)
			}
		})
	}
}

// Lo que llegue de más no se guarda: el historial de la tarea no puede acabar
// almacenando lo que a un cliente se le ocurra mandar.
func TestPuerta_DescartaLasClavesNoDeclaradas(t *testing.T) {
	s := gateSvc(gateForm())
	res, err := s.CheckGate(42, 1, "en_revisión", map[string]any{
		"enlace": "https://x.test/a", "resultado": "aprobado",
		"es_admin": true, "inyectado": "lo que sea",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, cuela := res.Data["es_admin"]; cuela {
		t.Fatalf("no debería guardarse una clave ajena al esquema: %v", res.Data)
	}
	if len(res.Data) != 2 {
		t.Fatalf("sólo deberían quedar las dos claves declaradas, got %v", res.Data)
	}
}

// ---------------------------------------------------------------------------
// Resolución de la puerta
// ---------------------------------------------------------------------------

func TestPuerta_SoloAplicaASuColumna(t *testing.T) {
	s := gateSvc(gateForm())

	// La columna de la puerta: bloquea.
	if _, err := s.CheckGate(42, 1, "en_revisión", nil); err == nil {
		t.Fatal("la columna con puerta debería exigir formulario")
	}
	// Cualquier otra: pasa sin preguntar. Es lo que mantiene intacto el Modo Libre.
	if res, err := s.CheckGate(42, 1, "por_hacer", nil); err != nil || res != nil {
		t.Fatalf("una columna sin puerta no debe pedir nada, got %v / %v", res, err)
	}
}

func TestPuerta_NoAlcanzaOtroTableroNiOtraEmpresa(t *testing.T) {
	s := gateSvc(gateForm())
	if res, err := s.CheckGate(42, 99, "en_revisión", nil); err != nil || res != nil {
		t.Fatalf("otro tablero no debe heredar la puerta, got %v / %v", res, err)
	}
	if res, err := s.CheckGate(77, 1, "en_revisión", nil); err != nil || res != nil {
		t.Fatalf("otra empresa no debe ver la puerta, got %v / %v", res, err)
	}
}

// El primer rechazo lleva el formulario completo: es lo que permite que un cliente
// que no conocía la puerta dibuje el modal.
func TestPuerta_ElRechazoLlevaLaDefinicionDelFormulario(t *testing.T) {
	s := gateSvc(gateForm())
	_, err := s.CheckGate(42, 1, "en_revisión", nil)

	var gate *GateRequiredError
	if !errors.As(err, &gate) {
		t.Fatalf("se esperaba GateRequiredError, got %v", err)
	}
	if len(gate.Form.Fields) != 4 || gate.Form.Title == "" {
		t.Fatalf("el formulario debería viajar entero, got %+v", gate.Form)
	}
	if gate.ToStatus != "en_revisión" || gate.WorkflowID != 5 {
		t.Fatalf("faltan datos para reintentar: %+v", gate)
	}
	// Y tiene que poder serializarse tal cual hacia el cliente.
	if _, jerr := json.Marshal(gate); jerr != nil {
		t.Fatalf("el error de puerta debe ser serializable: %v", jerr)
	}
}

// Una puerta con el formulario roto NO puede convertirse en un muro: nadie podría
// completarlo, así que la columna quedaría cerrada para todo el equipo. Se deja pasar
// y se registra el problema.
func TestPuerta_UnFormularioInservibleDejaPasar(t *testing.T) {
	s := gateSvc(`{"fields": [`)
	res, err := s.CheckGate(42, 1, "en_revisión", nil)
	if err != nil || res != nil {
		t.Fatalf("una puerta rota no debe bloquear la columna, got %v / %v", res, err)
	}
}

// gateSvcReceta monta la misma puerta pero MATERIALIZADA DESDE UNA RECETA, con una
// copia vieja del formulario guardada en la fila: el estado real de cualquier puerta
// que alguien encendió antes de que la receta cambiara.
func gateSvcReceta(recipeKey, stored string) *WorkflowService {
	wf := models.Workflow{
		ID: 5, TenantID: 42, Enabled: true,
		TriggerType:   models.TriggerTaskEnteringPhase,
		BoardID:       1,
		TriggerConfig: `{"phase_id":12}`,
		RecipeKey:     recipeKey,
		FormSchema:    stored,
		Name:          "Revisión de entrega",
		CreatedBy:     42,
	}
	board := &models.Board{
		ID: 1, TenantID: 42, CreatedBy: 42,
		Phases: []models.Phase{
			{ID: 11, Name: "Por hacer", Status: "por_hacer"},
			{ID: 12, Name: "En revisión"},
		},
	}
	return newWfService(&wfRepo{rules: []models.Workflow{wf}}, board, nil, nil)
}

// El formulario de una receta lo define el catálogo. Lo guardado en la fila es una
// copia del día en que se encendió: si sigue mandando ella, cambiar un texto obliga a
// migrar todas las puertas ya creadas y, mientras tanto, cada empresa pide una cosa
// distinta.
func TestPuerta_LaRecetaMandaSobreLaCopiaGuardada(t *testing.T) {
	vieja := `{"title":"Vieja","fields":[{"key":"enlace","label":"Enlace","type":"url","required":true}]}`
	s := gateSvcReceta("puerta_entrega", vieja)

	_, err := s.CheckGate(42, 1, "en_revisión", nil)
	var gate *GateRequiredError
	if !errors.As(err, &gate) {
		t.Fatalf("se esperaba GateRequiredError, got %v", err)
	}
	if gate.Form.Title == "Vieja" {
		t.Fatal("la puerta sigue sirviendo la copia guardada en vez del catálogo")
	}
	for _, f := range gate.Form.Fields {
		if f.Key == "enlace" && f.Required {
			t.Fatal("el enlace ya no es obligatorio en la receta: la puerta no se enteró")
		}
	}
}

// Una puerta sin receta —las del constructor propio— sí manda con su esquema: ahí lo
// guardado es la fuente y no una copia.
func TestPuerta_SinRecetaMandaElEsquemaGuardado(t *testing.T) {
	propio := `{"title":"Propio","fields":[{"key":"nota","label":"Nota","type":"text","required":true}]}`
	s := gateSvcReceta("", propio)

	_, err := s.CheckGate(42, 1, "en_revisión", nil)
	var gate *GateRequiredError
	if !errors.As(err, &gate) || gate.Form.Title != "Propio" {
		t.Fatalf("una puerta sin receta debe usar su propio esquema, got %v", err)
	}
}

// Mucho trabajo es interno y no tiene nada que enlazar. Exigir el enlace sólo lograba
// que alguien pegara cualquier URL para poder mover la tarjeta: peor que no pedirlo,
// porque da por buena una evidencia que no lleva a ningún sitio. Lo que se exige es
// DECIR qué entregas.
func TestPuertaEntrega_ElTrabajoInternoPasaSinEnlace(t *testing.T) {
	s := gateSvcReceta("puerta_entrega", "")

	res, err := s.CheckGate(42, 1, "en_revisión", map[string]any{
		"resumen": "Revisión interna con el equipo de soporte; no hay nada que enlazar.",
	})
	if err != nil {
		t.Fatalf("sin enlace pero con resumen debería pasar, got %v", err)
	}
	if res == nil || res.Data["resumen"] == nil {
		t.Fatalf("el resumen tiene que quedar registrado, got %+v", res)
	}
	if _, hay := res.Data["enlace"]; hay {
		t.Fatal("un enlace vacío no debe guardarse")
	}

	// Y seguir dejando constancia sigue siendo obligatorio: la puerta no puede
	// convertirse en un botón de continuar.
	if _, err := s.CheckGate(42, 1, "en_revisión", map[string]any{"enlace": "https://a.test/pr/1"}); err == nil {
		t.Fatal("sin decir qué entregas la puerta no debería abrirse")
	}
}

// ---------------------------------------------------------------------------
// Integración con el movimiento de la tarjeta
// ---------------------------------------------------------------------------

func gatedTaskService(repo *histTaskRepo, gate *WorkflowService) *taskService {
	s := &taskService{
		repo:      repo,
		userRepo:  &dmUserRepo{users: map[uint]*models.User{}},
		boardRepo: &dmBoardRepo{board: &models.Board{ID: 1, TenantID: 42, CreatedBy: 42}},
		notifSvc:  &fakeNotifSvc{},
	}
	s.SetGateChecker(gate.CheckGate)
	return s
}

func TestMover_SinFormularioNoAvanza(t *testing.T) {
	repo := &histTaskRepo{
		initial: &models.Task{ID: 100, BoardID: 1, TenantID: 42, Status: models.TaskStatusTodo},
		final:   &models.Task{ID: 100, BoardID: 1, TenantID: 42, Status: models.TaskStatusTodo},
	}
	s := gatedTaskService(repo, gateSvc(gateForm()))

	_, _, err := s.Update(100, 42, 7, "profesional", false, true,
		map[string]interface{}{"status": "en_revisión"}, nil, nil)

	var gate *GateRequiredError
	if !errors.As(err, &gate) {
		t.Fatalf("se esperaba el rechazo de la puerta, got %v", err)
	}
	// Lo importante: la tarjeta NO se movió.
	if len(repo.updates) != 0 || len(repo.txEntries) != 0 {
		t.Fatalf("no debería haberse escrito nada: updates=%d tx=%d", len(repo.updates), len(repo.txEntries))
	}
}

func TestMover_ConFormularioValidoAvanzaYRegistraEnLaMismaTransaccion(t *testing.T) {
	repo := &histTaskRepo{
		initial: &models.Task{ID: 100, BoardID: 1, TenantID: 42, Status: models.TaskStatusTodo},
		final:   &models.Task{ID: 100, BoardID: 1, TenantID: 42, Status: "en_revisión"},
	}
	s := gatedTaskService(repo, gateSvc(gateForm()))

	_, _, err := s.Update(100, 42, 7, "profesional", false, true,
		map[string]interface{}{"status": "en_revisión"}, nil,
		map[string]any{"enlace": "https://entregas.test/informe.pdf", "resultado": "aprobado"})
	if err != nil {
		t.Fatal(err)
	}

	// Movimiento y registro van juntos: por el camino transaccional, no por el
	// best-effort. Si el registro no se pudiera guardar, el movimiento no ocurriría.
	if len(repo.txEntries) != 1 {
		t.Fatalf("se esperaba una escritura transaccional, hay %d", len(repo.txEntries))
	}
	if len(repo.history) != 0 {
		t.Fatalf("no debe duplicarse la bitácora por el camino best-effort, hay %d", len(repo.history))
	}

	entry := repo.txEntries[0]
	if entry.GateWorkflowID == nil || *entry.GateWorkflowID != 5 {
		t.Fatalf("debe quedar qué puerta se cruzó, got %v", entry.GateWorkflowID)
	}
	if entry.ChangedBy == nil || *entry.ChangedBy != 7 {
		t.Fatalf("debe quedar quién la cruzó, got %v", entry.ChangedBy)
	}
	if entry.FromStatus != "por_hacer" || entry.ToStatus != "en_revisión" {
		t.Fatalf("movimiento mal registrado: %q → %q", entry.FromStatus, entry.ToStatus)
	}
	// Y el formulario, que es el rastro de qué se aportó. Se guarda con ETIQUETA y
	// tipo junto al valor: el formulario de una puerta se puede editar después, y un
	// registro que resolviera las etiquetas contra el esquema vigente reescribiría el
	// pasado de todas las tareas que ya lo respondieron.
	// FormData es puntero: en un movimiento SIN puerta vale nil, que es lo que la
	// columna jsonb necesita para quedar en NULL.
	if entry.FormData == nil {
		t.Fatal("un movimiento con puerta debe llevar el formulario guardado")
	}
	var guardado models.GateSubmission
	if jerr := json.Unmarshal([]byte(*entry.FormData), &guardado); jerr != nil {
		t.Fatalf("el formulario debe quedar guardado como JSON: %v", jerr)
	}
	if len(guardado.Fields) != 2 {
		t.Fatalf("deberían guardarse los dos campos respondidos, got %+v", guardado.Fields)
	}

	enlace := guardado.Fields[0]
	if enlace.Key != "enlace" || enlace.Value != "https://entregas.test/informe.pdf" {
		t.Fatalf("el enlace entregado debería estar en el registro, got %+v", enlace)
	}
	if enlace.Label != "Enlace de entrega" || enlace.Type != "url" {
		t.Fatalf("la etiqueta y el tipo deben viajar con el valor, got %+v", enlace)
	}
	// Y en el orden del formulario, que es como se leerá en la ficha.
	if guardado.Fields[1].Key != "resultado" {
		t.Fatalf("debe conservarse el orden del formulario, got %+v", guardado.Fields)
	}
}

// Editar cualquier otra cosa de una tarea que YA está en la columna con puerta no
// vuelve a cobrar peaje: el formulario de edición reenvía el status actual en cada
// guardado, y exigirlo otra vez convertiría la puerta en un castigo.
func TestMover_EditarSinCambiarDeColumnaNoPideNada(t *testing.T) {
	repo := &histTaskRepo{
		initial: &models.Task{ID: 100, BoardID: 1, TenantID: 42, Status: "en_revisión"},
		final:   &models.Task{ID: 100, BoardID: 1, TenantID: 42, Status: "en_revisión"},
	}
	s := gatedTaskService(repo, gateSvc(gateForm()))

	_, _, err := s.Update(100, 42, 7, "profesional", false, true,
		map[string]interface{}{"title": "Otro título", "status": "en_revisión"}, nil, nil)
	if err != nil {
		t.Fatalf("no debería pedir formulario sin cambio de columna: %v", err)
	}
	if len(repo.updates) != 1 {
		t.Fatalf("la edición debería haberse guardado, updates=%d", len(repo.updates))
	}
}

// El botón "Finalizar tarea" no puede ser un atajo para entrar en una columna con
// puerta sin rellenarla. Una puerta con un agujero conocido no es una puerta.
func TestCompletar_NoEsUnAtajoParaSaltarseLaPuerta(t *testing.T) {
	board := &models.Board{
		ID: 1, TenantID: 42, CreatedBy: 42,
		Phases: []models.Phase{{ID: 12, Name: "Finalizado", Status: "finalizado"}},
	}
	wf := models.Workflow{
		ID: 5, TenantID: 42, Enabled: true,
		TriggerType: models.TriggerTaskEnteringPhase,
		BoardID:     1, TriggerConfig: `{"phase_id":12}`,
		FormSchema: gateForm(), Name: "Cierre", CreatedBy: 42,
	}
	gate := newWfService(&wfRepo{rules: []models.Workflow{wf}}, board, nil, nil)

	repo := &histTaskRepo{
		initial: &models.Task{ID: 100, BoardID: 1, TenantID: 42, Status: models.TaskStatusInProcess},
		final:   &models.Task{ID: 100, BoardID: 1, TenantID: 42, Status: models.TaskStatusInProcess},
	}
	s := gatedTaskService(repo, gate)
	s.boardRepo = &dmBoardRepo{board: board}

	_, err := s.ToggleCompletion(100, 42, 7, "profesional", false, true)

	var required *GateRequiredError
	if !errors.As(err, &required) {
		t.Fatalf("completar hacia una columna con puerta debe exigir el formulario, got %v", err)
	}
	if len(repo.updates) != 0 || len(repo.txEntries) != 0 {
		t.Fatal("la tarea no debería haberse completado")
	}
}

// Regresión. task_status_history.form_data es una columna jsonb, y la cadena vacía no
// es JSON válido: cuando el campo era un string, TODO movimiento sin puerta —la
// inmensa mayoría— intentaba insertar ” y Postgres rechazaba la fila entera. Como la
// bitácora se escribe best-effort, el error se quedaba en el log y el historial se
// vaciaba sin que nada fallara a la vista.
//
// Con puntero el valor por defecto es nil, y esta prueba fija que siga siéndolo: un
// movimiento normal no puede acabar mandando una cadena vacía a una columna jsonb.
func TestMover_UnMovimientoSinPuertaNoEscribeFormularioVacio(t *testing.T) {
	repo := &histTaskRepo{
		initial: &models.Task{ID: 100, BoardID: 1, TenantID: 42, Status: models.TaskStatusTodo},
		final:   &models.Task{ID: 100, BoardID: 1, TenantID: 42, Status: models.TaskStatusInProcess},
	}
	// Sin puerta cableada: el camino corriente.
	s := &taskService{
		repo:      repo,
		userRepo:  &dmUserRepo{users: map[uint]*models.User{}},
		boardRepo: &dmBoardRepo{board: &models.Board{ID: 1, TenantID: 42}},
		notifSvc:  &fakeNotifSvc{},
	}

	if _, _, err := s.Update(100, 42, 7, "profesional", false, true,
		map[string]interface{}{"status": "en_proceso"}, nil, nil); err != nil {
		t.Fatal(err)
	}

	if len(repo.history) != 1 {
		t.Fatalf("el movimiento debería anotarse, hay %d filas", len(repo.history))
	}
	if repo.history[0].FormData != nil {
		t.Fatalf("sin puerta, form_data debe quedar nulo y no una cadena: %q", *repo.history[0].FormData)
	}
}
