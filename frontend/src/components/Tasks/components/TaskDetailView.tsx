import type { Task, TaskAttachment } from '../../../types'
import { ColumnType } from '../types'
import { TaskAttachmentsSection } from './TaskAttachmentsSection'
import { TaskCommentsSection } from './TaskCommentsSection'
import { Select } from '../../ui/Select'
import { useConfirm } from '../../ui/ConfirmProvider'
import { Pencil, Trash2 } from 'lucide-react'
import { sanitizeRichHtml } from '../../../utils/sanitize'
import { formatDateOnly } from '../../../utils/date'

type TaskComment = NonNullable<Task['comments']>[number]

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
          <div className={styles['task-description-html']} dangerouslySetInnerHTML={{ __html: sanitizeRichHtml(task.description) }} />
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
    </>
  )
}
