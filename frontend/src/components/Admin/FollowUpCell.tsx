import type { FollowUpInfo } from '../../hooks/useAdmin'

// Bitácora de gestión de customer success. La misma celda sirve para el
// seguimiento de inactividad y el de ausencias, y ahora también para la ficha
// de empresa: vive aquí para que las tres pantallas no acaben con tres copias
// del mismo desplegable.
const FOLLOWUP_STYLES: Record<string, { bg: string; color: string }> = {
  contacted: { bg: 'rgba(59,130,246,0.12)', color: '#1d4ed8' },
  justified: { bg: 'rgba(16,185,129,0.12)', color: '#047857' },
  escalated: { bg: 'rgba(168,85,247,0.14)', color: '#7e22ce' },
}

export interface FollowUpCellProps {
  info?: FollowUpInfo
  onChange: (status: string) => void
  /** Solo lectura para quien no puede anotar (customer success en consulta). */
  disabled?: boolean
}

export function FollowUpCell({ info, onChange, disabled = false }: FollowUpCellProps) {
  const palette = info ? FOLLOWUP_STYLES[info.status] : undefined
  return (
    <div onClick={(e) => e.stopPropagation()}>
      <select
        value={info?.status || ''}
        onChange={(e) => { if (e.target.value) onChange(e.target.value) }}
        disabled={disabled}
        title={disabled ? 'Sin permiso para anotar gestión' : 'Estado de gestión del seguimiento'}
        style={{ padding: '5px 8px', borderRadius: '8px', border: '1px solid #e2e8f0', fontSize: '12px', fontWeight: 700, background: palette?.bg || '#f8fafc', color: palette?.color || '#64748b', cursor: disabled ? 'not-allowed' : 'pointer' }}
      >
        {!info && <option value="">— Gestionar —</option>}
        <option value="contacted">📞 Contactado</option>
        <option value="justified">✅ Justificado</option>
        <option value="escalated">⚠️ Escalado</option>
      </select>
      {info && (
        <small style={{ display: 'block', color: '#94a3b8', marginTop: '3px' }}>
          por {info.by_name} · {new Date(info.created_at).toLocaleDateString('es-ES')}
        </small>
      )}
    </div>
  )
}
