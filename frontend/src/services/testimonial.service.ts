import api from './client'

/** Quién escribe el testimonio. Cada audiencia tiene su plantilla. */
export type TestimonialAudience = 'professional' | 'company'

/**
 * Ciclo de vida:
 * - pending: enviado, esperando respuesta
 * - submitted: respondido y firmado, esperando revisión
 * - approved: aprobado, se puede usar
 * - rejected: descartado
 */
export type TestimonialStatus =
  | 'pending'
  | 'submitted'
  | 'approved'
  | 'rejected'
  /** Devuelto a su autor con un motivo, esperando que lo corrija. */
  | 'changes_requested'

export interface TestimonialTemplate {
  audience: TestimonialAudience
  label: string
  headline: string
  intro: string
  prompts: string[]
  consent_text: string
}

export interface TestimonialAnswer {
  prompt: string
  answer: string
}

/** Contenido público de la página del testimonio. */
export interface TestimonialLanding {
  status: TestimonialStatus
  audience: TestimonialAudience
  headline: string
  recipient_name: string
  recipient_role?: string
  recipient_company?: string
  intro_message: string
  prompts: string[]
  consent_text: string
  consent_version: string
  /** Distingue "el enlace venció" de "ya lo respondiste". */
  expired: boolean
  /** Solo en una corrección: qué pidió el equipo que se arregle. */
  change_reason?: string
  /**
   * Solo en una corrección: lo que ya se había escrito, para repoblar el
   * formulario. La firma NO viene: al corregir hay que volver a firmar.
   */
  previous?: {
    rating: number
    quote: string
    answers: TestimonialAnswer[]
    allow_public_name: boolean
    allow_role: boolean
    allow_photo: boolean
    allow_logo: boolean
    signature_name: string
  }
}

/** Lo que envía la página pública al firmar. */
export interface TestimonialSubmission {
  rating: number
  quote: string
  answers: TestimonialAnswer[]
  allow_public_name: boolean
  allow_role: boolean
  allow_photo: boolean
  allow_logo: boolean
  consent_accepted: boolean
  signature_name: string
  /** La firma como data URL PNG, sea cual sea la modalidad. */
  signature_image: string
  /** Cómo se firmó: parte de la evidencia, no adorno. */
  signature_mode: SignatureMode
}

/** Cómo se produjo la firma. */
export type SignatureMode = 'drawn' | 'uploaded' | 'typed'

/** Testimonio completo, como lo ve el panel interno. */
export interface Testimonial {
  id: number
  created_at: string
  audience: TestimonialAudience
  status: TestimonialStatus
  user_id: number
  requested_by: number
  recipient_name: string
  recipient_email: string
  recipient_role: string
  recipient_company: string
  prompts: string
  intro_message: string
  consent_text: string
  consent_version: string
  expires_at: string
  reminded_at?: string | null
  rating: number
  quote: string
  answers: string
  submitted_at?: string | null
  allow_public_name: boolean
  allow_role: boolean
  allow_photo: boolean
  allow_logo: boolean
  signature_name: string
  signature_mode: SignatureMode | ''
  signed_at?: string | null
  signer_ip: string
  signer_user_agent: string
  reviewed_by?: number | null
  reviewed_at?: string | null
  review_note: string
  published_quote: string
  change_reason: string
  change_requested_at?: string | null
  revisions: number
  user?: { id: number; name: string; email: string; avatar?: string } | null
}

export interface TestimonialListResult {
  items: Testimonial[]
  counts: Record<string, number>
}

export interface TestimonialRequestInput {
  user_id: number
  audience: TestimonialAudience
  intro_message?: string
  prompts?: string[]
  ttl_days?: number
}

/** Qué pasó con UNA persona de un envío por lote. */
export interface TestimonialBulkOutcome {
  user_id: number
  name: string
  sent: boolean
  /** Por qué no se envió. Solo viene cuando `sent` es false. */
  reason?: string
}

/**
 * Resultado de un envío por lote. Es un éxito PARCIAL por naturaleza: casi
 * siempre habrá alguien con una solicitud viva o sin correo.
 */
export interface TestimonialBulkResult {
  sent: number
  skipped: number
  outcomes: TestimonialBulkOutcome[]
}

export interface TestimonialReviewInput {
  approve: boolean
  note?: string
  published_quote?: string
}

/** Lee el JSON de respuestas guardado en el testimonio. */
export function parseTestimonialAnswers(raw: string): TestimonialAnswer[] {
  if (!raw) return []
  try {
    const parsed: unknown = JSON.parse(raw)
    return Array.isArray(parsed) ? (parsed as TestimonialAnswer[]) : []
  } catch {
    return []
  }
}

export const testimonialService = {
  // --- PÚBLICO. Quien firma puede no tener sesión; su credencial es el token
  // del enlace que recibió por correo. Va bajo /testimonial (singular) para no
  // chocar con el grupo interno.

  getLanding: async (token: string) => {
    const { data } = await api.get<TestimonialLanding>(`/testimonial/${token}`)
    return data
  },

  submit: async (token: string, payload: TestimonialSubmission) => {
    const { data } = await api.post<{ message: string }>(`/testimonial/${token}/submit`, payload)
    return data
  },

  // --- Interno (superadmin y customer success).

  getTemplates: async () => {
    const { data } = await api.get<{ templates: TestimonialTemplate[] }>('/testimonials/templates')
    return data.templates
  },

  list: async (params: { status?: string; audience?: string; search?: string } = {}) => {
    const { data } = await api.get<TestimonialListResult>('/testimonials', { params })
    return data
  },

  get: async (id: number) => {
    const { data } = await api.get<Testimonial>(`/testimonials/${id}`)
    return data
  },

  request: async (payload: TestimonialRequestInput) => {
    const { data } = await api.post<Testimonial>('/testimonials', payload)
    return data
  },

  /** Pide el mismo testimonio a varias personas. Nunca falla en bloque. */
  requestMany: async (userIds: number[], payload: Omit<TestimonialRequestInput, 'user_id'>) => {
    const { data } = await api.post<TestimonialBulkResult>('/testimonials/bulk', {
      ...payload,
      user_ids: userIds,
    })
    return data
  },

  resend: async (id: number) => {
    const { data } = await api.post<{ message: string }>(`/testimonials/${id}/resend`, {})
    return data
  },

  /**
   * Aprueba o descarta. `warning` NO es un error: la decisión se aplicó, pero
   * algo secundario no salió — típicamente que no se pudo archivar en el
   * expediente porque la persona no tiene ningún empleo registrado.
   */
  review: async (id: number, payload: TestimonialReviewInput) => {
    const { data } = await api.post<{ message: string; warning?: string }>(
      `/testimonials/${id}/review`,
      payload
    )
    return data
  },

  /** Devuelve el testimonio a su autor con un motivo para que lo corrija. */
  requestChanges: async (id: number, reason: string) => {
    const { data } = await api.post<{ message: string }>(`/testimonials/${id}/request-changes`, {
      reason,
    })
    return data
  },

  remove: async (id: number) => {
    const { data } = await api.delete<{ message: string }>(`/testimonials/${id}`)
    return data
  },

  /**
   * Descarga la constancia firmada. Va por XHR autenticado y no por un <a href>
   * porque el endpoint exige sesión: un enlace directo devolvería un 401.
   */
  downloadConsent: async (id: number, filename: string) => {
    const { data } = await api.get<Blob>(`/testimonials/${id}/consent.pdf`, {
      responseType: 'blob',
    })
    const url = URL.createObjectURL(data)
    const a = document.createElement('a')
    a.href = url
    a.download = filename
    document.body.appendChild(a)
    a.click()
    a.remove()
    URL.revokeObjectURL(url)
  },

  /**
   * Trae el trazo de la firma como object URL para pintarlo en un <img>. Quien
   * lo llame debe revocar la URL al desmontar.
   */
  signatureURL: async (id: number) => {
    const { data } = await api.get<Blob>(`/testimonials/${id}/signature`, {
      responseType: 'blob',
    })
    return URL.createObjectURL(data)
  },
}
