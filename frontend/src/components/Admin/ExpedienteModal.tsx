import { useState, useEffect, useCallback, useRef } from 'react'
import { FileText, Star, Trash2, Upload, Lock, Eye, Clock3, CheckSquare, CalendarDays, CalendarX, ClipboardList, Send, Snowflake, Download, Pencil, CalendarClock } from 'lucide-react'
import { adminService, authService, employerService, uploadService } from '../../services/api'
import { Modal, Button } from '../ui'

interface ExpedienteModalProps {
  userId: number
  employment: any
  canManage: boolean
  onClose: () => void
  // Modo profesional: solo lectura y vía endpoint propio (/me). El profesional
  // únicamente ve las entradas que la empresa marcó como compartidas.
  selfMode?: boolean
  // Modo empleador: gestiona el expediente de SU profesional vía /employer.
  employerMode?: boolean
}

interface ExpedienteData {
  employment: any
  summary: {
    days_employed: number
    total_hours: number
    approved_hours: number
    pending_hours: number
    tasks_assigned: number
    tasks_completed: number
    absences: number
    frozen_at?: string | null
  }
  notes: any[]
  documents: any[]
  absences: { date: string; reason: string; hours: number; approved: boolean }[]
  gestiones: { kind: string; status: string; note: string; by_name: string; created_at: string }[]
  contactos: { channel: string; by_name: string; note?: string; created_at: string }[]
}

const CONTACT_CHANNEL: Record<string, { label: string; icon: string }> = {
  email: { label: 'Email', icon: '✉️' },
  whatsapp: { label: 'WhatsApp', icon: '🟢' },
  chat: { label: 'Chat interno', icon: '💬' },
}

const GESTION_KIND: Record<string, string> = { inactivity: 'Inactividad', absence: 'Ausencia' }
const GESTION_STATUS: Record<string, { label: string; bg: string; color: string }> = {
  contacted: { label: '📞 Contactado', bg: 'rgba(59,130,246,0.12)', color: '#1d4ed8' },
  justified: { label: '✅ Justificado', bg: 'rgba(16,185,129,0.12)', color: '#047857' },
  escalated: { label: '⚠️ Escalado', bg: 'rgba(245,158,11,0.14)', color: '#b45309' },
}

const fmtHours = (n: number) => (Math.round((n || 0) * 10) / 10).toLocaleString('es-ES')

// Expediente laboral de un empleo: resumen (en vivo o congelado al salir),
// evaluaciones/notas y documentos. La empresa (RR.HH.) ve y gestiona todo;
// cada entrada puede marcarse como compartida para que el profesional la vea.
export function ExpedienteModal({ userId, employment, canManage, onClose, selfMode = false, employerMode = false }: ExpedienteModalProps) {
  const empId = employment.id
  // El profesional solo lee; el empleador gestiona a su gente; el admin todo.
  const manage = employerMode || (canManage && !selfMode)

  // Despacho de API según el modo: admin (/admin), empleador (/employer) o
  // profesional (/me, solo lectura).
  const apiSvc = {
    get: () =>
      selfMode ? authService.myExpediente(empId)
        : employerMode ? employerService.getExpediente(empId)
          : adminService.getExpediente(userId, empId),
    addNote: (p: any) => employerMode ? employerService.addNote(empId, p) : adminService.addExpedienteNote(userId, empId, p),
    updateNote: (id: number, p: any) => employerMode ? employerService.updateNote(empId, id, p) : adminService.updateExpedienteNote(userId, empId, id, p),
    deleteNote: (id: number) => employerMode ? employerService.deleteNote(empId, id) : adminService.deleteExpedienteNote(userId, empId, id),
    addDoc: (p: any) => employerMode ? employerService.addDocument(empId, p) : adminService.addExpedienteDocument(userId, empId, p),
    updateDoc: (id: number, p: any) => employerMode ? employerService.updateDocument(empId, id, p) : adminService.updateExpedienteDocument(userId, empId, id, p),
    deleteDoc: (id: number) => employerMode ? employerService.deleteDocument(empId, id) : adminService.deleteExpedienteDocument(userId, empId, id),
  }

  // Descarga autorizada: el backend valida visibilidad/propiedad (no exponemos
  // el archivo crudo). admin -> /admin, empleador -> /employer, profesional -> /me.
  const docHref = (docId: number) =>
    selfMode
      ? `/api/me/employments/${empId}/documents/${docId}/download`
      : employerMode
        ? `/api/employer/employments/${empId}/documents/${docId}/download`
        : `/api/admin/users/${userId}/employments/${empId}/documents/${docId}/download`
  const pdfHref = employerMode
    ? `/api/employer/employments/${empId}/expediente/pdf`
    : `/api/admin/users/${userId}/employments/${empId}/expediente/pdf`
  const [data, setData] = useState<ExpedienteData | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  // Form de nota / evaluación (también sirve para editar una existente)
  const [kind, setKind] = useState<'note' | 'evaluation'>('note')
  const [content, setContent] = useState('')
  const [rating, setRating] = useState<number>(0)
  const [shared, setShared] = useState(false)
  const [editingNoteId, setEditingNoteId] = useState<number | null>(null)

  // Subida de documento
  const fileRef = useRef<HTMLInputElement>(null)
  const [docTitle, setDocTitle] = useState('')
  const [docShared, setDocShared] = useState(false)
  const [docExpiry, setDocExpiry] = useState('')

  // Edición de un documento existente (metadatos)
  const [editingDocId, setEditingDocId] = useState<number | null>(null)
  const [editDocTitle, setEditDocTitle] = useState('')
  const [editDocShared, setEditDocShared] = useState(false)
  const [editDocExpiry, setEditDocExpiry] = useState('')

  const load = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const exp = await apiSvc.get()
      setData(exp)
    } catch {
      setError('No se pudo cargar el expediente')
    } finally {
      setLoading(false)
    }
  }, [userId, employment.id, selfMode])

  useEffect(() => { load() }, [load])

  const resetNoteForm = () => { setEditingNoteId(null); setContent(''); setRating(0); setShared(false); setKind('note') }

  const startEditNote = (n: any) => {
    setEditingNoteId(n.id)
    setKind(n.kind === 'evaluation' ? 'evaluation' : 'note')
    setContent(n.content || '')
    setRating(n.rating || 0)
    setShared(n.visibility === 'shared')
  }

  const saveNote = async () => {
    if (!content.trim()) return
    setBusy(true); setError(null)
    try {
      const payload = {
        kind,
        content: content.trim(),
        rating: kind === 'evaluation' && rating > 0 ? rating : null,
        visibility: (shared ? 'shared' : 'private') as 'shared' | 'private',
      }
      if (editingNoteId) {
        await apiSvc.updateNote(editingNoteId, payload)
      } else {
        await apiSvc.addNote(payload)
      }
      resetNoteForm()
      await load()
    } catch (e: any) {
      setError(e?.response?.data?.error || 'No se pudo guardar la nota')
    } finally {
      setBusy(false)
    }
  }

  const removeNote = async (noteId: number) => {
    setBusy(true)
    try { await apiSvc.deleteNote(noteId); await load() }
    catch { setError('No se pudo eliminar la nota') }
    finally { setBusy(false) }
  }

  const onPickFile = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return
    setBusy(true); setError(null)
    try {
      const up = await uploadService.upload(file)
      await apiSvc.addDoc({
        title: docTitle.trim() || undefined,
        file_name: up.filename,
        file_url: up.url,
        file_size: up.size,
        mime_type: up.type,
        visibility: docShared ? 'shared' : 'private',
        expires_at: docExpiry || undefined,
      })
      setDocTitle(''); setDocShared(false); setDocExpiry('')
      if (fileRef.current) fileRef.current.value = ''
      await load()
    } catch (e: any) {
      setError(e?.response?.data?.error || 'No se pudo subir el documento')
    } finally {
      setBusy(false)
    }
  }

  const startEditDoc = (d: any) => {
    setEditingDocId(d.id)
    setEditDocTitle(d.title || '')
    setEditDocShared(d.visibility === 'shared')
    setEditDocExpiry(d.expires_at ? String(d.expires_at).slice(0, 10) : '')
  }

  const saveDocEdit = async () => {
    if (!editingDocId) return
    setBusy(true); setError(null)
    try {
      await apiSvc.updateDoc(editingDocId, {
        title: editDocTitle.trim() || undefined,
        visibility: editDocShared ? 'shared' : 'private',
        expires_at: editDocExpiry || undefined,
      })
      setEditingDocId(null)
      await load()
    } catch (e: any) {
      setError(e?.response?.data?.error || 'No se pudo guardar el documento')
    } finally {
      setBusy(false)
    }
  }

  const removeDoc = async (docId: number) => {
    setBusy(true)
    try { await apiSvc.deleteDoc(docId); await load() }
    catch { setError('No se pudo eliminar el documento') }
    finally { setBusy(false) }
  }

  // Estado de vencimiento de un documento.
  const expiryInfo = (expires?: string | null) => {
    if (!expires) return null
    const d = new Date(expires)
    if (isNaN(d.getTime())) return null
    const days = Math.ceil((d.getTime() - Date.now()) / 86400000)
    const label = d.toLocaleDateString('es-ES', { day: '2-digit', month: 'short', year: 'numeric' })
    if (days < 0) return { text: `Vencido (${label})`, bg: 'rgba(239,68,68,0.12)', color: '#b91c1c' }
    if (days <= 30) return { text: `Vence ${label}`, bg: 'rgba(245,158,11,0.14)', color: '#b45309' }
    return { text: `Vence ${label}`, bg: 'rgba(100,116,139,0.1)', color: '#64748b' }
  }

  const s = data?.summary
  const frozen = !!s?.frozen_at

  return (
    <Modal
      isOpen
      onClose={onClose}
      size="lg"
      title={`Expediente · ${employment.company_name || 'Empresa'}`}
    >
      {loading ? (
        <p style={{ color: '#94a3b8' }}>Cargando expediente…</p>
      ) : (
        <div style={{ display: 'flex', flexDirection: 'column', gap: '1.25rem' }}>
          {error && <div style={{ color: '#b91c1c', fontSize: '0.85rem' }}>{error}</div>}

          {!selfMode && (
            <div style={{ display: 'flex', justifyContent: 'flex-end', marginBottom: '-0.5rem' }}>
              <a
                href={pdfHref}
                target="_blank"
                rel="noopener noreferrer"
                style={{ display: 'inline-flex', alignItems: 'center', gap: 6, padding: '6px 12px', borderRadius: 8, border: '1px solid #cbd5e1', background: '#fff', color: '#334155', fontWeight: 700, fontSize: '0.82rem', textDecoration: 'none' }}
              >
                <Download size={14} /> Descargar PDF
              </a>
            </div>
          )}

          {/* Resumen */}
          <section>
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: '10px' }}>
              <h4 style={{ margin: 0 }}>Resumen</h4>
              {frozen && (
                <span title={`Congelado el ${new Date(s!.frozen_at!).toLocaleDateString('es-ES')}`}
                  style={{ display: 'inline-flex', alignItems: 'center', gap: '4px', fontSize: '0.72rem', fontWeight: 700, color: '#0369a1', background: 'rgba(3,105,161,0.1)', padding: '2px 8px', borderRadius: 999 }}>
                  <Snowflake size={12} /> Congelado al salir
                </span>
              )}
            </div>
            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(120px, 1fr))', gap: '10px' }}>
              <SummaryStat icon={<CalendarDays size={16} />} label="Antigüedad" value={`${s?.days_employed ?? 0} días`} />
              <SummaryStat icon={<Clock3 size={16} />} label="Horas totales" value={fmtHours(s?.total_hours || 0)} />
              <SummaryStat icon={<Clock3 size={16} />} label="Horas aprobadas" value={fmtHours(s?.approved_hours || 0)} />
              <SummaryStat icon={<CheckSquare size={16} />} label="Tareas" value={`${s?.tasks_completed ?? 0}/${s?.tasks_assigned ?? 0}`} />
              <SummaryStat icon={<CalendarX size={16} />} label="Ausencias" value={`${s?.absences ?? 0}`} />
            </div>
          </section>

          {/* Ausencias */}
          <section>
            <h4 style={{ margin: '0 0 10px' }}>Ausencias</h4>
            {(data?.absences?.length ?? 0) === 0 ? (
              <span style={{ fontSize: '0.85rem', color: '#94a3b8' }}>Sin ausencias registradas.</span>
            ) : (
              <div style={{ display: 'flex', flexDirection: 'column', gap: '6px' }}>
                {data?.absences.map((a, i) => (
                  <div key={i} style={{ display: 'flex', alignItems: 'center', gap: '10px', border: '1px solid #e2e8f0', borderRadius: '8px', padding: '8px 12px' }}>
                    <CalendarX size={16} style={{ color: '#dc2626', flexShrink: 0 }} />
                    <span style={{ fontSize: '0.85rem', fontWeight: 600, color: '#0f172a' }}>
                      {new Date(a.date).toLocaleDateString('es-ES', { day: '2-digit', month: 'short', year: 'numeric' })}
                    </span>
                    <span style={{ fontSize: '0.83rem', color: '#475569' }}>{a.reason || 'Sin motivo'}</span>
                    <span style={{ marginLeft: 'auto', display: 'flex', alignItems: 'center', gap: '8px' }}>
                      {a.hours > 0 && <span style={{ fontSize: '0.78rem', color: '#94a3b8' }}>{a.hours}h</span>}
                      <span style={{ padding: '2px 8px', borderRadius: 999, fontSize: '0.7rem', fontWeight: 700, background: a.approved ? 'rgba(16,185,129,0.12)' : 'rgba(245,158,11,0.14)', color: a.approved ? '#047857' : '#b45309' }}>
                        {a.approved ? 'Justificada' : 'Pendiente'}
                      </span>
                    </span>
                  </div>
                ))}
              </div>
            )}
          </section>

          {/* Gestiones de CS (inactividad / ausencia) */}
          <section>
            <h4 style={{ margin: '0 0 4px' }}>Gestiones de seguimiento</h4>
            <p style={{ margin: '0 0 10px', fontSize: '0.8rem', color: '#94a3b8' }}>
              Seguimientos de customer success por inactividad o ausencias.
            </p>
            {(data?.gestiones?.length ?? 0) === 0 ? (
              <span style={{ fontSize: '0.85rem', color: '#94a3b8' }}>Sin gestiones registradas.</span>
            ) : (
              <div style={{ display: 'flex', flexDirection: 'column', gap: '6px' }}>
                {data?.gestiones.map((g, i) => {
                  const st = GESTION_STATUS[g.status] || { label: g.status, bg: 'rgba(100,116,139,0.12)', color: '#64748b' }
                  return (
                    <div key={i} style={{ display: 'flex', alignItems: 'center', gap: '10px', border: '1px solid #e2e8f0', borderRadius: '8px', padding: '8px 12px' }}>
                      <ClipboardList size={16} style={{ color: '#7c3aed', flexShrink: 0 }} />
                      <span style={{ padding: '2px 8px', borderRadius: 999, fontSize: '0.7rem', fontWeight: 700, background: st.bg, color: st.color }}>
                        {st.label}
                      </span>
                      <span style={{ fontSize: '0.78rem', color: '#64748b' }}>{GESTION_KIND[g.kind] || g.kind}</span>
                      {g.note && <span style={{ fontSize: '0.83rem', color: '#475569' }}>· {g.note}</span>}
                      <span style={{ marginLeft: 'auto', fontSize: '0.75rem', color: '#94a3b8', whiteSpace: 'nowrap' }}>
                        {g.by_name || 'CS'} · {new Date(g.created_at).toLocaleDateString('es-ES')}
                      </span>
                    </div>
                  )
                })}
              </div>
            )}
          </section>

          {/* Historial de contactos */}
          <section>
            <h4 style={{ margin: '0 0 4px' }}>Contactos</h4>
            <p style={{ margin: '0 0 10px', fontSize: '0.8rem', color: '#94a3b8' }}>
              Intentos de contacto (email, WhatsApp, chat) registrados al hacer clic en contactar.
            </p>
            {(data?.contactos?.length ?? 0) === 0 ? (
              <span style={{ fontSize: '0.85rem', color: '#94a3b8' }}>Sin contactos registrados.</span>
            ) : (
              <div style={{ display: 'flex', flexDirection: 'column', gap: '6px' }}>
                {data?.contactos.map((c, i) => {
                  const ch = CONTACT_CHANNEL[c.channel] || { label: c.channel, icon: '•' }
                  return (
                    <div key={i} style={{ display: 'flex', alignItems: 'center', gap: '10px', border: '1px solid #e2e8f0', borderRadius: '8px', padding: '8px 12px' }}>
                      <Send size={15} style={{ color: '#0ea5e9', flexShrink: 0 }} />
                      <span style={{ fontSize: '0.85rem', fontWeight: 600, color: '#0f172a' }}>{ch.icon} {ch.label}</span>
                      <span style={{ marginLeft: 'auto', fontSize: '0.75rem', color: '#94a3b8', whiteSpace: 'nowrap' }}>
                        {c.by_name || 'Equipo'} · {new Date(c.created_at).toLocaleDateString('es-ES')}
                      </span>
                    </div>
                  )
                })}
              </div>
            )}
          </section>

          {/* Evaluaciones / notas */}
          <section>
            <h4 style={{ margin: '0 0 10px' }}>Evaluaciones y notas</h4>

            <EvaluationTrend notes={data?.notes || []} />

            {manage && (
              <div style={{ border: '1px solid #e2e8f0', borderRadius: '10px', padding: '12px', marginBottom: '12px', background: '#f8fafc' }}>
                <div style={{ display: 'flex', gap: '8px', flexWrap: 'wrap', alignItems: 'center', marginBottom: '8px' }}>
                  <select value={kind} onChange={e => setKind(e.target.value as any)}
                    style={{ padding: '6px 8px', borderRadius: '8px', border: '1px solid #cbd5e1', fontSize: '0.85rem' }}>
                    <option value="note">Nota</option>
                    <option value="evaluation">Evaluación</option>
                  </select>
                  {kind === 'evaluation' && (
                    <div style={{ display: 'inline-flex', gap: '2px' }}>
                      {[1, 2, 3, 4, 5].map(n => (
                        <button key={n} type="button" onClick={() => setRating(n)} title={`${n} de 5`}
                          style={{ background: 'none', border: 'none', cursor: 'pointer', padding: 0, color: n <= rating ? '#f59e0b' : '#cbd5e1' }}>
                          <Star size={18} fill={n <= rating ? '#f59e0b' : 'none'} />
                        </button>
                      ))}
                    </div>
                  )}
                  <label style={{ display: 'inline-flex', alignItems: 'center', gap: '5px', fontSize: '0.8rem', color: '#475569', marginLeft: 'auto' }}>
                    <input type="checkbox" checked={shared} onChange={e => setShared(e.target.checked)} />
                    Visible para el profesional
                  </label>
                </div>
                <textarea value={content} onChange={e => setContent(e.target.value)} rows={3}
                  placeholder="Escribe una evaluación de desempeño o una anotación de seguimiento…"
                  style={{ width: '100%', padding: '8px 10px', borderRadius: '8px', border: '1px solid #cbd5e1', fontSize: '0.88rem', resize: 'vertical', boxSizing: 'border-box' }} />
                <div style={{ marginTop: '8px', textAlign: 'right', display: 'flex', gap: 8, justifyContent: 'flex-end' }}>
                  {editingNoteId && (
                    <Button onClick={resetNoteForm} disabled={busy} variant="secondary">Cancelar</Button>
                  )}
                  <Button onClick={saveNote} disabled={busy || !content.trim()} variant="primary">
                    {editingNoteId ? 'Actualizar' : 'Guardar'}
                  </Button>
                </div>
              </div>
            )}

            {data?.notes.length === 0 ? (
              <span style={{ fontSize: '0.85rem', color: '#94a3b8' }}>Sin evaluaciones ni notas.</span>
            ) : (
              <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
                {data?.notes.map(n => (
                  <div key={n.id} style={{ border: '1px solid #e2e8f0', borderRadius: '10px', padding: '10px 12px' }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: '4px', flexWrap: 'wrap' }}>
                      <span style={{ fontSize: '0.72rem', fontWeight: 700, textTransform: 'uppercase', letterSpacing: '0.03em', color: n.kind === 'evaluation' ? '#7c3aed' : '#0369a1' }}>
                        {n.kind === 'evaluation' ? 'Evaluación' : 'Nota'}
                      </span>
                      {n.kind === 'evaluation' && n.rating > 0 && (
                        <span style={{ display: 'inline-flex', gap: '1px', color: '#f59e0b' }}>
                          {Array.from({ length: n.rating }).map((_, i) => <Star key={i} size={13} fill="#f59e0b" />)}
                        </span>
                      )}
                      <VisibilityBadge shared={n.visibility === 'shared'} />
                      <span style={{ marginLeft: 'auto', fontSize: '0.75rem', color: '#94a3b8' }}>
                        {n.author_name || 'Autor'} · {new Date(n.created_at).toLocaleDateString('es-ES')}
                      </span>
                      {manage && (
                        <>
                          <button onClick={() => startEditNote(n)} title="Editar" disabled={busy}
                            style={{ background: 'none', border: 'none', cursor: 'pointer', color: '#475569', padding: 0 }}>
                            <Pencil size={14} />
                          </button>
                          <button onClick={() => removeNote(n.id)} title="Eliminar" disabled={busy}
                            style={{ background: 'none', border: 'none', cursor: 'pointer', color: '#b91c1c', padding: 0 }}>
                            <Trash2 size={15} />
                          </button>
                        </>
                      )}
                    </div>
                    <p style={{ margin: 0, fontSize: '0.88rem', color: '#0f172a', whiteSpace: 'pre-wrap' }}>{n.content}</p>
                  </div>
                ))}
              </div>
            )}
          </section>

          {/* Documentos */}
          <section>
            <h4 style={{ margin: '0 0 10px' }}>Documentos</h4>

            {manage && (
              <div style={{ display: 'flex', gap: '8px', flexWrap: 'wrap', alignItems: 'center', marginBottom: '12px' }}>
                <input value={docTitle} onChange={e => setDocTitle(e.target.value)} placeholder="Título (opcional)"
                  style={{ flex: '1 1 160px', padding: '7px 10px', borderRadius: '8px', border: '1px solid #cbd5e1', fontSize: '0.85rem' }} />
                <label style={{ display: 'inline-flex', alignItems: 'center', gap: '5px', fontSize: '0.78rem', color: '#475569' }} title="Fecha de vencimiento (opcional)">
                  <CalendarClock size={14} />
                  <input type="date" value={docExpiry} onChange={e => setDocExpiry(e.target.value)}
                    style={{ padding: '6px 8px', borderRadius: '8px', border: '1px solid #cbd5e1', fontSize: '0.82rem' }} />
                </label>
                <label style={{ display: 'inline-flex', alignItems: 'center', gap: '5px', fontSize: '0.8rem', color: '#475569' }}>
                  <input type="checkbox" checked={docShared} onChange={e => setDocShared(e.target.checked)} />
                  Compartir
                </label>
                <input ref={fileRef} type="file" onChange={onPickFile} style={{ display: 'none' }} />
                <Button onClick={() => fileRef.current?.click()} disabled={busy} variant="secondary" leftIcon={<Upload size={15} />}>
                  Subir documento
                </Button>
              </div>
            )}

            {data?.documents.length === 0 ? (
              <span style={{ fontSize: '0.85rem', color: '#94a3b8' }}>Sin documentos adjuntos.</span>
            ) : (
              <div style={{ display: 'flex', flexDirection: 'column', gap: '6px' }}>
                {data?.documents.map(d => {
                  const exp = expiryInfo(d.expires_at)
                  if (editingDocId === d.id) {
                    return (
                      <div key={d.id} style={{ border: '1px solid #8b5cf6', borderRadius: '8px', padding: '10px 12px', background: '#faf5ff' }}>
                        <div style={{ display: 'flex', gap: '8px', flexWrap: 'wrap', alignItems: 'center', marginBottom: 8 }}>
                          <input value={editDocTitle} onChange={e => setEditDocTitle(e.target.value)} placeholder="Título"
                            style={{ flex: '1 1 160px', padding: '6px 9px', borderRadius: '8px', border: '1px solid #cbd5e1', fontSize: '0.84rem' }} />
                          <label style={{ display: 'inline-flex', alignItems: 'center', gap: 5, fontSize: '0.78rem', color: '#475569' }} title="Vencimiento">
                            <CalendarClock size={14} />
                            <input type="date" value={editDocExpiry} onChange={e => setEditDocExpiry(e.target.value)}
                              style={{ padding: '5px 8px', borderRadius: '8px', border: '1px solid #cbd5e1', fontSize: '0.82rem' }} />
                          </label>
                          <label style={{ display: 'inline-flex', alignItems: 'center', gap: 5, fontSize: '0.8rem', color: '#475569' }}>
                            <input type="checkbox" checked={editDocShared} onChange={e => setEditDocShared(e.target.checked)} /> Compartir
                          </label>
                        </div>
                        <div style={{ display: 'flex', gap: 8, justifyContent: 'flex-end' }}>
                          <Button onClick={() => setEditingDocId(null)} disabled={busy} variant="secondary">Cancelar</Button>
                          <Button onClick={saveDocEdit} disabled={busy} variant="primary">Guardar</Button>
                        </div>
                      </div>
                    )
                  }
                  return (
                    <div key={d.id} style={{ display: 'flex', alignItems: 'center', gap: '10px', border: '1px solid #e2e8f0', borderRadius: '8px', padding: '8px 12px' }}>
                      <FileText size={18} style={{ color: '#64748b', flexShrink: 0 }} />
                      <a href={docHref(d.id)} target="_blank" rel="noopener noreferrer"
                        style={{ fontSize: '0.88rem', color: '#0f172a', textDecoration: 'none', fontWeight: 600 }}>
                        {d.title || d.file_name}
                      </a>
                      <VisibilityBadge shared={d.visibility === 'shared'} />
                      {exp && (
                        <span style={{ display: 'inline-flex', alignItems: 'center', gap: 3, padding: '2px 8px', borderRadius: 999, fontSize: '0.7rem', fontWeight: 700, background: exp.bg, color: exp.color }}>
                          <CalendarClock size={11} /> {exp.text}
                        </span>
                      )}
                      <span style={{ marginLeft: 'auto', fontSize: '0.75rem', color: '#94a3b8' }}>
                        {new Date(d.created_at).toLocaleDateString('es-ES')}
                      </span>
                      {manage && (
                        <>
                          <button onClick={() => startEditDoc(d)} title="Editar" disabled={busy}
                            style={{ background: 'none', border: 'none', cursor: 'pointer', color: '#475569', padding: 0 }}>
                            <Pencil size={14} />
                          </button>
                          <button onClick={() => removeDoc(d.id)} title="Eliminar" disabled={busy}
                            style={{ background: 'none', border: 'none', cursor: 'pointer', color: '#b91c1c', padding: 0 }}>
                            <Trash2 size={15} />
                          </button>
                        </>
                      )}
                    </div>
                  )
                })}
              </div>
            )}
          </section>
        </div>
      )}
    </Modal>
  )
}

function SummaryStat({ icon, label, value }: { icon: React.ReactNode; label: string; value: string }) {
  return (
    <div style={{ border: '1px solid #e2e8f0', borderRadius: '10px', padding: '10px 12px' }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: '6px', color: '#64748b', fontSize: '0.75rem', marginBottom: '4px' }}>
        {icon}{label}
      </div>
      <div style={{ fontSize: '1.15rem', fontWeight: 800, color: '#0f172a' }}>{value}</div>
    </div>
  )
}

// Color del rating: 1-2 rojo, 3 ámbar, 4-5 verde.
const ratingColor = (r: number) => (r <= 2 ? '#ef4444' : r === 3 ? '#f59e0b' : '#10b981')

// Tendencia de evaluaciones: promedio + mini-gráfico de la evolución del rating.
function EvaluationTrend({ notes }: { notes: any[] }) {
  const evals = (notes || [])
    .filter(n => n.kind === 'evaluation' && (n.rating ?? 0) > 0)
    .slice()
    .sort((a, b) => new Date(a.created_at).getTime() - new Date(b.created_at).getTime())
  if (evals.length === 0) return null
  const avg = evals.reduce((s, n) => s + (n.rating || 0), 0) / evals.length
  return (
    <div style={{ display: 'flex', alignItems: 'center', gap: 16, border: '1px solid #e2e8f0', borderRadius: 10, padding: '10px 14px', marginBottom: 12, background: '#fffdf7' }}>
      <div>
        <div style={{ display: 'flex', alignItems: 'baseline', gap: 6 }}>
          <span style={{ fontSize: '1.6rem', fontWeight: 800, color: '#0f172a' }}>{avg.toFixed(1)}</span>
          <span style={{ display: 'inline-flex', color: '#f59e0b' }}>
            {[1, 2, 3, 4, 5].map(i => <Star key={i} size={14} fill={i <= Math.round(avg) ? '#f59e0b' : 'none'} />)}
          </span>
        </div>
        <div style={{ fontSize: '0.72rem', color: '#94a3b8' }}>{evals.length} evaluación{evals.length === 1 ? '' : 'es'}</div>
      </div>
      {evals.length >= 2 && (
        <div style={{ display: 'flex', alignItems: 'flex-end', gap: 3, marginLeft: 'auto', height: 34 }} title="Evolución (antiguas → recientes)">
          {evals.map((n, i) => (
            <div key={i} title={`${n.rating}★ · ${new Date(n.created_at).toLocaleDateString('es-ES')}`}
              style={{ width: 7, height: `${(n.rating / 5) * 100}%`, minHeight: 4, borderRadius: 2, background: ratingColor(n.rating) }} />
          ))}
        </div>
      )}
    </div>
  )
}

function VisibilityBadge({ shared }: { shared: boolean }) {
  return shared ? (
    <span style={{ display: 'inline-flex', alignItems: 'center', gap: '3px', fontSize: '0.7rem', fontWeight: 700, color: '#047857', background: 'rgba(16,185,129,0.12)', padding: '2px 7px', borderRadius: 999 }}>
      <Eye size={11} /> Compartido
    </span>
  ) : (
    <span style={{ display: 'inline-flex', alignItems: 'center', gap: '3px', fontSize: '0.7rem', fontWeight: 700, color: '#64748b', background: 'rgba(100,116,139,0.12)', padding: '2px 7px', borderRadius: 999 }}>
      <Lock size={11} /> Privado
    </span>
  )
}
