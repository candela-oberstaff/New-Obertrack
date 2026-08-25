package service

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/obertrack/backend/internal/models"
)

// El alcance por tablero es lo que impide que una automatización se convierta en
// una puerta trasera: el rol dice si puedes configurar reglas, el tablero dice
// cuáles. Estas pruebas fijan esa frontera.

func boardOf(tenantID, creator uint, members ...models.User) *models.Board {
	return &models.Board{ID: 1, Name: "Proyecto", TenantID: tenantID, CreatedBy: creator, Members: members}
}

func adminSvc(board *models.Board) (*WorkflowService, *wfRepo) {
	repo := &wfRepo{}
	return newWfService(repo, board, map[uint]*models.User{}, nil), repo
}

// ---------------------------------------------------------------------------
// Portero del módulo
// ---------------------------------------------------------------------------

func TestCanConfigureWorkflows(t *testing.T) {
	casos := []struct {
		nombre string
		actor  WorkflowActor
		quiero bool
	}{
		{"superadmin", WorkflowActor{IsSuperadmin: true}, true},
		{"empleador", WorkflowActor{IsEmployer: true, TenantID: 42}, true},
		{"manager", WorkflowActor{IsManager: true, TenantID: 42}, true},
		// Un profesional raso no entra ni a mirar el catálogo. Es fail-closed a
		// propósito: el default permisivo del RBAC no vale para un módulo que
		// dispara correos.
		{"profesional", WorkflowActor{UserID: 7, TenantID: 42}, false},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			if got := CanConfigureWorkflows(c.actor); got != c.quiero {
				t.Fatalf("se esperaba %v y salió %v", c.quiero, got)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Alcance por tablero
// ---------------------------------------------------------------------------

func TestAlcance_ElEmpleadorAlcanzaLosTablerosDeSuEmpresa(t *testing.T) {
	s, _ := adminSvc(boardOf(42, 9))
	if _, err := s.authorizeBoard(WorkflowActor{UserID: 42, TenantID: 42, IsEmployer: true}, 1); err != nil {
		t.Fatalf("el empleador debería alcanzar su propio tablero: %v", err)
	}
}

// La frontera de empresa se comprueba ANTES que cualquier privilegio de rol: ser
// empleador no significa nada fuera del propio tenant.
func TestAlcance_NadieAlcanzaUnTableroDeOtraEmpresa(t *testing.T) {
	s, _ := adminSvc(boardOf(77, 9))
	_, err := s.authorizeBoard(WorkflowActor{UserID: 42, TenantID: 42, IsEmployer: true}, 1)
	if !errors.Is(err, ErrWorkflowBoardScope) {
		t.Fatalf("un tablero de otra empresa debe quedar fuera de alcance, got %v", err)
	}
}

func TestAlcance_UnManagerSoloAlcanzaSusTableros(t *testing.T) {
	// Es manager de la empresa, pero NO miembro de este tablero. En el módulo de
	// Tareas eso le bastaría; aquí no, y es la restricción más estricta que se
	// adoptó a propósito para este módulo.
	s, _ := adminSvc(boardOf(42, 9))
	_, err := s.authorizeBoard(WorkflowActor{UserID: 7, TenantID: 42, IsManager: true}, 1)
	if !errors.Is(err, ErrWorkflowBoardScope) {
		t.Fatalf("un manager ajeno al tablero no debe configurarlo, got %v", err)
	}

	// Miembro del tablero: sí.
	s2, _ := adminSvc(boardOf(42, 9, models.User{ID: 7}))
	if _, err := s2.authorizeBoard(WorkflowActor{UserID: 7, TenantID: 42, IsManager: true}, 1); err != nil {
		t.Fatalf("un manager miembro del tablero sí debería poder: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Recetas
// ---------------------------------------------------------------------------

func TestRecetas_ElCatalogoLlegaApagadoYSinMaterializar(t *testing.T) {
	s, _ := adminSvc(boardOf(42, 9))
	got, err := s.Recipes(WorkflowActor{UserID: 42, TenantID: 42, IsEmployer: true}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(workflowRecipes) {
		t.Fatalf("el catálogo debería traer %d recetas, trae %d", len(workflowRecipes), len(got))
	}
	for _, r := range got {
		if r.Exists || r.Enabled {
			t.Fatalf("una receta sin activar no puede aparecer como existente ni encendida: %+v", r)
		}
		if r.Explain == "" {
			t.Fatalf("la receta %q debe explicar a quién avisa: en esta pantalla el destinatario no se elige", r.Key)
		}
	}
}

func TestRecetas_ActivarMaterializaLaReglaUnaSolaVez(t *testing.T) {
	s, repo := adminSvc(boardOf(42, 9))
	actor := WorkflowActor{UserID: 42, TenantID: 42, IsEmployer: true}

	wf, err := s.SetRecipeEnabled(actor, 1, "prioridad_urgente", true, 0)
	if err != nil {
		t.Fatal(err)
	}
	if wf == nil || !wf.Enabled {
		t.Fatal("activar debería crear la regla encendida")
	}
	if wf.BoardID != 1 || wf.TenantID != 42 || wf.RecipeKey != "prioridad_urgente" {
		t.Fatalf("la regla no quedó acotada correctamente: %+v", wf)
	}
	// El alcance se hereda de quien la encendió, y el runner lo revalida en cada
	// ejecución: tiene que ser una persona real, no una cuenta genérica.
	if wf.CreatedBy != 42 {
		t.Fatalf("CreatedBy debería ser quien la activó, got %d", wf.CreatedBy)
	}
	if len(wf.Steps) != 2 {
		t.Fatalf("la receta de urgencia tiene 2 pasos, se materializaron %d", len(wf.Steps))
	}

	// Apagar y volver a encender NO debe duplicar la regla: se conserva con su
	// historial de ejecuciones.
	if _, err := s.SetRecipeEnabled(actor, 1, "prioridad_urgente", false, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetRecipeEnabled(actor, 1, "prioridad_urgente", true, 0); err != nil {
		t.Fatal(err)
	}
	if len(repo.rules) != 1 {
		t.Fatalf("el ciclo apagar/encender no debe duplicar la regla, hay %d", len(repo.rules))
	}
	if !repo.rules[0].Enabled {
		t.Fatal("tras volver a encenderla debería quedar activa")
	}
}

// Apagar una receta que nunca se activó no crea nada: dejar la fila apagada sólo
// sería basura.
func TestRecetas_ApagarUnaQueNoExisteNoCreaNada(t *testing.T) {
	s, repo := adminSvc(boardOf(42, 9))
	wf, err := s.SetRecipeEnabled(WorkflowActor{UserID: 42, TenantID: 42, IsEmployer: true}, 1, "creada_sin_fecha", false, 0)
	if err != nil {
		t.Fatal(err)
	}
	if wf != nil || len(repo.rules) != 0 {
		t.Fatalf("no debería haberse creado nada, hay %d reglas", len(repo.rules))
	}
}

func TestRecetas_UnaClaveDesconocidaSeRechaza(t *testing.T) {
	s, _ := adminSvc(boardOf(42, 9))
	_, err := s.SetRecipeEnabled(WorkflowActor{UserID: 42, TenantID: 42, IsEmployer: true}, 1, "no-existe", true, 0)
	if !errors.Is(err, ErrRecipeNotFound) {
		t.Fatalf("se esperaba ErrRecipeNotFound, got %v", err)
	}
}

// Sin alcance sobre el tablero no se puede activar nada, aunque el rol lo permita.
func TestRecetas_SinAlcanceNoSePuedeActivar(t *testing.T) {
	s, repo := adminSvc(boardOf(42, 9))
	_, err := s.SetRecipeEnabled(WorkflowActor{UserID: 7, TenantID: 42, IsManager: true}, 1, "prioridad_urgente", true, 0)
	if !errors.Is(err, ErrWorkflowBoardScope) {
		t.Fatalf("se esperaba un fallo de alcance, got %v", err)
	}
	if len(repo.rules) != 0 {
		t.Fatal("no debería haberse creado ninguna regla")
	}
}

// Las recetas se materializan como reglas normales: el motor no las distingue, y el
// constructor de la Fase 4 podrá editarlas sin ningún caso especial. Esta prueba fija
// que lo materializado sea ejecutable de verdad por el emisor.
func TestRecetas_LoMaterializadoLoReconoceElEmisor(t *testing.T) {
	s, repo := adminSvc(boardOf(42, 9))
	actor := WorkflowActor{UserID: 42, TenantID: 42, IsEmployer: true}
	if _, err := s.SetRecipeEnabled(actor, 1, "creada_sin_fecha", true, 0); err != nil {
		t.Fatal(err)
	}

	// Una tarea creada SIN fecha de fin en ese tablero: la regla debe encolarse.
	s.OnEvent(WorkflowEvent{
		Type: models.TriggerTaskCreated, TenantID: 42,
		Task: taskFor(1, models.TaskStatusTodo), ActorID: 9,
	})
	if len(repo.queued) != 1 {
		t.Fatalf("la receta activada debería haber encolado una ejecución, hay %d", len(repo.queued))
	}

	// Y sus condiciones deben cumplirse contra el contexto guardado.
	var ctx WorkflowContext
	if err := json.Unmarshal([]byte(repo.queued[0].Context), &ctx); err != nil {
		t.Fatal(err)
	}
	recipe, _ := findRecipe("creada_sin_fecha")
	ok, why := evalConditions(recipe.Conditions, conditionFields(ctx))
	if !ok {
		t.Fatalf("las condiciones de la receta deberían cumplirse para una tarea sin fecha: %s", why)
	}
}

// ---------------------------------------------------------------------------
// Recetas de PUERTA: exigen columna
// ---------------------------------------------------------------------------

func boardWithPhases(tenantID uint) *models.Board {
	return &models.Board{
		ID: 1, Name: "Proyecto", TenantID: tenantID, CreatedBy: 9,
		Phases: []models.Phase{
			{ID: 11, Name: "Por hacer", Status: "por_hacer"},
			{ID: 12, Name: "En revisión"},
		},
	}
}

// Una puerta sin columna sería un peaje en todas las columnas del tablero, incluida
// aquella de la que se sale. Se rechaza antes de crear nada.
func TestPuertaReceta_SinColumnaNoSeActiva(t *testing.T) {
	s, repo := adminSvc(boardWithPhases(42))
	actor := WorkflowActor{UserID: 42, TenantID: 42, IsEmployer: true}

	_, err := s.SetRecipeEnabled(actor, 1, "puerta_entrega", true, 0)
	if !errors.Is(err, ErrPhaseRequired) {
		t.Fatalf("se esperaba ErrPhaseRequired, got %v", err)
	}
	if len(repo.rules) != 0 {
		t.Fatal("no debería haberse creado ninguna regla")
	}
}

// Y la columna tiene que ser de ESTE tablero: sin la comprobación, un phase_id ajeno
// pondría el punto de control donde no toca.
func TestPuertaReceta_LaColumnaDebeSerDelTablero(t *testing.T) {
	s, repo := adminSvc(boardWithPhases(42))
	actor := WorkflowActor{UserID: 42, TenantID: 42, IsEmployer: true}

	_, err := s.SetRecipeEnabled(actor, 1, "puerta_entrega", true, 999)
	if !errors.Is(err, ErrPhaseNotInBoard) {
		t.Fatalf("se esperaba ErrPhaseNotInBoard, got %v", err)
	}
	if len(repo.rules) != 0 {
		t.Fatal("no debería haberse creado ninguna regla")
	}
}

func TestPuertaReceta_ActivarDejaLaPuertaLista(t *testing.T) {
	s, _ := adminSvc(boardWithPhases(42))
	actor := WorkflowActor{UserID: 42, TenantID: 42, IsEmployer: true}

	wf, err := s.SetRecipeEnabled(actor, 1, "puerta_entrega", true, 12)
	if err != nil {
		t.Fatal(err)
	}
	if wf.TriggerType != models.TriggerTaskEnteringPhase {
		t.Fatalf("debería ser una puerta, got %q", wf.TriggerType)
	}
	if phaseIDOf(*wf) != 12 {
		t.Fatalf("debería quedar atada a la columna 12, got %d", phaseIDOf(*wf))
	}
	// Y el formulario tiene que ser utilizable: si no, la columna quedaría cerrada.
	if err := ValidateGateForm(wf.FormSchema); err != nil {
		t.Fatalf("la receta materializó un formulario inválido: %v", err)
	}

	// El catálogo devuelve la columna elegida, que es lo que la pantalla necesita
	// para no volver a preguntarla.
	estados, err := s.Recipes(actor, 1)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range estados {
		if e.Key != "puerta_entrega" {
			continue
		}
		if !e.NeedsPhase {
			t.Fatal("una receta de puerta debe declarar que necesita columna")
		}
		if e.PhaseID != 12 {
			t.Fatalf("debería recordar la columna elegida, got %d", e.PhaseID)
		}
		return
	}
	t.Fatal("la receta de puerta no aparece en el catálogo")
}

// Las recetas reactivas siguen sin pedir columna: sólo las puertas la exigen.
func TestPuertaReceta_LasReactivasNoPidenColumna(t *testing.T) {
	s, _ := adminSvc(boardWithPhases(42))
	actor := WorkflowActor{UserID: 42, TenantID: 42, IsEmployer: true}

	if _, err := s.SetRecipeEnabled(actor, 1, "prioridad_urgente", true, 0); err != nil {
		t.Fatalf("una receta reactiva no debería exigir columna: %v", err)
	}
}

// Todas las recetas del catálogo tienen que ser materializables. Si alguien añade una
// con el formulario mal escrito, el sitio donde debe fallar es aquí y no en
// producción, con una columna que nadie puede cruzar.
func TestCatalogo_TodasLasPuertasTienenFormularioValido(t *testing.T) {
	for _, r := range workflowRecipes {
		if r.TriggerType != models.TriggerTaskEnteringPhase {
			continue
		}
		if !r.NeedsPhase {
			t.Fatalf("la receta de puerta %q debe exigir columna", r.Key)
		}
		if err := ValidateGateForm(r.FormSchema); err != nil {
			t.Fatalf("la receta %q tiene un formulario inválido: %v", r.Key, err)
		}
	}
}

// Equivocarse de columna al activar una puerta no puede obligar a borrarla y volver a
// crearla: eso perdería su historial de ejecuciones. La columna se cambia en sitio.
func TestPuertaReceta_LaColumnaSePuedeCambiarDespues(t *testing.T) {
	s, repo := adminSvc(boardWithPhases(42))
	actor := WorkflowActor{UserID: 42, TenantID: 42, IsEmployer: true}

	wf, err := s.SetRecipeEnabled(actor, 1, "puerta_entrega", true, 12)
	if err != nil {
		t.Fatal(err)
	}
	idOriginal := wf.ID

	// Se mueve el punto de control a otra columna del mismo tablero.
	moved, err := s.SetRecipeEnabled(actor, 1, "puerta_entrega", true, 11)
	if err != nil {
		t.Fatal(err)
	}
	if moved.ID != idOriginal {
		t.Fatalf("debería seguir siendo la MISMA regla, era %d y ahora %d", idOriginal, moved.ID)
	}
	if phaseIDOf(*moved) != 11 {
		t.Fatalf("la columna debería haber cambiado a 11, got %d", phaseIDOf(*moved))
	}
	if len(repo.rules) != 1 {
		t.Fatalf("cambiar de columna no debe duplicar la regla, hay %d", len(repo.rules))
	}
}

// Y cambiarla apagada tampoco la enciende de rebote: son dos operaciones distintas.
func TestPuertaReceta_CambiarLaColumnaNoTocaElInterruptor(t *testing.T) {
	s, _ := adminSvc(boardWithPhases(42))
	actor := WorkflowActor{UserID: 42, TenantID: 42, IsEmployer: true}

	if _, err := s.SetRecipeEnabled(actor, 1, "puerta_entrega", true, 12); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetRecipeEnabled(actor, 1, "puerta_entrega", false, 12); err != nil {
		t.Fatal(err)
	}

	// Apagada, se le cambia la columna: debe seguir apagada.
	moved, err := s.SetRecipeEnabled(actor, 1, "puerta_entrega", false, 11)
	if err != nil {
		t.Fatal(err)
	}
	if moved.Enabled {
		t.Fatal("cambiar de columna no debe encender la puerta")
	}
	if phaseIDOf(*moved) != 11 {
		t.Fatalf("la columna debería ser 11, got %d", phaseIDOf(*moved))
	}
}

// La columna nueva sigue teniendo que ser del tablero.
func TestPuertaReceta_NoSeMueveAUnaColumnaAjena(t *testing.T) {
	s, _ := adminSvc(boardWithPhases(42))
	actor := WorkflowActor{UserID: 42, TenantID: 42, IsEmployer: true}

	if _, err := s.SetRecipeEnabled(actor, 1, "puerta_entrega", true, 12); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetRecipeEnabled(actor, 1, "puerta_entrega", true, 999); !errors.Is(err, ErrPhaseNotInBoard) {
		t.Fatalf("se esperaba ErrPhaseNotInBoard, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Recetas que MODIFICAN la tarea
// ---------------------------------------------------------------------------

// Una receta que reasigna o reprioriza cambia el tablero sola. Que lo declare no es
// cosmético: es lo que permite avisar a quien la enciende de que no está activando
// una campanita. Si alguien añade una receta con acciones que mutan y olvida la
// marca, esto falla aquí y no en el tablero de un cliente.
func TestCatalogo_LasRecetasQueMutanLoDeclaran(t *testing.T) {
	muta := map[string]bool{
		models.ActionSetPriority: true,
		models.ActionSetStatus:   true,
		models.ActionAssign:      true,
		models.ActionComment:     true,
	}

	for _, r := range workflowRecipes {
		tieneMutacion := false
		for _, st := range r.Steps {
			if muta[st.ActionType] {
				tieneMutacion = true
				break
			}
		}
		if tieneMutacion && !r.Mutates {
			t.Fatalf("la receta %q modifica la tarea y no lo declara", r.Key)
		}
		if !tieneMutacion && r.Mutates {
			t.Fatalf("la receta %q dice que modifica y sólo avisa", r.Key)
		}
	}
}

// Toda receta tiene que ser materializable con pasos válidos: el sitio donde debe
// romper un error de catálogo es la suite, no el tablero de una empresa.
func TestCatalogo_TodasLasRecetasSonCoherentes(t *testing.T) {
	conocida := map[string]bool{
		models.ActionNotify: true, models.ActionChatDM: true, models.ActionEmail: true,
		models.ActionSetPriority: true, models.ActionSetStatus: true,
		models.ActionAssign: true, models.ActionComment: true,
	}
	vistas := map[string]bool{}

	for _, r := range workflowRecipes {
		if vistas[r.Key] {
			t.Fatalf("la clave de receta %q está repetida", r.Key)
		}
		vistas[r.Key] = true

		if r.Name == "" || r.Explain == "" {
			t.Fatalf("la receta %q necesita nombre y explicación: son lo único que se lee antes de encenderla", r.Key)
		}
		if len(r.Steps) == 0 && r.TriggerType != models.TriggerTaskEnteringPhase {
			t.Fatalf("la receta %q no hace nada", r.Key)
		}
		for i, st := range r.Steps {
			if !conocida[st.ActionType] {
				t.Fatalf("la receta %q usa una acción desconocida en el paso %d: %q", r.Key, i+1, st.ActionType)
			}
			var cfg stepConfig
			if err := json.Unmarshal([]byte(nonEmptyJSON(st.Config)), &cfg); err != nil {
				t.Fatalf("la receta %q tiene el paso %d ilegible: %v", r.Key, i+1, err)
			}
		}
		// Las condiciones se evalúan contra un contexto vacío sólo para comprobar
		// que el árbol se puede interpretar; que den true o false da igual aquí.
		if r.Conditions != "" {
			if _, why := evalConditions(r.Conditions, map[string]any{}); strings.Contains(why, "no se pudieron interpretar") {
				t.Fatalf("la receta %q tiene condiciones ilegibles: %s", r.Key, why)
			}
		}
	}
}

// Una regla pertenece a la empresa DEL TABLERO, no a quien la enciende. El superadmin
// tiene inquilino 0: guardarla con el suyo creaba una regla que ninguna empresa veía y
// que el motor —que busca por el inquilino de la tarea— no disparaba nunca.
func TestReceta_LaEnciendeElSuperadminYPerteneceALaEmpresa(t *testing.T) {
	repo := &wfRepo{}
	board := &models.Board{ID: 7, TenantID: 42, CreatedBy: 9}
	s := newWfService(repo, board, nil, nil)

	superadmin := WorkflowActor{UserID: 1, TenantID: 0, IsSuperadmin: true}
	wf, err := s.SetRecipeEnabled(superadmin, 7, "creada_sin_fecha", true, 0)
	if err != nil {
		t.Fatal(err)
	}
	if wf.TenantID != 42 {
		t.Fatalf("la regla debería ser de la empresa del tablero, got tenant %d", wf.TenantID)
	}
}

// Y por eso mismo el superadmin tiene que ENCONTRAR la que la empresa ya tenía: si no,
// crea una segunda copia y la empresa recibe cada aviso dos veces.
func TestReceta_ElSuperadminNoDuplicaLaQueYaExiste(t *testing.T) {
	existente := models.Workflow{
		ID: 3, TenantID: 42, BoardID: 7, RecipeKey: "creada_sin_fecha",
		Enabled: false, TriggerType: models.TriggerTaskCreated, CreatedBy: 9,
	}
	repo := &wfRepo{rules: []models.Workflow{existente}}
	board := &models.Board{ID: 7, TenantID: 42, CreatedBy: 9}
	s := newWfService(repo, board, nil, nil)

	superadmin := WorkflowActor{UserID: 1, TenantID: 0, IsSuperadmin: true}
	wf, err := s.SetRecipeEnabled(superadmin, 7, "creada_sin_fecha", true, 0)
	if err != nil {
		t.Fatal(err)
	}
	if wf.ID != 3 {
		t.Fatalf("debería haber encendido la regla existente, no crear otra: %+v", wf)
	}
	if len(repo.rules) != 1 {
		t.Fatalf("no debería haber una segunda copia: %d reglas", len(repo.rules))
	}
}
