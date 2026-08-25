import { Paperclip, Clock } from 'lucide-react'
import { useSortable } from '@dnd-kit/sortable'
import { CSS } from '@dnd-kit/utilities'
import type { Task } from '../../types'
import { parseDateOnly, formatDateOnly, todayMidnight, daysSince } from '../../utils/date'
import styles from '../../pages/Tasks.module.css'

// A partir de cuántos días la tarjeta declara su antigüedad, y a partir de cuántos
// lo hace en ámbar. Ponerlo en cada tarjeta sería ruido: lo que hay que ver de un
// vistazo es qué se quedó quieto, no cuándo se movió lo que se mueve a diario.
const STALE_AFTER_DAYS = 2
const STALE_WARN_DAYS = 7

interface TaskCardProps {
  task: Task
  isDragging?: boolean
  isPlaceholder?: boolean
  onClick: () => void
}

export function TaskCard({ task, isDragging, isPlaceholder, onClick }: TaskCardProps) {
  const getPriorityColor = (priority: string) => {
    const colors: Record<string, string> = {
      urgent: '#ef4444',
      high: '#f97316',
      medium: '#eab308',
      low: '#22c55e',
    }
    return colors[priority] || '#6b7280'
  }

  // Una tarea terminada no está "estancada": lleva ahí lo que lleve y está bien.
  const daysInColumn = task.completed ? null : daysSince(task.status_changed_at)
  const showStale = daysInColumn !== null && daysInColumn >= STALE_AFTER_DAYS

  return (
    <div
      className={`${styles['kanban-card']} ${isDragging ? (styles['dragging'] || 'dragging') : ''} ${isPlaceholder ? (styles['placeholder'] || 'placeholder') : ''} ${task.completed ? (styles['completed'] || 'completed') : ''}`}
      onClick={onClick}
    >
      <div className={styles['card-priority']} style={{ backgroundColor: getPriorityColor(task.priority) }} />
      <h4 className={styles['card-title']}>{task.title}</h4>

      <div className={styles['card-meta']}>
        <div className={styles['card-dates']}>
          {task.start_date && (
            <span className={styles['card-date']}>
              {new Date(task.start_date).toLocaleDateString('es-ES', { day: 'numeric', month: 'short' })}
            </span>
          )}
          {task.end_date && (
            <span className={`${styles['card-date']} ${!task.completed && parseDateOnly(task.end_date).getTime() < todayMidnight().getTime() ? styles['card-date-overdue'] : ''}`}>
              {formatDateOnly(task.end_date, { day: 'numeric', month: 'short', year: 'numeric' })}
            </span>
          )}
          {showStale && (
            <span
              className={`${styles['card-stale']} ${daysInColumn >= STALE_WARN_DAYS ? styles['card-stale-warn'] : ''}`}
              title={`Sin moverse de esta columna desde hace ${daysInColumn} días`}
            >
              <Clock size={11} />
              {daysInColumn} d aquí
            </span>
          )}
        </div>
        {task.attachments && task.attachments.length > 0 && (
          <div className={styles['card-attachments'] || 'card-attachments'} title={`${task.attachments.length} archivos adjuntos`}>
            <Paperclip size={14} />
            <span>{task.attachments.length}</span>
          </div>
        )}
        {task.assignees && task.assignees.length > 0 && (
          <div className={styles['card-assignees']}>
            {task.assignees.slice(0, 3).map((user) => (
              <div key={user.id} className={styles['assignee-avatar']} title={user.name}>
                {user.name.charAt(0).toUpperCase()}
              </div>
            ))}
            {task.assignees.length > 3 && (
              <div className={`${styles['assignee-avatar']} ${styles['more'] || 'more'}`}>+{task.assignees.length - 3}</div>
            )}
          </div>
        )}
      </div>
    </div>
  )
}

interface SortableTaskCardProps {
  task: Task
  onClick: () => void
}

export function SortableTaskCard({ task, onClick }: SortableTaskCardProps) {
  const {
    attributes,
    listeners,
    setNodeRef,
    transform,
    transition,
    isDragging,
  } = useSortable({ id: task.id })

  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
  }

  return (
    <div ref={setNodeRef} style={style} {...attributes} {...listeners}>
      <TaskCard task={task} isPlaceholder={isDragging} onClick={onClick} />
    </div>
  )
}
