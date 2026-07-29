import api from './client'

/**
 * Estado de la inducción de un profesional recién contratado.
 * - pending: aún puede intentar
 * - passed: aprobó, ya tiene acceso
 * - blocked: agotó sus intentos; Soporte debe contactarlo
 */
export type InductionStatus = 'pending' | 'passed' | 'blocked'

export interface InductionQuestion {
  id: number
  text: string
  /** text | rating | choice */
  type: string
  options: string[]
  is_required: boolean
}

/** Contenido público de la landing. Nunca incluye las respuestas correctas. */
export interface InductionLanding {
  professional_name: string
  status: InductionStatus
  video_title?: string
  video_url?: string
  video_duration_min?: number
  survey_title: string
  description?: string
  questions: InductionQuestion[]
  passing_score: number
  attempts_left: number
  max_attempts: number
  best_score: number
}

export interface InductionAnswer {
  question_id: number
  value: string
}

/** Veredicto de un intento. El puntaje se calcula siempre en el servidor. */
export interface InductionResult {
  score: number
  passing_score: number
  passed: boolean
  status: InductionStatus
  attempts_left: number
  message: string
}

/** Configuración global de la inducción (fila única). */
export interface InductionConfig {
  id: number
  /** Video de Novedades/Tutoriales. Opcional: puede ser solo cuestionario. */
  tutorial_id?: number | null
  /** Cuestionario de Encuestas. Obligatorio para poder activarla. */
  survey_id?: number | null
  passing_score: number
  max_attempts: number
  invite_ttl_days: number
  is_active: boolean
  updated_at?: string
}

export interface InductionAttemptLog {
  id: number
  score: number
  passed: boolean
  created_at: string
}

/** Detalle de la inducción de un profesional, para Soporte. */
export interface InductionUserStatus {
  user_id: number
  name: string
  email: string
  status: InductionStatus
  attempts: number
  max_attempts: number
  best_score: number
  passing_score: number
  attempt_log: InductionAttemptLog[]
}

/**
 * Endpoints PÚBLICOS: el profesional todavía no tiene cuenta activa, así que su
 * única credencial es el token del enlace que recibió por correo.
 */
export const inductionService = {
  getLanding: async (token: string) => {
    const { data } = await api.get<InductionLanding>(`/induction/${token}`)
    return data
  },

  submit: async (token: string, answers: InductionAnswer[]) => {
    const { data } = await api.post<InductionResult>(`/induction/${token}/submit`, { answers })
    return data
  },

  // --- Internos. Van bajo /inductions (plural) para no chocar con el comodín
  // de la landing pública. Config: superadmin. Consulta/reset: Soporte.

  getConfig: async () => {
    const { data } = await api.get<InductionConfig>('/inductions/config')
    return data
  },

  saveConfig: async (cfg: Partial<InductionConfig>) => {
    const { data } = await api.put<InductionConfig>('/inductions/config', cfg)
    return data
  },

  getUserStatus: async (userId: number) => {
    const { data } = await api.get<InductionUserStatus>(`/inductions/users/${userId}`)
    return data
  },

  resetUser: async (userId: number) => {
    const { data } = await api.post<{ message: string }>(`/inductions/users/${userId}/reset`, {})
    return data
  },

  /**
   * Envía la inducción a un profesional que ya existe. Es la vía para los que
   * no llegaron por el puente de Obersuite (alta manual, alta desde la empresa
   * o importación), que de otro modo nunca pasarían por ella.
   */
  inviteUser: async (userId: number) => {
    const { data } = await api.post<{ message: string }>(`/inductions/users/${userId}/invite`, {})
    return data
  },
}
