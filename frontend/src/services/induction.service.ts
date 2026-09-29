import api from './client'
import type { UserBadge } from './badge.service'
import type { CertificateSummary } from './certificate.service'

/**
 * Estado de la inducción de un profesional recién contratado. Los mismos
 * valores describen cada bloque de su invitación.
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

/** Un bloque en la barra de progreso de la landing. */
export interface InductionLandingBlock {
  block_id: number
  order_index: number
  name: string
  status: InductionStatus
  attempts: number
  attempts_left: number
  best_score: number
  passing_score: number
  has_video: boolean
}

/** El bloque que toca responder, completo. Nunca incluye las respuestas correctas. */
export interface InductionCurrentBlock {
  block_id: number
  order_index: number
  name: string
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

/** Contenido público de la landing: progreso por bloque y bloque actual. */
export interface InductionLanding {
  professional_name: string
  status: InductionStatus
  program_name: string
  /** true = aprobar habilita el acceso (ingreso); false = solo cierra una capacitación. */
  gates_access: boolean
  max_attempts: number
  total_blocks: number
  completed_blocks: number
  blocks: InductionLandingBlock[]
  /** Ausente cuando la inducción ya se resolvió (aprobada o bloqueada). */
  current?: InductionCurrentBlock
  /** Insignias que ya ganó, para el panel lateral. */
  badges: UserBadge[]
  /** Certificado de esta capacitación, si ya se emitió. */
  certificate?: CertificateSummary
}

export interface InductionAnswer {
  question_id: number
  value: string
}

/** Veredicto de un intento sobre un bloque. El puntaje se calcula siempre en el servidor. */
export interface InductionResult {
  block_id: number
  /** Posición 1-based del bloque dentro del programa. */
  block_index: number
  block_name: string
  score: number
  passing_score: number
  passed: boolean
  /** Estado del bloque tras el intento. */
  block_status: InductionStatus
  /** Estado de la inducción completa tras el intento. */
  status: InductionStatus
  /** true cuando con este intento se aprobó el programa entero. */
  completed: boolean
  attempts_left: number
  next_block_id?: number
  next_block_name?: string
  message: string
  /** Insignias ganadas con este envío, para celebrarlas. */
  badges_earned: UserBadge[]
  /** Se emite al completar el programa, si tiene plantilla. */
  certificate?: CertificateSummary
}

/** Interruptor global de la inducción (fila única). */
export interface InductionConfig {
  id: number
  invite_ttl_days: number
  is_active: boolean
  updated_at?: string
}

/**
 * Bloque de la biblioteca: un video de Novedades (opcional) más su propio
 * cuestionario calificado. Reutilizable en cualquier programa.
 */
export interface InductionBlock {
  id: number
  name: string
  description: string
  tutorial_id?: number | null
  survey_id: number
  /** Mínimo propio; null = usa el del programa. */
  passing_score?: number | null
  /** Insignia que se gana al aprobar el bloque. Título vacío = nombre del bloque. */
  badge_title: string
  badge_icon: string
  badge_color: string
  created_by: number
  created_at: string
  updated_at: string
  // Solo lectura, para el panel.
  tutorial_title?: string
  survey_title?: string
  question_count: number
  program_names: string[]
  /** Posición dentro de un programa (solo cuando viene cargado como parte de uno). */
  order_index: number
}

export interface InductionBlockInput {
  name: string
  description: string
  tutorial_id?: number | null
  survey_id: number
  passing_score?: number | null
  badge_title?: string
  badge_icon?: string
  badge_color?: string
}

/** Programa: secuencia ordenada de bloques asignable a empresas. */
export interface InductionProgram {
  id: number
  name: string
  description: string
  default_passing_score: number
  /** Intentos permitidos POR BLOQUE. */
  max_attempts: number
  is_default: boolean
  is_active: boolean
  /** Insignia que se gana al completar el programa. Título vacío = nombre del programa. */
  badge_title: string
  badge_icon: string
  badge_color: string
  /** Plantilla del certificado; ausente = el programa no certifica. */
  certificate_template_id?: number | null
  created_by: number
  created_at: string
  updated_at: string
  blocks: InductionBlock[]
  company_ids: number[]
  block_count: number
  company_count: number
}

export interface InductionProgramInput {
  name: string
  description: string
  default_passing_score: number
  max_attempts: number
  is_default: boolean
  is_active: boolean
  badge_title?: string
  badge_icon?: string
  badge_color?: string
  /** 0 = sin certificado. */
  certificate_template_id?: number
}

export interface InductionAttemptLog {
  id: number
  block_id: number
  block_name?: string
  score: number
  passed: boolean
  created_at: string
}

/** Progreso de un bloque de la invitación, para Soporte. */
export interface InductionBlockStatus {
  block_id: number
  order_index: number
  name: string
  status: InductionStatus
  attempts: number
  best_score: number
  passing_score: number
  completed_at?: string
}

/** Detalle de la inducción de un profesional, para Soporte. */
export interface InductionUserStatus {
  user_id: number
  name: string
  email: string
  status: InductionStatus
  program_name: string
  /** true = esta invitación bloquea el acceso hasta aprobar. */
  gates_access: boolean
  /** Total de intentos sumando todos los bloques. */
  attempts: number
  /** Tope de intentos por bloque. */
  max_attempts: number
  total_blocks: number
  completed_blocks: number
  expires_at: string
  blocks: InductionBlockStatus[]
  attempt_log: InductionAttemptLog[]
  /** Invitación que describe esta vista (la actual). */
  invite_id: number
  /** Todas las capacitaciones de la persona, la actual incluida. */
  history: InductionHistoryItem[]
}

/** Resumen de una capacitación pasada o en curso. */
export interface InductionHistoryItem {
  id: number
  program_name: string
  status: InductionStatus
  gates_access: boolean
  total_blocks: number
  completed_blocks: number
  created_at: string
  completed_at?: string
  current: boolean
  /** Certificado emitido para esta capacitación, si lo hay. */
  certificate?: CertificateSummary
}

/** Capacitación pendiente del usuario de la sesión, con su enlace. */
export interface MyInduction {
  pending: boolean
  program_name: string
  token: string
  status: InductionStatus
  total_blocks: number
  completed_blocks: number
  expires_at: string
  gates_access: boolean
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

  submit: async (token: string, blockId: number, answers: InductionAnswer[]) => {
    const { data } = await api.post<InductionResult>(`/induction/${token}/submit`, {
      block_id: blockId,
      answers,
    })
    return data
  },

  // --- Internos. Van bajo /inductions (plural) para no chocar con el comodín
  // de la landing pública. Armar bloques y programas: superadmin. Consultar,
  // invitar y reiniciar: Soporte.

  getConfig: async () => {
    const { data } = await api.get<InductionConfig>('/inductions/config')
    return data
  },

  saveConfig: async (cfg: Partial<InductionConfig>) => {
    const { data } = await api.put<InductionConfig>('/inductions/config', cfg)
    return data
  },

  // Biblioteca de bloques
  listBlocks: async () => {
    const { data } = await api.get<{ data: InductionBlock[] }>('/inductions/blocks')
    return data.data
  },

  createBlock: async (input: InductionBlockInput) => {
    const { data } = await api.post<InductionBlock>('/inductions/blocks', input)
    return data
  },

  updateBlock: async (id: number, input: InductionBlockInput) => {
    const { data } = await api.put<InductionBlock>(`/inductions/blocks/${id}`, input)
    return data
  },

  deleteBlock: async (id: number) => {
    const { data } = await api.delete<{ message: string }>(`/inductions/blocks/${id}`)
    return data
  },

  // Programas
  listPrograms: async () => {
    const { data } = await api.get<{ data: InductionProgram[] }>('/inductions/programs')
    return data.data
  },

  getProgram: async (id: number) => {
    const { data } = await api.get<InductionProgram>(`/inductions/programs/${id}`)
    return data
  },

  createProgram: async (input: InductionProgramInput) => {
    const { data } = await api.post<InductionProgram>('/inductions/programs', input)
    return data
  },

  updateProgram: async (id: number, input: InductionProgramInput) => {
    const { data } = await api.put<InductionProgram>(`/inductions/programs/${id}`, input)
    return data
  },

  deleteProgram: async (id: number) => {
    const { data } = await api.delete<{ message: string }>(`/inductions/programs/${id}`)
    return data
  },

  /** Reemplaza la lista ordenada de bloques del programa. */
  setProgramBlocks: async (id: number, blockIds: number[]) => {
    const { data } = await api.put<InductionProgram>(`/inductions/programs/${id}/blocks`, {
      block_ids: blockIds,
    })
    return data
  },

  /** Reemplaza las empresas asignadas al programa. */
  setProgramCompanies: async (id: number, companyIds: number[]) => {
    const { data } = await api.put<InductionProgram>(`/inductions/programs/${id}/companies`, {
      company_ids: companyIds,
    })
    return data
  },

  // Soporte
  getUserStatus: async (userId: number) => {
    const { data } = await api.get<InductionUserStatus>(`/inductions/users/${userId}`)
    return data
  },

  resetUser: async (userId: number) => {
    const { data } = await api.post<{ message: string }>(`/inductions/users/${userId}/reset`, {})
    return data
  },

  /** Reinicia una capacitación concreta del historial. */
  resetInvite: async (inviteId: number) => {
    const { data } = await api.post<{ message: string }>(`/inductions/invites/${inviteId}/reset`, {})
    return data
  },

  /**
   * Envía la inducción a un profesional que ya existe. Es la vía para los que
   * no llegaron por el puente de Obersuite (alta manual, alta desde la empresa
   * o importación), que de otro modo nunca pasarían por ella. Sin programId se
   * resuelve por la empresa del profesional o el programa por defecto.
   */
  inviteUser: async (userId: number, programId?: number, gatesAccess = false) => {
    const { data } = await api.post<{ message: string }>(`/inductions/users/${userId}/invite`, {
      program_id: programId ?? 0,
      gates_access: gatesAccess,
    })
    return data
  },

  /** Mi capacitación pendiente (si hay), para ofrecerla desde dentro de la app. */
  myInduction: async () => {
    const { data } = await api.get<MyInduction>('/me/induction')
    return data
  },
}
