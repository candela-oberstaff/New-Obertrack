import { Paperclip } from 'lucide-react'
import { useSortable } from '@dnd-kit/sortable'
import { CSS } from '@dnd-kit/utilities'
import type { Task } from '../../types'

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
      className={`kanban-card ${isDragging ? 'dragging' : ''} ${task.completed ? 'completed' : ''}`}
      onClick={onClick}
    >
      <div className="card-priority" style={{ backgroundColor: getPriorityColor(task.priority) }} />
      <h4 className="card-title">{task.title}</h4>
      {task.description && (
        <p className="card-description" dangerouslySetInnerHTML={{ __html: task.description.replace(/<[^>]*>/g, ' ').substring(0, 100) + (task.description.length > 100 ? '...' : '') }} />
      )}
      <div className="card-meta">
        {task.start_date && (
          <span className="card-date">
            {new Date(task.start_date).toLocaleDateString('es-ES', { day: 'numeric', month: 'short' })}
          </span>
        )}
        {task.attachments && task.attachments.length > 0 && (
          <div className="card-attachments" title={`${task.attachments.length} archivos adjuntos`}>
            <Paperclip size={14} />
            <span>{task.attachments.length}</span>
          </div>
        )}
        {task.assignees && task.assignees.length > 0 && (
          <div className="card-assignees">
            {task.assignees.slice(0, 3).map((user) => (
              <div key={user.id} className="assignee-avatar" title={user.name}>
                {user.name.charAt(0).toUpperCase()}
              </div>
            ))}
            {task.assignees.length > 3 && (
              <div className="assignee-avatar more">+{task.assignees.length - 3}</div>
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
      <TaskCard task={task} onClick={onClick} />
    </div>
  )
}
