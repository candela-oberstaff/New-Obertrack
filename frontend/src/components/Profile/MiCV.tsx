import { useState, useEffect } from 'react'
import { Building2, Briefcase, Clock3, CheckSquare, CalendarDays, Star, FileText, Snowflake, Download } from 'lucide-react'
import { authService } from '../../services/api'
import styles from '../../pages/Profile.module.css'

interface CVEntry {
  employment: {
    id: number
    company_name: string
    job_title?: string
    status: string
    started_at: string
    ended_at?: string | null
    manager_name?: string
    end_reason?: string
  }
  summary: {
    days_employed: number
    total_hours: number
    approved_hours: number
    tasks_assigned: number
    tasks_completed: number
    absences: number
    frozen_at?: string | null
  }
  notes: { id: number; kind: string; rating?: number; content: string; author_name: string; created_at: string }[]
  documents: { id: number; employment_id: number; title?: string; file_name: string; created_at: string }[]
}

interface CVData {
  entries: CVEntry[]
  total_companies: number
  active_companies: number
  total_days: number
}

const monthYear = (v?: string | null) =>
  v ? new Date(v).toLocaleDateString('es-ES', { month: 'short', year: 'numeric' }) : ''

// Antigüedad legible: años/meses cuando es largo, días cuando es corto.
const humanDuration = (days: number) => {
  if (days >= 365) {
    const y = Math.floor(days / 365)
    const m = Math.floor((days % 365) / 30)
    return m > 0 ? `${y} año${y > 1 ? 's' : ''} ${m} mes${m > 1 ? 'es' : ''}` : `${y} año${y > 1 ? 's' : ''}`
  }
  if (days >= 30) {
    const m = Math.floor(days / 30)
    return `${m} mes${m > 1 ? 'es' : ''}`
  }
  return `${days} día${days === 1 ? '' : 's'}`
}

const fmtH = (n: number) => (Math.round((n || 0) * 10) / 10).toLocaleString('es-ES')

// Mi CV: la trayectoria laboral unificada del profesional en todas las empresas
// (activas y pasadas), con lo que cada una decidió compartir.
export function MiCV() {
  const [cv, setCv] = useState<CVData | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    authService.myCV()
      .then(setCv)
      .catch(() => setCv(null))
      .finally(() => setLoading(false))
  }, [])

  if (loading) return null
  if (!cv || cv.entries.length === 0) return null

  return (
    <div className={styles['sidebar-card']} style={{ marginTop: '16px' }}>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 8, flexWrap: 'wrap' }}>
        <h3 style={{ display: 'flex', alignItems: 'center', gap: 8, margin: 0 }}>
          <Briefcase size={18} /> Mi Trayectoria
        </h3>
        <a
          href="/api/me/cv/pdf"
          target="_blank"
          rel="noopener noreferrer"
          style={{ display: 'inline-flex', alignItems: 'center', gap: 6, padding: '6px 12px', borderRadius: 8, border: '1px solid #cbd5e1', background: '#fff', color: '#334155', fontWeight: 700, fontSize: '0.82rem', textDecoration: 'none' }}
        >
          <Download size={14} /> Descargar PDF
        </a>
      </div>
      <p style={{ margin: '6px 0 16px', fontSize: '0.82rem', color: '#94a3b8' }}>
        Tu CV vivo: dónde trabajaste, cuánto tiempo y lo que cada empresa compartió contigo.
      </p>

      {/* Agregados */}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(120px, 1fr))', gap: 10, marginBottom: 18 }}>
        <CvStat icon={<Building2 size={16} />} label="Empresas" value={`${cv.total_companies}`} />
        <CvStat icon={<Briefcase size={16} />} label="En curso" value={`${cv.active_companies}`} />
        <CvStat icon={<CalendarDays size={16} />} label="Trayectoria" value={humanDuration(cv.total_days)} />
      </div>

      {/* Línea de tiempo */}
      <div style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
        {cv.entries.map((e) => {
          const active = e.employment.status === 'active'
          return (
            <div key={e.employment.id} style={{ border: '1px solid #e2e8f0', borderRadius: 12, padding: '14px 16px' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap', marginBottom: 4 }}>
                <span style={{ fontWeight: 800, fontSize: '1rem', color: '#0f172a' }}>{e.employment.company_name}</span>
                <span style={{ padding: '2px 10px', borderRadius: 999, fontSize: '0.7rem', fontWeight: 700, background: active ? 'rgba(16,185,129,0.12)' : 'rgba(100,116,139,0.12)', color: active ? '#047857' : '#64748b' }}>
                  {active ? 'Actual' : 'Finalizado'}
                </span>
              </div>
              <div style={{ fontSize: '0.85rem', color: '#475569', marginBottom: 10 }}>
                {e.employment.job_title || 'Sin cargo'} · {monthYear(e.employment.started_at)} – {active ? 'actualidad' : monthYear(e.employment.ended_at)} · {humanDuration(e.summary.days_employed)}
                {e.employment.manager_name ? ` · Reporta a ${e.employment.manager_name}` : ''}
              </div>

              {/* Mini stats */}
              <div style={{ display: 'flex', gap: 16, flexWrap: 'wrap', fontSize: '0.8rem', color: '#64748b', marginBottom: e.notes.length || e.documents.length ? 12 : 0 }}>
                <span style={{ display: 'inline-flex', alignItems: 'center', gap: 5 }}><Clock3 size={14} /> {fmtH(e.summary.approved_hours)} h aprobadas</span>
                <span style={{ display: 'inline-flex', alignItems: 'center', gap: 5 }}><CheckSquare size={14} /> {e.summary.tasks_completed}/{e.summary.tasks_assigned} tareas</span>
                {e.summary.frozen_at && (
                  <span style={{ display: 'inline-flex', alignItems: 'center', gap: 5, color: '#0369a1' }}><Snowflake size={13} /> Histórico congelado</span>
                )}
              </div>

              {/* Evaluaciones compartidas */}
              {e.notes.length > 0 && (() => {
                const evals = e.notes.filter(n => n.kind === 'evaluation' && (n.rating ?? 0) > 0)
                if (evals.length === 0) return null
                const avg = evals.reduce((s, n) => s + (n.rating || 0), 0) / evals.length
                return (
                  <div style={{ display: 'flex', alignItems: 'center', gap: 6, fontSize: '0.8rem', color: '#64748b', marginBottom: 8 }}>
                    <span style={{ fontWeight: 800, color: '#0f172a' }}>{avg.toFixed(1)}</span>
                    <span style={{ display: 'inline-flex', color: '#f59e0b' }}>
                      {[1, 2, 3, 4, 5].map(i => <Star key={i} size={12} fill={i <= Math.round(avg) ? '#f59e0b' : 'none'} />)}
                    </span>
                    <span style={{ color: '#94a3b8' }}>· promedio de {evals.length} evaluación{evals.length === 1 ? '' : 'es'}</span>
                  </div>
                )
              })()}
              {e.notes.length > 0 && (
                <div style={{ marginBottom: e.documents.length ? 10 : 0 }}>
                  <div style={{ fontSize: '0.72rem', fontWeight: 700, textTransform: 'uppercase', letterSpacing: '0.04em', color: '#94a3b8', marginBottom: 6 }}>Evaluaciones</div>
                  <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
                    {e.notes.map((n) => (
                      <div key={n.id} style={{ background: '#f8fafc', borderRadius: 8, padding: '8px 10px' }}>
                        <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 2, flexWrap: 'wrap' }}>
                          {n.kind === 'evaluation' && (n.rating ?? 0) > 0 && (
                            <span style={{ display: 'inline-flex', gap: 1, color: '#f59e0b' }}>
                              {Array.from({ length: n.rating || 0 }).map((_, i) => <Star key={i} size={13} fill="#f59e0b" />)}
                            </span>
                          )}
                          <span style={{ marginLeft: 'auto', fontSize: '0.72rem', color: '#94a3b8' }}>
                            {n.author_name || 'Empresa'} · {new Date(n.created_at).toLocaleDateString('es-ES')}
                          </span>
                        </div>
                        <p style={{ margin: 0, fontSize: '0.85rem', color: '#0f172a', whiteSpace: 'pre-wrap' }}>{n.content}</p>
                      </div>
                    ))}
                  </div>
                </div>
              )}

              {/* Documentos compartidos */}
              {e.documents.length > 0 && (
                <div>
                  <div style={{ fontSize: '0.72rem', fontWeight: 700, textTransform: 'uppercase', letterSpacing: '0.04em', color: '#94a3b8', marginBottom: 6 }}>Documentos</div>
                  <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
                    {e.documents.map((d) => (
                      <a
                        key={d.id}
                        href={`/api/me/employments/${e.employment.id}/documents/${d.id}/download`}
                        target="_blank"
                        rel="noopener noreferrer"
                        style={{ display: 'inline-flex', alignItems: 'center', gap: 6, fontSize: '0.85rem', color: '#0f172a', textDecoration: 'none', fontWeight: 600 }}
                      >
                        <FileText size={15} style={{ color: '#64748b' }} /> {d.title || d.file_name}
                      </a>
                    ))}
                  </div>
                </div>
              )}
            </div>
          )
        })}
      </div>
    </div>
  )
}

function CvStat({ icon, label, value }: { icon: React.ReactNode; label: string; value: string }) {
  return (
    <div style={{ border: '1px solid #e2e8f0', borderRadius: 10, padding: '10px 12px' }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 6, color: '#64748b', fontSize: '0.74rem', marginBottom: 4 }}>{icon}{label}</div>
      <div style={{ fontSize: '1.05rem', fontWeight: 800, color: '#0f172a' }}>{value}</div>
    </div>
  )
}
