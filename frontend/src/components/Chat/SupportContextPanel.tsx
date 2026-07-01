import { LifeBuoy, Building2, Mail, Phone, Calendar, UserCheck, X, Tag, UserCog } from 'lucide-react'
import { useNavigate } from 'react-router-dom'
import type { Channel } from '../../types/chat'
import { supportStatusMeta } from './ChatUtils'
import { PROFILE_CHANGE_MODULE } from '../../constants/support'

interface SupportContextPanelProps {
  channel: Channel
  onClose?: () => void
}

/**
 * Panel lateral con el contexto de la solicitud de soporte: datos del solicitante,
 * estado y responsable. Acompaña al chat sin reemplazarlo.
 */
export function SupportContextPanel({ channel, onClose }: SupportContextPanelProps) {
  const navigate = useNavigate()
  const s = channel.support
  if (!s) return null

  const isProfileChange = s.module === PROFILE_CHANGE_MODULE
  const meta = supportStatusMeta(s.status)
  const created = s.created_at ? new Date(s.created_at).toLocaleString('es-ES', { dateStyle: 'medium', timeStyle: 'short' }) : '—'

  const row = (icon: React.ReactNode, label: string, value: React.ReactNode) => (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 3 }}>
      <span style={{ display: 'inline-flex', alignItems: 'center', gap: 6, fontSize: 11, fontWeight: 700, color: '#94a3b8', textTransform: 'uppercase', letterSpacing: '0.05em' }}>
        {icon} {label}
      </span>
      <span style={{ fontSize: 14, color: '#1e293b', wordBreak: 'break-word' }}>{value || '—'}</span>
    </div>
  )

  return (
    <aside
      style={{
        width: 280, flexShrink: 0, borderLeft: '1px solid #e2e8f0', background: '#fff',
        display: 'flex', flexDirection: 'column', overflowY: 'auto',
      }}
    >
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '16px 18px', borderBottom: '1px solid #f1f5f9' }}>
        <span style={{ display: 'inline-flex', alignItems: 'center', gap: 8, fontSize: 14, fontWeight: 800, color: '#1e293b' }}>
          <LifeBuoy size={16} color="#7c3aed" /> Solicitud de soporte
        </span>
        {onClose && (
          <button onClick={onClose} title="Ocultar panel" style={{ border: 'none', background: 'transparent', cursor: 'pointer', color: '#94a3b8', display: 'inline-flex' }}>
            <X size={16} />
          </button>
        )}
      </div>

      <div style={{ padding: 18, display: 'flex', flexDirection: 'column', gap: 16 }}>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
          <span style={{ fontSize: 11, fontWeight: 700, color: '#94a3b8', textTransform: 'uppercase', letterSpacing: '0.05em' }}>Estado</span>
          <span style={{ display: 'inline-flex', alignItems: 'center', gap: 6, alignSelf: 'flex-start', padding: '5px 12px', borderRadius: 999, fontSize: 13, fontWeight: 700, color: meta.color, background: meta.bg }}>
            {meta.label}
          </span>
        </div>

        {s.module && row(<Tag size={12} />, 'Módulo', s.module)}

        {isProfileChange && (
          <div style={{ padding: '12px 14px', borderRadius: 12, border: '1px solid #ddd6fe', background: '#f5f3ff', display: 'flex', flexDirection: 'column', gap: 10 }}>
            <span style={{ fontSize: 12.5, color: '#5b21b6', lineHeight: 1.4 }}>
              Esta solicitud <strong>no se resuelve por el chat</strong>. Revisa y aplica los cambios en la ficha del profesional (eso resuelve el ticket).
            </span>
            <button
              onClick={() => navigate(`/admin/users/${s.requester_id}`)}
              style={{ display: 'inline-flex', alignItems: 'center', justifyContent: 'center', gap: 6, padding: '8px 12px', borderRadius: 8, border: 'none', background: '#7c3aed', color: '#fff', fontSize: 13, fontWeight: 700, cursor: 'pointer' }}
            >
              <UserCog size={14} /> Aplicar cambios en la ficha
            </button>
          </div>
        )}

        {row(<UserCheck size={12} />, 'Responsable', s.assignee_name || <span style={{ color: '#b45309' }}>Sin asignar</span>)}
        {row(<LifeBuoy size={12} />, 'Solicitante', s.requester_name)}
        {row(<Building2 size={12} />, 'Empresa', s.company_name)}
        {row(
          <Mail size={12} />, 'Correo',
          s.requester_email ? <a href={`mailto:${s.requester_email}`} style={{ color: '#7c3aed', textDecoration: 'none' }}>{s.requester_email}</a> : '—'
        )}
        {row(
          <Phone size={12} />, 'Teléfono',
          s.requester_phone
            ? <a href={`https://wa.me/${s.requester_phone.replace(/\D/g, '')}`} target="_blank" rel="noopener noreferrer" style={{ color: '#16a34a', textDecoration: 'none' }}>{s.requester_phone}</a>
            : '—'
        )}
        {row(<Calendar size={12} />, 'Creada', created)}
      </div>
    </aside>
  )
}
