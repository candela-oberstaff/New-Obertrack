import { useCallback, useEffect, useState } from 'react'
import { GraduationCap, RotateCcw, Send } from 'lucide-react'

import { Button } from '../ui'
import { useConfirm } from '../ui/ConfirmProvider'
import { useNotification } from '../../context/NotificationContext'
import { inductionService, type InductionUserStatus } from '../../services/induction.service'

interface Props {
  userId: number
  /** Solo Soporte/superadmin pueden reiniciar los intentos o enviar la inducción. */
  canReset?: boolean
  /**
   * Solo los profesionales pasan por inducción. Sin esto, el panel de "enviar"
   * aparecería en la ficha de empresas, soporte y superadmins.
   */
  isProfessional?: boolean
}

const STATUS_LABEL: Record<string, { text: string; bg: string; fg: string }> = {
  pending: { text: 'Pendiente', bg: '#fef3c7', fg: '#92400e' },
  passed: { text: 'Aprobada', bg: '#dcfce7', fg: '#15803d' },
  blocked: { text: 'Bloqueado', bg: '#fee2e2', fg: '#b91c1c' },
}

/**
 * Panel de inducción en el detalle del profesional. Le da a Soporte el contexto
 * que necesita antes de contactar a quien no aprobó (puntajes por intento) y la
 * acción para devolverle sus intentos.
 *
 * Si el profesional no tiene inducción registrada, ofrece enviársela: es la vía
 * para quienes no llegaron por el puente de Obersuite (alta manual, alta desde
 * la empresa o importación). Ese caso solo se pinta con la inducción encendida,
 * para no ensuciar la ficha de las cuentas anteriores a esta funcionalidad.
 */
export function InductionStatusPanel({ userId, canReset = false, isProfessional = false }: Props) {
  const { success, error: showError } = useNotification()
  const confirm = useConfirm()

  const [status, setStatus] = useState<InductionUserStatus | null>(null)
  const [loading, setLoading] = useState(true)
  const [resetting, setResetting] = useState(false)
  const [inviting, setInviting] = useState(false)
  // Si la inducción está apagada no hay nada que enviar: sin esto, el panel
  // ofrecería una acción que el backend rechazaría.
  const [enabled, setEnabled] = useState(false)

  const load = useCallback(async () => {
    try {
      setStatus(await inductionService.getUserStatus(userId))
    } catch {
      // 404 = este profesional no pasó por inducción. No es un error a mostrar.
      setStatus(null)
    } finally {
      setLoading(false)
    }
  }, [userId])

  useEffect(() => {
    inductionService
      .getConfig()
      .then(cfg => setEnabled(!!cfg?.is_active && !!cfg?.survey_id))
      .catch(() => setEnabled(false))
  }, [])

  useEffect(() => {
    void load()
  }, [load])

  const handleReset = async () => {
    const ok = await confirm({
      title: 'Reiniciar inducción',
      message:
        'Se le devolverán todos sus intentos y se le reenviará el enlace por correo. Su acceso seguirá bloqueado hasta que apruebe.',
      confirmLabel: 'Reiniciar',
    })
    if (!ok) return

    setResetting(true)
    try {
      await inductionService.resetUser(userId)
      success('Inducción reiniciada. Se reenvió el enlace al profesional.')
      await load()
    } catch (err: any) {
      showError(err?.response?.data?.error ?? 'No se pudo reiniciar la inducción.')
    } finally {
      setResetting(false)
    }
  }

  const handleInvite = async () => {
    const ok = await confirm({
      title: 'Enviar inducción',
      message:
        'Se le enviará el enlace por correo y su acceso quedará bloqueado hasta que apruebe. Úsalo con profesionales que no pasaron por la inducción al ser dados de alta.',
      confirmLabel: 'Enviar inducción',
    })
    if (!ok) return

    setInviting(true)
    try {
      await inductionService.inviteUser(userId)
      success('Inducción enviada. El profesional recibirá el enlace por correo.')
      await load()
    } catch (err: any) {
      showError(err?.response?.data?.error ?? 'No se pudo enviar la inducción.')
    } finally {
      setInviting(false)
    }
  }

  if (loading) return null

  // Sin inducción registrada: solo se ofrece enviarla, y solo a un profesional
  // cuando está encendida y quien mira puede ejecutarla.
  if (!status) {
    if (!isProfessional || !enabled || !canReset) return null
    return (
      <div style={{ background: '#fff', border: '1px solid #e2e8f0', borderRadius: 16, padding: 24, marginTop: '1rem' }}>
        <h3 style={{ margin: '0 0 8px', fontSize: 16, fontWeight: 800, color: '#0f172a', display: 'flex', alignItems: 'center', gap: 8 }}>
          <GraduationCap size={18} /> Inducción
          <span style={{ background: '#f1f5f9', color: '#64748b', fontSize: 12, fontWeight: 700, padding: '4px 10px', borderRadius: 999 }}>
            Sin registrar
          </span>
        </h3>
        <p style={{ margin: '0 0 16px', fontSize: 13.5, color: '#64748b' }}>
          Este profesional no pasó por la inducción: la hacen automáticamente los contratados desde Obersuite,
          no las altas manuales ni las importaciones.
        </p>
        <Button variant="secondary" leftIcon={<Send size={16} />} loading={inviting} onClick={handleInvite}>
          Enviar inducción
        </Button>
      </div>
    )
  }

  const badge = STATUS_LABEL[status.status] ?? STATUS_LABEL.pending

  return (
    <div
      style={{
        background: '#fff',
        border: '1px solid #e2e8f0',
        borderRadius: 16,
        padding: 24,
        marginTop: '1rem',
      }}
    >
      <h3
        style={{
          margin: '0 0 16px',
          fontSize: 16,
          fontWeight: 800,
          color: '#0f172a',
          display: 'flex',
          alignItems: 'center',
          gap: 8,
        }}
      >
        <GraduationCap size={18} /> Inducción
        <span
          style={{
            background: badge.bg,
            color: badge.fg,
            fontSize: 12,
            fontWeight: 700,
            padding: '4px 10px',
            borderRadius: 999,
          }}
        >
          {badge.text}
        </span>
      </h3>

      <div style={{ display: 'flex', flexWrap: 'wrap', gap: 24, fontSize: 14, color: '#475569' }}>
        <span>
          Intentos: <strong>{status.attempts}</strong> de {status.max_attempts}
        </span>
        <span>
          Mejor puntaje: <strong>{Math.round(status.best_score)}%</strong>
        </span>
        <span>
          Mínimo: <strong>{status.passing_score}%</strong>
        </span>
      </div>

      {status.attempt_log.length > 0 && (
        <div style={{ marginTop: 16 }}>
          <div style={{ fontSize: 13, fontWeight: 700, color: '#334155', marginBottom: 8 }}>
            Historial de intentos
          </div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
            {status.attempt_log.map((a, i) => (
              <div
                key={a.id}
                style={{
                  display: 'flex',
                  justifyContent: 'space-between',
                  gap: 12,
                  padding: '8px 12px',
                  background: '#f8fafc',
                  border: '1px solid #e2e8f0',
                  borderRadius: 8,
                  fontSize: 13,
                }}
              >
                <span style={{ color: '#64748b' }}>Intento {i + 1}</span>
                <span style={{ fontWeight: 700, color: a.passed ? '#15803d' : '#b91c1c' }}>
                  {Math.round(a.score)}% {a.passed ? '· aprobado' : '· no aprobado'}
                </span>
                <span style={{ color: '#94a3b8' }}>
                  {new Date(a.created_at).toLocaleString('es-ES', {
                    day: '2-digit',
                    month: 'short',
                    hour: '2-digit',
                    minute: '2-digit',
                  })}
                </span>
              </div>
            ))}
          </div>
        </div>
      )}

      {canReset && status.status !== 'passed' && (
        <div style={{ marginTop: 20 }}>
          <Button
            variant="secondary"
            leftIcon={<RotateCcw size={16} />}
            loading={resetting}
            onClick={handleReset}
          >
            Reiniciar intentos y reenviar enlace
          </Button>
        </div>
      )}
    </div>
  )
}
