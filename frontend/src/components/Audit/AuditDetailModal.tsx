import { X, ListFilter, Shield, ExternalLink } from 'lucide-react'
import { AuditLog } from '../../services/audit.service'
import { describeAudit, moduleLabel, originLabel } from '../../lib/auditHumanize'

interface AuditDetailModalProps {
  log: AuditLog
  onClose: () => void
  /** Filter the audit list by this entity (entity_type + entity_id). */
  onViewEntity: (entityType: string, entityId: string) => void
  /** Navigate to the app page where the entity lives, if a mapping exists. */
  onGoToSource?: () => void
}

function prettyChanges(changes?: string): string {
  if (!changes) return ''
  try { return JSON.stringify(JSON.parse(changes), null, 2) } catch { return changes }
}

export default function AuditDetailModal({ log, onClose, onViewEntity, onGoToSource }: AuditDetailModalProps) {
  const Row = ({ label, value, mono }: { label: string; value?: React.ReactNode; mono?: boolean }) => (
    <div style={{ display: 'flex', gap: '0.75rem', padding: '0.5rem 0', borderBottom: '1px solid var(--glass-border, #f1f5f9)' }}>
      <div style={{ width: 140, flexShrink: 0, fontSize: '0.78rem', fontWeight: 700, color: 'var(--text-secondary)', textTransform: 'uppercase', letterSpacing: '0.02em' }}>{label}</div>
      <div style={{ fontSize: '0.88rem', color: 'var(--text-primary)', wordBreak: 'break-word', fontFamily: mono ? 'ui-monospace, monospace' : 'inherit' }}>{value || '—'}</div>
    </div>
  )

  const pretty = prettyChanges(log.changes)
  const hasEntity = !!log.entity_type

  return (
    <div onClick={onClose} style={{ position: 'fixed', inset: 0, background: 'rgba(15,23,42,0.45)', display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 1100, padding: '1rem' }}>
      <div onClick={(e) => e.stopPropagation()} style={{ background: 'var(--bg-primary, #fff)', borderRadius: '16px', width: '100%', maxWidth: '560px', maxHeight: '88vh', overflowY: 'auto', padding: '1.5rem', boxShadow: '0 20px 40px rgba(0,0,0,0.2)' }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '1rem' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
            <Shield size={19} style={{ color: 'var(--primary)' }} />
            <h2 style={{ fontSize: '1.05rem', fontWeight: 700, margin: 0 }}>Detalle de auditoría</h2>
            <span style={{ fontSize: '0.7rem', fontWeight: 700, padding: '2px 8px', borderRadius: '99px', background: log.kind === 'data' ? 'rgba(99,102,241,0.12)' : 'rgba(16,185,129,0.12)', color: log.kind === 'data' ? '#4f46e5' : '#059669' }}>
              {log.kind === 'data' ? 'Cambio de datos' : 'Actividad'}
            </span>
          </div>
          <button onClick={onClose} style={{ background: 'none', border: 'none', cursor: 'pointer', color: 'var(--text-secondary)' }}><X size={20} /></button>
        </div>

        <div style={{ background: 'var(--bg-secondary, #f8fafc)', borderRadius: '12px', padding: '0.85rem 1rem', marginBottom: '0.5rem' }}>
          <div style={{ fontSize: '1rem', fontWeight: 700, color: 'var(--text-primary)' }}>{describeAudit(log)}</div>
          <div style={{ fontSize: '0.8rem', color: 'var(--text-secondary)', marginTop: '2px' }}>
            {originLabel(log)} · {moduleLabel(log.module)} · {new Date(log.created_at).toLocaleString('es-ES')}
          </div>
        </div>
        <Row label="Área" value={moduleLabel(log.module)} />
        <Row label="Entidad" value={log.entity_type ? `${log.entity_type}${log.entity_id ? ` #${log.entity_id}` : ''}` : (log.target_id || '—')} />
        <Row label="Acción (técnica)" value={log.action} mono />
        {log.kind !== 'data' && <>
          <Row label="Usuario" value={log.actor_email} />
          <Row label="Rol" value={log.actor_role} />
          <Row label="IP" value={log.ip} mono />
          <Row label="Navegador" value={log.user_agent} />
        </>}
        <Row label="Método / Ruta" value={<span><b>{log.method}</b> <span style={{ fontFamily: 'ui-monospace, monospace' }}>{log.path}</span></span>} />
        <Row label="Estado" value={<span style={{ fontWeight: 700, color: log.success ? '#059669' : '#dc2626' }}>{log.status} {log.success ? '(OK)' : '(Error)'}</span>} />

        {pretty && (
          <div style={{ marginTop: '0.75rem' }}>
            <div style={{ fontSize: '0.78rem', fontWeight: 700, color: 'var(--text-secondary)', textTransform: 'uppercase', marginBottom: '0.35rem' }}>Cambios</div>
            <pre style={{ margin: 0, background: 'var(--bg-secondary, #f8fafc)', border: '1px solid var(--glass-border, #e2e8f0)', borderRadius: '10px', padding: '0.75rem', fontSize: '0.8rem', overflowX: 'auto', maxHeight: 240 }}>{pretty}</pre>
          </div>
        )}

        <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '0.5rem', marginTop: '1.25rem', flexWrap: 'wrap' }}>
          <button onClick={onClose} style={{ padding: '0.55rem 1rem', borderRadius: '10px', border: '1px solid var(--glass-border, #cbd5e1)', background: 'transparent', fontWeight: 600, cursor: 'pointer', fontSize: '0.85rem' }}>Cerrar</button>
          {hasEntity && (
            <button onClick={() => onViewEntity(log.entity_type!, log.entity_id || '')}
              style={{ display: 'inline-flex', alignItems: 'center', gap: '0.4rem', padding: '0.55rem 1rem', borderRadius: '10px', border: '1px solid var(--primary, #cc33cc)', background: 'transparent', color: 'var(--primary)', fontWeight: 600, cursor: 'pointer', fontSize: '0.85rem' }}>
              <ListFilter size={15} /> Ver su rastro
            </button>
          )}
          {onGoToSource && (
            <button onClick={onGoToSource}
              style={{ display: 'inline-flex', alignItems: 'center', gap: '0.4rem', padding: '0.55rem 1rem', borderRadius: '10px', border: 'none', background: 'var(--primary, #cc33cc)', color: '#fff', fontWeight: 600, cursor: 'pointer', fontSize: '0.85rem' }}>
              <ExternalLink size={15} /> Ir al origen
            </button>
          )}
        </div>
      </div>
    </div>
  )
}
