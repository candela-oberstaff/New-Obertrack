import api from './client'

/** Estado del vínculo. 'needs_reauth' = Google revocó el acceso y hay que reconectar. */
export type GoogleAccountStatus = 'active' | 'needs_reauth'

export interface GoogleCalendarAccount {
  id: number
  user_id: number
  google_email: string
  scopes: string
  calendar_id: string
  calendar_name: string
  status: GoogleAccountStatus
  last_error?: string
  connected_at: string
}

export interface GoogleCalendarStatus {
  /** false cuando la integración no está configurada en el servidor. */
  enabled: boolean
  connected: boolean
  account?: GoogleCalendarAccount
}

export const googleCalendarService = {
  getStatus: async () => {
    const { data } = await api.get<GoogleCalendarStatus>('/me/integrations/google/status')
    return data
  },

  /**
   * Devuelve la URL de consentimiento de Google. El backend la manda en JSON en
   * vez de responder con un 302 porque esta petición sale por XHR: un redirect
   * aquí lo seguiría axios, no el navegador. Navegar es responsabilidad del
   * llamador (window.location.href = authUrl).
   */
  getAuthUrl: async (returnTo?: string) => {
    const { data } = await api.post<{ auth_url: string }>('/me/integrations/google/connect', {
      return_to: returnTo,
    })
    return data.auth_url
  },

  disconnect: async () => {
    await api.delete('/me/integrations/google')
  },
}
