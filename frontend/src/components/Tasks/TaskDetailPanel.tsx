import { useState, useEffect } from 'react'
import { taskService } from '../../services/api'
import type { Task, User, TaskStatus, TaskPriority } from '../../types'
import { RichTextEditor } from './RichTextEditor'
import { ColumnType } from './types'

interface TaskDetailPanelProps {
  task: Task | null
  users: User[]
  onClose: () => void
  onUpdate: (id: number, data: Partial<Task>) => Promise<void>
  onDelete: (id: number) => Promise<void>
  columns: ColumnType[]
}

export function TaskDetailPanel({ task, users, onClose, onUpdate, onDelete, columns }: TaskDetailPanelProps) {
  const [isEditing, setIsEditing] = useState(false)
  const [newComment, setNewComment] = useState('')
  const [isSubmittingComment, setIsSubmittingComment] = useState(false)
  const [isUpdating, setIsUpdating] = useState(false)
  const [assigneeSearch, setAssigneeSearch] = useState('')
  const [isDeleting, setIsDeleting] = useState(false)
  const [taskComments, setTaskComments] = useState(task?.comments || [])
  const [formData, setFormData] = useState({
    title: '',
    description: '',
    priority: 'medium',
    status: 'por_hacer',
    start_date: '',
    end_date: '',
    assignees: [] as number[],
  })

  useEffect(() => {
    if (task) {
      setFormData({
        title: task.title,
        description: task.description || '',
        priority: task.priority,
        status: task.status,
        start_date: task.start_date?.split('T')[0] || '',
        end_date: task.end_date?.split('T')[0] || '',
        assignees: task.assignees?.map(a => a.id) || [],
      })
      setTaskComments(task.comments || [])
    }
  }, [task])

  const refreshComments = async () => {
    try {
      const updated = await taskService.getById(task!.id)
      setTaskComments(updated.comments || [])
    } catch (error) {
      console.error('Error refreshing comments:', error)
    }
  }

  useEffect(() => {
    if (task?.id) {
      refreshComments()
    }
  }, [task?.id])

  if (!task) return null

  const handleAddComment = async () => {
    if (!newComment.trim()) return
    setIsSubmittingComment(true)
    try {
      await taskService.addComment(task.id, newComment)
      setNewComment('')
      await refreshComments()
    } catch (error) {
      console.error('Error adding comment:', error)
    } finally {
      setIsSubmittingComment(false)
    }
  }

  const handleSave = async () => {
    setIsUpdating(true)
    await onUpdate(task.id, {
      title: formData.title,
      description: formData.description,
      priority: formData.priority as TaskPriority,
      status: formData.status as TaskStatus,
      start_date: formData.start_date || undefined,
      end_date: formData.end_date || undefined,
    })
    setIsEditing(false)
    setIsUpdating(false)
  }

  const handleStatusChange = async (newStatus: string) => {
    await onUpdate(task.id, { status: newStatus as Task['status'] })
  }

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
    <div className="task-detail-panel">
      <div className="panel-header">
        <h2>{isEditing ? 'Editar Tarea' : 'Detalles de la tarea'}</h2>
        <button className="close-btn" onClick={onClose}>✕</button>
      </div>

      <div className="panel-content">
        {isEditing ? (
          <div className="edit-form">
            <div className="form-group">
              <label>Título</label>
              <input
                type="text"
                value={formData.title}
                onChange={(e) => setFormData({ ...formData, title: e.target.value })}
              />
            </div>
            <div className="form-group">
              <label>Descripción</label>
              <RichTextEditor
                value={formData.description}
                onChange={(value) => setFormData({ ...formData, description: value })}
              />
            </div>
            <div className="form-row">
              <div className="form-group">
                <label>Prioridad</label>
                <select
                  value={formData.priority}
                  onChange={(e) => setFormData({ ...formData, priority: e.target.value })}
                >
                  <option value="low">Baja</option>
                  <option value="medium">Media</option>
                  <option value="high">Alta</option>
                  <option value="urgent">Urgente</option>
                </select>
              </div>
              <div className="form-group">
                <label>Estado</label>
                <select
                  value={formData.status}
                  onChange={(e) => setFormData({ ...formData, status: e.target.value })}
                >
                  <option value="por_hacer">Por hacer</option>
                  <option value="en_proceso">En proceso</option>
                  <option value="finalizado">Finalizado</option>
                </select>
              </div>
            </div>
            <div className="form-row">
              <div className="form-group">
                <label>Fecha inicio</label>
                <input
                  type="date"
                  value={formData.start_date}
                  onChange={(e) => setFormData({ ...formData, start_date: e.target.value })}
                />
              </div>
              <div className="form-group">
                <label>Fecha límite</label>
                <input
                  type="date"
                  value={formData.end_date}
                  onChange={(e) => setFormData({ ...formData, end_date: e.target.value })}
                />
              </div>
            </div>
            <div className="form-group">
              <label>Asignados</label>
              <div className="assignees-edit-list">
                {formData.assignees.length > 0 && (
                  <div className="assigned-chips">
                    {formData.assignees.map((id) => {
                      const user = users.find(u => u.id === id)
                      if (!user) return null
                      return (
                        <div key={user.id} className="assignee-chip">
                          <span>{user.name || user.email}</span>
                          <button
                            type="button"
                            className="remove-assignee"
                            onClick={() => setFormData({
                              ...formData,
                              assignees: formData.assignees.filter(aid => aid !== user.id)
                            })}
                          >
                            ×
                          </button>
                        </div>
                      )
                    })}
                  </div>
                )}
                <div className="assignee-dropdown-wrapper">
                  <input
                    type="text"
                    className="assignee-search"
                    placeholder="Buscar usuario..."
                    value={assigneeSearch}
                    onChange={(e) => setAssigneeSearch(e.target.value)}
                  />
                  <div className="assignee-dropdown">
                    {users
                      .filter(u => 
                        u.name?.toLowerCase().includes(assigneeSearch.toLowerCase()) ||
                        u.email?.toLowerCase().includes(assigneeSearch.toLowerCase())
                      )
                      .sort((a, b) => (a.name || a.email).localeCompare(b.name || b.email))
                      .map(user => {
                        const isAssigned = formData.assignees.includes(user.id)
                        return (
                          <div
                            key={user.id}
                            className={`assignee-option ${isAssigned ? 'assigned' : ''}`}
                            onClick={() => {
                              if (isAssigned) {
                                setFormData({
                                  ...formData,
                                  assignees: formData.assignees.filter(aid => aid !== user.id)
                                })
                              } else {
                                setFormData({
                                  ...formData,
                                  assignees: [...formData.assignees, user.id]
                                })
                              }
                            }}
                          >
                            <div className="chip-avatar">{user.name?.charAt(0).toUpperCase() || '?'}</div>
                            <span className="assignee-name">{user.name || user.email}</span>
                            {isAssigned && <span className="assignee-check">✓</span>}
                          </div>
                        )
                      })}
                  </div>
                </div>
              </div>
            </div>
            <div className="form-actions">
              <button onClick={() => setIsEditing(false)} disabled={isUpdating}>Cancelar</button>
              <button className="btn-primary" onClick={handleSave} disabled={isUpdating}>
                {isUpdating ? 'Guardando...' : 'Guardar'}
              </button>
            </div>
          </div>
        ) : (
          <>
            <div className="task-status-bar">
              <select
                value={task.status}
                onChange={(e) => handleStatusChange(e.target.value)}
                className="status-select"
              >
                {columns.map((col) => (
                  <option key={col.id} value={col.id}>{col.title}</option>
                ))}
              </select>
              <span
                className="priority-badge"
                style={{ backgroundColor: getPriorityColor(task.priority) }}
              >
                {task.priority}
              </span>
            </div>

            <h3 className="task-title">{task.title}</h3>

            <div className="task-section">
              <h4>Descripción</h4>
              {task.description ? (
                <div className="task-description-html" dangerouslySetInnerHTML={{ __html: task.description }} />
              ) : (
                <p>Sin descripción</p>
              )}
            </div>

            <div className="task-dates-row">
              {task.start_date && (
                <div className="date-item">
                  <span className="date-label">Inicio</span>
                  <span>{new Date(task.start_date).toLocaleDateString('es-ES', { weekday: 'short', day: 'numeric', month: 'short' })}</span>
                </div>
              )}
              {task.end_date && (
                <div className="date-item">
                  <span className="date-label">Fin</span>
                  <span>{new Date(task.end_date).toLocaleDateString('es-ES', { weekday: 'short', day: 'numeric', month: 'short' })}</span>
                </div>
              )}
            </div>

            <div className="task-section">
              <h4>Asignados</h4>
              <div className="assignees-list">
                {task.assignees && task.assignees.length > 0 ? (
                  task.assignees.map((user) => (
                    <div key={user.id} className="assignee-item">
                      <div className="assignee-avatar large">
                        {user.name.charAt(0).toUpperCase()}
                      </div>
                      <span>{user.name}</span>
                    </div>
                  ))
                ) : (
                  <span className="no-data">Sin asignar</span>
                )}
              </div>
            </div>

            <div className="task-section">
              <h4>Comentarios ({taskComments.length || 0})</h4>
              <div className="add-comment">
                <textarea
                  placeholder="Añadir un comentario..."
                  value={newComment}
                  onChange={(e) => setNewComment(e.target.value)}
                  rows={2}
                />
                <button 
                  className="btn-add-comment" 
                  onClick={handleAddComment}
                  disabled={!newComment.trim() || isSubmittingComment}
                >
                  {isSubmittingComment ? 'Publicando...' : 'Publicar'}
                </button>
              </div>
              <div className="comments-section">
                {taskComments.length > 0 ? (
                  taskComments.map((comment) => (
                    <div key={comment.id} className="comment-item">
                      <div className="comment-avatar">
                        {comment.user?.name?.charAt(0).toUpperCase() || '?'}
                      </div>
                      <div className="comment-content">
                        <span className="comment-author">{comment.user?.name || 'Usuario'}</span>
                        <p>{comment.content}</p>
                        <span className="comment-date">
                          {new Date(comment.created_at).toLocaleDateString()}
                        </span>
                      </div>
                    </div>
                  ))
                ) : (
                  <span className="no-data">No hay comentarios aún</span>
                )}
              </div>
            </div>

            <div className="panel-actions">
              <button className="btn-edit" onClick={() => setIsEditing(true)}>
                ✏️ Editar
              </button>
              <button className="btn-delete" onClick={() => {
                if (confirm('¿Eliminar esta tarea?')) {
                  setIsDeleting(true)
                  onDelete(task.id)
                  onClose()
                }
              }} disabled={isDeleting}>
                {isDeleting ? 'Eliminando...' : '🗑️ Eliminar'}
              </button>
            </div>
          </>
        )}
      </div>
    </div>
  )
}
