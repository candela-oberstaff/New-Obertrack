package service

import (
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/obertrack/backend/internal/models"
)

// Puerta de fase: la mitad síncrona del motor. Se evalúa DENTRO del request que
// intenta mover una tarjeta, y su respuesta decide si el movimiento ocurre.
//
// Todo lo de aquí corre en el servidor a propósito. El modal es la presentación de
// la puerta, no la puerta: si la regla viviera sólo en el frontend, bastaría un
// PUT /api/tasks/:id para saltársela —y la app móvil ya usa ese endpoint—.

// GateRequiredError es lo que devuelve taskService cuando un movimiento choca contra
// una puerta. No es un fallo del sistema: es la puerta haciendo su trabajo, y lleva
// consigo todo lo que el cliente necesita para pedir lo que falta.
type GateRequiredError struct {
	WorkflowID uint            `json:"workflow_id"`
	Workflow   string          `json:"workflow"`
	ToStatus   string          `json:"to_status"`
	Form       models.GateForm `json:"form"`
	// Errors va vacío en el primer rechazo (no se envió nada) y relleno cuando el
	// formulario llegó pero no pasó la validación, con un mensaje por campo.
	Errors map[string]string `json:"errors,omitempty"`
}

func (e *GateRequiredError) Error() string {
	if len(e.Errors) > 0 {
		return "el formulario de la puerta no está completo"
	}
	return "esta columna exige completar un formulario"
}

// gateSubmission es lo que envía el cliente: el par campo→valor del formulario.
type gateSubmission map[string]any

// parseGateForm lee el esquema guardado en la regla.
func parseGateForm(raw string) (models.GateForm, error) {
	var form models.GateForm
	if strings.TrimSpace(raw) == "" || raw == "{}" {
		return form, fmt.Errorf("la puerta no tiene formulario configurado")
	}
	if err := json.Unmarshal([]byte(raw), &form); err != nil {
		return form, fmt.Errorf("el formulario de la puerta no se pudo interpretar: %w", err)
	}
	if len(form.Fields) == 0 {
		return form, fmt.Errorf("el formulario de la puerta no tiene campos")
	}
	return form, nil
}

// ValidateGateForm comprueba que un esquema sea utilizable ANTES de guardarlo. Una
// puerta mal formada deja una columna inalcanzable, así que el momento de detectarlo
// es al configurarla, no cuando alguien intente mover una tarjeta.
func ValidateGateForm(raw string) error {
	form, err := parseGateForm(raw)
	if err != nil {
		return err
	}
	if len(form.Fields) > models.GateMaxFields {
		return fmt.Errorf("una puerta no puede pedir más de %d campos", models.GateMaxFields)
	}
	if excede(form.Title, models.GateMaxTitle) {
		return fmt.Errorf("el título no puede pasar de %d caracteres", models.GateMaxTitle)
	}
	if excede(form.Description, models.GateMaxDescription) {
		return fmt.Errorf("la descripción no puede pasar de %d caracteres", models.GateMaxDescription)
	}

	seen := make(map[string]bool, len(form.Fields))
	for i, f := range form.Fields {
		if strings.TrimSpace(f.Key) == "" {
			return fmt.Errorf("el campo %d no tiene clave", i+1)
		}
		if seen[f.Key] {
			return fmt.Errorf("la clave %q está repetida", f.Key)
		}
		seen[f.Key] = true
		// La clave se guarda en el historial de cada tarea y viaja en el JSON del
		// envío. Se acota a un identificador simple para que no acabe habiendo claves
		// con espacios, acentos o puntos que después nadie pueda buscar.
		if !claveValida(f.Key) {
			return fmt.Errorf("la clave %q sólo puede llevar letras minúsculas, números y guiones bajos (máx. %d)", f.Key, models.GateMaxKey)
		}

		if strings.TrimSpace(f.Label) == "" {
			return fmt.Errorf("el campo %q no tiene etiqueta", f.Key)
		}
		if excede(f.Label, models.GateMaxLabel) {
			return fmt.Errorf("la etiqueta de %q no puede pasar de %d caracteres", f.Key, models.GateMaxLabel)
		}
		if excede(f.Help, models.GateMaxHelp) {
			return fmt.Errorf("la ayuda de %q no puede pasar de %d caracteres", f.Key, models.GateMaxHelp)
		}
		if excede(f.Placeholder, models.GateMaxPlaceholder) {
			return fmt.Errorf("el texto de ejemplo de %q no puede pasar de %d caracteres", f.Key, models.GateMaxPlaceholder)
		}
		if !models.IsValidGateFieldType(f.Type) {
			return fmt.Errorf("el campo %q tiene un tipo desconocido: %q", f.Key, f.Type)
		}
		// Un select sin opciones es un campo imposible de responder: si además es
		// obligatorio, bloquea la columna para siempre.
		if f.Type == models.GateFieldSelect {
			if len(f.Options) == 0 {
				return fmt.Errorf("el campo %q es de selección y no tiene opciones", f.Key)
			}
			if len(f.Options) > models.GateMaxOptions {
				return fmt.Errorf("el campo %q no puede tener más de %d opciones", f.Key, models.GateMaxOptions)
			}
			vistos := make(map[string]bool, len(f.Options))
			for j, o := range f.Options {
				if strings.TrimSpace(o.Value) == "" || strings.TrimSpace(o.Label) == "" {
					return fmt.Errorf("la opción %d de %q está incompleta", j+1, f.Key)
				}
				if excede(o.Value, models.GateMaxOptionText) || excede(o.Label, models.GateMaxOptionText) {
					return fmt.Errorf("las opciones de %q no pueden pasar de %d caracteres", f.Key, models.GateMaxOptionText)
				}
				// Dos opciones con el mismo valor son la misma respuesta con dos
				// nombres: el registro no podría distinguirlas después.
				if vistos[o.Value] {
					return fmt.Errorf("el campo %q repite la opción %q", f.Key, o.Value)
				}
				vistos[o.Value] = true
			}
		}
		if f.Type == models.GateFieldNumber && f.Min != nil && f.Max != nil && *f.Min > *f.Max {
			return fmt.Errorf("el campo %q tiene un mínimo mayor que su máximo", f.Key)
		}
		if f.MaxLength < 0 || f.MaxLength > models.GateFieldMaxLength {
			return fmt.Errorf("el largo máximo de %q tiene que estar entre 1 y %d", f.Key, models.GateFieldMaxLength)
		}
	}
	return nil
}

// excede mide en RUNAS y no en bytes: con acentos, medir en bytes recortaría antes
// de lo que dice el mensaje de error y el usuario no entendería por qué.
func excede(s string, max int) bool {
	return len([]rune(s)) > max
}

var claveGate = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

func claveValida(k string) bool {
	return len(k) <= models.GateMaxKey && claveGate.MatchString(k)
}

// validateGateSubmission contrasta lo enviado contra el esquema y devuelve el valor
// ya normalizado —el que se guardará en el historial— o un error por campo.
//
// Sólo se conservan las claves DECLARADAS en el esquema: lo que llegue de más se
// descarta en silencio, para que el registro de la tarea no acabe guardando lo que a
// un cliente se le ocurriera mandar.
func validateGateSubmission(form models.GateForm, sub gateSubmission) (map[string]any, []models.GateSubmittedField, map[string]string) {
	clean := make(map[string]any, len(form.Fields))
	// El orden de los campos se conserva: es el del formulario, y es como se leerá
	// después en el historial.
	filled := make([]models.GateSubmittedField, 0, len(form.Fields))
	errs := make(map[string]string)

	for _, f := range form.Fields {
		raw, present := sub[f.Key]
		value := strings.TrimSpace(fmt.Sprintf("%v", raw))
		if !present || raw == nil || value == "" {
			if f.Required {
				errs[f.Key] = "Este campo es obligatorio"
			}
			continue
		}

		switch f.Type {
		case models.GateFieldText, models.GateFieldTextarea:
			max := f.MaxLength
			if max <= 0 {
				max = models.GateFieldMaxLength
			}
			if len([]rune(value)) > max {
				errs[f.Key] = fmt.Sprintf("No puede pasar de %d caracteres", max)
				continue
			}
			clean[f.Key] = value
			filled = append(filled, submitted(f, value))

		case models.GateFieldURL:
			u, err := url.Parse(value)
			// Se exige esquema y host: sin ellos "documento.pdf" pasaría por enlace
			// válido y el registro guardaría algo que nadie puede abrir después.
			if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
				errs[f.Key] = "Escribe un enlace válido que empiece por http:// o https://"
				continue
			}
			clean[f.Key] = value
			filled = append(filled, submitted(f, value))

		case models.GateFieldFile:
			// Un adjunto es una subida NUESTRA, no un enlace cualquiera: /api/uploads
			// devuelve una ruta relativa, así que exigir http(s) absoluto rechazaría todo
			// archivo legítimo. Y aceptar cualquier cadena convertiría el campo en un
			// hueco para colar enlaces externos disfrazados de evidencia.
			if !isUploadedFilePath(value) {
				errs[f.Key] = "Adjunta un archivo"
				continue
			}
			clean[f.Key] = value
			filled = append(filled, submitted(f, value))

		case models.GateFieldSelect:
			valid := false
			for _, o := range f.Options {
				if o.Value == value {
					valid = true
					break
				}
			}
			if !valid {
				errs[f.Key] = "Elige una de las opciones"
				continue
			}
			clean[f.Key] = value
			filled = append(filled, submitted(f, value))

		case models.GateFieldDate:
			if _, err := time.Parse("2006-01-02", value); err != nil {
				errs[f.Key] = "Usa una fecha válida"
				continue
			}
			clean[f.Key] = value
			filled = append(filled, submitted(f, value))

		case models.GateFieldNumber:
			n, err := strconv.ParseFloat(value, 64)
			if err != nil {
				errs[f.Key] = "Escribe un número"
				continue
			}
			if f.Min != nil && n < *f.Min {
				errs[f.Key] = fmt.Sprintf("No puede ser menor que %g", *f.Min)
				continue
			}
			if f.Max != nil && n > *f.Max {
				errs[f.Key] = fmt.Sprintf("No puede ser mayor que %g", *f.Max)
				continue
			}
			clean[f.Key] = n
			filled = append(filled, submitted(f, n))

		default:
			// Un tipo desconocido no se da por bueno: ValidateGateForm lo impide al
			// guardar, así que llegar aquí significa que el esquema se corrompió.
			errs[f.Key] = "Este campo tiene una configuración inválida"
		}
	}

	if len(errs) == 0 {
		return clean, filled, nil
	}
	return nil, nil, errs
}

// isUploadedFilePath acepta sólo lo que produce /api/uploads: una ruta relativa bajo
// ese prefijo, sin saltos de directorio. Comprobarlo aquí es lo que impide que el
// campo "adjunto" se use para guardar un enlace a cualquier parte.
func isUploadedFilePath(v string) bool {
	if !strings.HasPrefix(v, uploadPathPrefix) {
		return false
	}
	name := strings.TrimPrefix(v, uploadPathPrefix)
	return name != "" && !strings.Contains(name, "/") && !strings.Contains(name, "..")
}

// uploadPathPrefix es la ruta con la que el backend sirve lo subido (ver
// handlers/upload.go). Si cambia allí, cambia aquí.
const uploadPathPrefix = "/api/uploads/"

// submitted empareja un valor validado con la etiqueta y el tipo que tenía el campo
// EN ESE MOMENTO, para que el historial no dependa del esquema vigente.
func submitted(f models.GateField, value any) models.GateSubmittedField {
	return models.GateSubmittedField{Key: f.Key, Label: f.Label, Type: f.Type, Value: value}
}

// GateResult es un movimiento que YA pasó la puerta: qué regla la exigió y qué se
// respondió, listo para guardarse junto al cambio de estado.
type GateResult struct {
	WorkflowID uint
	// Data es el mapa campo→valor, que es lo que leerá la lógica (las consecuencias
	// según la respuesta, en la fase siguiente).
	Data map[string]any
	// Fields es lo mismo con etiqueta y tipo, en el orden del formulario: es lo que
	// se guarda para que el historial se lea sin consultar el esquema.
	Fields []models.GateSubmittedField
}

// GateChecker es lo que taskService necesita del motor para aplicar puertas sin tener
// que conocerlo. Se inyecta igual que el resto de enganches (SetSystemDM,
// SetCalendarSync, SetWorkflowEmitter).
//
// Devuelve (nil, nil) cuando la columna no tiene puerta: el caso corriente, y el que
// mantiene el Modo Libre intacto.
type GateChecker func(tenantID, boardID uint, toStatus string, submission map[string]any) (*GateResult, error)

// logGateMisconfigured deja constancia de una puerta que no se puede evaluar. Va
// aparte para que el motivo quede escrito en un solo sitio: se deja pasar, porque un
// formulario ilegible convertiría la columna en inalcanzable para todo el mundo.
func logGateMisconfigured(workflowID uint, err error) {
	log.Printf("[workflow] la puerta %d tiene un formulario inservible y se deja pasar: %v", workflowID, err)
}

// GateForStatus resuelve la puerta de una columna. Es una lectura por intento de
// movimiento, y sólo cuando el estado cambia de verdad.
func (s *WorkflowService) GateForStatus(tenantID, boardID uint, toStatus string) *models.Workflow {
	if s == nil || tenantID == 0 || boardID == 0 || toStatus == "" {
		return nil
	}
	candidates, err := s.repo.ListEnabledByTrigger(tenantID, models.TriggerTaskEnteringPhase)
	if err != nil || len(candidates) == 0 {
		return nil
	}

	board, err := s.boardRepo.GetByID(boardID)
	if err != nil || board == nil {
		return nil
	}

	for i := range candidates {
		wf := candidates[i]
		if wf.BoardID != boardID {
			continue
		}
		var scope struct {
			PhaseID uint `json:"phase_id"`
		}
		if err := json.Unmarshal([]byte(nonEmptyJSON(wf.TriggerConfig)), &scope); err != nil {
			continue
		}
		// Una puerta SIN fase no tiene sentido: sería un peaje en todas las columnas
		// del tablero, incluida aquella de la que se sale. Se descarta.
		if scope.PhaseID == 0 {
			continue
		}
		for _, p := range board.Phases {
			if p.ID == scope.PhaseID && PhaseColumnID(p) == toStatus {
				return &wf
			}
		}
	}
	return nil
}

// gateSchemaFor decide de dónde sale el formulario de una puerta.
//
// El de una receta lo define el catálogo, en código. Lo que se guardó en la fila al
// materializarla es sólo una copia: si mañana la receta deja de exigir el enlace, la
// puerta que alguien encendió la semana pasada tiene que dejar de exigirlo también, y
// no quedarse pidiendo lo que pedía tres versiones atrás. Hacerlo así evita además
// una migración por cada retoque de texto.
//
// Las puertas SIN receta —las del constructor propio, que llega en la Fase 4— usan su
// esquema guardado, que ahí sí es la fuente y no una copia.
func gateSchemaFor(wf *models.Workflow) string {
	if wf.RecipeKey != "" {
		if r, ok := findRecipe(wf.RecipeKey); ok && r.FormSchema != "" {
			return r.FormSchema
		}
	}
	return wf.FormSchema
}

// PhaseInUse dice si alguna puerta ENCENDIDA vigila esa columna, y cuál. Lo consulta
// el módulo de tableros antes de borrar una columna: borrarla dejaría la regla
// apuntando al vacío, viva en la pantalla y muda en el motor.
//
// Sólo mira las encendidas. Una apagada no está frenando a nadie, y su propia tarjeta
// ya avisa cuando la columna que vigilaba desaparece.
func (s *WorkflowService) PhaseInUse(tenantID, boardID, phaseID uint) (string, bool) {
	if s == nil || boardID == 0 || phaseID == 0 {
		return "", false
	}
	// El tenant es el DEL TABLERO, que es quien lo pasa: con el de quien borra, un
	// superadmin (tenant 0) no encontraría ninguna regla y se llevaría por delante la
	// columna que una empresa está vigilando.
	wfs, err := s.repo.ListByBoard(tenantID, boardID)
	if err != nil {
		// Ante la duda, no se bloquea el borrado: impedir una operación legítima por
		// un fallo de consulta es peor que dejar una regla huérfana, que además ya
		// se señala en su pantalla.
		return "", false
	}
	for _, wf := range wfs {
		if !wf.Enabled || wf.TriggerType != models.TriggerTaskEnteringPhase {
			continue
		}
		if phaseIDOf(wf) == phaseID {
			return wf.Name, true
		}
	}
	return "", false
}

// CheckGate es el punto único donde se decide si un movimiento pasa. Devuelve el
// formulario ya validado —listo para guardar en el historial— o un GateRequiredError
// que el handler traduce a 422.
//
// El resultado es nil, nil cuando la columna no tiene puerta: el caso corriente, y el
// que mantiene intacto el Modo Libre.
func (s *WorkflowService) CheckGate(tenantID, boardID uint, toStatus string, submission map[string]any) (*GateResult, error) {
	wf := s.GateForStatus(tenantID, boardID, toStatus)
	if wf == nil {
		return nil, nil
	}

	form, err := parseGateForm(gateSchemaFor(wf))
	if err != nil {
		// Una puerta rota no puede convertirse en un muro: si su formulario no se
		// puede interpretar, no hay forma de que nadie lo complete y la columna
		// quedaría inalcanzable. Se deja pasar y se registra el problema.
		logGateMisconfigured(wf.ID, err)
		return nil, nil
	}

	if submission == nil {
		return nil, &GateRequiredError{
			WorkflowID: wf.ID, Workflow: wf.Name, ToStatus: toStatus, Form: form,
		}
	}

	clean, filled, errs := validateGateSubmission(form, submission)
	if errs != nil {
		return nil, &GateRequiredError{
			WorkflowID: wf.ID, Workflow: wf.Name, ToStatus: toStatus, Form: form, Errors: errs,
		}
	}
	return &GateResult{WorkflowID: wf.ID, Data: clean, Fields: filled}, nil
}
