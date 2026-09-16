import { useEffect, useMemo, useState } from 'react'
import {
  Briefcase,
  Paperclip,
  AlertCircle,
  Search,
  Check,
  FileText,
  Clock,
  X,
  History,
  Calendar,
} from 'lucide-react'
import api from '../../services/api'
import styles from './TenantSubscriptions.module.css'

// Pestaña «Suscripciones»: los procesos de reclutamiento de la empresa, leídos
// de Obersuite. Reflejo visual del pipeline de Obersuite adaptado a la estética
// de la plataforma. Solo lectura: sin botones de acción ni mutaciones.

type Stage = { name: string; index: number; color?: string }
type Attachment = { id: string; name: string; size?: number; mime?: string; url?: string }
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
  subscriptions?: Subscription[]
  stages?: Stage[]
}

function fmtDate(s?: string) {
  if (!s) return ''
  const d = new Date(s)
  return isNaN(d.getTime()) ? s : d.toLocaleDateString('es-ES', { day: '2-digit', month: 'numeric', year: 'numeric' })
}

function fmtDateTime(s?: string) {
  if (!s) return ''
  const d = new Date(s)
  return isNaN(d.getTime())
    ? s
    : d.toLocaleDateString('es-ES', {
      day: '2-digit',
      month: 'short',
      year: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    })
}

function fmtSize(n?: number) {
  if (!n) return ''
  if (n < 1024) return `${n} B`
  if (n < 1024 * 1024) return `${Math.round(n / 1024)} KB`
  return `${(n / (1024 * 1024)).toFixed(1)} MB`
}

export function TenantSubscriptions({ tenantId, companyName }: { tenantId: number; companyName?: string }) {
  const [payload, setPayload] = useState<Payload | null>(null)
  const [error, setError] = useState(false)
  const [view, setView] = useState<'activas' | 'archivadas'>('activas')
  const [query, setQuery] = useState('')
  const [modalSub, setModalSub] = useState<Subscription | null>(null)

  useEffect(() => {
    let cancelled = false
    setPayload(null)
    setError(false)
    api.get<Payload>(`/admin/tenants/${tenantId}/subscriptions`)
      .then((res: { data: Payload }) => { if (!cancelled) setPayload(res.data) })
      .catch(() => { if (!cancelled) setError(true) })
    return () => { cancelled = true }
  }, [tenantId])

  // Cerrar modal con tecla Escape
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setModalSub(null)
    }
    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [])

  const all: Subscription[] = payload?.data?.subscriptions ?? payload?.subscriptions ?? []
  const stages: Stage[] = payload?.data?.stages ?? payload?.stages ?? []
  const activas = all.filter(s => s.status === 'active')
  const archivadas = all.filter(s => s.status !== 'active')

  // Buscar por vacante o por fase
  const visibles = useMemo(() => {
    const base = view === 'activas' ? activas : archivadas
    const q = query.trim().toLowerCase()
    if (!q) return base
    return base.filter(s =>
      (s.position?.title || '').toLowerCase().includes(q) ||
      (s.stage?.name || '').toLowerCase().includes(q) ||
      (s.recruiter?.name || '').toLowerCase().includes(q),
    )
  }, [view, query, activas, archivadas])

  // El enlace se pide SIEMPRE por nuestro backend, que es quien tiene el token.
  // Se le pasa la URL que Obersuite publica en el adjunto: sus ids de ruta no
  // son el external_id de la suscripción, así que construirla aquí era
  // adivinar su formato (y su servidor contestaba 400). Si un adjunto viejo no
  // trae `url`, se cae al camino por partes.
  const attachmentUrl = (sid: string, a: Attachment) =>
    a.url
      ? `/api/admin/tenants/${tenantId}/subscription-attachment?url=${encodeURIComponent(a.url)}`
      : `/api/admin/tenants/${tenantId}/subscriptions/${encodeURIComponent(sid)}/attachments/${encodeURIComponent(a.id)}`

  return (
    <div className={styles.container}>
      {/* Banner de cabecera informativo */}
      <div className={styles.bannerCard}>
        <div className={styles.bannerIcon}>
          <Briefcase size={22} />
        </div>
        <div className={styles.bannerInfo}>
          <h2 className={styles.bannerTitle}>
            SUSCRIPCIONES Y PROCESOS DE RECLUTAMIENTO
            <span className={styles.badgeOrigin}>· DESDE OBERSUITE</span>
            {payload?.available && (
              <span className={styles.activeCountBadge}>
                {activas.length} {activas.length === 1 ? 'activa' : 'activas'}
              </span>
            )}
          </h2>
          <p className={styles.bannerSubtitle}>
            Vacantes vinculadas a {companyName || 'esta empresa'} con su seguimiento de fases en Obersuite. Se leen aquí; se gestionan allí.
          </p>
        </div>
      </div>

      {error && (
        <div className={styles.emptyState}>
          <AlertCircle size={40} color="#ef4444" />
          <p>No se pudieron cargar los procesos de reclutamiento.</p>
        </div>
      )}

      {!error && payload === null && (
        <div className={styles.emptyState}>
          <Clock size={36} color="#94a3b8" />
          <p>Cargando información desde Obersuite…</p>
        </div>
      )}

      {payload && !payload.available && (
        <div className={styles.emptyState}>
          <AlertCircle size={40} color="#f59e0b" />
          <p>
            {payload.reason === 'not_configured'
              ? 'La conexión con Obersuite no está configurada en este entorno.'
              : 'Obersuite no responde ahora mismo. Vuelve a intentarlo en un momento.'}
          </p>
        </div>
      )}

      {payload?.available && (
        <>
          {/* Barra de Filtros: Pestañas de estado y Buscador */}
          <div className={styles.controlsBar}>
            <div className={styles.tabPillGroup} role="tablist" aria-label="Estado de las suscripciones">
              <button
                type="button"
                className={`${styles.tabPill} ${view === 'activas' ? styles.tabPillActive : ''}`}
                onClick={() => setView('activas')}
              >
                Activas <span className={styles.tabCount}>({activas.length})</span>
              </button>
              <button
                type="button"
                className={`${styles.tabPill} ${view === 'archivadas' ? styles.tabPillActive : ''}`}
                onClick={() => setView('archivadas')}
              >
                Archivadas <span className={styles.tabCount}>({archivadas.length})</span>
              </button>
            </div>

            <div className={styles.searchWrapper}>
              <Search size={16} className={styles.searchIcon} />
              <input
                type="search"
                className={styles.searchInput}
                placeholder="Buscar vacante o fase…"
                value={query}
                onChange={e => setQuery(e.target.value)}
                aria-label="Buscar vacante o fase"
              />
            </div>
          </div>

          {/* Estado sin resultados */}
          {visibles.length === 0 && (
            <div className={styles.emptyState}>
              <Briefcase size={42} color="#cbd5e1" />
              <p>
                {query.trim()
                  ? 'Ningún proceso coincide con la búsqueda.'
                  : view === 'activas'
                    ? 'No hay vacantes activas vinculadas a esta empresa.'
                    : 'No hay procesos archivados.'}
              </p>
              {!query.trim() && view === 'activas' && (
                <span>Las vacantes se vinculan desde Obersuite; aquí aparecen en cuanto existan.</span>
              )}
            </div>
          )}

          {/* Listado de Tarjetas de Vacantes */}
          {visibles.map(s => {
            const currentStageIndex = s.stage?.index ?? 0
            const totalStages = stages.length || 5
            const currentStepNum = currentStageIndex + 1

            // KPIs
            const totalCands = s.candidates?.total ?? 0
            const interviewingCands = s.candidates?.interviewing ?? s.candidates?.in_progress ?? 0
            const hiredCands = s.candidates?.hired ?? 0

            // Documentos
            const attachCount = s.attachments?.length ?? 0

            // Metadata autor / fecha
            const author = s.recruiter?.name || s.created_by
            const dateStr = s.created_at || s.updated_at

            return (
              <div key={s.external_id} className={styles.vacancyCard}>
                {/* 1. Fila Superior: Título, Badges y Métricas */}
                <div className={styles.topRow}>
                  <div className={styles.titleSection}>
                    <div className={styles.titleWithBadges}>
                      <h3 className={styles.vacancyTitle}>
                        {s.position?.title || 'Vacante sin título'}
                      </h3>

                      <span
                        className={`${styles.statusBadge} ${s.status === 'active' ? styles.statusBadgeActive : styles.statusBadgeArchived
                          }`}
                      >
                        {s.status === 'active' ? 'OPEN' : 'ARCHIVED'}
                      </span>

                      {s.position?.type && (
                        <span className={styles.typeBadge}>
                          {s.position.type}
                          {s.position.location ? ` · ${s.position.location}` : ''}
                        </span>
                      )}

                      {attachCount > 0 && (
                        <span className={styles.docBadge} title={`${attachCount} documento(s) adjunto(s)`}>
                          <Paperclip size={12} /> {attachCount} {attachCount === 1 ? 'doc' : 'docs'}
                        </span>
                      )}
                    </div>

                    <p className={styles.metaSubtitle}>
                      {dateStr && <span>Vinculada el {fmtDate(dateStr)}</span>}
                      {author && (
                        <>
                          <span>·</span>
                          <span>Por <strong>{author}</strong></span>
                        </>
                      )}
                    </p>
                  </div>

                  {/* KPIs a la derecha */}
                  <div className={styles.kpisGroup}>
                    <div className={`${styles.kpiBox} ${styles.kpiBoxNeutral}`}>
                      <span className={styles.kpiNum}>{totalCands}</span>
                      <span className={styles.kpiText}>Postulados</span>
                    </div>

                    <div className={`${styles.kpiBox} ${styles.kpiBoxPurple}`}>
                      <span className={styles.kpiNum}>{interviewingCands}</span>
                      <span className={styles.kpiText}>Entrevistas</span>
                    </div>

                    <div className={`${styles.kpiBox} ${styles.kpiBoxGreen}`}>
                      <span className={styles.kpiNum}>{hiredCands}</span>
                      <span className={styles.kpiText}>Contratados</span>
                    </div>
                  </div>
                </div>

                {/* 2. Cabecera del Pipeline: Fase actual y Paso X de Y */}
                <div className={styles.stageHeaderRow}>
                  <div>
                    <span className={styles.currentStageLabel}>Fase actual: </span>
                    <span className={styles.currentStageName}>
                      {s.stage?.name || 'En proceso'}
                    </span>
                  </div>
                  <div className={styles.stepCounter}>
                    Paso {currentStepNum} de {totalStages}
                  </div>
                </div>

                {/* 3. Pipeline de 5 Fases */}
                {stages.length > 0 && (
                  <div className={styles.pipelineGrid}>
                    {stages.map(st => {
                      const isDone = st.index < currentStageIndex
                      const isCurrent = st.index === currentStageIndex
                      const isUpcoming = st.index > currentStageIndex
                      const stepNum = st.index + 1

                      return (
                        <div
                          key={st.index}
                          className={`${styles.phaseCard} ${isDone
                            ? styles.phaseCardDone
                            : isCurrent
                              ? styles.phaseCardCurrent
                              : styles.phaseCardUpcoming
                            }`}
                          title={`Fase ${stepNum}: ${st.name}`}
                        >
                          <div className={styles.phaseTop}>
                            {isDone && (
                              <div className={styles.checkCircle}>
                                <Check size={13} strokeWidth={3} />
                              </div>
                            )}

                            {isCurrent && (
                              <>
                                <div className={styles.currentCircle}>
                                  {stepNum}
                                </div>
                                <div className={styles.pulsingDot} />
                              </>
                            )}

                            {isUpcoming && (
                              <div className={styles.upcomingCircle}>
                                {stepNum}
                              </div>
                            )}
                          </div>

                          <p className={styles.phaseName}>
                            {st.name}
                          </p>
                        </div>
                      )
                    })}
                  </div>
                )}

                {/* 4. Footer con botón para abrir Modal de Historial y Notas */}
                <div className={styles.cardFooter}>
                  <div className={styles.recruiterNote}>
                    {s.recruiter?.name && (
                      <span>Reclutador asignado: <strong>{s.recruiter.name}</strong></span>
                    )}
                  </div>

                  <button
                    type="button"
                    className={styles.openModalBtn}
                    onClick={() => setModalSub(s)}
                    title="Ver historial, notas y adjuntos en un modal"
                  >
                    <History size={14} />
                    Ver historial y notas
                  </button>
                </div>
              </div>
            )
          })}
        </>
      )}

      {/* Modal de Historial y Notas con Scroll */}
      {modalSub && (
        <div
          className={styles.modalOverlay}
          onClick={(e) => {
            if (e.target === e.currentTarget) setModalSub(null)
          }}
          role="dialog"
          aria-modal="true"
          aria-labelledby="modal-title"
        >
          <div className={styles.modalContainer}>
            {/* Header del Modal */}
            <div className={styles.modalHeader}>
              <div className={styles.modalHeaderLeft}>
                <h3 id="modal-title" className={styles.modalTitle}>
                  <Briefcase size={18} color="#7c3aed" />
                  {modalSub.position?.title || 'Vacante'}
                </h3>
                <p className={styles.modalSubtitle}>
                  Fase actual: <strong>{modalSub.stage?.name || 'En proceso'}</strong>
                  {modalSub.status === 'active' ? ' · Estado: Activa' : ' · Estado: Archivada'}
                </p>
              </div>
              <button
                type="button"
                className={styles.modalCloseBtn}
                onClick={() => setModalSub(null)}
                aria-label="Cerrar ventana"
                title="Cerrar ventana"
              >
                <X size={18} />
              </button>
            </div>

            {/* Cuerpo del Modal con Scroll */}
            <div className={styles.modalBody}>
              {/* Sección Notas */}
              {modalSub.notes && (
                <div>
                  <div className={styles.modalSectionTitle}>
                    <FileText size={16} color="#7c3aed" />
                    Notas del Proceso
                  </div>
                  <div className={styles.notesBlock}>
                    {modalSub.notes}
                  </div>
                </div>
              )}

              {/* Sección Adjuntos */}
              {(modalSub.attachments?.length ?? 0) > 0 && (
                <div>
                  <div className={styles.modalSectionTitle}>
                    <Paperclip size={16} color="#7c3aed" />
                    Documentos y Adjuntos ({modalSub.attachments!.length})
                  </div>
                  <div className={styles.attachmentsGroup}>
                    {modalSub.attachments!.map((a) => (
                      <a
                        key={a.id}
                        href={attachmentUrl(modalSub.external_id, a)}
                        className={styles.attachmentPill}
                        target="_blank"
                        rel="noreferrer"
                        title="Descargar archivo desde Obersuite"
                      >
                        <Paperclip size={14} />
                        <span>{a.name}</span>
                        {a.size && (
                          <span style={{ color: '#94a3b8', fontSize: '0.75rem' }}>
                            ({fmtSize(a.size)})
                          </span>
                        )}
                      </a>
                    ))}
                  </div>
                </div>
              )}

              {/* Sección Historial de Movimientos */}
              <div>
                <div className={styles.modalSectionTitle}>
                  <History size={16} color="#7c3aed" />
                  Historial de Movimientos y Fases
                </div>

                {(modalSub.history?.length ?? 0) > 0 ? (
                  <ul className={styles.historyList}>
                    {modalSub.history!.map((h, i) => (
                      <li key={i} className={styles.historyItem}>
                        <div className={styles.historyItemIcon}>
                          <Calendar size={14} />
                        </div>
                        <div className={styles.historyItemContent}>
                          <div className={styles.historyItemHeader}>
                            <div className={styles.historyItemPhase}>
                              {h.from ? (
                                <>
                                  <span style={{ color: '#64748b' }}>{h.from}</span>
                                  {' → '}
                                  <strong style={{ color: '#0f172a' }}>{h.to}</strong>
                                </>
                              ) : (
                                <>
                                  Inicio en <strong style={{ color: '#0f172a' }}>{h.to}</strong>
                                </>
                              )}
                            </div>
                            <span className={styles.historyItemDate}>{fmtDateTime(h.at)}</span>
                          </div>

                          {h.by && (
                            <div className={styles.historyItemAuthor}>
                              Registrado por: <strong>{h.by}</strong>
                            </div>
                          )}

                          {h.notes && (
                            <div className={styles.historyItemNotes}>
                              "{h.notes}"
                            </div>
                          )}
                        </div>
                      </li>
                    ))}
                  </ul>
                ) : (
                  <div className={styles.emptyBlock}>
                    No hay registros de cambios de fase previos en Obersuite.
                  </div>
                )}
              </div>
            </div>

            {/* Footer del Modal */}
            <div className={styles.modalFooter}>
              <button
                type="button"
                className={styles.closeButtonAction}
                onClick={() => setModalSub(null)}
              >
                Cerrar
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
