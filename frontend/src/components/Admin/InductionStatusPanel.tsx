import { useCallback, useEffect, useState } from 'react'
import { GraduationCap, RotateCcw, Send, CheckCircle2, Circle, XCircle, History } from 'lucide-react'

import { Button, Select } from '../ui'
import { useConfirm } from '../ui/ConfirmProvider'
import { useNotification } from '../../context/NotificationContext'
import {
  inductionService,
  type InductionProgram,
  type InductionUserStatus,
} from '../../services/induction.service'

interface Props {
  userId: number
  /** Solo Soporte/superadmin pueden reiniciar los intentos o enviar la inducción. */
  canReset?: boolean
  /**
   * Solo los profesionales pasan por inducción. Sin esto, el panel de "enviar"
   * aparecería en la ficha de empresas, soporte y superadmins.
   */
  isProfessional?: boolean
  /**
   * Estado de acceso del usuario. Si nunca ha tenido acceso (pendiente o
   * bloqueado) la invitación bloquea sí o sí; si ya trabaja, Soporte elige.
   */
  onboardingStatus?: string
}

const STATUS_LABEL: Record<string, { text: string; bg: string; fg: string }> = {
  pending: { text: 'Pendiente', bg: '#fef3c7', fg: '#92400e' },
  passed: { text: 'Aprobada', bg: '#dcfce7', fg: '#15803d' },
  blocked: { text: 'Bloqueado', bg: '#fee2e2', fg: '#b91c1c' },
}

const card: React.CSSProperties = {
  background: '#fff',
  border: '1px solid #e2e8f0',
  borderRadius: 16,
  padding: 24,
  marginTop: '1rem',
}

const titleStyle: React.CSSProperties = {
  margin: '0 0 8px',
  fontSize: 16,
  fontWeight: 800,
  color: '#0f172a',
  display: 'flex',
  alignItems: 'center',
  gap: 8,
  flexWrap: 'wrap',
}

const subTitle: React.CSSProperties = { fontSize: 13, fontWeight: 700, color: '#334155', marginBottom: 8 }

const rowStyle: React.CSSProperties = {
  display: 'flex',
  alignItems: 'center',
  gap: 12,
  padding: '8px 12px',
  background: '#f8fafc',
  border: '1px solid #e2e8f0',
  borderRadius: 8,
  fontSize: 13,
  flexWrap: 'wrap',
}

function badgeStyle(bg: string, fg: string): React.CSSProperties {
  return { background: bg, color: fg, fontSize: 12, fontWeight: 700, padding: '4px 10px', borderRadius: 999 }
}

function shortDate(iso: string) {
  return new Date(iso).toLocaleDateString('es-ES', { day: '2-digit', month: 'short', year: 'numeric' })
}

/**
 * Panel de inducción en el detalle del profesional. Le da a Soporte el contexto
 * que necesita antes de contactar a quien no aprobó (progreso por bloque y
 * puntajes por intento), la acción para devolverle sus intentos y el envío de
 * nuevas capacitaciones. Una persona acumula capacitaciones con el tiempo:
 * se muestra la actual en detalle y las demás como historial.
 */
export function InductionStatusPanel({ userId, canReset = false, isProfessional = false, onboardingStatus }: Props) {
  const { success, error: showError } = useNotification()
  const confirm = useConfirm()

  const [status, setStatus] = useState<InductionUserStatus | null>(null)
  const [loading, setLoading] = useState(true)
  const [resetting, setResetting] = useState(false)
  const [inviting, setInviting] = useState(false)
  // Si la inducción está apagada (o sin programa usable) no hay nada que
  // enviar: sin esto, el panel ofrecería una acción que el backend rechazaría.
  const [enabled, setEnabled] = useState(false)
  const [programs, setPrograms] = useState<InductionProgram[]>([])
  // 0 = automático: el de la empresa del profesional o el por defecto.
  const [programId, setProgramId] = useState<number>(0)
  // Bloquear el acceso hasta aprobar. Forzado para quien nunca ha entrado.
  const forceGate = onboardingStatus === 'pending' || onboardingStatus === 'blocked'
  const [gatesAccess, setGatesAccess] = useState(false)
  const effectiveGate = forceGate || gatesAccess

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
    Promise.all([inductionService.getConfig(), inductionService.listPrograms().catch(() => [] as InductionProgram[])])
      .then(([cfg, list]) => {
        const usable = list.filter((p) => p.is_active && p.block_count > 0)
        setPrograms(usable)
        setEnabled(!!cfg?.is_active && usable.some((p) => p.is_default))
      })
      .catch(() => setEnabled(false))
  }, [])

  useEffect(() => {
    void load()
  }, [load])

  const handleReset = async (inviteId?: number, gates?: boolean) => {
    const ok = await confirm({
      title: 'Reiniciar capacitación',
      message: gates
        ? 'Se le devolverán todos sus intentos en todos los bloques y se le reenviará el enlace por correo. Su acceso seguirá bloqueado hasta que apruebe.'
        : 'Se le devolverán todos sus intentos en todos los bloques y se le reenviará el enlace por correo. Su acceso no cambia.',
      confirmLabel: 'Reiniciar',
    })
    if (!ok) return

    setResetting(true)
    try {
      if (inviteId) await inductionService.resetInvite(inviteId)
      else await inductionService.resetUser(userId)
      success('Capacitación reiniciada. Se reenvió el enlace al profesional.')
      await load()
    } catch (err: any) {
      showError(err?.response?.data?.error ?? 'No se pudo reiniciar la capacitación.')
    } finally {
      setResetting(false)
    }
  }

  const handleInvite = async () => {
    const chosen = programs.find((p) => p.id === programId)
    const which = chosen ? ` el programa "${chosen.name}"` : ' el programa que le corresponda'
    const ok = await confirm({
      title: effectiveGate ? 'Enviar inducción de ingreso' : 'Enviar capacitación',
      message: effectiveGate
        ? `Se le enviará el enlace por correo y su acceso quedará bloqueado hasta que apruebe${which}.`
        : `Se le enviará el enlace por correo y por la campanita. Sigue entrando con normalidad y completa${which} cuando quiera; al aprobar gana sus insignias.`,
      confirmLabel: effectiveGate ? 'Enviar inducción' : 'Enviar capacitación',
    })
    if (!ok) return

    setInviting(true)
    try {
      await inductionService.inviteUser(userId, programId || undefined, effectiveGate)
      success(
        effectiveGate
          ? 'Inducción enviada. El profesional recibirá el enlace por correo.'
          : 'Capacitación enviada. El profesional recibirá el enlace por correo y en la app.'
      )
      await load()
    } catch (err: any) {
      showError(err?.response?.data?.error ?? 'No se pudo enviar la capacitación.')
    } finally {
      setInviting(false)
    }
  }

  if (loading) return null

  const canSend = isProfessional && enabled && canReset

  // Formulario de envío. Se usa tanto cuando no hay nada registrado como
  // debajo del historial, para mandar la siguiente capacitación.
  const sendForm = (
    <div>
      <label
        style={{
          display: 'flex',
          alignItems: 'center',
          gap: 10,
          fontSize: 13.5,
          color: '#334155',
          marginBottom: 14,
          cursor: forceGate ? 'not-allowed' : 'pointer',
        }}
      >
        <input
          type="checkbox"
          checked={effectiveGate}
          disabled={forceGate}
          onChange={(e) => setGatesAccess(e.target.checked)}
          style={{ width: 17, height: 17, accentColor: 'var(--primary)' }}
        />
        Bloquear el acceso hasta que apruebe
        {forceGate && <span style={{ color: '#94a3b8' }}>(obligatorio: aún no tiene acceso)</span>}
      </label>
      <div style={{ display: 'flex', gap: 12, alignItems: 'center', flexWrap: 'wrap' }}>
        <div style={{ minWidth: 260 }}>
          <Select
            fullWidth
            value={programId}
            onChange={(v) => setProgramId(Number(v) || 0)}
            options={[
              { value: 0, label: 'Programa automático (empresa o por defecto)' },
              ...programs.map((p) => ({
                value: p.id,
                label: `${p.name}${p.is_default ? ' (por defecto)' : ''} · ${p.block_count} bloques`,
              })),
            ]}
          />
        </div>
        <Button variant="secondary" leftIcon={<Send size={16} />} loading={inviting} onClick={handleInvite}>
          {effectiveGate ? 'Enviar inducción' : 'Enviar capacitación'}
        </Button>
      </div>
    </div>
  )

  // Sin nada registrado: solo se ofrece enviar.
  if (!status) {
    if (!canSend) return null
    return (
      <div style={card}>
        <h3 style={titleStyle}>
          <GraduationCap size={18} /> Inducción y capacitaciones
          <span style={badgeStyle('#f1f5f9', '#64748b')}>Sin registrar</span>
        </h3>
        <p style={{ margin: '0 0 16px', fontSize: 13.5, color: '#64748b' }}>
          Este profesional no tiene ninguna capacitación registrada. Puedes enviarle un programa: como
          capacitación (sigue entrando con normalidad) o como inducción de ingreso (sin acceso hasta aprobar).
        </p>
        {sendForm}
      </div>
    )
  }

  const badge = STATUS_LABEL[status.status] ?? STATUS_LABEL.pending
  const hasPending = status.status === 'pending'
  const history = status.history ?? []

  return (
    <div style={card}>
      <h3 style={{ ...titleStyle, margin: '0 0 16px' }}>
        <GraduationCap size={18} /> {hasPending ? 'Capacitación en curso' : 'Última capacitación'}
        <span style={badgeStyle(badge.bg, badge.fg)}>{badge.text}</span>
        {status.program_name && <span style={badgeStyle('#f1f5f9', '#64748b')}>{status.program_name}</span>}
        <span
          style={badgeStyle(status.gates_access ? '#fee2e2' : '#e0f2fe', status.gates_access ? '#b91c1c' : '#0369a1')}
          title={status.gates_access ? 'No puede entrar hasta aprobar' : 'Sigue entrando con normalidad'}
        >
          {status.gates_access ? 'Bloquea acceso' : 'Sin bloqueo'}
        </span>
      </h3>

      <div style={{ display: 'flex', flexWrap: 'wrap', gap: 24, fontSize: 14, color: '#475569' }}>
        <span>
          Bloques aprobados: <strong>{status.completed_blocks}</strong> de {status.total_blocks}
        </span>
        <span>
          Intentos usados: <strong>{status.attempts}</strong> (máx. {status.max_attempts} por bloque)
        </span>
        {status.expires_at && hasPending && (
          <span>
            Enlace vence: <strong>{shortDate(status.expires_at)}</strong>
          </span>
        )}
      </div>

      {status.blocks.length > 0 && (
        <div style={{ marginTop: 16 }}>
          <div style={subTitle}>Progreso por bloque</div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
            {status.blocks.map((b) => {
              const icon =
                b.status === 'passed' ? (
                  <CheckCircle2 size={16} color="#15803d" />
                ) : b.status === 'blocked' ? (
                  <XCircle size={16} color="#b91c1c" />
                ) : (
                  <Circle size={16} color="#94a3b8" />
                )
              return (
                <div key={b.block_id} style={rowStyle}>
                  {icon}
                  <span style={{ fontWeight: 700, color: '#0f172a', flex: 1, minWidth: 0 }}>
                    {b.order_index + 1}. {b.name}
                  </span>
                  <span style={{ color: '#64748b' }}>
                    {b.attempts} {b.attempts === 1 ? 'intento' : 'intentos'}
                  </span>
                  <span style={{ color: '#64748b' }}>
                    mejor <strong>{Math.round(b.best_score)}%</strong> · mín. {b.passing_score}%
                  </span>
                </div>
              )
            })}
          </div>
        </div>
      )}

      {status.attempt_log.length > 0 && (
        <div style={{ marginTop: 16 }}>
          <div style={subTitle}>Historial de intentos</div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
            {status.attempt_log.map((a, i) => (
              <div key={a.id} style={{ ...rowStyle, justifyContent: 'space-between' }}>
                <span style={{ color: '#64748b' }}>
                  Intento {i + 1}
                  {a.block_name ? ` · ${a.block_name}` : ''}
                </span>
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
            onClick={() => handleReset(status.invite_id, status.gates_access)}
          >
            Reiniciar intentos y reenviar enlace
          </Button>
        </div>
      )}

      {history.length > 1 && (
        <div style={{ marginTop: 22 }}>
          <div style={{ ...subTitle, display: 'flex', alignItems: 'center', gap: 6 }}>
            <History size={14} /> Todas las capacitaciones ({history.length})
          </div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
            {history.map((h) => {
              const hb = STATUS_LABEL[h.status] ?? STATUS_LABEL.pending
              return (
                <div key={h.id} style={rowStyle}>
                  <span style={{ fontWeight: 700, color: '#0f172a', flex: 1, minWidth: 0 }}>
                    {h.program_name || 'Programa'}
                    {h.current && <span style={{ ...badgeStyle('#fdf4ff', '#a21caf'), marginLeft: 8 }}>Actual</span>}
                  </span>
                  <span style={badgeStyle(hb.bg, hb.fg)}>{hb.text}</span>
                  <span style={{ color: '#64748b' }}>
                    {h.completed_blocks}/{h.total_blocks} bloques
                  </span>
                  <span style={{ color: '#64748b' }}>{h.gates_access ? 'Ingreso' : 'Capacitación'}</span>
                  <span style={{ color: '#94a3b8' }}>{shortDate(h.completed_at || h.created_at)}</span>
                </div>
              )
            })}
          </div>
        </div>
      )}

      {canSend && !hasPending && (
        <div style={{ marginTop: 22, paddingTop: 18, borderTop: '1px solid #e2e8f0' }}>
          <div style={subTitle}>Enviar otra capacitación</div>
          <p style={{ margin: '0 0 14px', fontSize: 13.5, color: '#64748b' }}>
            Las capacitaciones terminadas se conservan en el historial y las insignias ganadas no se pierden.
          </p>
          {sendForm}
        </div>
      )}
      {canSend && hasPending && (
        <p style={{ margin: '16px 0 0', fontSize: 13, color: '#94a3b8' }}>
          Mientras esta capacitación esté en curso no se puede enviar otra.
        </p>
      )}
    </div>
  )
}
