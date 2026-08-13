import { useState, useEffect, useCallback, useRef } from 'react'
import { FileText, Star, Trash2, Upload, Lock, Eye, Clock3, CheckSquare, CalendarDays, CalendarX, ClipboardList, Send, Snowflake, Download, Pencil, CalendarClock, StickyNote, X } from 'lucide-react'
import { adminService, authService, employerService, uploadService } from '../../services/api'
import { useDirtySnapshot } from '../ui/useCloseGuard'
import { Modal, Button, Select, DatePicker } from '../ui'
import styles from './Expediente.module.css'

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
  // Modo incrustado: el MISMO cuerpo sin el envoltorio de modal, para vivir
  // dentro de una pestaña. El expediente se lee desde varias pantallas y
  // duplicar su maquetación garantizaría que una se quede atrás de la otra.
  inline?: boolean
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
const GESTION_STATUS: Record<string, { label: string; className: string }> = {
  contacted: { label: 'Contactado', className: styles.pillInfo },
  justified: { label: 'Justificado', className: styles.pillSuccess },
  escalated: { label: 'Escalado', className: styles.pillWarn },
}

const NOTE_KINDS = [
  { value: 'note', label: 'Nota' },
  { value: 'evaluation', label: 'Evaluación' },
]

const fmtHours = (n: number) => (Math.round((n || 0) * 10) / 10).toLocaleString('es-ES')

const fmtSize = (b: number) =>
  b < 1024 ? `${b} B` : b < 1048576 ? `${Math.round(b / 1024)} KB` : `${(b / 1048576).toFixed(1)} MB`

// Expediente laboral de un empleo: resumen (en vivo o congelado al salir),
// evaluaciones/notas y documentos. La empresa (RR.HH.) ve y gestiona todo;
// cada entrada puede marcarse como compartida para que el profesional la vea.
export function ExpedienteModal({ userId, employment, canManage, onClose, selfMode = false, employerMode = false, inline = false }: ExpedienteModalProps) {
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

  // Subida de documento. pendingFile es el archivo elegido pero todavía NO
  // subido: existe para que haya un paso entre escogerlo y meterlo en el
  // expediente.
  const fileRef = useRef<HTMLInputElement>(null)
  const [pendingFile, setPendingFile] = useState<File | null>(null)
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

  // Elegir el archivo NO lo sube: queda a la espera. Antes, el mismo gesto de
  // seleccionarlo lo subía Y lo guardaba con el título y el vencimiento que
  // hubiera en ese instante —normalmente ninguno, porque el botón está debajo
  // de los campos—. Un documento del expediente de una persona no es algo que
  // se arregle después: se revisa antes de entrar.
  const onPickFile = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (file) { setPendingFile(file); setError(null) }
  }

  const discardFile = () => {
    setPendingFile(null)
    if (fileRef.current) fileRef.current.value = ''
  }

  const uploadDoc = async () => {
    if (!pendingFile) return
    setBusy(true); setError(null)
    try {
      const up = await uploadService.upload(pendingFile)
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
      discardFile()
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

  // Estado de vencimiento de un documento: vencido en rojo, a menos de un mes
  // en ámbar, el resto neutro. Un contrato caducado no puede leerse igual que
  // uno que vence el año que viene.
  const expiryInfo = (expires?: string | null) => {
    if (!expires) return null
    const d = new Date(expires)
    if (isNaN(d.getTime())) return null
    const days = Math.ceil((d.getTime() - Date.now()) / 86400000)
    const label = d.toLocaleDateString('es-ES', { day: '2-digit', month: 'short', year: 'numeric' })
    if (days < 0) return { text: `Vencido (${label})`, className: styles.pillDanger }
    if (days <= 30) return { text: `Vence ${label}`, className: styles.pillWarn }
    return { text: `Vence ${label}`, className: styles.pillNeutral }
  }

  const s = data?.summary
  const frozen = !!s?.frozen_at

  // El hook vive aquí y no en las props del modal: en modo incrustado no hay
  // modal que envolver, y llamarlo dentro de una rama rompería el orden de hooks.
  // Un archivo elegido y sin guardar también es trabajo a medias: cerrar con
  // él dentro tiene que avisar igual que cerrar con una nota a medio escribir.
  const isDirty = useDirtySnapshot([content, rating, docTitle, pendingFile?.name ?? ''])

  const body = loading ? (
    <p className={styles.loading}>Cargando expediente…</p>
  ) : (
    <div className={styles.body}>
      {error && <div className={styles.error}>{error}</div>}

      {!selfMode && (
        <div className={styles.pdfRow}>
          <a className={styles.pdfLink} href={pdfHref} target="_blank" rel="noopener noreferrer">
            <Download size={14} /> Descargar PDF
          </a>
        </div>
      )}

      {/* Resumen */}
      <section className={styles.section}>
        <div className={styles.sectionHead}>
          <h4 className={styles.sectionTitle}>Resumen</h4>
          {frozen && (
            <span
              className={`${styles.pill} ${styles.pillFrozen}`}
              title={`Congelado el ${new Date(s!.frozen_at!).toLocaleDateString('es-ES')}`}
            >
              <Snowflake size={12} /> Congelado al salir
            </span>
          )}
        </div>
        <div className={styles.statGrid}>
          <SummaryStat icon={<CalendarDays size={16} />} label="Antigüedad" value={`${s?.days_employed ?? 0} días`} />
          <SummaryStat icon={<Clock3 size={16} />} label="Horas totales" value={fmtHours(s?.total_hours || 0)} />
          <SummaryStat icon={<Clock3 size={16} />} label="Horas aprobadas" value={fmtHours(s?.approved_hours || 0)} />
          <SummaryStat icon={<CheckSquare size={16} />} label="Tareas" value={`${s?.tasks_completed ?? 0}/${s?.tasks_assigned ?? 0}`} />
          <SummaryStat icon={<CalendarX size={16} />} label="Ausencias" value={`${s?.absences ?? 0}`} />
        </div>
      </section>

      {/* Ausencias */}
      <section className={styles.section}>
        <div className={styles.sectionHead}>
          <h4 className={styles.sectionTitle}>Ausencias</h4>
        </div>
        {(data?.absences?.length ?? 0) === 0 ? (
          <span className={styles.emptyLine}>Sin ausencias registradas.</span>
        ) : (
          <div className={styles.rows}>
            {data?.absences.map((a, i) => (
              <div key={i} className={styles.row}>
                <CalendarX size={16} className={styles.rowIcon} style={{ color: '#dc2626' }} />
                <span className={styles.rowMain}>
                  {new Date(a.date).toLocaleDateString('es-ES', { day: '2-digit', month: 'short', year: 'numeric' })}
                </span>
                <span className={styles.rowText}>{a.reason || 'Sin motivo'}</span>
                <span className={styles.rowMeta}>
                  {a.hours > 0 && <span>{a.hours}h</span>}
                  <span className={`${styles.pill} ${a.approved ? styles.pillSuccess : styles.pillWarn}`}>
                    {a.approved ? 'Justificada' : 'Pendiente'}
                  </span>
                </span>
              </div>
            ))}
          </div>
        )}
      </section>

      {/* Gestiones de CS (inactividad / ausencia) */}
      <section className={styles.section}>
        <div className={styles.sectionHead}>
          <h4 className={styles.sectionTitle}>Gestiones de seguimiento</h4>
        </div>
        <p className={styles.sectionHint}>
          Seguimientos de customer success por inactividad o ausencias.
        </p>
        {(data?.gestiones?.length ?? 0) === 0 ? (
          <span className={styles.emptyLine}>Sin gestiones registradas.</span>
        ) : (
          <div className={styles.rows}>
            {data?.gestiones.map((g, i) => {
              const st = GESTION_STATUS[g.status] ?? { label: g.status, className: styles.pillNeutral }
              return (
                <div key={i} className={styles.row}>
                  <ClipboardList size={16} className={styles.rowIcon} style={{ color: '#7c3aed' }} />
                  <span className={`${styles.pill} ${st.className}`}>{st.label}</span>
                  <span className={styles.rowText}>{GESTION_KIND[g.kind] || g.kind}</span>
                  {g.note && <span className={styles.rowText}>· {g.note}</span>}
                  <span className={styles.rowMeta}>
                    {g.by_name || 'CS'} · {new Date(g.created_at).toLocaleDateString('es-ES')}
                  </span>
                </div>
              )
            })}
          </div>
        )}
      </section>

      {/* Historial de contactos */}
      <section className={styles.section}>
        <div className={styles.sectionHead}>
          <h4 className={styles.sectionTitle}>Contactos</h4>
        </div>
        <p className={styles.sectionHint}>
          Intentos de contacto (email, WhatsApp, chat) registrados al hacer clic en contactar.
        </p>
        {(data?.contactos?.length ?? 0) === 0 ? (
          <span className={styles.emptyLine}>Sin contactos registrados.</span>
        ) : (
          <div className={styles.rows}>
            {data?.contactos.map((c, i) => {
              const ch = CONTACT_CHANNEL[c.channel] || { label: c.channel, icon: '•' }
              return (
                <div key={i} className={styles.row}>
                  <Send size={15} className={styles.rowIcon} style={{ color: '#0ea5e9' }} />
                  <span className={styles.rowMain}>{ch.icon} {ch.label}</span>
                  <span className={styles.rowMeta}>
                    {c.by_name || 'Equipo'} · {new Date(c.created_at).toLocaleDateString('es-ES')}
                  </span>
                </div>
              )
            })}
          </div>
        )}
      </section>

      {/* Evaluaciones / notas */}
      <section className={styles.section}>
        <div className={styles.sectionHead}>
          <h4 className={styles.sectionTitle}>Evaluaciones y notas</h4>
        </div>

        <EvaluationTrend notes={data?.notes || []} />

        <div className={manage ? styles.withForm : ''}>
        {manage && (
          <div className={styles.composer}>
            <p className={styles.formTitle}>{editingNoteId ? 'Editar entrada' : 'Nueva entrada'}</p>
            <div className={styles.composerHead}>
              <Select
                options={NOTE_KINDS}
                value={kind}
                onChange={v => setKind(v as 'note' | 'evaluation')}
                ariaLabel="Tipo de entrada"
              />
              {kind === 'evaluation' && (
                <div className={styles.stars}>
                  {[1, 2, 3, 4, 5].map(n => (
                    <button
                      key={n}
                      type="button"
                      onClick={() => setRating(n)}
                      title={`${n} de 5`}
                      className={`${styles.star} ${n <= rating ? styles.starOn : ''}`}
                    >
                      <Star size={18} fill={n <= rating ? '#f59e0b' : 'none'} />
                    </button>
                  ))}
                </div>
              )}
              <label className={styles.checkLabel}>
                <input type="checkbox" checked={shared} onChange={e => setShared(e.target.checked)} />
                Visible para el profesional
              </label>
            </div>
            <textarea
              className={styles.textarea}
              value={content}
              onChange={e => setContent(e.target.value)}
              rows={3}
              placeholder="Escribe una evaluación de desempeño o una anotación de seguimiento…"
            />
            <div className={styles.formActions}>
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
          <div className={styles.emptyBox}>
            <StickyNote size={26} />
            <span className={styles.emptyBoxTitle}>Sin evaluaciones ni notas</span>
            <span className={styles.emptyBoxHint}>
              {manage
                ? 'Lo que escribas a la izquierda aparecerá aquí, con su autor y su fecha.'
                : 'Aquí aparecerán las evaluaciones de desempeño y las anotaciones de seguimiento.'}
            </span>
          </div>
        ) : (
          <div className={styles.notes}>
            {data?.notes.map(n => (
              <div key={n.id} className={styles.note}>
                <div className={styles.noteHead}>
                  <span className={`${styles.noteKind} ${n.kind === 'evaluation' ? styles.kindEvaluation : styles.kindNote}`}>
                    {n.kind === 'evaluation' ? 'Evaluación' : 'Nota'}
                  </span>
                  {n.kind === 'evaluation' && n.rating > 0 && (
                    <span className={styles.starsRead}>
                      {Array.from({ length: n.rating }).map((_, i) => <Star key={i} size={13} fill="#f59e0b" />)}
                    </span>
                  )}
                  <VisibilityBadge shared={n.visibility === 'shared'} />
                  <span className={styles.rowMeta}>
                    {n.author_name || 'Autor'} · {new Date(n.created_at).toLocaleDateString('es-ES')}
                  </span>
                  {manage && (
                    <>
                      <button className={styles.rowAction} onClick={() => startEditNote(n)} title="Editar" disabled={busy}>
                        <Pencil size={14} />
                      </button>
                      <button className={`${styles.rowAction} ${styles.rowActionDanger}`} onClick={() => removeNote(n.id)} title="Eliminar" disabled={busy}>
                        <Trash2 size={15} />
                      </button>
                    </>
                  )}
                </div>
                <p className={styles.noteBody}>{n.content}</p>
              </div>
            ))}
          </div>
        )}
        </div>
      </section>

      {/* Documentos */}
      <section className={styles.section}>
        <div className={styles.sectionHead}>
          <h4 className={styles.sectionTitle}>Documentos</h4>
        </div>

        <div className={manage ? styles.withForm : ''}>
        {manage && (
          <div className={styles.toolbar}>
            <p className={styles.formTitle}>Adjuntar documento</p>
            <input
              className={styles.input}
              value={docTitle}
              onChange={e => setDocTitle(e.target.value)}
              placeholder="Título (opcional)"
            />
            <div className={styles.toolbarRow}>
              <DatePicker
                compact
                clearable
                value={docExpiry}
                onChange={setDocExpiry}
                placeholder="Vencimiento"
                ariaLabel="Fecha de vencimiento (opcional)"
                title="Fecha de vencimiento (opcional)"
              />
              <label className={styles.checkLabel}>
                <input type="checkbox" checked={docShared} onChange={e => setDocShared(e.target.checked)} />
                Compartir
              </label>
            </div>
            <input ref={fileRef} type="file" onChange={onPickFile} style={{ display: 'none' }} />

            {/* El archivo elegido se enseña antes de subirlo: es la única
                oportunidad de ver que no te equivocaste de fichero. */}
            {pendingFile && (
              <div className={styles.pendingFile}>
                <FileText size={15} />
                <span className={styles.pendingName} title={pendingFile.name}>{pendingFile.name}</span>
                <span className={styles.pendingSize}>{fmtSize(pendingFile.size)}</span>
                <button
                  className={`${styles.rowAction} ${styles.rowActionDanger}`}
                  onClick={discardFile}
                  title="Quitar el archivo"
                  disabled={busy}
                >
                  <X size={15} />
                </button>
              </div>
            )}

            <div className={styles.formActions}>
              {pendingFile ? (
                <>
                  <Button onClick={() => fileRef.current?.click()} disabled={busy} variant="secondary">
                    Cambiar
                  </Button>
                  <Button onClick={uploadDoc} disabled={busy} variant="primary" leftIcon={<Upload size={15} />}>
                    {busy ? 'Subiendo…' : 'Guardar documento'}
                  </Button>
                </>
              ) : (
                <Button onClick={() => fileRef.current?.click()} disabled={busy} variant="secondary" leftIcon={<Upload size={15} />}>
                  Elegir archivo
                </Button>
              )}
            </div>
          </div>
        )}

        {data?.documents.length === 0 ? (
          <div className={styles.emptyBox}>
            <FileText size={26} />
            <span className={styles.emptyBoxTitle}>Sin documentos adjuntos</span>
            <span className={styles.emptyBoxHint}>
              {manage
                ? 'Contratos, certificados y evaluaciones firmadas. Puedes ponerles fecha de vencimiento para que avisen antes de caducar.'
                : 'Aquí aparecerán los documentos del expediente.'}
            </span>
          </div>
        ) : (
          <div className={styles.rows}>
            {data?.documents.map(d => {
              const exp = expiryInfo(d.expires_at)
              if (editingDocId === d.id) {
                return (
                  <div key={d.id} className={styles.docEditing}>
                    <div className={styles.composerHead}>
                      <input
                        className={styles.input}
                        value={editDocTitle}
                        onChange={e => setEditDocTitle(e.target.value)}
                        placeholder="Título"
                      />
                      <DatePicker
                        compact
                        clearable
                        value={editDocExpiry}
                        onChange={setEditDocExpiry}
                        placeholder="Vencimiento"
                        ariaLabel="Vencimiento"
                        title="Vencimiento"
                      />
                      <label className={styles.checkLabel}>
                        <input type="checkbox" checked={editDocShared} onChange={e => setEditDocShared(e.target.checked)} /> Compartir
                      </label>
                    </div>
                    <div className={styles.formActions}>
                      <Button onClick={() => setEditingDocId(null)} disabled={busy} variant="secondary">Cancelar</Button>
                      <Button onClick={saveDocEdit} disabled={busy} variant="primary">Guardar</Button>
                    </div>
                  </div>
                )
              }
              return (
                <div key={d.id} className={styles.row}>
                  <FileText size={18} className={styles.rowIcon} style={{ color: '#64748b' }} />
                  <a className={styles.docLink} href={docHref(d.id)} target="_blank" rel="noopener noreferrer">
                    {d.title || d.file_name}
                  </a>
                  <VisibilityBadge shared={d.visibility === 'shared'} />
                  {exp && (
                    <span className={`${styles.pill} ${exp.className}`}>
                      <CalendarClock size={11} /> {exp.text}
                    </span>
                  )}
                  <span className={styles.rowMeta}>
                    {new Date(d.created_at).toLocaleDateString('es-ES')}
                  </span>
                  {manage && (
                    <>
                      <button className={styles.rowAction} onClick={() => startEditDoc(d)} title="Editar" disabled={busy}>
                        <Pencil size={14} />
                      </button>
                      <button className={`${styles.rowAction} ${styles.rowActionDanger}`} onClick={() => removeDoc(d.id)} title="Eliminar" disabled={busy}>
                        <Trash2 size={15} />
                      </button>
                    </>
                  )}
                </div>
              )
            })}
          </div>
        )}
        </div>
      </section>
    </div>
  )

  if (inline) return body

  return (
    <Modal isDirty={isDirty}
      isOpen
      onClose={onClose}
      size="lg"
      title={`Expediente · ${employment.company_name || 'Empresa'}`}
    >
      {body}
    </Modal>
  )
}

function SummaryStat({ icon, label, value }: { icon: React.ReactNode; label: string; value: string }) {
  return (
    <div className={styles.stat}>
      <div className={styles.statLabel}>{icon}{label}</div>
      <div className={styles.statValue}>{value}</div>
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
    <div className={styles.trend}>
      <div>
        <div className={styles.trendScore}>
          <span className={styles.trendAvg}>{avg.toFixed(1)}</span>
          <span className={styles.starsRead}>
            {[1, 2, 3, 4, 5].map(i => <Star key={i} size={14} fill={i <= Math.round(avg) ? '#f59e0b' : 'none'} />)}
          </span>
        </div>
        <div className={styles.trendCount}>{evals.length} evaluación{evals.length === 1 ? '' : 'es'}</div>
      </div>
      {evals.length >= 2 && (
        <div className={styles.trendBars} title="Evolución (antiguas → recientes)">
          {evals.map((n, i) => (
            <div
              key={i}
              className={styles.trendBar}
              title={`${n.rating}★ · ${new Date(n.created_at).toLocaleDateString('es-ES')}`}
              style={{ height: `${(n.rating / 5) * 100}%`, background: ratingColor(n.rating) }}
            />
          ))}
        </div>
      )}
    </div>
  )
}

// Quién ve esta entrada. Se dice siempre, también cuando es privada: quien
// escribe una evaluación tiene que saber si el profesional va a leerla.
function VisibilityBadge({ shared }: { shared: boolean }) {
  return shared ? (
    <span className={`${styles.pill} ${styles.pillSuccess}`}>
      <Eye size={11} /> Compartido
    </span>
  ) : (
    <span className={`${styles.pill} ${styles.pillNeutral}`}>
      <Lock size={11} /> Privado
    </span>
  )
}
