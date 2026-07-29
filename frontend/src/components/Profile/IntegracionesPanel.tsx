import { Plug, AlertTriangle, Check, Link2, Unlink } from 'lucide-react'
import { googleCalendarService } from '../../services/google-calendar.service'
import { useGoogleConnection } from '../../hooks/useGoogleConnection'
import { useConfirm } from '../ui/ConfirmProvider'
import styles from '../../pages/Profile.module.css'

/** Ruta a la que vuelve el navegador tras el consentimiento de Google. */
const RETURN_TO = '/profile'

export function IntegracionesPanel() {
  // El estado del vínculo y el retorno del consentimiento viven en el hook: la
  // página de Sesiones necesita exactamente lo mismo, y tener dos copias de
  // esta lógica garantizaba que una de las dos se quedara atrás.
  const {
    status, account, loading, busy, setBusy, banner, setBanner, reload, connect, needsReauth,
  } = useGoogleConnection(RETURN_TO)
  const confirm = useConfirm()

  const handleConnect = connect

  const handleDisconnect = async () => {
    const ok = await confirm({
      title: 'Desconectar cuenta de Google',
      // Se enumera lo que deja de funcionar —no solo "el calendario"— porque la
      // cuenta alimenta dos cosas y perder las sesiones sorprendería a quien
      // creyera estar desconectando solo la sincronización de tareas.
      message:
        'Tus tareas dejarán de sincronizarse con el calendario y no podrás convocar sesiones de Meet. ' +
        'Las reuniones ya creadas seguirán existiendo en Google. Podrás volver a conectarla cuando quieras.',
      confirmLabel: 'Desconectar',
      cancelLabel: 'Cancelar',
      variant: 'danger',
    })
    if (!ok) return

    setBusy(true)
    try {
      await googleCalendarService.disconnect()
      setBanner({ type: 'success', text: 'Cuenta de Google desconectada.' })
      await reload()
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
        {/* Icono neutro y no un calendario: el vínculo dejó de ser solo Calendar. */}
        <Plug size={16} />
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
            Asocia tu cuenta de Google para llevar tus tareas al calendario y convocar
            sesiones con Google Meet. Funciona con cualquier correo de Gmail o de
            dominio propio.
          </p>
          <button
            className={styles['btn-primary']}
            style={{ width: '100%', padding: '9px', display: 'flex', alignItems: 'center', justifyContent: 'center', gap: '8px' }}
            onClick={handleConnect}
            disabled={busy}
          >
            <Link2 size={15} />
            {busy ? 'Conectando…' : 'Conectar cuenta de Google'}
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
                Perdimos el acceso a tu cuenta de Google. Vuelve a conectarla para
                reactivar la sincronización y las sesiones.
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

          {/* Qué habilita la cuenta, en vez del antiguo "Sincroniza con: tu
              calendario principal": desde el módulo de Sesiones el vínculo hace
              dos cosas, y quien mira este panel debería saber cuáles. */}
          {!needsReauth && (
            <div className={styles['stat-item']}>
              <span className={styles['stat-label']}>Activa</span>
              <span className={styles['stat-value']} style={{ textAlign: 'right' }}>
                Tareas en tu calendario
                <br />
                Sesiones con Meet
              </span>
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
