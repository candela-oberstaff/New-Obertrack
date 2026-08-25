import api from './client'

/** Una receta del catálogo con su estado en un tablero concreto. */
export interface WorkflowRecipe {
  key: string
  name: string
  description: string
  /** A quién avisa. En esta versión el destinatario no se elige, así que hay que
   *  poder leerlo antes de encender nada. */
  explain: string
  trigger_type: string
  /** true si la receta ya se materializó alguna vez en este tablero. */
  exists: boolean
  enabled: boolean
  workflow_id?: number
  /**
   * Las recetas de PUERTA exigen elegir la columna que se convierte en punto de
   * control. Sin columna sería un peaje en todo el tablero, así que el servidor
   * rechaza activarlas a ciegas.
   */
  needs_phase?: boolean
  /** Dónde conviene ponerla: elegir mal la columna no da error, da una regla que parece rota. */
  phase_hint?: string
  /** La columna que vigilaba ya no está en el tablero: la regla figura activa pero no dispara. */
  phase_missing?: boolean
  /**
   * true si la receta MODIFICA la tarea (reasigna, reprioriza, mueve o comenta) en
   * vez de limitarse a avisar. Quien la enciende tiene que saber que su tablero va a
   * cambiar solo.
   */
  mutates?: boolean
  /** Columna elegida, en las puertas ya activadas. */
  phase_id?: number
}

export interface WorkflowSummary {
  id: number
  name: string
  description: string
  enabled: boolean
  trigger_type: string
  board_id: number
  recipe_key?: string
  step_count: number
}

export interface WorkflowRun {
  id: number
  status: 'pending' | 'running' | 'done' | 'failed' | 'skipped'
  skip_reason?: string
  last_error?: string
  attempts: number
  entity_id: number
  created_at: string
}

// ── Puertas propias (constructor) ──

/** Tipos de campo que el motor sabe pedir y validar. */
export type GateFieldType = 'text' | 'textarea' | 'url' | 'select' | 'file' | 'date' | 'number'

export interface GateOption {
  value: string
  label: string
}

export interface GateField {
  key: string
  label: string
  type: GateFieldType
  required: boolean
  help?: string
  placeholder?: string
  options?: GateOption[]
  min?: number
  max?: number
  max_length?: number
}

export interface GateForm {
  title: string
  description?: string
  fields: GateField[]
}

/** Una puerta creada desde el tablero, con su formulario para poder editarlo. */
export interface BoardGate {
  id: number
  name: string
  enabled: boolean
  board_id: number
  phase_id: number
  /** La columna que vigilaba ya no está: figura activa y no dispara nunca. */
  phase_missing?: boolean
  form: GateForm
}

export interface GateInput {
  board_id: number
  phase_id: number
  name: string
  enabled: boolean
  form: GateForm
}

/** Topes del servidor. Se repiten aquí para poder avisar ANTES de enviar. */
export const GATE_LIMITS = {
  fields: 12,
  title: 120,
  description: 300,
  label: 80,
  help: 200,
  placeholder: 80,
  options: 12,
  optionText: 60,
  key: 40,
  name: 80,
}

export const workflowService = {
  recipes: async (boardId: number) => {
    const { data } = await api.get<WorkflowRecipe[]>('/workflows/recipes', {
      params: { board_id: boardId },
    })
    return data
  },
  list: async (boardId: number) => {
    const { data } = await api.get<WorkflowSummary[]>('/workflows', {
      params: { board_id: boardId },
    })
    return data
  },
  setRecipe: async (boardId: number, recipe: string, enabled: boolean, phaseId?: number) => {
    const { data } = await api.post<{ id?: number; enabled: boolean }>('/workflows/recipes', {
      board_id: boardId,
      recipe,
      enabled,
      ...(phaseId ? { phase_id: phaseId } : {}),
    })
    return data
  },
  runs: async (workflowId: number) => {
    const { data } = await api.get<WorkflowRun[]>(`/workflows/${workflowId}/runs`)
    return data
  },

  gates: async (boardId: number) => {
    const { data } = await api.get<BoardGate[]>('/workflows/gates', {
      params: { board_id: boardId },
    })
    return data
  },
  createGate: async (input: GateInput) => {
    const { data } = await api.post<{ id: number }>('/workflows/gates', input)
    return data
  },
  updateGate: async (id: number, input: GateInput) => {
    const { data } = await api.put<{ id: number }>(`/workflows/gates/${id}`, input)
    return data
  },
  deleteGate: async (id: number) => {
    await api.delete(`/workflows/gates/${id}`)
  },
}
