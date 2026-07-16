import { LifeBuoy, Check, UserCheck, UserCog } from 'lucide-react'
import { useNavigate } from 'react-router-dom'
import type { Channel } from '../../types/chat'
import type { User } from '../../types'
import { supportStatusMeta } from './ChatUtils'
import { PROFILE_CHANGE_MODULE } from '../../constants/support'
import { Select } from '../ui/Select'

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
 * - Si el ticket lo atiende OTRO agente, la conversación es suya: Reasignar y
 *   Resolver quedan bloqueados y el único camino es "Tomar" el ticket.
 */
export function SupportTicketControls({
  channel, currentUserId, isSupportAgent, supportAgents, onClaim, onAssign, onResolve, busy,
}: SupportTicketControlsProps) {
  const navigate = useNavigate()
  const support = channel.support
  if (!support) return null

  const isProfileChange = support.module === PROFILE_CHANGE_MODULE

  const meta = supportStatusMeta(support.status)
  const assignedToMe = !!support.assigned_to && support.assigned_to === currentUserId
  const attendedByOther = !!support.assigned_to && !assignedToMe
  const pillText =
    support.status === 'resolved'
      ? 'Resuelto'
      : support.assigned_to
        ? `Atendido por ${assignedToMe ? 'ti' : support.assignee_name || 'un agente'}`
        // El solicitante (cliente) ve un estado amigable; el agente ve "Sin asignar".
        : (isSupportAgent ? 'Sin asignar' : 'En cola')

  const pill = (
    <span style={{ display: 'inline-flex', alignItems: 'center', gap: 6, padding: '5px 10px', borderRadius: 999, fontSize: 12, fontWeight: 700, color: meta.color, background: meta.bg, whiteSpace: 'nowrap' }}>
      <LifeBuoy size={13} /> {pillText}
    </span>
  )

  const subjectLabel = support.subject ? (
    <span title={support.subject} style={{ fontSize: 13, fontWeight: 600, color: '#334155', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis', maxWidth: 220 }}>
      {support.subject}
    </span>
  ) : null

  if (!isSupportAgent) {
    return (
      <div style={{ display: 'inline-flex', alignItems: 'center', gap: 8, minWidth: 0 }}>
        {pill}
        {subjectLabel}
      </div>
    )
  }

  const baseBtn: React.CSSProperties = {
    display: 'inline-flex', alignItems: 'center', gap: 5, padding: '6px 12px',
    borderRadius: 8, fontSize: 13, fontWeight: 700, border: '1px solid #e2e8f0',
    background: '#fff', color: '#334155', cursor: busy ? 'wait' : 'pointer', width: 'auto',
  }

  return (
    <div style={{ display: 'inline-flex', alignItems: 'center', gap: 8, flexWrap: 'wrap' }}>
      {pill}
      {subjectLabel}

      {!assignedToMe && support.status !== 'resolved' && (
        <button disabled={busy} onClick={onClaim} style={{ ...baseBtn, background: '#7c3aed', color: '#fff', border: 'none' }} title="Tomar el ticket">
          <UserCheck size={14} /> Tomar
        </button>
      )}

      {support.status !== 'resolved' && supportAgents.length > 0 && (
        <div style={{ width: 150 }} title={attendedByOther ? 'Este ticket lo atiende otro agente. Tómalo para reasignarlo.' : undefined}>
          <Select
            value=""
            onChange={(v) => { const id = Number(v); if (id) onAssign(id) }}
            placeholder="Reasignar…"
            disabled={busy || attendedByOther}
            fullWidth
            className="ui-select__trigger--compact"
            ariaLabel="Reasignar a otro agente"
            leftIcon={<UserCog size={14} />}
            options={supportAgents
              .filter(a => a.id !== support.assigned_to)
              .map(a => ({ value: a.id, label: a.name }))}
          />
        </div>
      )}

      {support.status !== 'resolved' ? (
        isProfileChange ? (
          <button
            onClick={() => navigate(`/admin/users/${support.requester_id}`)}
            style={{ ...baseBtn, background: '#7c3aed', color: '#fff', border: 'none' }}
            title="Revisar y aplicar los cambios en la ficha del profesional"
          >
            <UserCog size={14} /> Aplicar en ficha
          </button>
        ) : (
          <button
            disabled={busy || attendedByOther}
            onClick={onResolve}
            style={{ ...baseBtn, background: '#16a34a', color: '#fff', border: 'none', opacity: attendedByOther ? 0.5 : 1, cursor: attendedByOther ? 'not-allowed' : busy ? 'wait' : 'pointer' }}
            title={attendedByOther ? 'Este ticket lo atiende otro agente. Tómalo para resolverlo.' : 'Marcar como resuelto'}
          >
            <Check size={14} /> Resolver
          </button>
        )
      ) : (
        <button disabled={busy} onClick={onClaim} style={baseBtn} title="Reabrir el ticket">
          Reabrir
        </button>
      )}
    </div>
  )
}
