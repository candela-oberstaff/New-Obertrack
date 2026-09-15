import { useEffect, useState } from 'react'
import { Briefcase, Paperclip, Users, ChevronDown, ChevronUp, AlertCircle } from 'lucide-react'
import api from '../../services/api'
import styles from './Tenants.module.css'

// Los procesos de reclutamiento de la empresa, leídos de Obersuite. Es un espejo
// de SU pestaña "Suscripciones": de solo lectura aquí, a propósito. Si algún día
// Customer Success necesita mover una fase desde este lado, es un endpoint de
// escritura de ellos por definir; no se asume.
//
// El bloque nunca rompe la ficha: si Obersuite no está configurado o no
// responde, dice por qué y la pantalla sigue.

type Stage = { name: string; index: number; color?: string }
type Attachment = { id: string; name: string; size?: number; mime?: string }
type HistoryEntry = { from: string | null; to: string; notes: string | null; by: string; at: string; attachments?: Attachment[] }
type Subscription = {
  external_id: string
  position: { title: string; status?: string; type?: string; location?: string }
  stage: Stage
  status: 'active' | 'archived' | string
  candidates?: { total?: number; in_progress?: number; interviewing?: number; hired?: number }
  recruiter?: { name: string; email?: string } | null
  notes?: string
  created_by?: string
  created_at?: string
  updated_at?: string
  attachments?: Attachment[]
  history?: HistoryEntry[]
}
type Payload = {
  available: boolean
  reason?: 'not_configured' | 'unreachable'
  data?: { subscriptions: Subscription[]; stages: Stage[]; counts?: { active?: number; archived?: number } }
  // Forma del 404 (empresa desconocida para Obersuite): lista vacía en la raíz.
  subscriptions?: Subscription[]
  stages?: Stage[]
}

const STAGE_COLORS: Record<string, string> = {
  blue: '#2563eb', purple: '#7c3aed', amber: '#b45309', orange: '#c2410c', green: '#059669', red: '#dc2626', gray: '#64748b',
}

function fmtDate(s?: string) {
  if (!s) return ''
  const d = new Date(s)
  return isNaN(d.getTime()) ? s : d.toLocaleDateString('es-ES', { day: '2-digit', month: 'short', year: 'numeric' })
}

function fmtSize(n?: number) {
  if (!n) return ''
  if (n < 1024) return `${n} B`
  if (n < 1024 * 1024) return `${Math.round(n / 1024)} KB`
  return `${(n / (1024 * 1024)).toFixed(1)} MB`
}

export function TenantSubscriptions({ tenantId }: { tenantId: number }) {
  const [payload, setPayload] = useState<Payload | null>(null)
  const [error, setError] = useState(false)
  const [open, setOpen] = useState<Record<string, boolean>>({})

  useEffect(() => {
    let cancelled = false
    setPayload(null); setError(false)
    api.get<Payload>(`/admin/tenants/${tenantId}/subscriptions`)
      .then((res: { data: Payload }) => { if (!cancelled) setPayload(res.data) })
      .catch(() => { if (!cancelled) setError(true) })
    return () => { cancelled = true }
  }, [tenantId])

  const subs: Subscription[] = payload?.data?.subscriptions ?? payload?.subscriptions ?? []
  const stages: Stage[] = payload?.data?.stages ?? payload?.stages ?? []
  const active = subs.filter(s => s.status === 'active').length

  const attachmentUrl = (sid: string, aid: string) =>
    `/api/admin/tenants/${tenantId}/subscriptions/${encodeURIComponent(sid)}/attachments/${encodeURIComponent(aid)}`

  return (
    <div className={styles.sidebarCard} style={{ margin: 0, marginTop: 16 }}>
      <h2 className={styles.sidebarCardTitle} style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
        <Briefcase size={16} style={{ color: '#b45309' }} /> Procesos de reclutamiento
        <span style={{ fontWeight: 500, color: '#94a3b8', fontSize: '0.78rem' }}>· desde Obersuite</span>
        {payload?.available && subs.length > 0 && (
          <span className={styles.pill} style={{ marginLeft: 'auto', background: '#fef3c7', color: '#b45309' }}>
            {active} {active === 1 ? 'activo' : 'activos'}
          </span>
        )}
      </h2>

      {error && <p style={{ color: '#64748b', fontSize: '0.9rem' }}>No se pudieron cargar los procesos.</p>}
      {!error && payload === null && <p style={{ color: '#94a3b8', fontSize: '0.9rem' }}>Cargando…</p>}
      {payload && !payload.available && (
        <p style={{ color: '#64748b', fontSize: '0.9rem', display: 'flex', gap: 6, alignItems: 'center' }}>
          <AlertCircle size={14} />
          {payload.reason === 'not_configured'
            ? 'La conexión con Obersuite no está configurada en este entorno.'
            : 'Obersuite no responde ahora mismo. Vuelve a intentarlo en un momento.'}
        </p>
      )}
      {payload?.available && subs.length === 0 && (
        <p style={{ color: '#94a3b8', fontSize: '0.9rem' }}>Esta empresa no tiene procesos de reclutamiento en Obersuite.</p>
      )}

      {subs.map(s => {
        const isOpen = !!open[s.external_id]
        const stageColor = STAGE_COLORS[s.stage?.color || ''] || '#7c3aed'
        return (
          <div key={s.external_id} className={styles.timelineCard} style={{ marginTop: 12, cursor: 'default' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap' }}>
              <strong style={{ fontSize: '0.98rem' }}>{s.position?.title || 'Vacante sin título'}</strong>
              <span
                className={styles.pill}
                style={s.status === 'active' ? { background: '#dcfce7', color: '#166534' } : { background: '#f1f5f9', color: '#475569' }}
              >
                {s.status === 'active' ? 'Activo' : s.status === 'archived' ? 'Archivado' : s.status}
              </span>
              {s.position?.type && <span style={{ color: '#64748b', fontSize: '0.8rem' }}>{s.position.type}{s.position.location ? ` · ${s.position.location}` : ''}</span>}
              <button
                type="button"
                className={styles.iconBtn}
                style={{ marginLeft: 'auto' }}
                onClick={() => setOpen(o => ({ ...o, [s.external_id]: !isOpen }))}
                title={isOpen ? 'Ocultar detalle' : 'Ver historial y adjuntos'}
                aria-label={isOpen ? 'Ocultar detalle' : 'Ver historial y adjuntos'}
              >
                {isOpen ? <ChevronUp size={16} /> : <ChevronDown size={16} />}
              </button>
            </div>

            {/* El pipeline entero, con la fase actual marcada. stage.index es la
                posición en el catálogo que manda Obersuite, no un número nuestro. */}
            {stages.length > 0 && (
              <ol style={{ display: 'flex', gap: 4, listStyle: 'none', padding: 0, margin: '10px 0 6px', flexWrap: 'wrap' }}>
                {stages.map(st => {
                  const done = st.index < (s.stage?.index ?? -1)
                  const current = st.index === s.stage?.index
                  return (
                    <li
                      key={st.index}
                      title={st.name}
                      style={{
                        flex: '1 1 0', minWidth: 90, padding: '4px 8px', borderRadius: 6, fontSize: '0.74rem', textAlign: 'center',
                        background: current ? stageColor : done ? '#e2e8f0' : '#f8fafc',
                        color: current ? '#fff' : done ? '#334155' : '#94a3b8',
                        fontWeight: current ? 700 : 500,
                        border: current ? 'none' : '1px solid #e2e8f0',
                        whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis',
                      }}
                    >
                      {st.name}
                    </li>
                  )
                })}
              </ol>
            )}

            <div style={{ display: 'flex', gap: 14, flexWrap: 'wrap', color: '#64748b', fontSize: '0.82rem', marginTop: 4 }}>
              {s.candidates && (
                <span style={{ display: 'inline-flex', alignItems: 'center', gap: 4 }}>
                  <Users size={13} /> {s.candidates.total ?? 0} candidatos
                  {typeof s.candidates.hired === 'number' && s.candidates.hired > 0 && ` · ${s.candidates.hired} contratados`}
                </span>
              )}
              {s.recruiter?.name && <span>Reclutador: <strong style={{ color: '#334155' }}>{s.recruiter.name}</strong></span>}
              {s.updated_at && <span>Actualizado {fmtDate(s.updated_at)}</span>}
            </div>

            {isOpen && (
              <div style={{ marginTop: 10, borderTop: '1px solid #e2e8f0', paddingTop: 10 }}>
                {s.notes && <p className={styles.noteBody} style={{ marginBottom: 10 }}>{s.notes}</p>}

                {(s.attachments?.length ?? 0) > 0 && (
                  <div style={{ marginBottom: 10 }}>
                    {s.attachments!.map(a => (
                      <a
                        key={a.id}
                        href={attachmentUrl(s.external_id, a.id)}
                        style={{ display: 'inline-flex', alignItems: 'center', gap: 6, fontSize: '0.84rem', marginRight: 12, color: 'var(--primary, #cc33cc)' }}
                        title="Se descarga a través de Obertrack; el archivo vive en Obersuite"
                      >
                        <Paperclip size={13} /> {a.name}{a.size ? <span style={{ color: '#94a3b8' }}> ({fmtSize(a.size)})</span> : null}
                      </a>
                    ))}
                  </div>
                )}

                {(s.history?.length ?? 0) > 0 && (
                  <ul style={{ listStyle: 'none', padding: 0, margin: 0, fontSize: '0.82rem', color: '#475569' }}>
                    {s.history!.map((h, i) => (
                      <li key={i} style={{ padding: '4px 0', borderBottom: i < s.history!.length - 1 ? '1px dashed #e2e8f0' : 'none' }}>
                        <span style={{ color: '#94a3b8' }}>{fmtDate(h.at)}</span>
                        {' · '}
                        {h.from ? <>{h.from} → <strong style={{ color: '#334155' }}>{h.to}</strong></> : <>Inicio: <strong style={{ color: '#334155' }}>{h.to}</strong></>}
                        {h.by && <span style={{ color: '#94a3b8' }}> · {h.by}</span>}
                        {h.notes && <div style={{ color: '#64748b', marginTop: 2 }}>{h.notes}</div>}
                      </li>
                    ))}
                  </ul>
                )}

                {s.created_by && (
                  <p style={{ color: '#94a3b8', fontSize: '0.78rem', marginTop: 8, marginBottom: 0 }}>
                    Creado por {s.created_by}{s.created_at ? ` el ${fmtDate(s.created_at)}` : ''}
                  </p>
                )}
              </div>
            )}
          </div>
        )
      })}
    </div>
  )
}
