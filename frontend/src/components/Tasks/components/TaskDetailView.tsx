import { useState, useMemo, useEffect } from 'react'
import type { Task, TaskAttachment } from '../../../types'
import { ColumnType } from '../types'
import { TaskAttachmentsSection } from './TaskAttachmentsSection'
import { TaskCommentsSection } from './TaskCommentsSection'
import { Select } from '../../ui/Select'
import { useConfirm } from '../../ui/ConfirmProvider'
import { Pencil, Trash2, X, Download } from 'lucide-react'
import { sanitizeRichHtml } from '../../../utils/sanitize'
import { formatDateOnly } from '../../../utils/date'

type TaskComment = NonNullable<Task['comments']>[number]

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
  attachments: TaskAttachment[]
  comments: TaskComment[]
  isLoadingComments: boolean
  isDeleting: boolean
  styles: any
  onStatusChange: (status: string) => Promise<void>
  onEdit: () => void
  onDelete: () => void
  refreshTask: () => Promise<void>
  onAttachmentAdded: (attachment: TaskAttachment) => void
  onAttachmentDeleted: (id: number) => void
}

export function TaskDetailView({
  task,
  columns,
  attachments,
  comments,
  isLoadingComments,
  isDeleting,
  styles,
  onStatusChange,
  onEdit,
  onDelete,
  refreshTask,
  onAttachmentAdded,
  onAttachmentDeleted
}: TaskDetailViewProps) {
  const confirm = useConfirm()
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

  const getPriorityColor = (priority: string) => {
    const colors: Record<string, string> = {
      urgent: '#ef4444',
      high: '#f97316',
      medium: '#eab308',
      low: '#22c55e',
    }
    return colors[priority] || '#6b7280'
  }

  const handleDelete = async () => {
    const ok = await confirm({
      title: 'Eliminar tarea',
      message: '¿Seguro que deseas eliminar esta tarea? Esta acción no se puede deshacer.',
      confirmLabel: 'Eliminar',
      variant: 'danger',
    })
    if (ok) onDelete()
  }

  return (
    <>
      <div className={styles['task-status-bar']}>
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
      </div>

      <h3 className={styles['task-title']}>{task.title}</h3>

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

      <TaskAttachmentsSection
        taskId={task.id}
        attachments={attachments}
        onAttachmentAdded={onAttachmentAdded}
        onAttachmentDeleted={onAttachmentDeleted}
        styles={styles}
      />

      <TaskCommentsSection
        taskId={task.id}
        comments={comments}
        isLoadingComments={isLoadingComments}
        refreshTask={refreshTask}
        styles={styles}
      />

      <div className={styles['panel-actions']}>
        <button className={styles['btn-edit']} onClick={onEdit}>
          <Pencil size={16} /> Editar
        </button>
        <button className={styles['btn-delete']} onClick={handleDelete} disabled={isDeleting}>
          {isDeleting ? 'Eliminando...' : <><Trash2 size={16} /> Eliminar</>}
        </button>
      </div>

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
