package service

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/obertrack/backend/internal/models"
)

// Cara de administración del motor: lo que consume la pantalla de Automatizaciones.
// Toda operación empieza comprobando el alcance sobre el TABLERO, no sólo el rol:
// el rol dice si puedes configurar automatizaciones, el tablero dice cuáles.

var (
	ErrWorkflowBoardScope = errors.New("no tienes acceso a ese tablero")
	ErrWorkflowNotFound   = errors.New("la automatización no existe")
	ErrRecipeNotFound     = errors.New("esa receta no existe")
	// ErrPhaseRequired: una puerta sin columna sería un peaje en todo el tablero.
	ErrPhaseRequired   = errors.New("elige la columna sobre la que actúa")
	ErrPhaseNotInBoard = errors.New("esa columna no pertenece al tablero")
)

// WorkflowActor es quien realiza la operación. Se pasa explícito en vez de leerlo
// del contexto HTTP para que el servicio siga siendo probable sin un request.
type WorkflowActor struct {
	UserID       uint
	TenantID     uint
	IsSuperadmin bool
	IsEmployer   bool
	IsManager    bool
}

// CanConfigureWorkflows: quién puede tocar automatizaciones, con independencia del
// tablero. Un profesional raso no entra, ni siquiera a mirar el catálogo.
//
// Es fail-closed a propósito y no se apoya en RequirePermission: ese middleware deja
// pasar a cualquiera que no tenga roles RBAC asignados, que es un default razonable
// para consultar tareas e inaceptable para un módulo que dispara correos a media
// empresa. El módulo RBAC "workflows" propiamente dicho llega en la Fase 4 y se
// sumará a esta comprobación, no la sustituirá.
func CanConfigureWorkflows(a WorkflowActor) bool {
	return a.IsSuperadmin || a.IsEmployer || a.IsManager
}

// authorizeBoard comprueba que el actor alcance ese tablero: el empleador alcanza
// todos los de su empresa; un manager o supervisor, sólo aquellos de los que es
// miembro o creador.
//
// Devuelve el tablero, y de ahí sale el inquilino de TODO lo que se haga después: una
// regla pertenece a la empresa del tablero sobre el que actúa, no a quien la enciende.
// Usar el del actor rompía con el superadmin, cuyo tenant es 0: creaba reglas
// fantasma que ninguna empresa veía y que el motor nunca disparaba, porque los
// eventos llegan con el inquilino de la tarea.
func (s *WorkflowService) authorizeBoard(a WorkflowActor, boardID uint) (*models.Board, error) {
	if boardID == 0 {
		return nil, ErrWorkflowBoardScope
	}
	board, err := s.boardRepo.GetByID(boardID)
	if err != nil || board == nil {
		return nil, ErrWorkflowBoardScope
	}
	if a.IsSuperadmin {
		return board, nil
	}
	// La frontera de empresa se comprueba SIEMPRE, antes que cualquier privilegio
	// de rol: ser manager no significa nada fuera del propio tenant.
	if board.TenantID != a.TenantID || a.TenantID == 0 {
		return nil, ErrWorkflowBoardScope
	}
	if a.IsEmployer {
		return board, nil
	}
	if board.CreatedBy == a.UserID {
		return board, nil
	}
	for _, m := range board.Members {
		if m.ID == a.UserID {
			return board, nil
		}
	}
	return nil, ErrWorkflowBoardScope
}

// Recipes devuelve el catálogo con el estado de cada receta en ese tablero.
func (s *WorkflowService) Recipes(a WorkflowActor, boardID uint) ([]RecipeState, error) {
	board, err := s.authorizeBoard(a, boardID)
	if err != nil {
		return nil, err
	}

	existing, err := s.repo.ListByBoard(board.TenantID, boardID)
	if err != nil {
		return nil, err
	}
	byKey := make(map[string]models.Workflow, len(existing))
	for _, wf := range existing {
		if wf.RecipeKey != "" {
			byKey[wf.RecipeKey] = wf
		}
	}

	out := make([]RecipeState, 0, len(workflowRecipes))
	for _, r := range workflowRecipes {
		state := RecipeState{WorkflowRecipe: r}
		if wf, ok := byKey[r.Key]; ok {
			state.Exists = true
			state.WorkflowID = wf.ID
			state.Enabled = wf.Enabled
			state.PhaseID = phaseIDOf(wf)
			// Una puerta cuya columna ya no está en el tablero no dispara nunca. Se
			// dice aquí porque es lo único que explica por qué "no hace nada".
			state.PhaseMissing = state.PhaseID != 0 && !boardHasPhase(board, state.PhaseID)
		}
		out = append(out, state)
	}
	return out, nil
}

// SetRecipeEnabled enciende o apaga una receta en un tablero, materializándola la
// primera vez. Es la única escritura que expone la Fase 1: sin constructor, la
// pantalla se reduce a este interruptor.
//
// Apagar NO borra la regla: conserva su historial de ejecuciones y permite volver a
// encenderla sin recrearla.
func (s *WorkflowService) SetRecipeEnabled(a WorkflowActor, boardID uint, recipeKey string, enabled bool, phaseID uint) (*models.Workflow, error) {
	board, err := s.authorizeBoard(a, boardID)
	if err != nil {
		return nil, err
	}
	recipe, ok := findRecipe(recipeKey)
	if !ok {
		return nil, ErrRecipeNotFound
	}

	// Por el inquilino DEL TABLERO: si no, un superadmin no encontraría la regla que
	// la empresa ya tiene encendida y crearía una segunda, duplicando sus avisos.
	wf, err := s.repo.FindByRecipe(board.TenantID, boardID, recipeKey)
	if err != nil {
		return nil, err
	}

	if wf != nil {
		// Cambiar de columna una puerta ya creada. Se hace ANTES de mirar el
		// interruptor porque cambiar sólo la columna es una operación legítima por sí
		// sola: equivocarse al elegirla no puede obligar a borrar la regla y perder su
		// historial de ejecuciones.
		if recipe.NeedsPhase && phaseID != 0 && phaseID != phaseIDOf(*wf) {
			if !boardHasPhase(board, phaseID) {
				return nil, ErrPhaseNotInBoard
			}
			cfg := fmt.Sprintf(`{"phase_id":%d}`, phaseID)
			if err := s.repo.SetTriggerConfig(wf.ID, cfg); err != nil {
				return nil, err
			}
			wf.TriggerConfig = cfg
		}

		if wf.Enabled != enabled {
			if err := s.repo.SetEnabled(wf.ID, enabled); err != nil {
				return nil, err
			}
			wf.Enabled = enabled
		}
		return wf, nil
	}

	// Primera activación: se materializa. Apagar una receta que nunca se activó no
	// tiene nada que hacer, y crear la fila apagada sólo dejaría basura.
	if !enabled {
		return nil, nil
	}

	// Una puerta necesita columna, y esa columna tiene que ser de ESTE tablero: sin
	// la comprobación, un phase_id de otro tablero pondría el peaje donde no toca.
	triggerConfig := "{}"
	if recipe.NeedsPhase {
		if phaseID == 0 {
			return nil, ErrPhaseRequired
		}
		if !boardHasPhase(board, phaseID) {
			return nil, ErrPhaseNotInBoard
		}
		triggerConfig = fmt.Sprintf(`{"phase_id":%d}`, phaseID)
	}

	// El esquema se valida ANTES de guardar: una puerta con un formulario imposible
	// de completar deja la columna inalcanzable para todo el equipo.
	if recipe.FormSchema != "" {
		if err := ValidateGateForm(recipe.FormSchema); err != nil {
			return nil, fmt.Errorf("la receta %q tiene un formulario inválido: %w", recipe.Key, err)
		}
	}

	steps := make([]models.WorkflowStep, 0, len(recipe.Steps))
	for i, st := range recipe.Steps {
		steps = append(steps, models.WorkflowStep{
			Order:      i,
			ActionType: st.ActionType,
			Config:     st.Config,
			Conditions: nonEmptyJSON(st.Conditions),
		})
	}

	created := &models.Workflow{
		TenantID:    board.TenantID,
		Name:        recipeWorkflowName(recipe, board.Name),
		Description: recipe.Description,
		Enabled:     true,
		TriggerType: recipe.TriggerType,
		BoardID:     boardID,
		RecipeKey:   recipe.Key,
		Conditions:  nonEmptyJSON(recipe.Conditions),
		// CreatedBy es de quién hereda el alcance: el runner lo revalida en cada
		// ejecución, así que tiene que ser la persona real que la encendió y no
		// una cuenta genérica.
		CreatedBy:     a.UserID,
		TriggerConfig: triggerConfig,
		FormSchema:    nonEmptyJSON(recipe.FormSchema),
		Steps:         steps,
	}
	if err := s.repo.CreateWorkflow(created); err != nil {
		return nil, fmt.Errorf("creando la automatización: %w", err)
	}
	return created, nil
}

// WorkflowSummary es una regla tal como la lista la pantalla, con lo justo para
// entenderla de un vistazo.
type WorkflowSummary struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Enabled     bool   `json:"enabled"`
	TriggerType string `json:"trigger_type"`
	BoardID     uint   `json:"board_id"`
	RecipeKey   string `json:"recipe_key,omitempty"`
	StepCount   int    `json:"step_count"`
}

func (s *WorkflowService) ListForBoard(a WorkflowActor, boardID uint) ([]WorkflowSummary, error) {
	board, err := s.authorizeBoard(a, boardID)
	if err != nil {
		return nil, err
	}
	wfs, err := s.repo.ListByBoard(board.TenantID, boardID)
	if err != nil {
		return nil, err
	}
	out := make([]WorkflowSummary, 0, len(wfs))
	for _, wf := range wfs {
		out = append(out, WorkflowSummary{
			ID: wf.ID, Name: wf.Name, Description: wf.Description,
			Enabled: wf.Enabled, TriggerType: wf.TriggerType,
			BoardID: wf.BoardID, RecipeKey: wf.RecipeKey, StepCount: len(wf.Steps),
		})
	}
	return out, nil
}

// RunView es una ejecución tal como se muestra en el historial.
type RunView struct {
	ID         uint   `json:"id"`
	Status     string `json:"status"`
	SkipReason string `json:"skip_reason,omitempty"`
	LastError  string `json:"last_error,omitempty"`
	Attempts   int    `json:"attempts"`
	EntityID   uint   `json:"entity_id"`
	CreatedAt  string `json:"created_at"`
}

// Runs devuelve el historial de una regla. Es de sólo lectura y sirve para
// responder "¿esto está haciendo algo?" sin entrar a la base de datos, que es la
// primera pregunta en cuanto alguien enciende una automatización.
func (s *WorkflowService) Runs(a WorkflowActor, workflowID uint) ([]RunView, error) {
	wf, err := s.repo.GetWorkflow(workflowID)
	if err != nil || wf == nil {
		return nil, ErrWorkflowNotFound
	}
	// El alcance se comprueba por el TABLERO de la regla, no por su tenant: un
	// manager de la empresa no tiene por qué ver las automatizaciones de un tablero
	// del que no forma parte.
	if _, err := s.authorizeBoard(a, wf.BoardID); err != nil {
		return nil, err
	}

	runs, err := s.repo.ListRuns(workflowID, workflowRunHistory)
	if err != nil {
		return nil, err
	}
	out := make([]RunView, 0, len(runs))
	for _, r := range runs {
		out = append(out, RunView{
			ID: r.ID, Status: r.Status, SkipReason: r.SkipReason,
			LastError: r.LastError, Attempts: r.Attempts, EntityID: r.EntityID,
			CreatedAt: r.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		})
	}
	return out, nil
}

// phaseIDOf lee la columna sobre la que actúa una regla, o 0 si no tiene ninguna.
func phaseIDOf(wf models.Workflow) uint {
	var scope struct {
		PhaseID uint `json:"phase_id"`
	}
	if err := json.Unmarshal([]byte(nonEmptyJSON(wf.TriggerConfig)), &scope); err != nil {
		return 0
	}
	return scope.PhaseID
}

func boardHasPhase(board *models.Board, phaseID uint) bool {
	for _, p := range board.Phases {
		if p.ID == phaseID {
			return true
		}
	}
	return false
}
