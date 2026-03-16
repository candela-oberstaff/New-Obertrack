import { useState, useEffect, useRef } from 'react'
import {
  DndContext,
  DragOverlay,
  closestCorners,
  KeyboardSensor,
  PointerSensor,
  useSensor,
  useSensors,
  type DragStartEvent,
  type DragEndEvent,
} from '@dnd-kit/core'
import {
  SortableContext,
  sortableKeyboardCoordinates,
  verticalListSortingStrategy,
  useSortable,
} from '@dnd-kit/sortable'
import { CSS } from '@dnd-kit/utilities'
import { taskService, userService } from '../services/api'
import type { Task, User, TaskStatus, TaskPriority } from '../types'
import './Tasks.css'

interface RichTextEditorProps {
  value: string
  onChange: (value: string) => void
  placeholder?: string
}

function RichTextEditor({ value, onChange, placeholder }: RichTextEditorProps) {
  const editorRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (editorRef.current && editorRef.current.innerHTML !== value) {
      editorRef.current.innerHTML = value
    }
  }, [])

  const handleInput = () => {
    if (editorRef.current) {
      onChange(editorRef.current.innerHTML)
    }
  }

  const execCommand = (command: string, value?: string) => {
    document.execCommand(command, false, value)
    if (editorRef.current) {
      onChange(editorRef.current.innerHTML)
    }
    editorRef.current?.focus()
  }

  return (
    <div className="rich-text-editor">
      <div className="rich-text-toolbar">
        <button type="button" onClick={() => execCommand('bold')} title="Negrita">
          <strong>B</strong>
        </button>
        <button type="button" onClick={() => execCommand('italic')} title="Cursiva">
          <em>I</em>
        </button>
        <button type="button" onClick={() => execCommand('underline')} title="Subrayado">
          <u>U</u>
        </button>
        <span className="toolbar-separator">|</span>
        <button type="button" onClick={() => execCommand('insertUnorderedList')} title="Viñetas">
          •
        </button>
        <button type="button" onClick={() => execCommand('insertOrderedList')} title="Numeración">
          1.
        </button>
        <span className="toolbar-separator">|</span>
        <button type="button" onClick={() => {
          const url = prompt('Ingresa la URL:')
          if (url) execCommand('createLink', url)
        }} title="Enlace">
          🔗
        </button>
        <button type="button" onClick={() => execCommand('formatBlock', 'h2')} title="Título">
          H2
        </button>
        <button type="button" onClick={() => execCommand('formatBlock', 'p')} title="Párrafo">
          P
        </button>
      </div>
      <div
        ref={editorRef}
        className="rich-text-content"
        contentEditable
        onInput={handleInput}
        onBlur={handleInput}
        dangerouslySetInnerHTML={{ __html: value }}
        data-placeholder={placeholder || 'Escribe aquí...'}
        style={{ minHeight: '100px' }}
      />
    </div>
  )
}

const COLUMNS = [
  { id: 'por_hacer', title: 'Por hacer', color: '#6b7280' },
  { id: 'en_proceso', title: 'En proceso', color: '#3b82f6' },
  { id: 'finalizado', title: 'Finalizado', color: '#22c55e' },
]

interface TaskCardProps {
  task: Task
  isDragging?: boolean
  onClick: () => void
}

function TaskCard({ task, isDragging, onClick }: TaskCardProps) {
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
            📅 {new Date(task.start_date).toLocaleDateString('es-ES', { day: 'numeric', month: 'short' })}
          </span>
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

function SortableTaskCard({ task, onClick }: SortableTaskCardProps) {
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

interface ColumnProps {
  column: typeof COLUMNS[0]
  tasks: Task[]
  onTaskClick: (task: Task) => void
}

function Column({ column, tasks, onTaskClick }: ColumnProps) {
  return (
    <div className="kanban-column">
      <div className="column-header">
        <div className="column-title">
          <span className="column-dot" style={{ backgroundColor: column.color }} />
          <span>{column.title}</span>
        </div>
        <span className="column-count">{tasks.length}</span>
      </div>
      <SortableContext items={tasks.map(t => t.id)} strategy={verticalListSortingStrategy}>
        <div className="column-content">
          {tasks.map((task) => (
            <SortableTaskCard key={task.id} task={task} onClick={() => onTaskClick(task)} />
          ))}
        </div>
      </SortableContext>
    </div>
  )
}

interface TaskDetailPanelProps {
  task: Task | null
  onClose: () => void
  onUpdate: (id: number, data: Partial<Task>) => Promise<void>
  onDelete: (id: number) => Promise<void>
}

function TaskDetailPanel({ task, onClose, onUpdate, onDelete }: TaskDetailPanelProps) {
  const [isEditing, setIsEditing] = useState(false)
  const [newComment, setNewComment] = useState('')
  const [isSubmittingComment, setIsSubmittingComment] = useState(false)
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
    }
  }, [task])

  if (!task) return null

  const handleAddComment = async () => {
    if (!newComment.trim()) return
    setIsSubmittingComment(true)
    try {
      await taskService.addComment(task.id, newComment)
      setNewComment('')
      onUpdate(task.id, {})
    } catch (error) {
      console.error('Error adding comment:', error)
    } finally {
      setIsSubmittingComment(false)
    }
  }

  const handleSave = async () => {
    await onUpdate(task.id, {
      title: formData.title,
      description: formData.description,
      priority: formData.priority as TaskPriority,
      status: formData.status as TaskStatus,
      start_date: formData.start_date || undefined,
      end_date: formData.end_date || undefined,
    })
    setIsEditing(false)
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
            <div className="form-actions">
              <button onClick={() => setIsEditing(false)}>Cancelar</button>
              <button className="btn-primary" onClick={handleSave}>Guardar</button>
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
                {COLUMNS.map((col) => (
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
                  <span className="date-label">📅 Inicio</span>
                  <span>{new Date(task.start_date).toLocaleDateString('es-ES', { weekday: 'short', day: 'numeric', month: 'short' })}</span>
                </div>
              )}
              {task.end_date && (
                <div className="date-item">
                  <span className="date-label">⏰ Fin</span>
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
              <h4>Comentarios ({task.comments?.length || 0})</h4>
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
                  {isSubmittingComment ? '...' : 'Publicar'}
                </button>
              </div>
              <div className="comments-section">
                {task.comments && task.comments.length > 0 ? (
                  task.comments.map((comment) => (
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
                  onDelete(task.id)
                  onClose()
                }
              }}>
                🗑️ Eliminar
              </button>
            </div>
          </>
        )}
      </div>
    </div>
  )
}

export default function Tasks() {
  const [tasks, setTasks] = useState<Task[]>([])
  const [users, setUsers] = useState<User[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [activeTask, setActiveTask] = useState<Task | null>(null)
  const [selectedTask, setSelectedTask] = useState<Task | null>(null)
  const [showNewTaskModal, setShowNewTaskModal] = useState(false)
  const [newTaskData, setNewTaskData] = useState({
    title: '',
    description: '',
    priority: 'medium',
    start_date: '',
    end_date: '',
    assignees: [] as number[],
  })
  const [assigneeSearch, setAssigneeSearch] = useState('')

  const filteredUsers = users.filter(user => 
    user.name?.toLowerCase().includes(assigneeSearch.toLowerCase()) ||
    user.email?.toLowerCase().includes(assigneeSearch.toLowerCase())
  )

  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 5 } }),
    useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates })
  )

  useEffect(() => {
    fetchData()
  }, [])

  const fetchData = async () => {
    try {
      const [tasksRes, usersRes] = await Promise.all([
        taskService.getAll({}),
        userService.getAll(),
      ])
      setTasks(tasksRes.data)
      setUsers(usersRes.data)
    } catch (error) {
      console.error('Error fetching data:', error)
    } finally {
      setIsLoading(false)
    }
  }

  const getTasksByStatus = (status: string) => {
    return tasks.filter((task) => task.status === status)
  }

  const handleDragStart = (event: DragStartEvent) => {
    const { active } = event
    const task = tasks.find((t) => t.id === active.id)
    if (task) setActiveTask(task)
  }

  const handleDragOver = (event: { active: { id: unknown }; over: { id: unknown } | null }) => {
    const { active, over } = event
    if (!over) return

    const activeId = Number(active.id)
    const overId = Number(over.id)

    const activeTask = tasks.find((t) => t.id === activeId)

    if (!activeTask) return

    const overTask = tasks.find((t) => t.id === overId)
    const overColumnId = String(overId)
    const newStatus = overTask?.status || (COLUMNS.find(c => c.id === overColumnId)?.id as TaskStatus)

    if (newStatus && activeTask.status !== newStatus) {
      setTasks((tasks) =>
        tasks.map((t) =>
          t.id === activeId ? { ...t, status: newStatus } : t
        )
      )
    }
  }

  const handleDragEnd = async (event: DragEndEvent) => {
    const { active, over } = event
    setActiveTask(null)

    if (!over) return

    const activeId = Number(active.id)
    const overId = Number(over.id)

    if (activeId === overId) return

    const activeTask = tasks.find((t) => t.id === activeId)
    const overTask = tasks.find((t) => t.id === overId)

    if (!activeTask) return

    let newStatus = activeTask.status

    if (overTask) {
      newStatus = overTask.status
    } else {
      const overColumnId = String(overId)
      const column = COLUMNS.find(col => col.id === overColumnId)
      if (column) {
        newStatus = column.id as TaskStatus
      }
    }

    if (newStatus !== activeTask.status) {
      try {
        await taskService.update(activeId, { status: newStatus })
        setTasks((tasks) =>
          tasks.map((t) =>
            t.id === activeId ? { ...t, status: newStatus } : t
          )
        )
      } catch (error) {
        console.error('Error updating task status:', error)
        fetchData()
      }
    }
  }

  const handleCreateTask = async (e: React.FormEvent) => {
    e.preventDefault()
    try {
      await taskService.create({
        title: newTaskData.title,
        description: newTaskData.description,
        priority: newTaskData.priority as TaskPriority,
        start_date: newTaskData.start_date || undefined,
        end_date: newTaskData.end_date || undefined,
        assignees: newTaskData.assignees as unknown as User[],
      })
      setShowNewTaskModal(false)
      setNewTaskData({
        title: '',
        description: '',
        priority: 'medium',
        start_date: '',
        end_date: '',
        assignees: [],
      })
      fetchData()
    } catch (error) {
      console.error('Error creating task:', error)
    }
  }

  const handleUpdateTask = async (id: number, data: Partial<Task>) => {
    try {
      await taskService.update(id, data)
      fetchData()
      if (selectedTask && selectedTask.id === id) {
        const updated = await taskService.getById(id)
        setSelectedTask(updated)
      }
    } catch (error) {
      console.error('Error updating task:', error)
    }
  }

  const handleDeleteTask = async (id: number) => {
    try {
      await taskService.delete(id)
      fetchData()
    } catch (error) {
      console.error('Error deleting task:', error)
    }
  }

  if (isLoading) {
    return (
      <div className="tasks-loading">
        <div className="spinner" />
        <p>Cargando tareas...</p>
      </div>
    )
  }

  return (
    <div className="tasks-page">
      <div className="page-header">
        <h1>📋 Tareas</h1>
        <button className="btn-primary" onClick={() => setShowNewTaskModal(true)}>
          + Nueva Tarea
        </button>
      </div>

      <DndContext
        sensors={sensors}
        collisionDetection={closestCorners}
        onDragStart={handleDragStart}
        onDragOver={handleDragOver}
        onDragEnd={handleDragEnd}
      >
        <div className="kanban-board">
          {COLUMNS.map((column) => (
            <Column
              key={column.id}
              column={column}
              tasks={getTasksByStatus(column.id)}
              onTaskClick={setSelectedTask}
            />
          ))}
        </div>

        <DragOverlay>
          {activeTask ? (
            <TaskCard task={activeTask} onClick={() => {}} />
          ) : null}
        </DragOverlay>
      </DndContext>

      {selectedTask && (
        <TaskDetailPanel
          task={selectedTask}
          onClose={() => setSelectedTask(null)}
          onUpdate={handleUpdateTask}
          onDelete={handleDeleteTask}
        />
      )}

      {showNewTaskModal && (
        <div className="modal-overlay" onClick={() => setShowNewTaskModal(false)}>
          <div className="new-task-modal" onClick={(e) => e.stopPropagation()}>
            <div className="new-task-header">
              <h2>Crear nueva tarea</h2>
              <button className="close-btn" onClick={() => setShowNewTaskModal(false)}>
                <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                  <path d="M18 6L6 18M6 6l12 12" />
                </svg>
              </button>
            </div>
            
            <form onSubmit={handleCreateTask} id="create-task-form" className="new-task-content">
              <div className="task-form-main">
                <div className="form-field">
                  <label>Título</label>
                  <input
                    type="text"
                    value={newTaskData.title}
                    onChange={(e) => setNewTaskData({ ...newTaskData, title: e.target.value })}
                    placeholder="¿Qué necesitas hacer?"
                    className="title-field"
                    required
                    autoFocus
                  />
                </div>
                
                <div className="form-field">
                  <label>Descripción</label>
                  <RichTextEditor
                    value={newTaskData.description}
                    onChange={(value) => setNewTaskData({ ...newTaskData, description: value })}
                    placeholder="Agrega detalles, listas, enlaces..."
                  />
                </div>
              </div>
              
              <div className="task-form-side">
                <div className="settings-card">
                  <h3>Configuración</h3>
                  
                  <div className="setting-item">
                    <label>Prioridad</label>
                    <select
                      value={newTaskData.priority}
                      onChange={(e) => setNewTaskData({ ...newTaskData, priority: e.target.value })}
                      className="setting-select"
                    >
                      <option value="low">🟢 Baja</option>
                      <option value="medium">🟡 Media</option>
                      <option value="high">🟠 Alta</option>
                      <option value="urgent">🔴 Urgente</option>
                    </select>
                  </div>
                  
                  <div className="setting-item">
                    <label>Fecha de inicio</label>
                    <input
                      type="date"
                      value={newTaskData.start_date}
                      onChange={(e) => setNewTaskData({ ...newTaskData, start_date: e.target.value })}
                      className="setting-input"
                    />
                  </div>
                  
                  <div className="setting-item">
                    <label>Fecha límite</label>
                    <input
                      type="date"
                      value={newTaskData.end_date}
                      onChange={(e) => setNewTaskData({ ...newTaskData, end_date: e.target.value })}
                      className="setting-input"
                    />
                  </div>
                </div>
                
                <div className="assign-card">
                  <h3>Asignar a</h3>
                  <input
                    type="text"
                    placeholder="Buscar usuario..."
                    value={assigneeSearch}
                    onChange={(e) => setAssigneeSearch(e.target.value)}
                    className="assignee-search"
                  />
                  <div className="assignees-scroll">
                    {filteredUsers.length === 0 ? (
                      <p className="no-users">No hay usuarios disponibles</p>
                    ) : (
                      filteredUsers.map((user) => (
                        <label key={user.id} className="assign-option">
                          <input
                            type="checkbox"
                            checked={newTaskData.assignees.includes(user.id)}
                            onChange={(e) => {
                              if (e.target.checked) {
                                setNewTaskData({
                                  ...newTaskData,
                                  assignees: [...newTaskData.assignees, user.id],
                                })
                              } else {
                                setNewTaskData({
                                  ...newTaskData,
                                  assignees: newTaskData.assignees.filter((id) => id !== user.id),
                                })
                              }
                            }}
                          />
                          <span className="user-chip">
                            <span className="chip-avatar">{user.name?.charAt(0).toUpperCase()}</span>
                            <span className="chip-name">{user.name}</span>
                          </span>
                        </label>
                      ))
                    )}
                  </div>
                </div>
              </div>
            </form>
            
            <div className="new-task-footer">
              <button type="button" onClick={() => setShowNewTaskModal(false)} className="btn-cancel">
                Cancelar
              </button>
              <button type="submit" form="create-task-form" className="btn-create">
                Crear tarea
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
