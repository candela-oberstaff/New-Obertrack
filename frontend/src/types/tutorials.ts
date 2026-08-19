import type { User } from './tasks'

/** Audiencia del tutorial: 'all' (todos), 'empleador' (empresas) o 'profesional'. */
export type TutorialAudience = 'all' | 'empleador' | 'profesional'

/**
 * Qué lleva dentro la novedad: un video (Drive/YouTube), una imagen subida o
 * un texto con formato. Decide qué campo de contenido manda y cómo se pinta.
 */
export type TutorialContentType = 'video' | 'imagen' | 'texto'

/**
 * Publico objetivo: acota a QUIEN va dirigida la novedad por encima del tipo
 * de cuenta. Los criterios se combinan con Y. Todos vacios = toda la audiencia.
 */
export interface TutorialTarget {
  company_ids: number[]
  countries: string[]
  group_ids: number[]
  managers_only: boolean
}

export const EMPTY_TARGET: TutorialTarget = {
  company_ids: [],
  countries: [],
  group_ids: [],
  managers_only: false,
}

export function isEmptyTarget(target?: TutorialTarget | null): boolean {
  if (!target) return true
  return !target.company_ids?.length
    && !target.countries?.length
    && !target.group_ids?.length
    && !target.managers_only
}

export interface TutorialAudienceOption {
  id: number
  name: string
  count: number
}

/** Lo elegible al acotar el publico, calculado de los datos vivos. */
export interface TutorialAudienceOptions {
  companies: TutorialAudienceOption[]
  countries: string[]
  groups: TutorialAudienceOption[]
}

export interface TutorialAudiencePreview {
  reach: number
  by_audience: TutorialAudienceStat[]
}

export interface Tutorial {
  id: number
  title: string
  description: string
  content_type: TutorialContentType
  /** Video de Drive o YouTube. Solo se usa cuando content_type es 'video'. */
  google_drive_url: string
  /** Imagen subida a /uploads. Solo cuando content_type es 'imagen'. */
  image_url: string
  /** HTML con formato. Solo cuando content_type es 'texto'. */
  body: string
  icon_name: string
  category: string
  audience: TutorialAudience
  /** Publico objetivo, por encima del tipo de cuenta. */
  target: TutorialTarget
  duration_min: number
  order_index: number
  is_active: boolean
  /** Botón de acción opcional: texto y destino. */
  cta_label: string
  cta_url: string
  /** Programación: publicarse y retirarse solas. */
  publish_at?: string | null
  expires_at?: string | null
  /** Último recordatorio a los pendientes. */
  reminded_at?: string | null
  /** Exige confirmar la lectura en vez de bastar con cerrar. */
  require_ack: boolean
  /** Momento en que se anunció al equipo. Ausente = nunca se anunció. */
  announced_at?: string | null
  /** Días que el aviso emergente insiste con la novedad. 0 = sin emergente. */
  announce_days: number
  created_by: number
  creator?: User
  created_at: string
  updated_at: string
}

export interface CreateTutorialInput {
  title: string
  description: string
  content_type: TutorialContentType
  google_drive_url: string
  image_url: string
  body: string
  icon_name: string
  category: string
  audience: TutorialAudience
  target: TutorialTarget
  duration_min: number
  order_index: number
  announce_days: number
  cta_label: string
  cta_url: string
  publish_at?: string | null
  expires_at?: string | null
  require_ack: boolean
  is_active: boolean
}

export type UpdateTutorialInput = Partial<CreateTutorialInput>

/** De dónde salió una vista: el aviso a pantalla completa o la sección. */
export type TutorialViewSource = 'anuncio' | 'seccion'

export interface TutorialAudienceStat {
  user_type: string
  reach: number
  views: number
}

export interface TutorialViewer {
  user_id: number
  name: string
  email: string
  user_type: string
  source: TutorialViewSource
  viewed_at: string
  acknowledged: boolean
  clicked: boolean
}

/** Desempeño de una novedad: a cuántos llegó, cuántos la vieron y por dónde. */
export interface TutorialMetrics {
  tutorial_id: number
  audience: TutorialAudience
  /** Cuentas activas a las que iba dirigida: el denominador. */
  reach: number
  views: number
  pending: number
  /** Porcentaje visto, con un decimal. */
  view_rate: number
  from_announcement: number
  from_section: number
  /** Personas que pulsaron el botón de acción, y su % sobre quienes la vieron. */
  clicks: number
  click_rate: number
  /** Confirmaciones de lectura, cuando la novedad las exige. */
  acknowledged: number
  require_ack: boolean
  reminded_at?: string | null
  by_audience: TutorialAudienceStat[]
  recent_viewers: TutorialViewer[]
  announced_at?: string | null
  /** El aviso emergente sigue activo. */
  announce_open: boolean
}
