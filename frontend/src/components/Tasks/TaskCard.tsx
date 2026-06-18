import { Paperclip } from 'lucide-react'
import { useSortable } from '@dnd-kit/sortable'
import { CSS } from '@dnd-kit/utilities'
import type { Task } from '../../types'
import { htmlToText } from '../../utils/sanitize'
import styles from '../../pages/Tasks.module.css'

interface TaskCardProps {
  task: Task
  isDragging?: boolean
  onClick: () => void
}

export function TaskCard({ task, isDragging, onClick }: TaskCardProps) {
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
    <div
      className={`${styles['kanban-card']} ${isDragging ? (styles['dragging'] || 'dragging') : ''} ${task.completed ? (styles['completed'] || 'completed') : ''}`}
      onClick={onClick}
    >
      <div className={styles['card-priority']} style={{ backgroundColor: getPriorityColor(task.priority) }} />
      <h4 className={styles['card-title']}>{task.title}</h4>
      {task.description && (
        <p className={styles['card-description']}>
          {(() => { const t = htmlToText(task.description); return t.length > 100 ? t.substring(0, 100) + '...' : t })()}
        </p>
      )}
      <div className={styles['card-meta']}>
        <div className={styles['card-dates']}>
          {task.start_date && (
            <span className={styles['card-date']}>
              {new Date(task.start_date).toLocaleDateString('es-ES', { day: 'numeric', month: 'short' })}
            </span>
          )}
          {task.end_date && (
            <span className={`${styles['card-date']} ${new Date(task.end_date) < new Date() && task.status !== 'finalizado' ? styles['card-date-overdue'] : ''}`}>
              {new Date(task.end_date).toLocaleDateString('es-ES', { day: 'numeric', month: 'short', year: 'numeric' })}
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
    opacity: isDragging ? 0.5 : 1,
  }

  return (
    <div ref={setNodeRef} style={style} {...attributes} {...listeners}>
      <TaskCard task={task} isDragging={isDragging} onClick={onClick} />
    </div>
  )
}
