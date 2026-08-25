import { useState, useMemo, useEffect } from 'react'
import type { Task, TaskStatusHistoryEntry } from '../../../types'
import { ColumnType } from '../types'
import { taskService } from '../../../services/task.service'
import { Select } from '../../ui/Select'
import { X, Download, CheckCheck, Clock, Paperclip } from 'lucide-react'
import { sanitizeRichHtml } from '../../../utils/sanitize'
import { formatDateOnly, formatDaysSince } from '../../../utils/date'

interface GateEntry {
  key: string
  label: string
  type: string
  value: string
}

// El formulario que se rellenó para cruzar una puerta viaja como JSON. Se lee con
// tolerancia deliberada: es un registro histórico, puede venir de un esquema que ya
// cambió o de una versión anterior del formato, y un contenido inesperado tiene que
// ignorarse en vez de romper la ficha de la tarea.
//
// Se admiten dos formas: la actual, con etiqueta y tipo junto al valor, y la primera,
// que era un simple mapa clave→valor. En esa vieja la clave hace de etiqueta, que es
// lo único que se puede decir con honestidad sobre lo que se preguntó entonces.
function parseGateData(raw?: string): GateEntry[] {
  if (!raw) return []
  try {
    const parsed = JSON.parse(raw)
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) return []

    if (Array.isArray(parsed.fields)) {
      return parsed.fields
        .filter((f: unknown) => !!f && typeof f === 'object')
        .map((f: Record<string, unknown>) => ({
          key: String(f.key ?? ''),
          label: String(f.label || f.key || ''),
          type: String(f.type ?? 'text'),
          value: String(f.value ?? ''),
        }))
        .filter((f: GateEntry) => f.value !== '')
    }

    return Object.entries(parsed as Record<string, unknown>).map(([k, v]) => ({
      key: k, label: k, type: 'text', value: String(v),
    }))
  } catch {
    return []
  }
}

// Los archivos subidos se guardan como <usuario>_<marca de tiempo>_<nombre>. Enseñar
// eso entero es enseñar fontanería: en el historial interesa el nombre que la persona
// reconoce.
function fileLabel(url: string): string {
  const last = url.split('/').pop() ?? url
  return decodeURIComponent(last.replace(/^[0-9]+_[0-9]+_/, ''))
}

// Iconos (lucide) como SVG en línea para los botones que se inyectan dentro del
// HTML de la descripción.
const EXPAND_SVG =
  '<svg xmlns="http://www.w3.org/2000/svg" width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="15 3 21 3 21 9"/><polyline points="9 21 3 21 3 15"/><line x1="21" x2="14" y1="3" y2="10"/><line x1="3" x2="10" y1="21" y2="14"/></svg>'
const DOWNLOAD_SVG =
  '<svg xmlns="http://www.w3.org/2000/svg" width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" x2="12" y1="15" y2="3"/></svg>'

// Envuelve cada <img> del HTML (ya sanitizado) en un contenedor con dos botones
// superpuestos: ampliar y descargar. Los botones sólo llevan data-img-action; el
// src se lee del <img> hermano al hacer clic (así no duplicamos base64 gigantes).
function enhanceDescriptionImages(safeHtml: string): string {
  if (!safeHtml || typeof window === 'undefined') return safeHtml
  const doc = new DOMParser().parseFromString(safeHtml, 'text/html')
  const imgs = Array.from(doc.body.querySelectorAll('img'))
  if (imgs.length === 0) return safeHtml

  const mkBtn = (action: 'expand' | 'download', title: string, svg: string) => {
    const b = doc.createElement('button')
    b.type = 'button'
    b.className = 'ot-img-btn'
    b.setAttribute('data-img-action', action)
    b.setAttribute('title', title)
    b.setAttribute('aria-label', title)
    b.innerHTML = svg
    return b
  }

  imgs.forEach((img) => {
    const wrap = doc.createElement('span')
    wrap.className = 'ot-img-wrap'
    const actions = doc.createElement('span')
    actions.className = 'ot-img-actions'
    actions.appendChild(mkBtn('expand', 'Ampliar imagen', EXPAND_SVG))
    actions.appendChild(mkBtn('download', 'Descargar imagen', DOWNLOAD_SVG))
    img.replaceWith(wrap)
    wrap.appendChild(img)
    wrap.appendChild(actions)
  })
  return doc.body.innerHTML
}

// Descarga la imagen forzando el guardado (fetch -> blob) en vez de navegar.
async function downloadImage(src: string) {
  try {
    const res = await fetch(src, { credentials: 'same-origin' })
    const blob = await res.blob()
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = src.split('/').pop()?.split('?')[0] || 'imagen'
    document.body.appendChild(a)
    a.click()
    a.remove()
    URL.revokeObjectURL(url)
  } catch {
    window.open(src, '_blank')
  }
}

interface TaskDetailViewProps {
  task: Task
  columns: ColumnType[]
  styles: any
  onStatusChange: (status: string) => Promise<void>
  children?: React.ReactNode
}

export function TaskDetailView({
  task,
  columns,
  styles,
  onStatusChange,
  children
}: TaskDetailViewProps) {
  const [lightboxSrc, setLightboxSrc] = useState<string | null>(null)

  // Descripción con los botones (ampliar/descargar) ya inyectados en cada imagen.
  const descriptionHtml = useMemo(
    () => enhanceDescriptionImages(sanitizeRichHtml(task.description || '')),
    [task.description],
  )

  // Delegación: un solo handler para los botones de las imágenes. El src se lee
  // del <img> dentro del mismo contenedor.
  const handleDescriptionClick = (e: React.MouseEvent) => {
    const target = e.target as HTMLElement
    const btn = target.closest('[data-img-action]') as HTMLElement | null
    if (btn) {
      const img = btn.closest('.ot-img-wrap')?.querySelector('img') as HTMLImageElement | null
      const src = img?.getAttribute('src')
      if (!src) return
      if (btn.getAttribute('data-img-action') === 'expand') setLightboxSrc(src)
      else downloadImage(src)
      return
    }
    // Respaldo (sobre todo en móvil, sin hover): tocar la imagen la amplía.
    if (target.tagName === 'IMG' && target.closest('.ot-img-wrap')) {
      const src = target.getAttribute('src')
      if (src) setLightboxSrc(src)
    }
  }

  // Cerrar el lightbox con Escape.
  useEffect(() => {
    if (!lightboxSrc) return
    const onKey = (e: KeyboardEvent) => { if (e.key === 'Escape') setLightboxSrc(null) }
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }, [lightboxSrc])

  // Antigüedad en la columna actual y bitácora de movimientos. El historial se pide
  // sólo al desplegarlo: la mayoría de las veces que se abre una tarea no se mira.
  const [history, setHistory] = useState<TaskStatusHistoryEntry[] | null>(null)
  const [showHistory, setShowHistory] = useState(false)
  const [historyError, setHistoryError] = useState(false)

  // Otra tarea: se empieza de cero, con el historial plegado.
  useEffect(() => {
    setHistory(null)
    setShowHistory(false)
    setHistoryError(false)
  }, [task.id])

  // La misma tarea se movió de columna: lo cargado ya no incluye ese movimiento.
  // Se invalida, pero NO se pliega el panel: quien acaba de mover la tarjeta con
  // el historial abierto espera ver aparecer su movimiento, no que se le cierre.
  useEffect(() => {
    setHistory(null)
    setHistoryError(false)
  }, [task.status])

  // Carga perezosa: la mayoría de las veces que se abre una tarea no se mira el
  // historial, así que sólo se pide cuando hace falta mostrarlo.
  useEffect(() => {
    if (!showHistory || history !== null || historyError) return
    let cancelled = false
    taskService
      .getHistory(task.id)
      .then((rows) => { if (!cancelled) setHistory(rows) })
      .catch(() => { if (!cancelled) setHistoryError(true) })
    return () => { cancelled = true }
  }, [showHistory, history, historyError, task.id])

  const columnTitle = (statusId: string) =>
    columns.find((c) => c.id === statusId)?.title || statusId

  const ageLabel = formatDaysSince(task.status_changed_at)

  const getPriorityColor = (priority: string) => {
    const colors: Record<string, string> = {
      urgent: '#ef4444',
      high: '#f97316',
      medium: '#eab308',
      low: '#22c55e',
    }
    return colors[priority] || '#6b7280'
  }

  return (
    <>
      <div className={styles['task-status-bar']} style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
        <Select
          value={task.status}
          onChange={(v) => onStatusChange(String(v))}
          options={columns.map((col) => ({ value: col.id, label: col.title }))}
        />
        <span
          className={styles['priority-badge']}
          style={{ backgroundColor: getPriorityColor(task.priority) }}
        >
          {task.priority}
        </span>
        <div style={{ flex: 1 }} />
        {(() => {
          const doneCol = columns.find(c => c.id === 'finalizado' || c.title.toLowerCase().includes('finaliz'))
          const doneId = doneCol ? doneCol.id : 'finalizado'
          if (task.status !== doneId) {
            return (
              <button
                type="button"
                onClick={() => onStatusChange(doneId)}
                style={{
                  background: '#22c55e',
                  color: '#fff',
                  border: 'none',
                  borderRadius: '20px',
                  padding: '6px 14px',
                  fontSize: '12px',
                  fontWeight: 600,
                  cursor: 'pointer',
                  display: 'flex',
                  alignItems: 'center',
                  gap: '6px',
                  boxShadow: '0 2px 6px rgba(34, 197, 94, 0.3)'
                }}
              >
                <CheckCheck size={14} /> Finalizar Tarea
              </button>
            )
          }
          return null
        })()}
      </div>

      <h3 className={styles['task-title']}>{task.title}</h3>

      {/* Antigüedad en la columna actual. Sin fecha sellada (tareas anteriores a
          la bitácora que nadie ha movido desde) no se muestra nada en vez de
          inventar un "hoy" que sería falso. */}
      {ageLabel && (
        <div className={styles['task-age']}>
          <Clock size={13} />
          <span>
            En <strong>{columnTitle(task.status)}</strong> desde {ageLabel}
          </span>
          <button
            type="button"
            className={styles['task-history-toggle']}
            onClick={() => setShowHistory((v) => !v)}
          >
            {showHistory ? 'Ocultar historial' : 'Ver historial'}
          </button>
        </div>
      )}

      {showHistory && (
        <div className={styles['task-section']}>
          <h4>Historial de columnas</h4>
          {historyError ? (
            <p>No se pudo cargar el historial.</p>
          ) : history === null ? (
            <p>Cargando…</p>
          ) : history.length === 0 ? (
            <p>Esta tarea no ha cambiado de columna desde que existe la bitácora.</p>
          ) : (
            <ul className={styles['task-history-list']}>
              {history.map((h) => (
                <li key={h.id} className={styles['task-history-item']}>
                  {h.from_status === ''
                    ? <>Creada en <strong>{columnTitle(h.to_status)}</strong></>
                    : <>De <strong>{columnTitle(h.from_status)}</strong> a <strong>{columnTitle(h.to_status)}</strong></>}
                  {h.actor_name && <> · {h.actor_name}</>}
                  {/* Lo aportado al cruzar la puerta. Es el rastro que pedía el
                      concepto: qué usuario aprobó, cuándo y con qué datos. */}
                  {(() => {
                    const aportado = parseGateData(h.form_data)
                    if (aportado.length === 0) return null
                    return (
                      <dl className={styles['task-gate-data']}>
                        {aportado.map((f) => (
                          <div key={f.key}>
                            <dt>{f.label}</dt>
                            <dd>
                              {f.type === 'file' ? (
                                <a className={styles['task-gate-file']} href={f.value} target="_blank" rel="noreferrer">
                                  <Paperclip size={12} />
                                  {fileLabel(f.value)}
                                </a>
                              ) : /^https?:\/\//.test(f.value) ? (
                                <a href={f.value} target="_blank" rel="noreferrer">{f.value}</a>
                              ) : (
                                f.value
                              )}
                            </dd>
                          </div>
                        ))}
                      </dl>
                    )
                  })()}
                  <span className={styles['task-history-when']}>
                    {new Date(h.changed_at).toLocaleString('es-ES', {
                      day: 'numeric', month: 'short', year: 'numeric',
                      hour: '2-digit', minute: '2-digit',
                    })}
                  </span>
                </li>
              ))}
            </ul>
          )}
        </div>
      )}

      <div className={styles['task-section']}>
        <h4>Descripción</h4>
        {task.description ? (
          <div
            className={styles['task-description-html']}
            onClick={handleDescriptionClick}
            dangerouslySetInnerHTML={{ __html: descriptionHtml }}
          />
        ) : (
          <p>Sin descripción</p>
        )}
      </div>

      <div className={styles['task-dates-row']}>
        {task.start_date && (
          <div className={styles['date-item']}>
            <span className={styles['date-label']}>Inicio</span>
            <span>
              {new Date(task.start_date).toLocaleDateString('es-ES', {
                weekday: 'short',
                day: 'numeric',
                month: 'short'
              })}
            </span>
          </div>
        )}
        {task.end_date && (
          <div className={styles['date-item']}>
            <span className={styles['date-label']}>Fin</span>
            <span>
              {formatDateOnly(task.end_date, {
                weekday: 'short',
                day: 'numeric',
                month: 'short'
              })}
            </span>
          </div>
        )}
      </div>

      <div className={styles['task-section']}>
        <h4>Asignados</h4>
        <div className={styles['assignees-list']}>
          {task.assignees && task.assignees.length > 0 ? (
            task.assignees.map((user) => (
              <div key={user.id} className={styles['assignee-item']}>
                <span>{user.name}</span>
              </div>
            ))
          ) : (
            <span className={styles['no-data']}>Sin asignar</span>
          )}
        </div>
      </div>

      {children}

      {lightboxSrc && (
        <div className={styles['img-lightbox']} onClick={() => setLightboxSrc(null)}>
          <div className={styles['img-lightbox-toolbar']} onClick={(e) => e.stopPropagation()}>
            <button
              type="button"
              className={styles['img-lightbox-btn']}
              onClick={() => downloadImage(lightboxSrc)}
              title="Descargar imagen"
            >
              <Download size={18} />
            </button>
            <button
              type="button"
              className={styles['img-lightbox-btn']}
              onClick={() => setLightboxSrc(null)}
              title="Cerrar"
            >
              <X size={18} />
            </button>
          </div>
          <img
            src={lightboxSrc}
            alt="Imagen de la descripción"
            className={styles['img-lightbox-img']}
            onClick={(e) => e.stopPropagation()}
          />
        </div>
      )}
    </>
  )
}
