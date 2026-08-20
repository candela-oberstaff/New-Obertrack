import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { UserMinus, Ban, Eye, Archive, ChevronLeft, ChevronRight } from 'lucide-react'
import Avatar from '../Common/Avatar'

interface ArchivedEntry {
  kind: 'ended_employment' | 'deactivated_user'
  user_id: number
  employment_id: number
  name: string
  email: string
  avatar?: string
  company: string
  company_id: number
  job_title?: string
  reason?: string
  archived_at: string
}

interface ArchivedListProps {
  entries: ArchivedEntry[]
  // Mostrar la empresa de cada fila (útil en la vista global).
  showCompany?: boolean
}

const fmtDate = (v?: string) => {
  if (!v) return ''
  const d = new Date(v)
  return isNaN(d.getTime()) ? '' : d.toLocaleDateString('es-ES', { day: '2-digit', month: 'short', year: 'numeric' })
}

// Lista reutilizable de profesionales archivados (bajas de empleo + cuentas
// desactivadas). Cada fila lleva al detalle de la persona, donde se reactiva.
export function ArchivedList({ entries, showCompany = true }: ArchivedListProps) {
  const navigate = useNavigate()
  const [page, setPage] = useState(1)
  const keyOf = (e: ArchivedEntry) => `${e.kind}-${e.user_id}-${e.employment_id}`

  if (entries.length === 0) {
    return (
      <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 8, padding: 36, color: '#94a3b8' }}>
        <Archive size={38} />
        <p style={{ margin: 0 }}>No hay profesionales archivados</p>
      </div>
    )
  }

  const PER_PAGE = 7
  const totalPages = Math.max(1, Math.ceil(entries.length / PER_PAGE))
  const currentPage = Math.min(page, totalPages)
  const paginated = entries.slice((currentPage - 1) * PER_PAGE, currentPage * PER_PAGE)

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
      {paginated.map((e) => {
        const ended = e.kind === 'ended_employment'
        return (
          <div
            key={keyOf(e)}
            onClick={() => navigate(`/admin/users/${e.user_id}`)}
            style={{ display: 'flex', alignItems: 'center', gap: 12, padding: '10px 14px', border: '1px solid #e2e8f0', borderRadius: 10, cursor: 'pointer' }}
            title="Ver detalle de la persona"
          >
            <Avatar src={e.avatar} name={e.name} size="sm" />
            <div style={{ minWidth: 0, flex: 1 }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap' }}>
                <span style={{ fontWeight: 700, color: '#0f172a' }}>{e.name}</span>
                <span style={{ display: 'inline-flex', alignItems: 'center', gap: 4, padding: '2px 8px', borderRadius: 999, fontSize: '0.7rem', fontWeight: 700, background: ended ? 'rgba(245,158,11,0.14)' : 'rgba(239,68,68,0.12)', color: ended ? '#b45309' : '#b91c1c' }}>
                  {ended ? <><UserMinus size={11} /> Baja de empleo</> : <><Ban size={11} /> Cuenta desactivada</>}
                </span>
              </div>
              <div style={{ fontSize: '0.8rem', color: '#94a3b8' }}>
                {e.email}
                {showCompany && e.company && e.company !== '-' ? ` · ${e.company}` : ''}
                {e.job_title ? ` · ${e.job_title}` : ''}
                {e.archived_at ? ` · ${fmtDate(e.archived_at)}` : ''}
                {ended && e.reason ? ` · ${e.reason}` : ''}
              </div>
            </div>
            <span
              style={{ display: 'inline-flex', alignItems: 'center', gap: 6, padding: '6px 12px', borderRadius: 8, border: '1px solid #cbd5e1', background: '#fff', color: '#334155', fontWeight: 700, fontSize: '0.82rem', whiteSpace: 'nowrap' }}
            >
              <Eye size={14} /> Ver detalle
            </span>
          </div>
        )
      })}

      {totalPages > 1 && (
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '12px', flexWrap: 'wrap', padding: '14px 4px' }}>
          <span style={{ fontSize: '13px', color: '#64748b' }}>
            Mostrando {(currentPage - 1) * PER_PAGE + 1}–{Math.min(currentPage * PER_PAGE, entries.length)} de {entries.length} archivados
          </span>
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
            <button
              type="button"
              onClick={() => setPage(p => Math.max(1, p - 1))}
              disabled={currentPage <= 1}
              style={{
                display: 'inline-flex',
                alignItems: 'center',
                justifyContent: 'center',
                width: '32px',
                height: '32px',
                borderRadius: '8px',
                border: '1px solid #cbd5e1',
                background: '#fff',
                color: '#334155',
                cursor: currentPage <= 1 ? 'not-allowed' : 'pointer',
                opacity: currentPage <= 1 ? 0.4 : 1
              }}
              title="Página anterior"
            >
              <ChevronLeft size={16} />
            </button>
            <span style={{ fontSize: '13px', fontWeight: 600, color: '#334155', whiteSpace: 'nowrap' }}>
              Página {currentPage} de {totalPages}
            </span>
            <button
              type="button"
              onClick={() => setPage(p => Math.min(totalPages, p + 1))}
              disabled={currentPage >= totalPages}
              style={{
                display: 'inline-flex',
                alignItems: 'center',
                justifyContent: 'center',
                width: '32px',
                height: '32px',
                borderRadius: '8px',
                border: '1px solid #cbd5e1',
                background: '#fff',
                color: '#334155',
                cursor: currentPage >= totalPages ? 'not-allowed' : 'pointer',
                opacity: currentPage >= totalPages ? 0.4 : 1
              }}
              title="Página siguiente"
            >
              <ChevronRight size={16} />
            </button>
          </div>
        </div>
      )}
    </div>
  )
}
