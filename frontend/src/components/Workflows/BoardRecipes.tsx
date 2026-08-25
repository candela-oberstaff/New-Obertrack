import { useEffect, useState, useCallback, useMemo } from 'react'
import { AlertCircle, Activity, Users, Columns3, Lock, Wand2, Inbox, Plus, Pencil, Trash2 } from 'lucide-react'
import {
  workflowService,
  type BoardGate,
  type WorkflowRecipe,
  type WorkflowRun,
} from '../../services/workflow.service'
import { Select } from '../ui/Select'
import { formatRelative } from '../../utils/date'
import { GateBuilder } from './GateBuilder'
import styles from './BoardRecipes.module.css'

// Automatizaciones de UN tablero: catálogo fijo de recetas que se encienden con un
// interruptor. Vive dentro del tablero —y no en una pantalla aparte con su propio
// selector— porque una automatización sólo significa algo respecto del tablero
// sobre el que actúa; elegirlo otra vez era pedirle al usuario que repitiera una
// decisión que ya había tomado al entrar.
//
// Todavía no hay constructor, así que cada tarjeta tiene que responder sola a las
// tres preguntas de quien va a encenderla: cuándo salta, qué hace y a quién avisa.

/** Cuándo salta cada regla, en la lengua del usuario y no en la del motor. */
const TRIGGER_LABELS: Record<string, string> = {
  'task.created': 'Al crear una tarea',
  'task.status_changed': 'Al cambiar de columna',
  'task.priority_changed': 'Al cambiar la prioridad',
  'task.assigned': 'Al asignar a alguien',
  // Las puertas se leen distinto porque hacen algo distinto: no reaccionan al
  // cambio, lo interceptan.
  'task.entering_phase': 'Al entrar en la columna',
  // Y estas dos no las provoca nadie: las provoca el calendario.
  'task.overdue': 'Al vencer la fecha',
  'task.due_soon': 'El día antes de vencer',
}

const RUN_LABELS: Record<WorkflowRun['status'], string> = {
  pending: 'En cola',
  running: 'Ejecutándose',
  done: 'Ejecutada',
  failed: 'Falló',
  skipped: 'Sin efecto',
}

// Las recetas se agrupan por lo que HACEN, no por el orden en que las devuelve el
// motor. Un aviso, una regla que reescribe la tarea y una puerta que bloquea el
// tablero se encienden con el mismo interruptor pero no se deciden con el mismo
// criterio, y en una lista corrida se activaban con la misma ligereza. Van de menos
// a más consecuencia.
const GROUPS = [
  {
    key: 'avisos',
    title: 'Avisos',
    note: 'Mandan un mensaje cuando pasa algo. No cambian la tarea.',
  },
  {
    key: 'acciones',
    title: 'Acciones automáticas',
    note: 'Cambian la tarea solas: prioridad, responsable o un comentario.',
  },
  {
    key: 'puertas',
    title: 'Puntos de control',
    note: 'Frenan la tarjeta al entrar en una columna hasta que alguien rellena el formulario.',
  },
  {
    key: 'calendario',
    title: 'Por calendario',
    note: 'Saltan solas al pasar el día, sin que nadie toque la tarjeta. Son las que atrapan el trabajo olvidado.',
  },
] as const

const TIEMPO = ['task.overdue', 'task.due_soon']

const groupOf = (recipe: WorkflowRecipe) => {
  if (recipe.trigger_type === 'task.entering_phase') return 'puertas'
  // El calendario va aparte aunque la regla modifique la tarea: lo que la distingue
  // de las demás no es lo que hace, sino que actúa sin que nadie haga nada.
  if (TIEMPO.includes(recipe.trigger_type)) return 'calendario'
  return recipe.mutates ? 'acciones' : 'avisos'
}

interface BoardRecipesProps {
  boardId: number
  /** Columnas del tablero: las puertas actúan sobre una de ellas. */
  phases: { id: number; name: string }[]
  /** Cuántas están encendidas, para que el contenedor pueda titularlo. */
  onCountChange?: (active: number, total: number) => void
}

export function BoardRecipes({ boardId, phases, onCountChange }: BoardRecipesProps) {
  const [recipes, setRecipes] = useState<WorkflowRecipe[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  // Interruptor en vuelo: deshabilita sólo esa fila mientras viaja, no la lista.
  const [saving, setSaving] = useState<string | null>(null)
  const [historyFor, setHistoryFor] = useState<number | null>(null)
  // Columna elegida para cada receta de PUERTA que aún no se ha activado. El
  // servidor la exige, así que el interruptor no se habilita sin ella.
  const [phaseChoice, setPhaseChoice] = useState<Record<string, number>>({})
  const [runs, setRuns] = useState<WorkflowRun[] | null>(null)
  // Puertas propias del tablero y el constructor abierto sobre una de ellas.
  // 'nueva' distingue crear de editar sin necesitar un segundo estado.
  const [gates, setGates] = useState<BoardGate[]>([])
  const [editing, setEditing] = useState<BoardGate | 'nueva' | null>(null)

  const loadGates = useCallback(async (id: number) => {
    try {
      setGates(await workflowService.gates(id))
    } catch {
      // Un fallo aquí no puede tapar el catálogo: son dos listas independientes.
      setGates([])
    }
  }, [])

  const loadRecipes = useCallback(async (id: number) => {
    setError('')
    try {
      setRecipes(await workflowService.recipes(id))
    } catch {
      // El backend comprueba el alcance tablero a tablero, así que un 403 aquí es
      // información real y no un fallo: este tablero no es tuyo.
      setError('No tienes acceso a las automatizaciones de este tablero')
      setRecipes([])
    }
  }, [])

  useEffect(() => {
    setHistoryFor(null)
    setRuns(null)
    setPhaseChoice({})
    setEditing(null)
    setLoading(true)
    loadGates(boardId)
    loadRecipes(boardId).finally(() => setLoading(false))
  }, [boardId, loadRecipes, loadGates])

  // El recuento del botón cuenta TODO lo encendido: para quien mira el tablero, una
  // puerta propia activa pesa igual que una receta activa.
  const activeCount = useMemo(
    () => recipes.filter((r) => r.enabled).length + gates.filter((g) => g.enabled).length,
    [recipes, gates],
  )
  const total = recipes.length + gates.length

  useEffect(() => {
    onCountChange?.(activeCount, total)
  }, [activeCount, total, onCountChange])

  // Columnas ya ocupadas por un punto de control, propias y de receta: una columna
  // sólo admite uno, y el constructor tiene que poder decirlo antes de guardar.
  const takenPhases = useMemo(() => {
    const out: { id: number; by: string }[] = []
    recipes.forEach((r) => {
      if (r.trigger_type === 'task.entering_phase' && r.exists && r.phase_id) {
        out.push({ id: r.phase_id, by: r.name })
      }
    })
    gates.forEach((g) => g.phase_id && out.push({ id: g.phase_id, by: g.name }))
    return out
  }, [recipes, gates])

  const toggleGate = async (gate: BoardGate, enabled: boolean) => {
    setGates((prev) => prev.map((g) => (g.id === gate.id ? { ...g, enabled } : g)))
    try {
      await workflowService.updateGate(gate.id, {
        board_id: boardId,
        phase_id: gate.phase_id,
        name: gate.name,
        enabled,
        form: gate.form,
      })
      await loadGates(boardId)
    } catch {
      setGates((prev) => prev.map((g) => (g.id === gate.id ? { ...g, enabled: !enabled } : g)))
      setError('No se pudo guardar el cambio')
    }
  }

  const removeGate = async (gate: BoardGate) => {
    // Borrar una puerta es quitar un control que alguien puso a propósito: se
    // pregunta, y el nombre va en la pregunta para que no se borre la equivocada.
    if (!window.confirm(`¿Eliminar el punto de control "${gate.name}"? Las tarjetas volverán a entrar en esa columna sin formulario.`)) return
    try {
      await workflowService.deleteGate(gate.id)
      await loadGates(boardId)
    } catch {
      setError('No se pudo eliminar la puerta')
    }
  }

  const toggle = async (recipe: WorkflowRecipe, enabled: boolean) => {
    setSaving(recipe.key)
    // Optimista: el interruptor responde al instante y se revierte si falla.
    setRecipes((prev) => prev.map((r) => (r.key === recipe.key ? { ...r, enabled } : r)))
    try {
      await workflowService.setRecipe(boardId, recipe.key, enabled, phaseChoice[recipe.key] || recipe.phase_id)
      await loadRecipes(boardId)
    } catch {
      setRecipes((prev) => prev.map((r) => (r.key === recipe.key ? { ...r, enabled: !enabled } : r)))
      setError('No se pudo guardar el cambio')
    } finally {
      setSaving(null)
    }
  }

  // Cambiar la columna de una puerta YA creada. Se guarda al momento porque es un
  // único valor y no hay nada más que confirmar; si falla, se recarga el catálogo
  // para que la lista no se quede mostrando una elección que no llegó a cuajar.
  const changePhase = async (recipe: WorkflowRecipe, phaseId: number) => {
    setPhaseChoice((prev) => ({ ...prev, [recipe.key]: phaseId }))
    if (!recipe.exists || !phaseId) return
    setSaving(recipe.key)
    try {
      await workflowService.setRecipe(boardId, recipe.key, recipe.enabled, phaseId)
      await loadRecipes(boardId)
    } catch {
      setError('No se pudo cambiar la columna')
      await loadRecipes(boardId)
    } finally {
      setSaving(null)
    }
  }

  const openHistory = async (recipe: WorkflowRecipe) => {
    if (!recipe.workflow_id) return
    if (historyFor === recipe.workflow_id) {
      setHistoryFor(null)
      return
    }
    setHistoryFor(recipe.workflow_id)
    setRuns(null)
    try {
      setRuns(await workflowService.runs(recipe.workflow_id))
    } catch {
      setRuns([])
    }
  }

  if (loading) {
    return (
      <div className={styles['list']}>
        <div className={styles['skeleton-card']} />
        <div className={styles['skeleton-card']} />
      </div>
    )
  }

  const renderRecipe = (recipe: WorkflowRecipe) => {
    const isOpen = historyFor === recipe.workflow_id
    // Una puerta necesita saber sobre qué columna actúa. Mientras no se elija,
    // el interruptor queda bloqueado: encenderla sin columna la haría un peaje
    // en todo el tablero, y el servidor la rechazaría igualmente.
    // Una puerta no avisa: impide. Merece leerse distinto de un vistazo.
    const isGate = recipe.trigger_type === 'task.entering_phase'
    const needsColumn = !!recipe.needs_phase && !recipe.exists
    const columnaElegida = phaseChoice[recipe.key] || recipe.phase_id
    return (
      <li
        key={recipe.key}
        className={`${styles['recipe']} ${recipe.enabled ? styles['recipe-on'] : ''} ${isGate ? styles['recipe-gate'] : ''}`}
      >
        <div className={styles['recipe-main']}>
          <div className={styles['recipe-text']}>
            <div className={styles['recipe-head']}>
              <h3>{recipe.name}</h3>
              <span className={`${styles['trigger']} ${isGate ? styles['trigger-gate'] : ''}`}>
                {isGate && <Lock size={11} />}
                {TRIGGER_LABELS[recipe.trigger_type] || recipe.trigger_type}
              </span>
              {/* Una regla que reasigna o reprioriza cambia el tablero sola. Que
                  se vea antes de encenderla evita activarla con la ligereza con
                  la que se activa un aviso. */}
              {recipe.mutates && (
                <span className={`${styles['trigger']} ${styles['trigger-mutates']}`}>
                  <Wand2 size={11} /> Modifica tareas
                </span>
              )}
            </div>

            <p className={styles['recipe-desc']}>{recipe.description}</p>

            {/* A quién avisa. Va con icono y en tono neutro, no en azul de
                enlace: es información, no algo en lo que se pueda pulsar. */}
            <p className={styles['recipe-who']}>
              <Users size={14} />
              <span>{recipe.explain}</span>
            </p>

            {recipe.needs_phase && (
              <div className={styles['recipe-column']}>
                <Columns3 size={14} />
                <div className={styles['recipe-column-picker']}>
                  <span>
                    {recipe.exists
                      ? 'Punto de control (se puede cambiar)'
                      : 'Columna que será el punto de control'}
                  </span>
                  {/* Dónde va. Una puerta en la columna equivocada no falla: se queda
                      a medias, y desde fuera parece que no hace nada. */}
                  {recipe.phase_hint && (
                    <span className={styles['recipe-column-hint']}>{recipe.phase_hint}</span>
                  )}
                  {/* Borrar una columna no borra la puerta que la vigilaba. Sin decirlo,
                      la regla se ve "activa" y no dispara nunca. */}
                  {recipe.phase_missing && (
                    <span className={styles['recipe-column-broken']} role="alert">
                      <AlertCircle size={12} /> La columna que vigilaba ya no existe: elige
                      otra o apágala. Mientras tanto no actúa.
                    </span>
                  )}
                  <Select
                    value={phaseChoice[recipe.key] ?? recipe.phase_id ?? ''}
                    disabled={saving === recipe.key}
                    placeholder="Elige una columna…"
                    options={phases.map((p) => ({ value: p.id, label: p.name }))}
                    onChange={(v) => changePhase(recipe, Number(v))}
                    ariaLabel={`Columna para ${recipe.name}`}
                    fullWidth
                  />
                  {/* Una puerta encendida deja de vigilar una columna y empieza a
                      vigilar otra en cuanto se cambia. Decirlo evita que alguien la
                      mueva creyendo que sólo prepara el cambio. */}
                  {recipe.exists && recipe.enabled && (
                    <span className={styles['recipe-column-warn']}>
                      El cambio surte efecto de inmediato.
                    </span>
                  )}
                </div>
              </div>
            )}
          </div>

          <div className={styles['recipe-switch']}>
            <label className={styles['switch']}>
              <input
                type="checkbox"
                checked={recipe.enabled}
                disabled={saving === recipe.key || (needsColumn && !columnaElegida)}
                aria-busy={saving === recipe.key}
                onChange={(e) => toggle(recipe, e.target.checked)}
                aria-label={`${recipe.enabled ? 'Desactivar' : 'Activar'} ${recipe.name}`}
              />
              <span className={styles['slider']} />
            </label>
            {/* Una puerta sin columna no se puede encender: sería un peaje en todo el
                tablero y el servidor la rechaza igualmente. El interruptor queda
                bloqueado, y hasta ahora sin decir por qué: se leía como que la
                pantalla no respondía. */}
            <span
              className={`${styles['switch-state']} ${
                needsColumn && !columnaElegida ? styles['switch-blocked'] : ''
              }`}
            >
              {needsColumn && !columnaElegida
                ? 'Elige la columna'
                : recipe.enabled
                  ? 'Activa'
                  : 'Apagada'}
            </span>
          </div>
        </div>

        {recipe.exists && (
          <div className={styles['recipe-footer']}>
            <button
              type="button"
              className={styles['history-btn']}
              onClick={() => openHistory(recipe)}
              aria-expanded={isOpen}
            >
              <Activity size={13} />
              {isOpen ? 'Ocultar actividad' : 'Ver actividad'}
            </button>

            {isOpen &&
              (runs === null ? (
                <p className={styles['muted']}>Cargando actividad…</p>
              ) : runs.length === 0 ? (
                <p className={styles['muted']}>
                  Todavía no se ha ejecutado. Ocurrirá la próxima vez que se cumpla
                  el disparador en este tablero.
                </p>
              ) : (
                <ul className={styles['runs']}>
                  {runs.map((run) => (
                    <li key={run.id} className={styles['run']}>
                      <span className={styles['run-line']}>
                        <span
                          className={`${styles['badge']} ${styles[`badge-${run.status}`] || ''}`}
                        >
                          {RUN_LABELS[run.status]}
                        </span>
                        <span className={styles['run-task']}>Tarea #{run.entity_id}</span>
                        <span className={styles['run-when']}>
                          {formatRelative(run.created_at)}
                        </span>
                      </span>
                      {/* El motivo es lo que distingue una regla que decidió no
                          actuar de una regla rota. */}
                      {run.skip_reason && (
                        <span className={styles['run-reason']}>{run.skip_reason}</span>
                      )}
                      {run.last_error && (
                        <span className={styles['run-error']}>{run.last_error}</span>
                      )}
                    </li>
                  ))}
                </ul>
              ))}
          </div>
        )}
      </li>
    )
  }

  const groups = GROUPS.map((g) => ({ ...g, items: recipes.filter((r) => groupOf(r) === g.key) }))
    .filter((g) => g.items.length > 0)

  return (
    <div className={styles['list']}>
      {error && (
        <div className={styles['error']} role="alert">
          <AlertCircle size={16} /> {error}
        </div>
      )}

      {!error && recipes.length === 0 && (
        <div className={styles['empty']}>
          <Inbox size={26} />
          <p>No hay automatizaciones disponibles.</p>
          <span>El catálogo llega del servidor; si está vacío, no hay recetas activas para tu empresa.</span>
        </div>
      )}

      {/* Puertas propias: el constructor. Van ARRIBA del catálogo porque son las que
          esta empresa se hizo a medida; las recetas están siempre y no cambian. */}
      <section className={styles['section']}>
        <div className={styles['section-head']}>
          <h4>Tus puntos de control</h4>
          <span className={styles['section-count']}>
            {gates.filter((g) => g.enabled).length}/{gates.length}
          </span>
        </div>
        <p className={styles['section-note']}>
          Formularios propios: tú decides qué se pregunta antes de dejar entrar una
          tarjeta en una columna.
        </p>

        {editing ? (
          <div className={styles['builder-wrap']}>
            <GateBuilder
              boardId={boardId}
              phases={phases}
              gate={editing === 'nueva' ? undefined : editing}
              takenPhases={takenPhases}
              onCancel={() => setEditing(null)}
              onSaved={() => {
                setEditing(null)
                loadGates(boardId)
              }}
            />
          </div>
        ) : (
          <>
            <ul className={styles['gates']}>
              {gates.map((gate) => (
                <li
                  key={gate.id}
                  className={`${styles['gate']} ${gate.enabled ? styles['gate-on'] : ''}`}
                >
                  <div className={styles['gate-main']}>
                    <div className={styles['gate-text']}>
                      <h3>
                        <Lock size={13} /> {gate.name}
                      </h3>
                      <p className={styles['recipe-desc']}>
                        {phases.find((p) => p.id === gate.phase_id)?.name ?? 'Columna desconocida'}
                        {' · '}
                        {gate.form?.fields?.length ?? 0}{' '}
                        {(gate.form?.fields?.length ?? 0) === 1 ? 'pregunta' : 'preguntas'}
                      </p>
                      {gate.phase_missing && (
                        <span className={styles['recipe-column-broken']} role="alert">
                          <AlertCircle size={12} /> La columna que vigilaba ya no existe:
                          edítala o bórrala. Mientras tanto no actúa.
                        </span>
                      )}
                    </div>
                    <div className={styles['gate-acciones']}>
                      <button type="button" onClick={() => setEditing(gate)} title="Editar">
                        <Pencil size={14} />
                      </button>
                      <button
                        type="button"
                        className={styles['gate-borrar']}
                        onClick={() => removeGate(gate)}
                        title="Eliminar"
                      >
                        <Trash2 size={14} />
                      </button>
                      <label className={styles['switch']}>
                        <input
                          type="checkbox"
                          checked={gate.enabled}
                          onChange={(e) => toggleGate(gate, e.target.checked)}
                          aria-label={`${gate.enabled ? 'Desactivar' : 'Activar'} ${gate.name}`}
                        />
                        <span className={styles['slider']} />
                      </label>
                    </div>
                  </div>
                </li>
              ))}
            </ul>

            <button
              type="button"
              className={styles['nueva-puerta']}
              onClick={() => setEditing('nueva')}
            >
              <Plus size={15} /> Nuevo punto de control
            </button>
          </>
        )}
      </section>

      {groups.map((group) => (
        <section key={group.key} className={styles['section']}>
          <div className={styles['section-head']}>
            <h4>{group.title}</h4>
            <span className={styles['section-count']}>
              {group.items.filter((r) => r.enabled).length}/{group.items.length}
            </span>
          </div>
          <p className={styles['section-note']}>{group.note}</p>
          <ul className={styles['recipes']}>{group.items.map(renderRecipe)}</ul>
        </section>
      ))}
    </div>
  )
}
