package service

import (
	"errors"
	"strings"
	"testing"

	"github.com/obertrack/backend/internal/models"
)

// Constructor de puertas. Lo que se prueba aquí no es que se guarde: es que NO se
// pueda guardar una puerta que deje una columna inalcanzable. Hasta ahora los
// formularios los escribíamos nosotros; desde el constructor los escribe cualquiera
// con permiso, y una puerta es lo único del motor que puede impedir trabajar.

func tableroConFases() *models.Board {
	return &models.Board{
		ID: 1, TenantID: 42, CreatedBy: 9,
		Phases: []models.Phase{
			{ID: 11, Name: "Por hacer", Status: "por_hacer"},
			{ID: 12, Name: "En revisión"},
			{ID: 13, Name: "Finalizado", Status: "finalizado"},
		},
	}
}

func gateAdminSvc(reglas ...models.Workflow) (*WorkflowService, *wfRepo) {
	repo := &wfRepo{rules: reglas}
	return newWfService(repo, tableroConFases(), nil, nil), repo
}

func formularioValido() models.GateForm {
	return models.GateForm{
		Title: "Antes de revisar",
		Fields: []models.GateField{
			{Key: "expediente", Label: "Nº de expediente", Type: models.GateFieldText, Required: true},
			{Key: "notas", Label: "Notas", Type: models.GateFieldTextarea},
		},
	}
}

func entradaValida() GateInput {
	return GateInput{BoardID: 1, PhaseID: 12, Name: "Control de revisión", Enabled: true, Form: formularioValido()}
}

var empleador = WorkflowActor{UserID: 9, TenantID: 42, IsEmployer: true}

func TestConstructor_CreaUnaPuertaPropia(t *testing.T) {
	s, repo := gateAdminSvc()

	wf, err := s.CreateGate(empleador, entradaValida())
	if err != nil {
		t.Fatal(err)
	}
	if wf.RecipeKey != "" {
		t.Fatal("una puerta propia no pertenece a ninguna receta")
	}
	if wf.TriggerType != models.TriggerTaskEnteringPhase || wf.BoardID != 1 {
		t.Fatalf("la puerta no quedó bien montada: %+v", wf)
	}
	// El inquilino sale del TABLERO, no de quien la crea: con el del actor, un
	// superadmin crearía una regla que el motor no dispara nunca.
	if wf.TenantID != 42 {
		t.Fatalf("la puerta debería ser de la empresa del tablero, got %d", wf.TenantID)
	}
	if !strings.Contains(wf.TriggerConfig, "12") {
		t.Fatalf("la puerta tiene que apuntar a su columna: %q", wf.TriggerConfig)
	}
	if len(repo.rules) != 1 {
		t.Fatalf("debería haberse guardado, got %d", len(repo.rules))
	}
}

// Lo que impide que el constructor se convierta en un arma: un formulario imposible
// de rellenar deja la columna cerrada para todo el equipo.
func TestConstructor_RechazaFormulariosQueBloquearianLaColumna(t *testing.T) {
	casos := []struct {
		nombre string
		form   models.GateForm
		motivo string
	}{
		{"sin campos", models.GateForm{Title: "X"}, "campos"},
		{
			"selección obligatoria sin opciones",
			models.GateForm{Title: "X", Fields: []models.GateField{
				{Key: "a", Label: "A", Type: models.GateFieldSelect, Required: true}}},
			"opciones",
		},
		{
			"clave con espacios",
			models.GateForm{Title: "X", Fields: []models.GateField{
				{Key: "nº expediente", Label: "A", Type: models.GateFieldText}}},
			"clave",
		},
		{
			"clave repetida",
			models.GateForm{Title: "X", Fields: []models.GateField{
				{Key: "a", Label: "A", Type: models.GateFieldText},
				{Key: "a", Label: "B", Type: models.GateFieldText}}},
			"repetida",
		},
		{
			"etiqueta interminable",
			models.GateForm{Title: "X", Fields: []models.GateField{
				{Key: "a", Label: strings.Repeat("x", models.GateMaxLabel+1), Type: models.GateFieldText}}},
			"etiqueta",
		},
		{
			"tipo inventado",
			models.GateForm{Title: "X", Fields: []models.GateField{
				{Key: "a", Label: "A", Type: "firma"}}},
			"tipo desconocido",
		},
		{
			"opciones repetidas",
			models.GateForm{Title: "X", Fields: []models.GateField{
				{Key: "a", Label: "A", Type: models.GateFieldSelect, Options: []models.GateOption{
					{Value: "si", Label: "Sí"}, {Value: "si", Label: "Sí, claro"}}}}},
			"repite la opción",
		},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			s, repo := gateAdminSvc()
			in := entradaValida()
			in.Form = c.form

			_, err := s.CreateGate(empleador, in)
			if err == nil {
				t.Fatal("debería rechazarse")
			}
			// Tipado, para que el constructor reciba 400 con el detalle y no un 500
			// genérico: quien está montando el formulario necesita saber qué corregir.
			if !errors.Is(err, ErrGateForm) {
				t.Fatalf("debería ser un error de formulario, got %v", err)
			}
			if !strings.Contains(err.Error(), c.motivo) {
				t.Fatalf("el motivo debería mencionar %q, got %q", c.motivo, err)
			}
			if len(repo.rules) != 0 {
				t.Fatal("no debería haberse guardado nada")
			}
		})
	}
}

// Dos puertas sobre la misma columna son dos formularios para un solo movimiento: el
// motor aplicaría la primera que encuentre y la otra quedaría muda sin explicación.
func TestConstructor_UnaSolaPuertaPorColumna(t *testing.T) {
	existente := models.Workflow{
		ID: 5, TenantID: 42, BoardID: 1, Enabled: true,
		TriggerType: models.TriggerTaskEnteringPhase, TriggerConfig: `{"phase_id":12}`,
		Name: "La que ya estaba",
	}
	s, _ := gateAdminSvc(existente)

	_, err := s.CreateGate(empleador, entradaValida())
	if !errors.Is(err, ErrPhaseAlreadyGated) {
		t.Fatalf("la columna ya tenía puerta, got %v", err)
	}

	// Otra columna sí.
	otra := entradaValida()
	otra.PhaseID = 13
	if _, err := s.CreateGate(empleador, otra); err != nil {
		t.Fatalf("otra columna debería aceptarse: %v", err)
	}
}

func TestConstructor_LaColumnaTieneQueSerDelTablero(t *testing.T) {
	s, _ := gateAdminSvc()

	sinColumna := entradaValida()
	sinColumna.PhaseID = 0
	if _, err := s.CreateGate(empleador, sinColumna); !errors.Is(err, ErrPhaseRequired) {
		t.Fatalf("sin columna sería un peaje en todo el tablero, got %v", err)
	}

	ajena := entradaValida()
	ajena.PhaseID = 999
	if _, err := s.CreateGate(empleador, ajena); !errors.Is(err, ErrPhaseNotInBoard) {
		t.Fatalf("la columna de otro tablero no vale, got %v", err)
	}

	sinNombre := entradaValida()
	sinNombre.Name = "   "
	if _, err := s.CreateGate(empleador, sinNombre); !errors.Is(err, ErrGateNameRequired) {
		t.Fatalf("sin nombre no se reconoce en la lista, got %v", err)
	}
}

func TestConstructor_EditarCambiaFormularioColumnaYNombre(t *testing.T) {
	s, _ := gateAdminSvc()
	creada, err := s.CreateGate(empleador, entradaValida())
	if err != nil {
		t.Fatal(err)
	}

	nueva := entradaValida()
	nueva.PhaseID = 13
	nueva.Name = "Control de cierre"
	nueva.Form.Title = "Antes de cerrar"
	nueva.Enabled = false

	editada, err := s.UpdateGate(empleador, creada.ID, nueva)
	if err != nil {
		t.Fatal(err)
	}
	if editada.Name != "Control de cierre" || editada.Enabled {
		t.Fatalf("no se aplicó la edición: %+v", editada)
	}
	if !strings.Contains(editada.TriggerConfig, "13") {
		t.Fatalf("la puerta debería haberse mudado de columna: %q", editada.TriggerConfig)
	}
	// Editar no puede chocar consigo misma: la columna que ya ocupaba es suya.
	misma := entradaValida()
	misma.PhaseID = 13
	if _, err := s.UpdateGate(empleador, creada.ID, misma); err != nil {
		t.Fatalf("una puerta no se estorba a sí misma: %v", err)
	}
}

// Una puerta de receta se enciende y se apaga, pero su formulario lo define el
// catálogo: editarla aquí crearía dos fuentes para lo mismo y la copia guardada
// volvería a envejecer, que es el error que ya arreglamos una vez.
func TestConstructor_NoEditaNiBorraLasDelCatalogo(t *testing.T) {
	receta := models.Workflow{
		ID: 5, TenantID: 42, BoardID: 1, Enabled: true, RecipeKey: "puerta_entrega",
		TriggerType: models.TriggerTaskEnteringPhase, TriggerConfig: `{"phase_id":12}`,
	}
	s, _ := gateAdminSvc(receta)

	if _, err := s.UpdateGate(empleador, 5, entradaValida()); !errors.Is(err, ErrGateIsRecipe) {
		t.Fatalf("una puerta de receta no se edita aquí, got %v", err)
	}
	if err := s.DeleteGate(empleador, 5); !errors.Is(err, ErrGateIsRecipe) {
		t.Fatalf("una puerta de receta no se borra aquí, got %v", err)
	}
}

// multiTableroRepo devuelve un tablero DISTINTO por id. Hace falta para probar el
// alcance: con un repositorio que devuelve siempre el mismo tablero, cualquier id
// parecería propio y la comprobación pasaría sin comprobar nada.
type multiTableroRepo struct {
	wfBoardRepo
	boards map[uint]*models.Board
}

func (r *multiTableroRepo) GetByID(id uint) (*models.Board, error) {
	if b, ok := r.boards[id]; ok {
		return b, nil
	}
	return nil, nil
}

// El alcance sale del tablero DE LA PUERTA, no del que venga en la petición: si no,
// bastaría mandar el id de un tablero propio para editar la puerta de otro.
func TestConstructor_NoSeEditaLaPuertaDeOtroTablero(t *testing.T) {
	ajena := models.Workflow{
		ID: 5, TenantID: 77, BoardID: 99, Enabled: true,
		TriggerType: models.TriggerTaskEnteringPhase, TriggerConfig: `{"phase_id":12}`,
	}
	repo := &wfRepo{rules: []models.Workflow{ajena}}
	s := NewWorkflowService(
		repo, nil,
		&multiTableroRepo{boards: map[uint]*models.Board{
			1:  tableroConFases(),
			99: {ID: 99, TenantID: 77, CreatedBy: 500},
		}},
		&wfUserRepo{users: map[uint]*models.User{}},
		&wfEmpRepo{}, &fakeNotifSvc{}, nil,
	)

	in := entradaValida()
	in.BoardID = 1 // el suyo, para intentar colarse
	if _, err := s.UpdateGate(empleador, 5, in); !errors.Is(err, ErrWorkflowBoardScope) {
		t.Fatalf("no debería alcanzar la puerta de otro tablero, got %v", err)
	}
	// Y tampoco borrarla.
	if err := s.DeleteGate(empleador, 5); !errors.Is(err, ErrWorkflowBoardScope) {
		t.Fatalf("tampoco debería poder borrarla, got %v", err)
	}
}

func TestConstructor_ListaLasPropiasYSeñalaLaColumnaPerdida(t *testing.T) {
	s, _ := gateAdminSvc(models.Workflow{
		ID: 5, TenantID: 42, BoardID: 1, Enabled: true, RecipeKey: "puerta_entrega",
		TriggerType: models.TriggerTaskEnteringPhase, TriggerConfig: `{"phase_id":12}`,
	})
	// Una propia sobre una columna que ya no está en el tablero.
	huerfana := entradaValida()
	huerfana.PhaseID = 13
	creada, err := s.CreateGate(empleador, huerfana)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.repo.SetTriggerConfig(creada.ID, `{"phase_id":404}`); err != nil {
		t.Fatal(err)
	}

	gates, err := s.ListGates(empleador, 1)
	if err != nil {
		t.Fatal(err)
	}
	// La de receta no sale aquí: se enciende en su propia pantalla.
	if len(gates) != 1 {
		t.Fatalf("sólo se listan las propias, got %d", len(gates))
	}
	if !gates[0].PhaseMissing {
		t.Fatal("una puerta cuya columna se borró tiene que señalarse: figura activa y no dispara nunca")
	}
	if len(gates[0].Form.Fields) != 2 {
		t.Fatalf("el formulario tiene que viajar para poder editarlo, got %+v", gates[0].Form)
	}
}

func TestConstructor_BorrarQuitaLaPuerta(t *testing.T) {
	s, repo := gateAdminSvc()
	creada, err := s.CreateGate(empleador, entradaValida())
	if err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteGate(empleador, creada.ID); err != nil {
		t.Fatal(err)
	}
	if len(repo.rules) != 0 {
		t.Fatalf("la puerta debería haberse borrado, quedan %d", len(repo.rules))
	}
	if err := s.DeleteGate(empleador, 404); !errors.Is(err, ErrGateNotFound) {
		t.Fatalf("borrar lo que no existe devuelve no encontrado, got %v", err)
	}
}
