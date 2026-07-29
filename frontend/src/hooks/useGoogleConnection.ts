import { useCallback, useEffect, useState } from 'react'
import {
  googleCalendarService,
  type GoogleCalendarStatus,
} from '../services/google-calendar.service'

/** Resultado de volver del consentimiento de Google, ya traducido a mensaje. */
export type GoogleBanner = { type: 'success' | 'error'; text: string } | null

// Motivos que devuelve el callback del backend en ?reason=. Los que no estén
// aquí caen a un mensaje genérico: son fallos técnicos que el usuario no puede
// accionar y no ayuda mostrarle el código crudo.
const ERROR_MESSAGES: Record<string, string> = {
  access_denied: 'Cancelaste la conexión con Google.',
  expired: 'El enlace de conexión expiró. Vuelve a intentarlo.',
  disabled: 'La integración con Google no está disponible ahora mismo.',
  invalid_request: 'La respuesta de Google no llegó completa. Vuelve a intentarlo.',
}

/**
 * useGoogleConnection centraliza el estado del vínculo con Google y el retorno
 * del consentimiento. Vive aquí —y no dentro del panel de Perfil, que fue donde
 * nació— porque ahora hay dos pantallas que lo necesitan: Integraciones y
 * Sesiones, que sin cuenta conectada no puede convocar nada.
 *
 * `returnTo` es la ruta del SPA a la que debe volver el navegador tras aceptar
 * en Google.
 */
export function useGoogleConnection(returnTo: string) {
  const [status, setStatus] = useState<GoogleCalendarStatus | null>(null)
  const [loading, setLoading] = useState(true)
  const [busy, setBusy] = useState(false)
  const [banner, setBanner] = useState<GoogleBanner>(null)

  const reload = useCallback(async () => {
    try {
      const data = await googleCalendarService.getStatus()
      setStatus(data)
      return data
    } catch {
      setStatus({ enabled: false, connected: false })
      return null
    } finally {
      setLoading(false)
    }
  }, [])

  // Resultado de la vinculación: el callback del backend redirige aquí con
  // ?google=ok|error. Se limpia de la URL para que un refresco no repita el
  // aviso y para no dejar el parámetro en el historial.
  useEffect(() => {
    const params = new URLSearchParams(window.location.search)
    const result = params.get('google')
    if (!result) return

    if (result === 'ok') {
      setBanner({ type: 'success', text: 'Tu cuenta de Google quedó conectada.' })
    } else {
      const reason = params.get('reason') || ''
      setBanner({
        type: 'error',
        text: ERROR_MESSAGES[reason] || 'No se pudo conectar tu cuenta de Google. Vuelve a intentarlo.',
      })
    }

    params.delete('google')
    params.delete('reason')
    const query = params.toString()
    window.history.replaceState({}, '', window.location.pathname + (query ? `?${query}` : ''))
  }, [])

  useEffect(() => {
    reload()
  }, [reload])

  const connect = useCallback(async () => {
    setBusy(true)
    setBanner(null)
    try {
      window.location.href = await googleCalendarService.getAuthUrl(returnTo)
      // Sin setBusy(false) en el camino feliz: la página está navegando a Google.
    } catch {
      setBanner({ type: 'error', text: 'No se pudo iniciar la conexión con Google.' })
      setBusy(false)
    }
  }, [returnTo])

  const account = status?.account
  return {
    status,
    account,
    loading,
    busy,
    setBusy,
    banner,
    setBanner,
    reload,
    connect,
    /** La integración está configurada en el servidor. */
    enabled: !!status?.enabled,
    connected: !!status?.connected,
    /** Google revocó el acceso: hay que reconectar antes de poder convocar. */
    needsReauth: account?.status === 'needs_reauth',
  }
}
