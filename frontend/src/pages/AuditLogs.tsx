import { useCallback, useEffect, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useNavigate } from 'react-router-dom'
import { Shield, Download, Search, ChevronLeft, ChevronRight, Filter, X, Cpu, CheckCircle2, XCircle } from 'lucide-react'
import { AuditLog, AuditLogParams, auditService } from '../services/audit.service'
import AuditDetailModal from '../components/Audit/AuditDetailModal'
import { describeAudit, moduleLabel } from '../lib/auditHumanize'

// Maps an audited entity (table/module + id) to the app page where it lives.
// Returns null when there is no dedicated source page.
function entityRoute(entityType?: string, entityId?: string): string | null {
  if (!entityType) return null
  const t = entityType.replace(/-/g, '_')
  const id = entityId || ''
  switch (t) {
    case 'users': return id ? `/admin/users/${id}` : '/admin'
    case 'work_hours': return '/work-hours'
    case 'tasks':
    case 'boards': return '/tasks'
    case 'channels':
    case 'channel_messages':
    case 'messages': return '/chat'
    case 'tutorials': return '/tutoriales'
    case 'surveys': return id ? `/survey/${id}` : null
    case 'tickets': return id && /^\d+$/.test(id) ? `/tickets/internal/${id}` : '/tickets'
    case 'auth': return null
    default: return null
  }
}

const MODULES = ['auth', 'users', 'admin', 'work-hours', 'tickets', 'tasks', 'boards', 'channels', 'email', 'surveys', 'tutorials', 'notifications']

function initials(email: string): string {
  const base = (email || '?').split('@')[0]
  return base.slice(0, 2).toUpperCase()
}

function relativeTime(iso: string): string {
  const d = new Date(iso)
  const diff = (Date.now() - d.getTime()) / 1000
  if (diff < 60) return 'hace un momento'
  if (diff < 3600) return `hace ${Math.floor(diff / 60)} min`
  if (diff < 86400) return `hace ${Math.floor(diff / 3600)} h`
  if (diff < 604800) return `hace ${Math.floor(diff / 86400)} d`
  return d.toLocaleDateString('es-ES')
}

export default function AuditLogs() {
  const navigate = useNavigate()
  const limit = 25

  const [q, setQ] = useState('')
  const [module, setModule] = useState('')
  const [kind, setKind] = useState('')
  const [success, setSuccess] = useState('')
  const [startDate, setStartDate] = useState('')
  const [endDate, setEndDate] = useState('')
  const [entity, setEntity] = useState<{ type: string; id: string } | null>(null)
  const [selectedLog, setSelectedLog] = useState<AuditLog | null>(null)

  const buildParams = useCallback((p: number): AuditLogParams => ({
    page: p, limit,
    q: q || undefined,
    module: module || undefined,
    kind: kind || undefined,
    success: success || undefined,
    start_date: startDate || undefined,
    end_date: endDate || undefined,
    entity_type: entity?.type || undefined,
    entity_id: entity?.id || undefined,
  }), [q, module, kind, success, startDate, endDate, entity])

  // Applied params drive the query. Filters apply on demand (Filtrar/Enter),
  // not on every keystroke — so we snapshot them into `params`.
  const [params, setParams] = useState<AuditLogParams>(() => ({ page: 1, limit }))

  const { data, isLoading: loading, error: queryError } = useQuery({
    queryKey: ['audit-logs', params],
    queryFn: () => auditService.getAuditLogs(params),
    placeholderData: (prev) => prev, // keep previous page visible while loading the next
  })

  const logs: AuditLog[] = data?.data ?? []
  const total = data?.total ?? 0
  const page = data?.page ?? params.page ?? 1
  const error = queryError ? ((queryError as any)?.response?.data?.error ?? 'No se pudo cargar la auditoría.') : null

  // Apply current filters (resets to page 1) or jump to a specific page.
  const fetchLogs = useCallback((p: number) => { setParams(buildParams(p)) }, [buildParams])

  // Re-apply (page 1) whenever the entity-trace filter is set or cleared.
  useEffect(() => { setParams(buildParams(1)) // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [entity])

  const totalPages = Math.max(1, Math.ceil(total / limit))

  const handleExport = async () => {
    const res = await auditService.getAuditLogs({ ...buildParams(1), limit: 1000 })
    const headers = ['Fecha', 'Quién', 'Qué pasó', 'Área', 'Resultado', 'Origen', 'Acción (técnica)', 'Ruta', 'IP']
    const rows = (res.data ?? []).map(l => [
      new Date(l.created_at).toLocaleString('es-ES'),
      l.kind === 'data' ? 'Sistema (automático)' : `${l.actor_email || '—'}${l.actor_role ? ' (' + l.actor_role + ')' : ''}`,
      describeAudit(l),
      moduleLabel(l.module),
      l.success ? 'Correcto' : 'Error',
      l.kind === 'data' ? 'Cambio automático' : 'Acción de usuario',
      l.action, l.path, l.ip,
    ])
    const csv = '﻿' + [headers.join('\t'), ...rows.map(r => r.map(v => `"${String(v).replace(/"/g, '""')}"`).join('\t'))].join('\n')
    const blob = new Blob([csv], { type: 'application/vnd.ms-excel;charset=utf-8;' })
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.setAttribute('href', url)
    link.setAttribute('download', `Auditoria_${new Date().toISOString().slice(0, 10)}.xls`)
    document.body.appendChild(link); link.click(); document.body.removeChild(link)
  }

  const card: React.CSSProperties = {
    background: 'var(--glass-bg, #fff)', border: '1px solid var(--glass-border, #e2e8f0)',
    borderRadius: 'var(--radius, 16px)', boxShadow: 'var(--shadow-sm, 0 1px 3px rgba(0,0,0,0.06))',
  }
  const inputStyle: React.CSSProperties = {
    padding: '0.5rem 0.7rem', borderRadius: '10px', border: '1px solid var(--glass-border, #cbd5e1)',
    fontSize: '0.85rem', background: 'var(--bg-primary, #fff)',
  }
  const badge = (c: { bg: string; fg: string }): React.CSSProperties => ({
    background: c.bg, color: c.fg, fontWeight: 700, fontSize: '0.72rem',
    padding: '3px 9px', borderRadius: '99px', whiteSpace: 'nowrap', display: 'inline-block',
  })

  return (
    <div style={{ padding: '1.5rem', display: 'flex', flexDirection: 'column', gap: '1rem' }}>
      <style>{`
        .audit-row { transition: background 0.12s; }
        .audit-row:hover { background: rgba(99,102,241,0.05); }
        @keyframes auditShimmer { 0% { opacity: .5 } 50% { opacity: 1 } 100% { opacity: .5 } }
        .audit-skel { animation: auditShimmer 1.2s ease-in-out infinite; }
      `}</style>

      {/* Header */}
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', flexWrap: 'wrap', gap: '0.75rem' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: '0.7rem' }}>
          <span style={{ width: 42, height: 42, borderRadius: '12px', display: 'flex', alignItems: 'center', justifyContent: 'center', background: 'linear-gradient(135deg, #6366f1, #8b5cf6)', color: '#fff' }}>
            <Shield size={22} />
          </span>
          <div>
            <h1 style={{ margin: 0, fontSize: '1.4rem' }}>Auditoría</h1>
            <div style={{ fontSize: '0.8rem', color: 'var(--text-secondary)' }}>{total} eventos registrados</div>
          </div>
        </div>
        <button onClick={handleExport} disabled={total === 0}
          style={{ display: 'inline-flex', alignItems: 'center', gap: '0.4rem', padding: '0.6rem 1.1rem', borderRadius: '12px', border: '1px solid var(--glass-border, #cbd5e1)', background: 'var(--bg-primary, #fff)', fontWeight: 600, cursor: 'pointer', fontSize: '0.85rem', opacity: total === 0 ? 0.5 : 1 }}>
          <Download size={15} /> Exportar Excel
        </button>
      </div>

      {/* Filtros */}
      <div style={{ ...card, padding: '0.85rem', display: 'flex', flexWrap: 'wrap', gap: '0.5rem', alignItems: 'center' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: '0.4rem', flex: '1 1 240px', ...inputStyle, padding: '0 0.6rem' }}>
          <Search size={15} style={{ color: 'var(--text-secondary)', flexShrink: 0 }} />
          <input value={q} onChange={e => setQ(e.target.value)} onKeyDown={e => { if (e.key === 'Enter') fetchLogs(1) }}
            placeholder="Buscar email, acción o ruta…" style={{ flex: 1, border: 'none', outline: 'none', background: 'transparent', padding: '0.5rem 0', fontSize: '0.85rem' }} />
        </div>
        <select value={kind} onChange={e => setKind(e.target.value)} style={inputStyle}>
          <option value="">Actividad y datos</option>
          <option value="activity">Actividad (acciones)</option>
          <option value="data">Datos (cambios en BD)</option>
        </select>
        <select value={module} onChange={e => setModule(e.target.value)} style={inputStyle}>
          <option value="">Todas las áreas</option>
          {MODULES.map(m => <option key={m} value={m}>{moduleLabel(m)}</option>)}
        </select>
        <select value={success} onChange={e => setSuccess(e.target.value)} style={inputStyle}>
          <option value="">Éxito y fallo</option>
          <option value="true">Solo éxito</option>
          <option value="false">Solo fallo</option>
        </select>
        <input type="date" value={startDate} onChange={e => setStartDate(e.target.value)} style={inputStyle} />
        <input type="date" value={endDate} onChange={e => setEndDate(e.target.value)} style={inputStyle} />
        <button onClick={() => fetchLogs(1)}
          style={{ display: 'inline-flex', alignItems: 'center', gap: '0.4rem', padding: '0.55rem 1.1rem', borderRadius: '10px', border: 'none', background: 'var(--primary, #cc33cc)', color: '#fff', fontWeight: 600, cursor: 'pointer', fontSize: '0.85rem' }}>
          <Filter size={14} /> Filtrar
        </button>
      </div>

      {entity && (
        <div style={{ display: 'inline-flex', alignItems: 'center', gap: '0.5rem', alignSelf: 'flex-start', padding: '0.4rem 0.75rem', borderRadius: '99px', background: 'rgba(99,102,241,0.1)', color: '#4f46e5', fontSize: '0.82rem', fontWeight: 600 }}>
          Registro de entidad: {entity.type}{entity.id ? ` #${entity.id}` : ''}
          <button onClick={() => setEntity(null)} style={{ background: 'none', border: 'none', cursor: 'pointer', color: '#4f46e5', display: 'inline-flex' }}><X size={14} /></button>
        </div>
      )}

      {error && <div style={{ color: '#dc2626', fontSize: '0.9rem' }}>{error}</div>}

      {/* Tabla */}
      <div style={{ ...card, overflow: 'hidden' }}>
        <div style={{ overflowX: 'auto' }}>
          <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '0.82rem' }}>
            <thead>
              <tr style={{ background: 'var(--bg-secondary, #f8fafc)', textAlign: 'left' }}>
                {['Cuándo', 'Quién', 'Qué pasó', 'Área', 'Resultado'].map(h => (
                  <th key={h} style={{ padding: '0.7rem 0.9rem', fontWeight: 700, color: 'var(--text-secondary)', whiteSpace: 'nowrap', fontSize: '0.72rem', textTransform: 'uppercase', letterSpacing: '0.03em' }}>{h}</th>
                ))}
              </tr>
            </thead>
            <tbody>
              {loading ? (
                Array.from({ length: 6 }).map((_, i) => (
                  <tr key={i} style={{ borderTop: '1px solid var(--glass-border, #f1f5f9)' }}>
                    <td colSpan={5} style={{ padding: '0.85rem 0.9rem' }}>
                      <div className="audit-skel" style={{ height: 14, borderRadius: 6, background: 'var(--bg-secondary, #eef2f7)' }} />
                    </td>
                  </tr>
                ))
              ) : logs.length === 0 ? (
                <tr><td colSpan={5} style={{ textAlign: 'center', padding: '3.5rem', color: 'var(--gray-400)' }}>
                  <Shield size={34} style={{ opacity: 0.4 }} />
                  <div style={{ marginTop: '0.5rem' }}>No hay registros para estos filtros.</div>
                </td></tr>
              ) : logs.map(l => {
                const isSystem = l.kind === 'data'
                return (
                <tr key={l.id} className="audit-row" onClick={() => setSelectedLog(l)} style={{ borderTop: '1px solid var(--glass-border, #f1f5f9)', cursor: 'pointer' }}>
                  <td style={{ padding: '0.8rem 0.9rem', whiteSpace: 'nowrap' }} title={new Date(l.created_at).toLocaleString('es-ES')}>
                    {relativeTime(l.created_at)}
                  </td>
                  <td style={{ padding: '0.8rem 0.9rem' }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
                      <span style={{ width: 28, height: 28, borderRadius: '50%', flexShrink: 0, color: '#fff', fontSize: '0.7rem', fontWeight: 700, display: 'flex', alignItems: 'center', justifyContent: 'center', background: isSystem ? 'var(--gray-400, #94a3b8)' : 'linear-gradient(135deg,#6366f1,#8b5cf6)' }}>
                        {isSystem ? <Cpu size={14} /> : initials(l.actor_email)}
                      </span>
                      <div>
                        <div style={{ fontWeight: 600 }}>{isSystem ? 'Sistema' : (l.actor_email || '—')}</div>
                        <div style={{ fontSize: '0.7rem', color: 'var(--gray-400)' }}>{isSystem ? 'automático' : (l.actor_role || '')}</div>
                      </div>
                    </div>
                  </td>
                  <td style={{ padding: '0.8rem 0.9rem', fontWeight: 600, color: 'var(--text-primary)' }}>{describeAudit(l)}</td>
                  <td style={{ padding: '0.8rem 0.9rem' }}>
                    <span style={badge({ bg: 'rgba(99,102,241,0.1)', fg: '#4f46e5' })}>{moduleLabel(l.module)}</span>
                  </td>
                  <td style={{ padding: '0.8rem 0.9rem' }}>
                    {l.success ? (
                      <span style={{ display: 'inline-flex', alignItems: 'center', gap: '0.3rem', ...badge({ bg: 'rgba(16,185,129,0.12)', fg: '#059669' }) }}><CheckCircle2 size={13} /> Correcto</span>
                    ) : (
                      <span style={{ display: 'inline-flex', alignItems: 'center', gap: '0.3rem', ...badge({ bg: 'rgba(239,68,68,0.12)', fg: '#dc2626' }) }}><XCircle size={13} /> Error</span>
                    )}
                  </td>
                </tr>
                )
              })}
            </tbody>
          </table>
        </div>

        {/* Paginación */}
        {total > limit && (
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', padding: '0.75rem 1rem', borderTop: '1px solid var(--glass-border, #f1f5f9)' }}>
            <span style={{ fontSize: '0.8rem', color: 'var(--text-secondary)' }}>Página {page} de {totalPages}</span>
            <div style={{ display: 'flex', gap: '0.5rem' }}>
              <button onClick={() => page > 1 && fetchLogs(page - 1)} disabled={page <= 1}
                style={{ display: 'inline-flex', alignItems: 'center', gap: '0.3rem', padding: '0.45rem 0.85rem', borderRadius: '8px', border: '1px solid var(--glass-border, #cbd5e1)', background: 'var(--bg-primary, #fff)', cursor: 'pointer', fontSize: '0.82rem', opacity: page <= 1 ? 0.5 : 1 }}>
                <ChevronLeft size={15} /> Anterior
              </button>
              <button onClick={() => page < totalPages && fetchLogs(page + 1)} disabled={page >= totalPages}
                style={{ display: 'inline-flex', alignItems: 'center', gap: '0.3rem', padding: '0.45rem 0.85rem', borderRadius: '8px', border: '1px solid var(--glass-border, #cbd5e1)', background: 'var(--bg-primary, #fff)', cursor: 'pointer', fontSize: '0.82rem', opacity: page >= totalPages ? 0.5 : 1 }}>
                Siguiente <ChevronRight size={15} />
              </button>
            </div>
          </div>
        )}
      </div>

      {selectedLog && (
        <AuditDetailModal
          log={selectedLog}
          onClose={() => setSelectedLog(null)}
          onViewEntity={(type, id) => { setEntity({ type, id }); setSelectedLog(null) }}
          onGoToSource={(() => {
            const route = entityRoute(selectedLog.entity_type, selectedLog.entity_id)
            return route ? () => navigate(route) : undefined
          })()}
        />
      )}
    </div>
  )
}
