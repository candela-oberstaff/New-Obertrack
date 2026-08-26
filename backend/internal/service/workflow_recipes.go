package service

import (
	"fmt"

	"github.com/obertrack/backend/internal/models"
)

// Recetas: automatizaciones preconfiguradas que una empresa activa sobre un tablero
// con un interruptor, sin pasar por un constructor de reglas.
//
// Van antes que el constructor visual a propósito. Una regla escrita a mano exige
// que alguien sepa de antemano qué automatizar; una receta se activa en un clic y
// nos dice, por lo que la gente activa, qué merece la pena construir después. Al
// activarse se MATERIALIZAN como filas normales de workflows + workflow_steps, así
// que el motor no distingue una receta de una regla escrita a mano y el constructor
// de la Fase 4 podrá editarlas sin ningún caso especial.

// WorkflowRecipe es la plantilla de una receta. Vive en código y no en la base:
// sembrar cuatro filas por empresa dejaría copias divergentes en cuanto se corrigiera
// un texto, y ninguna empresa habría pedido las que no activó.
type WorkflowRecipe struct {
	Key         string `json:"key"`
	Name        string `json:"name"`
	Description string `json:"description"`
	// Explain describe en una línea a quién avisa, porque en esta pantalla el
	// destinatario no se elige (llega con el constructor, Fase 4) y hay que poder
	// leerlo antes de encender nada.
	Explain     string `json:"explain"`
	TriggerType string `json:"trigger_type"`
	Conditions  string `json:"-"`
	// FormSchema sólo lo llevan las recetas de PUERTA. Su presencia es lo que
	// distingue una receta que bloquea de una que sólo reacciona.
	FormSchema string `json:"-"`
	// NeedsPhase marca las recetas que exigen elegir una columna al activarlas. Una
	// puerta sin columna sería un peaje en todo el tablero, así que no se puede
	// encender a ciegas.
	NeedsPhase bool `json:"needs_phase"`
	// PhaseHint dice DÓNDE conviene ponerla. Elegir la columna equivocada no da
	// error: da una regla que parece rota. Una puerta de veredicto sobre la columna
	// de finalizados no puede mover a finalizados lo que ya está allí, y quien la
	// puso ve un formulario que "no hace nada".
	PhaseHint string `json:"phase_hint,omitempty"`
	// Mutates marca las recetas que MODIFICAN la tarea en vez de limitarse a avisar.
	// Quien las enciende tiene que saber que su tablero va a cambiar solo: una regla
	// que reasigna o reprioriza no se puede activar con la misma ligereza que una que
	// manda una campanita.
	Mutates bool         `json:"mutates"`
	Steps   []recipeStep `json:"-"`
}

type recipeStep struct {
	ActionType string
	Config     string
	// Conditions acota este paso. Vacío = se ejecuta siempre. Es lo que permite que
	// una receta de puerta haga una cosa u otra según lo respondido.
	Conditions string
}

// workflowRecipes es el catálogo de la v1. Los cuatro cubren los avisos que más se
// piden y, entre los cuatro, ejercitan los cuatro disparadores y las tres acciones.
var workflowRecipes = []WorkflowRecipe{
	// RETIRADA: "en_proceso_sin_responsable" (avisaba al líder del proyecto cuando
	// una tarea entraba en una columna sin nadie asignado).
	//
	// La cubre entera "asignar_si_empieza_sin_responsable": mismo disparador, misma
	// condición y el MISMO aviso al mismo destinatario, más la asignación y el
	// comentario. Tener las dos era ofrecer el mismo hecho con dos nombres, y quien
	// encendía ambas recibía el aviso por duplicado.
	{
		Key:         "prioridad_urgente",
		Name:        "Tarea marcada como urgente",
		Description: "Cuando alguien sube la prioridad de una tarea a urgente.",
		Explain:     "Avisa a los responsables y a sus managers.",
		TriggerType: models.TriggerTaskPriorityChanged,
		// "Cuando ALGUIEN la sube" hay que comprobarlo, no sólo decirlo: sin la
		// segunda condición, esta receta también reaccionaba al cambio que hace
		// "Marcar urgente al vencer", y quien tuviera las dos encendidas recibía dos
		// avisos por el mismo hecho.
		Conditions: `{"all":[{"field":"prioridad","op":"eq","value":"urgent"},{"field":"actor_es_sistema","op":"eq","value":false}]}`,
		Steps: []recipeStep{
			{
				ActionType: models.ActionNotify,
				Config: mustJSON(stepConfig{
					Recipient: models.RecipientAssignees,
					Title:     "Tarea urgente",
					Message:   `{{actor.nombre}} marcó como urgente: "{{tarea.titulo}}"`,
				}),
			},
			{
				ActionType: models.ActionNotify,
				Config: mustJSON(stepConfig{
					Recipient: models.RecipientAssigneeManager,
					Title:     "Tarea urgente en tu equipo",
					Message:   `"{{tarea.titulo}}" ({{tarea.asignados}}) pasó a urgente`,
				}),
			},
		},
	},
	// RETIRADA: "asignacion_por_chat" (aviso por chat al asignar). Tareas ya manda
	// ese mismo mensaje directo por su cuenta —task_service, al sumar responsables—,
	// así que la receta sólo lograba que quien la encendiera recibiera el aviso dos
	// veces. Se quitó del catálogo y una migración apagó las que estaban vivas; las
	// filas se conservan por su historial de ejecuciones.
	{
		Key:         "creada_sin_fecha",
		Name:        "Tarea creada sin fecha de fin",
		Description: "Cuando se crea una tarea y nadie le pone fecha límite.",
		Explain:     "Avisa a quien creó el tablero.",
		TriggerType: models.TriggerTaskCreated,
		Conditions:  `{"all":[{"field":"tiene_fecha_fin","op":"eq","value":false}]}`,
		Steps: []recipeStep{
			{
				ActionType: models.ActionNotify,
				Config: mustJSON(stepConfig{
					Recipient: models.RecipientBoardCreator,
					Title:     "Tarea sin fecha límite",
					Message:   `{{actor.nombre}} creó "{{tarea.titulo}}" sin fecha de fin`,
				}),
			},
		},
	},
	{
		Key:         "puerta_entrega",
		Name:        "Exigir entrega al pasar de columna",
		Description: "Convierte una columna en punto de control: para entrar hay que aportar la entrega.",
		Explain:     "Nadie mueve una tarjeta a esa columna sin rellenar el formulario. Queda registrado en el historial de la tarea.",
		TriggerType: models.TriggerTaskEnteringPhase,
		NeedsPhase:  true,
		PhaseHint:   "Va en la columna donde se entrega: revisión, validación o la que hagáis servir para eso.",
		// Lo obligatorio es DECIR qué entregas, no enlazarlo: mucho trabajo es interno
		// —una revisión, una llamada, algo que vive dentro de la propia herramienta— y
		// no tiene nada que enlazar. Exigir el enlace ahí sólo conseguía que la gente
		// pegara cualquier URL para poder mover la tarjeta, que es peor que no pedirlo:
		// ensucia el historial y da por buena una evidencia que no lleva a ningún sitio.
		FormSchema: `{
			"title": "Entrega para revisión",
			"description": "Deja constancia de qué entregas antes de mover la tarjeta.",
			"fields": [
				{"key":"resumen","label":"Qué entregas","type":"textarea","required":true,
				 "help":"Un par de líneas para quien lo revise. Es lo que queda en el historial de la tarea."},
				{"key":"enlace","label":"Enlace de la entrega","type":"url","required":false,
				 "placeholder":"https://…","help":"Repositorio, documento o despliegue. Déjalo vacío si el trabajo es interno y no hay nada que enlazar."},
				{"key":"evidencia","label":"Evidencia (opcional)","type":"file","required":false}
			]}`,
	},
	{
		Key:         "puerta_cierre",
		Name:        "Exigir reporte al cerrar",
		Description: "Para dar una tarea por terminada hay que decir cómo terminó.",
		Explain:     "Pide resultado y dedicación al entrar en la columna elegida, normalmente la de finalizadas.",
		TriggerType: models.TriggerTaskEnteringPhase,
		NeedsPhase:  true,
		PhaseHint:   "Va en la columna de finalizados: es lo último que se rellena antes de dar algo por cerrado.",
		FormSchema: `{
			"title": "Cierre de la tarea",
			"fields": [
				{"key":"resultado","label":"¿Cómo terminó?","type":"select","required":true,
				 "options":[
				   {"value":"completado","label":"Completado"},
				   {"value":"parcial","label":"Parcialmente, queda trabajo"},
				   {"value":"descartado","label":"Se descartó"}]},
				{"key":"horas","label":"Horas dedicadas","type":"number","required":false,"min":0,"max":999},
				{"key":"notas","label":"Notas de cierre","type":"textarea","required":false}
			]}`,
	},
	{
		// Las tres siguientes son las del CALENDARIO. No las provoca nadie: las emite
		// el barrido al pasar el día. Son las que atrapan el trabajo olvidado, que por
		// definición no se mueve y por tanto no disparaba nada.
		Key:         "aviso_al_vencer",
		Name:        "Aviso cuando una tarea vence",
		Description: "Cuando pasa la fecha de fin y la tarea sigue sin terminarse.",
		Explain:     "Avisa a los responsables y a sus managers. Una sola vez por vencimiento, no cada día.",
		TriggerType: models.TriggerTaskOverdue,
		Steps: []recipeStep{
			{
				ActionType: models.ActionNotify,
				Config: mustJSON(stepConfig{
					Recipient: models.RecipientAssignees,
					Title:     "Se te pasó una fecha",
					Message:   `"{{tarea.titulo}}" vencía el {{tarea.fecha_fin}} y sigue en {{tarea.estado}}`,
				}),
			},
			{
				ActionType: models.ActionNotify,
				Config: mustJSON(stepConfig{
					Recipient: models.RecipientAssigneeManager,
					Title:     "Una tarea de tu equipo venció",
					Message:   `"{{tarea.titulo}}" ({{tarea.asignados}}) vencía el {{tarea.fecha_fin}}`,
				}),
			},
		},
	},
	{
		Key:         "recordatorio_vispera",
		Name:        "Recordatorio el día antes",
		Description: "Cuando a una tarea le queda un día para su fecha de fin.",
		Explain:     "Avisa a los responsables la víspera. Es el aviso que evita el vencimiento, no el que lo lamenta.",
		TriggerType: models.TriggerTaskDueSoon,
		Steps: []recipeStep{
			{
				ActionType: models.ActionNotify,
				Config: mustJSON(stepConfig{
					Recipient: models.RecipientAssignees,
					Title:     "Mañana vence una tarea",
					Message:   `"{{tarea.titulo}}" vence el {{tarea.fecha_fin}}`,
				}),
			},
		},
	},
	{
		Key:         "urgente_al_vencer",
		Name:        "Marcar urgente al vencer",
		Description: "Cuando pasa la fecha de fin, sube la prioridad sin que nadie tenga que mirarlo.",
		Explain:     "Pone la tarea en urgente y avisa a los responsables y a sus managers. Actúa aunque nadie toque la tarjeta.",
		TriggerType: models.TriggerTaskOverdue,
		Mutates:     true,
		// Ya urgente no se vuelve a subir: sería una acción sin efecto y un aviso de
		// más. La comprobación está también en la acción, pero decirlo aquí ahorra
		// una ejecución entera.
		Conditions: `{"all":[{"field":"prioridad","op":"neq","value":"urgent"}]}`,
		Steps: []recipeStep{
			{
				ActionType: models.ActionSetPriority,
				Config:     mustJSON(stepConfig{Priority: "urgent"}),
			},
			{
				ActionType: models.ActionNotify,
				Config: mustJSON(stepConfig{
					Recipient: models.RecipientAssignees,
					Title:     "Tu tarea pasó a urgente",
					Message:   `"{{tarea.titulo}}" venció el {{tarea.fecha_fin}} y se marcó como urgente`,
				}),
			},
			{
				// El manager también, porque esta receta absorbió a la que hacía lo
				// mismo al mover la tarjeta ("urgente si va con retraso") y aquella sí
				// escalaba. Retirarla sin heredar su destinatario habría sido perder
				// una capacidad por el camino, no simplificar.
				ActionType: models.ActionNotify,
				Config: mustJSON(stepConfig{
					Recipient: models.RecipientAssigneeManager,
					Title:     "Una tarea de tu equipo pasó a urgente",
					Message:   `"{{tarea.titulo}}" ({{tarea.asignados}}) venció el {{tarea.fecha_fin}}`,
				}),
			},
		},
	},
	{
		// La puerta con consecuencia: quien revisa no sólo deja constancia, decide.
		// Sin esto, aprobar significaba rellenar un formulario y ADEMÁS acordarse de
		// arrastrar la tarjeta a finalizados, que es justo el paso que se olvida y
		// deja los tableros llenos de cosas terminadas que siguen en revisión.
		Key:         "puerta_veredicto",
		Name:        "Revisión con veredicto",
		Description: "Quien revisa decide en el momento: aprobar cierra la tarea, rechazar la devuelve.",
		Explain:     "Al entrar en la columna elegida pide el veredicto. Si aprueba, la tarjeta pasa sola a la columna de cierre; si rechaza, vuelve a la de entrada y avisa a los responsables.",
		TriggerType: models.TriggerTaskEnteringPhase,
		NeedsPhase:  true,
		PhaseHint:   "Va en la columna de REVISIÓN, no en la de finalizados: es ahí donde alguien decide. Puesta sobre la de finalizados, aprobar no tiene a dónde mover la tarjeta y sólo funciona la mitad de la regla.",
		Mutates:     true,
		FormSchema: `{
			"title": "Revisión",
			"description": "Tu respuesta mueve la tarjeta sola.",
			"fields": [
				{"key":"veredicto","label":"¿Cómo queda?","type":"select","required":true,
				 "options":[
				   {"value":"aprobado","label":"Aprobado: darla por terminada"},
				   {"value":"rechazado","label":"Devolver con cambios"}]},
				{"key":"comentario","label":"Comentario","type":"textarea","required":true,
				 "help":"Qué apruebas, o qué falta por corregir. Queda en la tarea, así que escríbelo para quien lo va a leer."}
			]}`,
		Steps: []recipeStep{
			{
				// Aprobado: a la columna de cierre. El destino se resuelve por el
				// papel de la columna en ESTE tablero, no por un nombre fijo.
				ActionType: models.ActionSetStatus,
				Conditions: `{"all":[{"field":"respuesta.veredicto","op":"eq","value":"aprobado"}]}`,
				Config:     mustJSON(stepConfig{Status: statusBoardEnd}),
			},
			{
				ActionType: models.ActionComment,
				Conditions: `{"all":[{"field":"respuesta.veredicto","op":"eq","value":"aprobado"}]}`,
				Config: mustJSON(stepConfig{
					Content: `Aprobada en revisión por {{actor.nombre}}: {{respuestas.comentario}}`,
				}),
			},
			{
				ActionType: models.ActionSetStatus,
				Conditions: `{"all":[{"field":"respuesta.veredicto","op":"eq","value":"rechazado"}]}`,
				Config:     mustJSON(stepConfig{Status: statusBoardStart}),
			},
			{
				// El motivo va a la tarea antes que el aviso: cuando alguien abra la
				// campanita y entre a la tarjeta, el comentario ya tiene que estar.
				ActionType: models.ActionComment,
				Conditions: `{"all":[{"field":"respuesta.veredicto","op":"eq","value":"rechazado"}]}`,
				Config: mustJSON(stepConfig{
					Content: `Devuelta por {{actor.nombre}}: {{respuestas.comentario}}`,
				}),
			},
			{
				ActionType: models.ActionNotify,
				Conditions: `{"all":[{"field":"respuesta.veredicto","op":"eq","value":"rechazado"}]}`,
				Config: mustJSON(stepConfig{
					Recipient: models.RecipientAssignees,
					Title:     "Tu entrega volvió con cambios",
					Message:   `{{actor.nombre}} devolvió "{{tarea.titulo}}": {{respuestas.comentario}}`,
				}),
			},
		},
	},
	{
		Key:         "asignar_si_empieza_sin_responsable",
		Name:        "Asignar sola si empieza sin responsable",
		Description: "Cuando una tarea cambia de columna y no tiene a nadie asignado.",
		Explain:     "Se la asigna al líder del proyecto —manager del responsable, supervisor del tablero o quien creó el tablero—, lo deja explicado en un comentario y le avisa.",
		TriggerType: models.TriggerTaskStatusChanged,
		Mutates:     true,
		Conditions:  `{"all":[{"field":"tiene_responsable","op":"eq","value":false},{"field":"completada","op":"eq","value":false}]}`,
		Steps: []recipeStep{
			{
				ActionType: models.ActionAssign,
				Config:     mustJSON(stepConfig{Recipient: models.RecipientProjectLead}),
			},
			{
				// Un cambio automático sin explicación deja a la gente preguntándose
				// quién tocó su tarjeta. El comentario lo cuenta donde se va a leer.
				ActionType: models.ActionComment,
				Config: mustJSON(stepConfig{
					Content: `Asignada automáticamente: entró en {{tarea.estado}} sin responsable.`,
				}),
			},
			{
				ActionType: models.ActionNotify,
				Config: mustJSON(stepConfig{
					Recipient: models.RecipientProjectLead,
					Title:     "Se te asignó una tarea sin responsable",
					Message:   `"{{tarea.titulo}}" pasó a {{tarea.estado}} sin nadie asignado`,
				}),
			},
		},
	},
	// RETIRADA: "urgente_si_va_con_retraso" (subía a urgente al MOVER una tarjeta ya
	// vencida).
	//
	// Dependía de que alguien la moviera, que es justo lo que no pasa con el trabajo
	// olvidado. La cubre "urgente_al_vencer", que hace lo mismo por calendario —sin
	// esperar a que nadie la toque— y desde ahora avisa también a los managers, que
	// era lo único que ésta añadía.
}

func findRecipe(key string) (WorkflowRecipe, bool) {
	for _, r := range workflowRecipes {
		if r.Key == key {
			return r, true
		}
	}
	return WorkflowRecipe{}, false
}

// RecipeState es una receta con su estado en un tablero concreto: si ya se activó
// alguna vez y si está encendida ahora. La pantalla necesita las dos cosas —una
// receta apagada que existe se vuelve a encender sin recrearla.
type RecipeState struct {
	WorkflowRecipe
	WorkflowID uint `json:"workflow_id,omitempty"`
	Enabled    bool `json:"enabled"`
	Exists     bool `json:"exists"`
	// PhaseID es la columna sobre la que quedó puesta una puerta ya activada. 0 en
	// las recetas reactivas y en las puertas que aún no se han encendido.
	PhaseID uint `json:"phase_id,omitempty"`
	// PhaseMissing marca una puerta que vigila una columna que ya no está en el
	// tablero. Borrar la columna no borra la regla, así que sin esto la pantalla la
	// enseñaba "activa" mientras el motor no la disparaba nunca: encendida por fuera
	// y muda por dentro, que es la peor combinación posible.
	PhaseMissing bool `json:"phase_missing,omitempty"`
}

// refreshedSteps devuelve los pasos que hay que ejecutar, con el texto y las
// condiciones del CATÁLOGO cuando la regla nació de una receta.
//
// Al encender una receta sus pasos se copian a la fila. Esa copia envejece: corregir
// el mensaje de una receta no llegaba a quien ya la tenía encendida, y cada empresa
// acababa con la redacción del día en que pulsó el interruptor. Igual que con el
// formulario de una puerta, manda el catálogo.
//
// Se conservan los IDs guardados —son los que usa el reintento para no repetir un
// aviso ya enviado— y sólo se refresca lo que es texto: configuración y condiciones.
// Si la receta cambió de FORMA (otro número de pasos, u otra acción en una posición),
// eso ya no es un retoque de redacción sino otra regla: se respeta lo guardado, que
// es lo que la gente encendió, y el cambio pide una migración explícita.
func refreshedSteps(wf *models.Workflow) []models.WorkflowStep {
	if wf == nil || wf.RecipeKey == "" {
		return wf.Steps
	}
	r, ok := findRecipe(wf.RecipeKey)
	if !ok || len(r.Steps) != len(wf.Steps) {
		return wf.Steps
	}
	out := make([]models.WorkflowStep, len(wf.Steps))
	for i, st := range wf.Steps {
		if r.Steps[i].ActionType != st.ActionType {
			return wf.Steps
		}
		out[i] = st
		out[i].Config = nonEmptyJSON(r.Steps[i].Config)
		out[i].Conditions = nonEmptyJSON(r.Steps[i].Conditions)
	}
	return out
}

// recipeWorkflowName es cómo se llama la regla materializada. Sirve además para
// reconocerla: el vínculo receta↔regla se guarda en trigger_config.recipe.
func recipeWorkflowName(r WorkflowRecipe, boardName string) string {
	if boardName == "" {
		return r.Name
	}
	return fmt.Sprintf("%s · %s", r.Name, boardName)
}
