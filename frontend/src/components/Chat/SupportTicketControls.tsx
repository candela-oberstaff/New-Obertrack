import { LifeBuoy, Check, UserCheck } from 'lucide-react'
import type { Channel } from '../../types/chat'
import type { User } from '../../types'
import { supportStatusMeta } from './ChatUtils'

interface SupportTicketControlsProps {
  channel: Channel
  currentUserId?: number
  isSupportAgent: boolean
  supportAgents: User[]
  onClaim: () => void
  onAssign: (assigneeId: number) => void
  onResolve: () => void
  busy?: boolean
}

/**
 * Controles de gestión del ticket de soporte en el encabezado del chat.
 * - Para el solicitante: solo una píldora de estado (lectura).
 * - Para Customer Success / superadmins: Tomar, Reasignar y Resolver.
 */
export function SupportTicketControls({
  channel, currentUserId, isSupportAgent, supportAgents, onClaim, onAssign, onResolve, busy,
}: SupportTicketControlsProps) {
  const support = channel.support
  if (!support) return null

  const meta = supportStatusMeta(support.status)
  const assignedToMe = !!support.assigned_to && support.assigned_to === currentUserId
  const pillText =
    support.status === 'resolved'
      ? 'Resuelto'
      : support.assigned_to
        ? `Atendido por ${assignedToMe ? 'ti' : support.assignee_name || 'un agente'}`
        : 'Sin asignar'

  const pill = (
    <span style={{ display: 'inline-flex', alignItems: 'center', gap: 6, padding: '5px 10px', borderRadius: 999, fontSize: 12, fontWeight: 700, color: meta.color, background: meta.bg, whiteSpace: 'nowrap' }}>
      <LifeBuoy size={13} /> {pillText}
    </span>
  )

  if (!isSupportAgent) return pill

  const baseBtn: React.CSSProperties = {
    display: 'inline-flex', alignItems: 'center', gap: 5, padding: '6px 12px',
    borderRadius: 8, fontSize: 13, fontWeight: 700, border: '1px solid #e2e8f0',
    background: '#fff', color: '#334155', cursor: busy ? 'wait' : 'pointer', width: 'auto',
  }

  return (
    <div style={{ display: 'inline-flex', alignItems: 'center', gap: 8, flexWrap: 'wrap' }}>
      {pill}

      {!assignedToMe && support.status !== 'resolved' && (
        <button disabled={busy} onClick={onClaim} style={{ ...baseBtn, background: '#7c3aed', color: '#fff', border: 'none' }} title="Tomar el ticket">
          <UserCheck size={14} /> Tomar
        </button>
      )}

      {support.status !== 'resolved' && supportAgents.length > 0 && (
        <select
          value=""
          disabled={busy}
          onChange={(e) => { const v = Number(e.target.value); if (v) onAssign(v) }}
          title="Reasignar a otro agente"
          style={{ border: '1px solid #e2e8f0', borderRadius: 8, padding: '6px 8px', fontSize: 13, color: '#334155', cursor: busy ? 'wait' : 'pointer', background: '#fff' }}
        >
          <option value="">Reasignar…</option>
          {supportAgents.filter(a => a.id !== support.assigned_to).map(a => (
            <option key={a.id} value={a.id}>{a.name}</option>
          ))}
        </select>
      )}

      {support.status !== 'resolved' ? (
        <button disabled={busy} onClick={onResolve} style={{ ...baseBtn, background: '#16a34a', color: '#fff', border: 'none' }} title="Marcar como resuelto">
          <Check size={14} /> Resolver
        </button>
      ) : (
        <button disabled={busy} onClick={onClaim} style={baseBtn} title="Reabrir el ticket">
          Reabrir
        </button>
      )}
    </div>
  )
}
