package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/obertrack/backend/internal/models"
)

// Constructor de puertas (G4): definir un punto de control desde la interfaz, sin
// tocar código.
//
// Hasta aquí una puerta sólo podía nacer del catálogo: tres formularios escritos por
// nosotros que sirven para los casos comunes y para nada más. Una empresa que necesita
// preguntar otra cosa —el número de expediente, a qué cliente pertenece, qué checklist
// se completó— no tenía forma de pedirlo sin que alguien escribiera Go.
//
// Lo que se puede definir es el FORMULARIO: qué se pregunta y qué es obligatorio. Las
// consecuencias (aprobar cierra, rechazar devuelve) siguen siendo de las recetas, y a
// propósito: encadenar acciones desde una interfaz es el constructor de reglas
// completo, otro problema y otro tamaño.
//
// Una puerta propia se distingue de una de receta en una sola cosa: RecipeKey vacío.
// De ahí sale todo lo demás —su formulario manda sobre el catálogo, la pantalla la
// deja editar— sin necesidad de una tabla ni una entidad aparte.

var (
	ErrGateNotFound = errors.New("esa puerta no existe")
	// ErrGateNameRequired: una puerta sin nombre es imposible de reconocer en la
	// lista de automatizaciones y en el registro de ejecuciones.
	ErrGateNameRequired = errors.New("ponle un nombre a la puerta")
	// ErrPhaseAlreadyGated: dos puertas sobre la misma columna significan dos
	// formularios para un solo movimiento. El motor sólo puede aplicar una.
	ErrPhaseAlreadyGated = errors.New("esa columna ya tiene un punto de control")
	// ErrGateIsRecipe: una puerta de receta se enciende y se apaga, pero su
	// formulario lo define el catálogo. Editarla aquí crearía dos fuentes para lo
	// mismo y la copia guardada volvería a envejecer.
	ErrGateIsRecipe = errors.New("esta puerta viene del catálogo: enciéndela o apágala, pero su formulario no se edita aquí")
	// ErrGateForm envuelve lo que devuelve la validación del esquema. Los mensajes
	// de dentro están escritos para quien está construyendo el formulario ("el campo
	// X no tiene etiqueta"): perderlos detrás de un 500 genérico dejaría al
	// constructor sin poder decir qué corregir.
	ErrGateForm = errors.New("el formulario no es válido")
)

// GateInput es lo que llega del constructor.
type GateInput struct {
	BoardID uint            `json:"board_id"`
	PhaseID uint            `json:"phase_id"`
	Name    string          `json:"name"`
	Enabled bool            `json:"enabled"`
	Form    models.GateForm `json:"form"`
}

// GateView es una puerta propia tal como la lista la pantalla.
type GateView struct {
	ID      uint   `json:"id"`
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
	BoardID uint   `json:"board_id"`
	PhaseID uint   `json:"phase_id"`
	// PhaseMissing marca la puerta cuya columna ya no está en el tablero: figura
	// activa y no dispara nunca.
	PhaseMissing bool            `json:"phase_missing,omitempty"`
	Form         models.GateForm `json:"form"`
}

// ListGates devuelve las puertas PROPIAS de un tablero. Las de receta salen por su
// pantalla de recetas: son otra cosa: allí se enciende un formulario ya escrito.
func (s *WorkflowService) ListGates(a WorkflowActor, boardID uint) ([]GateView, error) {
	board, err := s.authorizeBoard(a, boardID)
	if err != nil {
		return nil, err
	}
	wfs, err := s.repo.ListByBoard(board.TenantID, boardID)
	if err != nil {
		return nil, err
	}
	out := make([]GateView, 0)
	for _, wf := range wfs {
		if wf.RecipeKey != "" || wf.TriggerType != models.TriggerTaskEnteringPhase {
			continue
		}
		v, verr := gateViewOf(wf)
		if verr != nil {
			// Una puerta ilegible se lista igualmente, vacía: esconderla dejaría a
			// alguien sin forma de borrar la regla que le está bloqueando una columna.
			v = GateView{ID: wf.ID, Name: wf.Name, Enabled: wf.Enabled, BoardID: wf.BoardID}
		}
		v.PhaseMissing = v.PhaseID != 0 && !boardHasPhase(board, v.PhaseID)
		out = append(out, v)
	}
	return out, nil
}

// CreateGate crea una puerta propia.
func (s *WorkflowService) CreateGate(a WorkflowActor, in GateInput) (*models.Workflow, error) {
	board, err := s.authorizeBoard(a, in.BoardID)
	if err != nil {
		return nil, err
	}
	nombre, esquema, err := s.validarPuerta(board, in, 0)
	if err != nil {
		return nil, err
	}

	wf := &models.Workflow{
		TenantID:    board.TenantID,
		Name:        nombre,
		Description: "Punto de control creado desde el tablero.",
		Enabled:     in.Enabled,
		TriggerType: models.TriggerTaskEnteringPhase,
		BoardID:     in.BoardID,
		// CreatedBy es de quién hereda el alcance: el motor lo revalida en cada
		// ejecución, así que tiene que ser la persona real que la creó.
		CreatedBy:     a.UserID,
		TriggerConfig: fmt.Sprintf(`{"phase_id":%d}`, in.PhaseID),
		Conditions:    "{}",
		FormSchema:    esquema,
	}
	if err := s.repo.CreateWorkflow(wf); err != nil {
		return nil, fmt.Errorf("creando la puerta: %w", err)
	}
	return wf, nil
}

// UpdateGate reescribe una puerta propia: nombre, columna, formulario e interruptor.
//
// El cambio surte efecto de inmediato, y eso importa más aquí que en el resto del
// motor: quien esté moviendo una tarjeta ahora mismo verá el formulario nuevo. Es la
// razón de que la pantalla lo advierta antes de guardar.
func (s *WorkflowService) UpdateGate(a WorkflowActor, gateID uint, in GateInput) (*models.Workflow, error) {
	wf, err := s.repo.GetWorkflow(gateID)
	if err != nil || wf == nil {
		return nil, ErrGateNotFound
	}
	if wf.TriggerType != models.TriggerTaskEnteringPhase {
		return nil, ErrGateNotFound
	}
	if wf.RecipeKey != "" {
		return nil, ErrGateIsRecipe
	}
	// El alcance se comprueba por el TABLERO de la puerta, no por el que venga en la
	// petición: si no, bastaría con mandar el id de un tablero propio para editar la
	// puerta de otro.
	board, err := s.authorizeBoard(a, wf.BoardID)
	if err != nil {
		return nil, err
	}
	in.BoardID = wf.BoardID

	nombre, esquema, err := s.validarPuerta(board, in, wf.ID)
	if err != nil {
		return nil, err
	}
	if err := s.repo.UpdateGate(wf.ID, nombre, fmt.Sprintf(`{"phase_id":%d}`, in.PhaseID), esquema, in.Enabled); err != nil {
		return nil, err
	}
	wf.Name, wf.FormSchema, wf.Enabled = nombre, esquema, in.Enabled
	wf.TriggerConfig = fmt.Sprintf(`{"phase_id":%d}`, in.PhaseID)
	return wf, nil
}

// DeleteGate borra una puerta propia.
//
// Borrar y no apagar: una puerta apagada sigue en la lista y se puede volver a
// encender, que es lo que se quiere casi siempre; borrarla es para la que nunca debió
// existir. El borrado es lógico, así que su historial de ejecuciones sobrevive: sigue
// explicando los movimientos que ya bloqueó.
func (s *WorkflowService) DeleteGate(a WorkflowActor, gateID uint) error {
	wf, err := s.repo.GetWorkflow(gateID)
	if err != nil || wf == nil || wf.TriggerType != models.TriggerTaskEnteringPhase {
		return ErrGateNotFound
	}
	if wf.RecipeKey != "" {
		return ErrGateIsRecipe
	}
	if _, err := s.authorizeBoard(a, wf.BoardID); err != nil {
		return err
	}
	return s.repo.DeleteWorkflow(wf.ID)
}

// validarPuerta reúne todo lo que tiene que cumplirse antes de guardar, para que
// crear y editar no puedan divergir.
func (s *WorkflowService) validarPuerta(board *models.Board, in GateInput, exceptoID uint) (string, string, error) {
	nombre := strings.TrimSpace(in.Name)
	if nombre == "" {
		return "", "", ErrGateNameRequired
	}
	if len([]rune(nombre)) > models.GateMaxName {
		return "", "", fmt.Errorf("el nombre no puede pasar de %d caracteres", models.GateMaxName)
	}

	// Sin columna, la puerta sería un peaje en todo el tablero.
	if in.PhaseID == 0 {
		return "", "", ErrPhaseRequired
	}
	if !boardHasPhase(board, in.PhaseID) {
		return "", "", ErrPhaseNotInBoard
	}

	// Una sola puerta por columna. Con dos, el motor aplicaría la primera que
	// encuentre y la otra quedaría muda sin que nadie supiera por qué.
	ocupada, err := s.columnaOcupada(board, in.PhaseID, exceptoID)
	if err != nil {
		return "", "", err
	}
	if ocupada {
		return "", "", ErrPhaseAlreadyGated
	}

	// El esquema se valida ANTES de guardar: una puerta con un formulario imposible
	// de completar deja la columna inalcanzable para todo el equipo.
	crudo, err := json.Marshal(in.Form)
	if err != nil {
		return "", "", fmt.Errorf("el formulario no se pudo interpretar: %w", err)
	}
	if err := ValidateGateForm(string(crudo)); err != nil {
		return "", "", fmt.Errorf("%w: %s", ErrGateForm, err)
	}
	return nombre, string(crudo), nil
}

func (s *WorkflowService) columnaOcupada(board *models.Board, phaseID, exceptoID uint) (bool, error) {
	wfs, err := s.repo.ListByBoard(board.TenantID, board.ID)
	if err != nil {
		return false, err
	}
	for _, wf := range wfs {
		if wf.ID == exceptoID || wf.TriggerType != models.TriggerTaskEnteringPhase {
			continue
		}
		if phaseIDOf(wf) == phaseID {
			return true, nil
		}
	}
	return false, nil
}

func gateViewOf(wf models.Workflow) (GateView, error) {
	form, err := parseGateForm(wf.FormSchema)
	v := GateView{
		ID: wf.ID, Name: wf.Name, Enabled: wf.Enabled,
		BoardID: wf.BoardID, PhaseID: phaseIDOf(wf), Form: form,
	}
	return v, err
}
