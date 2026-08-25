package models

// Modo Workflow: la puerta de fase.
//
// A diferencia de los disparadores reactivos —que corren en un worker DESPUÉS de que
// el cambio ocurrió— una puerta se interpone ANTES: intercepta el intento de mover
// una tarjeta a su columna, exige un formulario y sólo entonces deja pasar.
//
// Vive sobre la misma entidad Workflow (mismo ámbito por tablero y fase, mismas
// condiciones, mismo interruptor, misma bitácora) con un trigger_type propio y un
// esquema de formulario. Una entidad paralela habría duplicado pantalla, permisos y
// registro para no ganar nada.

// TriggerTaskEnteringPhase es el disparador de puerta: alguien intenta llevar una
// tarea a la fase configurada. Es el único que BLOQUEA.
const TriggerTaskEnteringPhase = "task.entering_phase"

// Tipos de campo admitidos en el formulario.
const (
	GateFieldText     = "text"
	GateFieldTextarea = "textarea"
	GateFieldURL      = "url"
	GateFieldSelect   = "select"
	// GateFieldFile guarda la URL que devuelve /api/uploads. El archivo se sube
	// ANTES de enviar el formulario, así que el movimiento nunca tiene que ser
	// transaccional con un fichero: cuando la puerta se evalúa, la subida ya
	// terminó y lo que viaja es una cadena como cualquier otra.
	GateFieldFile   = "file"
	GateFieldDate   = "date"
	GateFieldNumber = "number"
)

func IsValidGateFieldType(t string) bool {
	switch t {
	case GateFieldText, GateFieldTextarea, GateFieldURL,
		GateFieldSelect, GateFieldFile, GateFieldDate, GateFieldNumber:
		return true
	}
	return false
}

// GateOption es una opción de un campo de selección.
type GateOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// GateField es un campo del formulario. Se serializa tal cual hacia el cliente: la
// respuesta 422 lleva este esquema para que el modal se dibuje sin que el frontend
// tenga que conocer la puerta de antemano. Eso es lo que permite que una app que no
// se ha actualizado respete una puerta nueva.
type GateField struct {
	Key      string `json:"key"`
	Label    string `json:"label"`
	Type     string `json:"type"`
	Required bool   `json:"required"`
	// Help es el texto de ayuda bajo el campo. Opcional.
	Help string `json:"help,omitempty"`
	// Placeholder sólo aplica a los campos de texto.
	Placeholder string `json:"placeholder,omitempty"`
	// Options sólo aplica a select; se exige no vacío al validar el esquema.
	Options []GateOption `json:"options,omitempty"`
	// Min y Max acotan los campos numéricos. Nulos = sin tope.
	Min *float64 `json:"min,omitempty"`
	Max *float64 `json:"max,omitempty"`
	// MaxLength acota los de texto. 0 = el límite por defecto.
	MaxLength int `json:"max_length,omitempty"`
}

// GateForm es el formulario completo de una puerta.
type GateForm struct {
	Title       string      `json:"title"`
	Description string      `json:"description,omitempty"`
	Fields      []GateField `json:"fields"`
}

// GateFieldMaxLength es el tope por defecto de un campo de texto. Existe para que un
// formulario sin límites declarados no permita pegar un documento entero en una
// celda que después se muestra en el historial de la tarea.
const GateFieldMaxLength = 2000

// GateMaxFields acota cuántos campos puede pedir una puerta. Un formulario más largo
// que esto deja de ser un punto de control y pasa a ser un obstáculo: quien mueve una
// tarjeta lo rellenará a la ligera, y entonces el registro no vale nada.
const GateMaxFields = 12

// Topes de lo que se puede escribir en un formulario.
//
// Hasta ahora los formularios venían del catálogo, escritos por nosotros. Desde el
// constructor los escribe cualquiera con permiso, y estos números son la diferencia
// entre una pantalla y un formulario que nadie puede rellenar: una etiqueta de tres
// mil caracteres o un desplegable de doscientas opciones no se rechazan por gusto,
// se rechazan porque bloquearían la columna para todo el equipo.
const (
	GateMaxTitle       = 120
	GateMaxDescription = 300
	GateMaxLabel       = 80
	GateMaxHelp        = 200
	GateMaxPlaceholder = 80
	GateMaxOptions     = 12
	GateMaxOptionText  = 60
	// GateMaxKey acota la clave con la que se guarda la respuesta en el historial.
	GateMaxKey = 40
	// GateMaxName acota el nombre de la puerta, que es como aparece en la pantalla
	// de automatizaciones y en el registro de ejecuciones.
	GateMaxName = 80
)

// GateSubmittedField es un campo tal como se respondió al cruzar una puerta: la
// etiqueta y el tipo viajan JUNTO al valor, no se buscan después en el esquema.
//
// Es deliberado. El formulario de una puerta se puede editar, y un registro de
// auditoría tiene que decir qué se preguntó ENTONCES, no qué se pregunta ahora. Si
// el historial resolviera las etiquetas contra el esquema vigente, cambiar una
// pregunta reescribiría el pasado de todas las tareas que ya la respondieron.
type GateSubmittedField struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Type  string `json:"type"`
	Value any    `json:"value"`
}

// GateSubmission es lo que se guarda en task_status_history.form_data.
type GateSubmission struct {
	Fields []GateSubmittedField `json:"fields"`
}
