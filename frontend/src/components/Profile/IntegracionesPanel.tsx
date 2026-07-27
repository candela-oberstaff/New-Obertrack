import { useState, useEffect, useCallback } from 'react'
import { CalendarDays, AlertTriangle, Check, Link2, Unlink } from 'lucide-react'
import {
  googleCalendarService,
  type GoogleCalendarStatus,
} from '../../services/google-calendar.service'
import { useConfirm } from '../ui/ConfirmProvider'
import styles from '../../pages/Profile.module.css'

/** Ruta a la que vuelve el navegador tras el consentimiento de Google. */
const RETURN_TO = '/profile'

// Motivos que devuelve el callback del backend en ?reason=. Los que no estén
// aquí caen a un mensaje genérico: son fallos técnicos que el usuario no puede
// accionar y no ayuda mostrarle el código crudo.
const ERROR_MESSAGES: Record<string, string> = {
  access_denied: 'Cancelaste la conexión con Google.',
  expired: 'El enlace de conexión expiró. Vuelve a intentarlo.',
  disabled: 'La integración con Google Calendar no está disponible ahora mismo.',
  invalid_request: 'La respuesta de Google no llegó completa. Vuelve a intentarlo.',
}

type Banner = { type: 'success' | 'error'; text: string } | null

export function IntegracionesPanel() {
  const [status, setStatus] = useState<GoogleCalendarStatus | null>(null)
  const [loading, setLoading] = useState(true)
  const [busy, setBusy] = useState(false)
  const [banner, setBanner] = useState<Banner>(null)
  const confirm = useConfirm()

  const account = status?.account
  const needsReauth = account?.status === 'needs_reauth'

  const loadStatus = useCallback(async () => {
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
    loadStatus()
  }, [loadStatus])

  const handleConnect = async () => {
    setBusy(true)
    setBanner(null)
    try {
      window.location.href = await googleCalendarService.getAuthUrl(RETURN_TO)
    } catch {
      setBanner({ type: 'error', text: 'No se pudo iniciar la conexión con Google.' })
      setBusy(false)
    }
    // Sin setBusy(false) en el camino feliz: la página está navegando a Google.
  }

  const handleDisconnect = async () => {
    const ok = await confirm({
      title: 'Desconectar Google Calendar',
      message: 'Obertrack dejará de tener acceso a tu calendario. Podrás volver a conectarlo cuando quieras.',
      confirmLabel: 'Desconectar',
      cancelLabel: 'Cancelar',
      variant: 'danger',
    })
    if (!ok) return

    setBusy(true)
    try {
      await googleCalendarService.disconnect()
      setBanner({ type: 'success', text: 'Cuenta de Google desconectada.' })
      await loadStatus()
    } catch {
      setBanner({ type: 'error', text: 'No se pudo desconectar la cuenta.' })
    } finally {
      setBusy(false)
    }
  }

  // Con la integración apagada en el servidor el panel no se muestra: no tiene
  // sentido ofrecer un botón que solo puede fallar.
  if (loading || !status?.enabled) return null

  return (
    <div className={styles['sidebar-card']} style={{ marginBottom: '16px' }}>
      <h3 style={{ marginBottom: '12px', display: 'flex', alignItems: 'center', gap: '8px' }}>
        <CalendarDays size={16} />
        Integraciones
      </h3>

      {banner && (
        <div
          style={{
            padding: '10px 12px',
            borderRadius: '10px',
            fontSize: '0.82rem',
            marginBottom: '12px',
            border: `1px solid ${banner.type === 'success' ? '#a7f3d0' : '#fecaca'}`,
            background: banner.type === 'success' ? '#ecfdf5' : '#fef2f2',
            color: banner.type === 'success' ? '#065f46' : '#991b1b',
          }}
        >
          {banner.text}
        </div>
      )}

      {!status.connected ? (
        <>
          <p style={{ fontSize: '0.84rem', color: 'var(--text-secondary, #6b7280)', marginBottom: '12px' }}>
            Conecta tu cuenta de Google para llevar tu agenda de Obertrack a tu calendario.
            Funciona con cualquier correo de Gmail o de dominio propio.
          </p>
          <button
            className={styles['btn-primary']}
            style={{ width: '100%', padding: '9px', display: 'flex', alignItems: 'center', justifyContent: 'center', gap: '8px' }}
            onClick={handleConnect}
            disabled={busy}
          >
            <Link2 size={15} />
            {busy ? 'Conectando…' : 'Conectar con Google Calendar'}
          </button>
        </>
      ) : (
        <>
          {needsReauth && (
            <div
              style={{
                display: 'flex',
                gap: '10px',
                padding: '10px 12px',
                borderRadius: '10px',
                border: '1px solid #fcd34d',
                background: '#fffbeb',
                marginBottom: '12px',
              }}
            >
              <AlertTriangle size={16} color="#b45309" style={{ flexShrink: 0, marginTop: '1px' }} />
              <div style={{ fontSize: '0.82rem', color: '#92400e' }}>
                Perdimos el acceso a tu calendario. Vuelve a conectar tu cuenta para reactivarlo.
              </div>
            </div>
          )}

          <div className={styles['stat-item']}>
            <span className={styles['stat-label']}>Cuenta</span>
            <span className={styles['stat-value']} style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
              {!needsReauth && <Check size={14} color="#10b981" />}
              {account?.google_email}
            </span>
          </div>

          {account?.connected_at && (
            <div className={styles['stat-item']}>
              <span className={styles['stat-label']}>Conectada</span>
              <span className={styles['stat-value']}>
                {new Date(account.connected_at).toLocaleDateString('es-ES', {
                  day: 'numeric',
                  month: 'long',
                  year: 'numeric',
                })}
              </span>
            </div>
          )}

          {!needsReauth && (
            <div className={styles['stat-item']}>
              <span className={styles['stat-label']}>Sincroniza con</span>
              <span className={styles['stat-value']}>Tu calendario principal</span>
            </div>
          )}

          <div style={{ display: 'flex', gap: '8px', marginTop: '14px' }}>
            {needsReauth && (
              <button
                className={styles['btn-primary']}
                style={{ flex: 1, padding: '8px' }}
                onClick={handleConnect}
                disabled={busy}
              >
                Reconectar
              </button>
            )}
            <button
              className={styles['btn-outline']}
              style={{ flex: 1, padding: '8px', display: 'flex', alignItems: 'center', justifyContent: 'center', gap: '6px' }}
              onClick={handleDisconnect}
              disabled={busy}
            >
              <Unlink size={14} />
              Desconectar
            </button>
          </div>
        </>
      )}
    </div>
  )
}
