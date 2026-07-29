import api from './client'

export type MeetingStatus = 'scheduled' | 'cancelled'

export interface MeetingAttendee {
  id: number
  session_id: number
  /** Ausente en invitados externos (los que no tienen cuenta en Obertrack). */
  user_id?: number
  email: string
  name?: string
}

export interface Meeting {
  id: number
  tenant_id: number
  title: string
  description: string
  /** RFC3339 en UTC; la UI la muestra en la zona del navegador. */
  start_at: string
  end_at: string
  /** Zona IANA con la que se convocó ("America/Bogota"). */
  time_zone: string
  organizer_id: number
  organizer?: { id: number; name: string; email: string; avatar?: string }
  meet_url: string
  html_link?: string
  status: MeetingStatus
  task_id?: number
  recurrence_rule?: string
  /** Fin de la última ocurrencia. Ausente = la serie no termina. */
  series_ends_at?: string
  /**
   * Próxima ocurrencia, calculada por el backend desde la RRULE. Para una serie,
   * `start_at` es solo la PRIMERA reunión, así que es esto —y no `start_at`— lo
   * que hay que mostrar. Ausente = la serie ya se agotó.
   */
  next_start_at?: string
  next_end_at?: string
  attendees?: MeetingAttendee[]
  created_at: string
}

export interface MeetingPayload {
  title: string
  description?: string
  start_at: string
  end_at: string
  time_zone: string
  attendee_user_ids?: number[]
  attendee_emails?: string[]
  task_id?: number
  recurrence_rule?: string
}

/**
 * Errores del módulo que el usuario puede resolver por su cuenta. El backend
 * los manda como 409 con una bandera, para que la UI ofrezca "Conectar Google"
 * en vez de un error genérico que no dice qué hacer.
 */
export interface MeetingActionError {
  message: string
  googleNotConnected: boolean
  needsReauth: boolean
  /** El token es anterior al scope meetings.space.readonly: hay que reconectar. */
  meetScopeMissing: boolean
}

export function parseMeetingError(err: unknown): MeetingActionError {
  const data = (err as { response?: { data?: Record<string, unknown> } })?.response?.data
  return {
    message: (data?.error as string) || 'No se pudo completar la operación.',
    googleNotConnected: data?.google_not_connected === true,
    needsReauth: data?.needs_reauth === true,
    meetScopeMissing: data?.meet_scope_missing === true,
  }
}

/** Quién está conectado AHORA a la sala. */
export interface MeetPresence {
  /** Hay una conferencia en curso. Si es false, `active` siempre es 0. */
  live: boolean
  active: number
  /** Nombres, cuando Google los reporta (los anónimos no siempre traen). */
  names?: string[]
}

export const meetingService = {
  list: async (opts?: { past?: boolean; taskId?: number }) => {
    const { data } = await api.get<{ meetings: Meeting[] }>('/meetings', {
      params: { past: opts?.past ? 'true' : undefined, task_id: opts?.taskId },
    })
    return data.meetings ?? []
  },

  upcoming: async (limit = 3) => {
    const { data } = await api.get<{ meetings: Meeting[] }>('/meetings/upcoming', {
      params: { limit },
    })
    return data.meetings ?? []
  },

  get: async (id: number) => {
    const { data } = await api.get<{ meeting: Meeting }>(`/meetings/${id}`)
    return data.meeting
  },

  create: async (payload: MeetingPayload) => {
    const { data } = await api.post<{ meeting: Meeting }>('/meetings', payload)
    return data.meeting
  },

  update: async (id: number, payload: MeetingPayload) => {
    const { data } = await api.put<{ meeting: Meeting }>(`/meetings/${id}`, payload)
    return data.meeting
  },

  cancel: async (id: number) => {
    await api.delete(`/meetings/${id}`)
  },

  presence: async (id: number) => {
    const { data } = await api.get<MeetPresence>(`/meetings/${id}/presence`)
    return data
  },
}
