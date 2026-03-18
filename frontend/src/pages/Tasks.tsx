import { useState, useEffect } from 'react'
import { 
  DndContext,
  DragOverlay,
  closestCorners,
  KeyboardSensor,
  PointerSensor,
  useSensor,
  useSensors,
  useDroppable,
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
import { taskService, userService, boardService } from '../services/api'
import type { Task, User, TaskStatus, TaskPriority, CreateTaskInput, Board } from '../types'
import './Tasks.css'

interface RichTextEditorProps {
  value: string
  onChange: (value: string) => void
  placeholder?: string
}

function RichTextEditor({ value, onChange, placeholder }: RichTextEditorProps) {
  return (
    <div className="rich-text-editor">
      <div className="rich-text-toolbar">
        <button type="button" onClick={() => {
          const selection = window.getSelection()?.toString() || ''
          if (selection) onChange(value + `<strong>${selection}</strong>`)
        }} title="Negrita">
          <strong>B</strong>
        </button>
        <button type="button" onClick={() => {
          const selection = window.getSelection()?.toString() || ''
          if (selection) onChange(value + `<em>${selection}</em>`)
        }} title="Cursiva">
          <em>I</em>
        </button>
        <button type="button" onClick={() => {
          const selection = window.getSelection()?.toString() || ''
          if (selection) onChange(value + `<u>${selection}</u>`)
        }} title="Subrayado">
          <u>U</u>
        </button>
        <span className="toolbar-separator">|</span>
        <button type="button" onClick={() => onChange(value + '<ul>\n  <li>Elemento 1</li>\n  <li>Elemento 2</li>\n</ul>')} title="Viñetas">
          •
        </button>
        <button type="button" onClick={() => onChange(value + '<ol>\n  <li>Elemento 1</li>\n  <li>Elemento 2</li>\n</ol>')} title="Numeración">
          1.
        </button>
        <span className="toolbar-separator">|</span>
        <button type="button" onClick={() => {
          const url = prompt('Ingresa la URL:')
          if (url) onChange(value + `<a href="${url}">${url}</a>`)
        }} title="Enlace">
          🔗
        </button>
        <button type="button" onClick={() => onChange(value + '<h2>Título</h2>')} title="Título">
          H2
        </button>
        <button type="button" onClick={() => onChange(value + '<p>Párrafo</p>')} title="Párrafo">
          P
        </button>
      </div>
      <textarea
        className="rich-text-content"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder || 'Escribe aquí...'}
        rows={8}
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
  const { setNodeRef, isOver } = useDroppable({
    id: column.id,
  })

  return (
    <div className={`kanban-column ${isOver ? 'drag-over' : ''}`}>
      <div className="column-header">
        <div className="column-title">
          <span className="column-dot" style={{ backgroundColor: column.color }} />
          <span>{column.title}</span>
        </div>
        <span className="column-count">{tasks.length}</span>
      </div>
      <SortableContext items={tasks.map(t => t.id)} strategy={verticalListSortingStrategy}>
        <div className="column-content" ref={setNodeRef}>
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
                  {isSubmittingComment ? '...' : 'Publicar'}
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
  const [boards, setBoards] = useState<Board[]>([])
  const [selectedBoard, setSelectedBoard] = useState<Board | null>(null)
  const [showAllTasks, setShowAllTasks] = useState(false)
  const [isLoading, setIsLoading] = useState(true)
  const [activeTask, setActiveTask] = useState<Task | null>(null)
  const [selectedTask, setSelectedTask] = useState<Task | null>(null)
  const [showNewTaskModal, setShowNewTaskModal] = useState(false)
  const [showBoardModal, setShowBoardModal] = useState(false)
  const [showBoardMembersModal, setShowBoardMembersModal] = useState(false)
  const [optimisticMembers, setOptimisticMembers] = useState<number[]>([])
  const [isCreatingTask, setIsCreatingTask] = useState(false)
  const [newBoardData, setNewBoardData] = useState({
    name: '',
    description: '',
    color: '#3b82f6',
    member_ids: [] as number[],
  })
  const [newTaskData, setNewTaskData] = useState({
    title: '',
    description: '',
    priority: 'medium',
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

  useEffect(() => {
    fetchTasks()
  }, [selectedBoard])

  const fetchData = async () => {
    try {
      const [usersRes, boardsRes] = await Promise.all([
        userService.getAll(),
        boardService.getAll(),
      ])
      setUsers(usersRes.data)
      setBoards(boardsRes)
      if (boardsRes.length > 0 && !selectedBoard) {
        setSelectedBoard(boardsRes[0])
      }
    } catch (error) {
      console.error('Error fetching data:', error)
    } finally {
      setIsLoading(false)
    }
  }

  const fetchTasks = async () => {
    try {
      const params: Record<string, unknown> = {}
      if (!showAllTasks && selectedBoard) {
        params.board_id = selectedBoard.id
      }
      const tasksRes = await taskService.getAll(params)
      setTasks(tasksRes.data)
    } catch (error) {
      console.error('Error fetching tasks:', error)
    }
  }

  useEffect(() => {
    fetchTasks()
  }, [selectedBoard, showAllTasks])

  const handleCreateBoard = async () => {
    if (!newBoardData.name.trim()) return
    try {
      const newBoard = await boardService.create({
        name: newBoardData.name,
        description: newBoardData.description,
        color: newBoardData.color,
        member_ids: newBoardData.member_ids,
      })
      setNewBoardData({ name: '', description: '', color: '#3b82f6', member_ids: [] })
      setShowBoardModal(false)
      
      const boardsRes = await boardService.getAll()
      setBoards(boardsRes)
      setSelectedBoard(newBoard)
    } catch (error) {
      console.error('Error creating board:', error)
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
    const activeTask = tasks.find((t) => t.id === activeId)

    if (!activeTask) return

    const overId = over.id
    let newStatus: string | undefined

    if (typeof overId === 'string') {
      const column = COLUMNS.find(c => c.id === overId)
      if (column) {
        newStatus = column.id
      }
    } else {
      const overTask = tasks.find((t) => t.id === Number(overId))
      if (overTask) {
        newStatus = overTask.status
      }
    }

    if (newStatus && activeTask.status !== newStatus) {
      setTasks((tasks) =>
        tasks.map((t) =>
          t.id === activeId ? { ...t, status: newStatus as TaskStatus } : t
        )
      )
    }
  }

  const handleDragEnd = async (event: DragEndEvent) => {
    const { active, over } = event
    setActiveTask(null)

    if (!over) return

    const activeId = Number(active.id)
    const overId = over.id

    const activeTask = tasks.find((t) => t.id === activeId)
    if (!activeTask) return

    let newStatus: string | undefined

    if (typeof overId === 'string') {
      const column = COLUMNS.find(c => c.id === overId)
      if (column) {
        newStatus = column.id
      }
    } else {
      const overTask = tasks.find((t) => t.id === Number(overId))
      if (overTask) {
        newStatus = overTask.status
      }
    }

    if (newStatus && newStatus !== activeTask.status) {
      console.log('Updating task status:', activeId, 'to', newStatus)
      try {
        await taskService.update(activeId, { status: newStatus as TaskStatus })
        setTasks((tasks) =>
          tasks.map((t) =>
            t.id === activeId ? { ...t, status: newStatus as TaskStatus } : t
          )
        )
      } catch (error) {
        console.error('Error updating task status:', error)
        fetchTasks()
      }
    }
  }

  const handleCreateTask = async (e: React.FormEvent) => {
    e.preventDefault()
    try {
      const taskData: CreateTaskInput = {
        title: newTaskData.title,
        description: newTaskData.description || undefined,
        priority: newTaskData.priority || undefined,
        board_id: selectedBoard?.id,
      }
      
      if (newTaskData.end_date) {
        taskData.end_date = newTaskData.end_date
      }
      
      if (newTaskData.assignees.length > 0) {
        taskData.assignees = newTaskData.assignees
      }
      
      console.log('Creating task with data:', taskData)
      setIsCreatingTask(true)
      await taskService.create(taskData)
      setShowNewTaskModal(false)
      setNewTaskData({
        title: '',
        description: '',
        priority: 'medium',
        end_date: '',
        assignees: [],
      })
      fetchTasks()
    } catch (error) {
      console.error('Error creating task:', error)
    } finally {
      setIsCreatingTask(false)
    }
  }

  const handleUpdateTask = async (id: number, data: Partial<Task>) => {
    try {
      await taskService.update(id, data)
      fetchTasks()
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
      fetchTasks()
    } catch (error) {
      console.error('Error deleting task:', error)
    }
  }

  if (isLoading) {
    return (
      <div className="tasks-page">
        <div className="tasks-loading">
          <div className="spinner" />
          <p>Cargando...</p>
        </div>
      </div>
    )
  }

  if (boards.length === 0) {
    return (
      <div className="tasks-page">
        <div className="page-header">
          <h1>📋 Tareas</h1>
        </div>
        <div className="tasks-loading">
          <div style={{ fontSize: '48px', marginBottom: '16px' }}>📋</div>
          <h2>No tienes tableros</h2>
          <p>Crea tu primer tablero para organizar tus tareas</p>
          <button className="btn-primary" onClick={() => setShowBoardModal(true)}>
            + Crear Tablero
          </button>
        </div>
        {showBoardModal && (
          <div className="modal-overlay" onClick={() => setShowBoardModal(false)}>
            <div className="modal board-modal" onClick={(e) => e.stopPropagation()}>
              <h2>Crear Tablero</h2>
              <form onSubmit={(e) => { e.preventDefault(); handleCreateBoard() }}>
                <div className="form-group">
                  <label>Nombre del tablero</label>
                  <input
                    type="text"
                    value={newBoardData.name}
                    onChange={(e) => setNewBoardData({ ...newBoardData, name: e.target.value })}
                    placeholder="Ej: Marketing, IT, Diseño..."
                    required
                  />
                </div>
                <div className="form-group">
                  <label>Descripción</label>
                  <textarea
                    value={newBoardData.description}
                    onChange={(e) => setNewBoardData({ ...newBoardData, description: e.target.value })}
                    placeholder="Descripción opcional..."
                    rows={3}
                  />
                </div>
                <div className="form-group">
                  <label>Color</label>
                  <input
                    type="color"
                    value={newBoardData.color}
                    onChange={(e) => setNewBoardData({ ...newBoardData, color: e.target.value })}
                  />
                </div>
                <div className="modal-actions">
                  <button type="button" onClick={() => setShowBoardModal(false)}>Cancelar</button>
                  <button type="submit" className="btn-primary">Crear Tablero</button>
                </div>
              </form>
            </div>
          </div>
        )}
      </div>
    )
  }

  return (
    <div className="tasks-page">
      <div className="page-header">
        <div className="header-left">
          <h1>📋 Tareas</h1>
          <div className="board-selector">
            {boards.length > 0 ? (
              <>
                <select 
                  value={showAllTasks ? 'all' : (selectedBoard?.id || '')} 
                  onChange={(e) => {
                    if (e.target.value === 'all') {
                      setShowAllTasks(true)
                      setSelectedBoard(null)
                    } else {
                      setShowAllTasks(false)
                      const board = boards.find(b => b.id === Number(e.target.value))
                      setSelectedBoard(board || null)
                    }
                  }}
                  style={{ borderLeftColor: showAllTasks ? '#6366f1' : (selectedBoard?.color || '#3b82f6') }}
                >
                  <option value="all">📋 Todos los tableros</option>
                  {boards.map(board => (
                    <option key={board.id} value={board.id}>{board.name}</option>
                  ))}
                </select>
                <button className="btn-icon" onClick={() => setShowBoardModal(true)} title="Crear tablero">
                  ➕
                </button>
                {selectedBoard && (
                  <button 
                    className="btn-icon" 
                    onClick={() => {
                      setOptimisticMembers(selectedBoard.members?.map(m => m.id) || [])
                      setShowBoardMembersModal(true)
                    }}
                    title="Gestionar miembros"
                    style={{ marginLeft: '4px' }}
                  >
                    👥
                  </button>
                )}
              </>
            ) : (
              <button className="btn-primary btn-sm" onClick={() => setShowBoardModal(true)}>
                + Crear Primer Tablero
              </button>
            )}
          </div>
        </div>
        <button className="btn-primary" onClick={() => setShowNewTaskModal(true)}>
          + Nueva Tarea
        </button>
      </div>

      {showBoardModal && (
        <div className="modal-overlay" onClick={() => setShowBoardModal(false)}>
          <div className="modal board-modal" onClick={(e) => e.stopPropagation()}>
            <h2>Crear Tablero</h2>
            <form onSubmit={(e) => { e.preventDefault(); handleCreateBoard() }}>
              <div className="form-group">
                <label>Nombre del tablero</label>
                <input
                  type="text"
                  value={newBoardData.name}
                  onChange={(e) => setNewBoardData({ ...newBoardData, name: e.target.value })}
                  placeholder="Ej: Marketing, IT, Diseño..."
                  required
                />
              </div>
              <div className="form-group">
                <label>Descripción</label>
                <textarea
                  value={newBoardData.description}
                  onChange={(e) => setNewBoardData({ ...newBoardData, description: e.target.value })}
                  placeholder="Descripción opcional..."
                  rows={2}
                />
              </div>
              <div className="form-group">
                <label>Color</label>
                <input
                  type="color"
                  value={newBoardData.color}
                  onChange={(e) => setNewBoardData({ ...newBoardData, color: e.target.value })}
                />
              </div>
              <div className="form-group">
                <label>Agregar miembros (opcional)</label>
                <div className="members-select">
                  {users.map(user => (
                    <label key={user.id} className="member-checkbox">
                      <div className="left-section">
                        <input
                          type="checkbox"
                          checked={newBoardData.member_ids.includes(user.id)}
                          onChange={(e) => {
                            if (e.target.checked) {
                              setNewBoardData({
                                ...newBoardData,
                                member_ids: [...newBoardData.member_ids, user.id]
                              })
                            } else {
                              setNewBoardData({
                                ...newBoardData,
                                member_ids: newBoardData.member_ids.filter(id => id !== user.id)
                              })
                            }
                          }}
                        />
                        <span className="checkbox-custom"></span>
                        <div className="member-avatar">
                          {user.name.split(' ').map(n => n[0]).join('').substring(0, 2).toUpperCase()}
                        </div>
                        <div className="member-info">
                          <span className="member-name">{user.name}</span>
                          <span className="member-role">{user.user_type === 'empleado' ? 'Profesional' : user.user_type}</span>
                        </div>
                      </div>
                    </label>
                  ))}
                </div>
              </div>
              <div className="modal-actions">
                <button type="button" onClick={() => setShowBoardModal(false)}>Cancelar</button>
                <button type="submit" className="btn-primary">Crear Tablero</button>
              </div>
            </form>
          </div>
        </div>
      )}

      {showBoardMembersModal && selectedBoard && (
        <div className="modal-overlay" onClick={() => { setShowBoardMembersModal(false); setOptimisticMembers([]) }}>
          <div className="modal board-modal wide" onClick={(e) => e.stopPropagation()}>
            <h2>Miembros del Tablero</h2>
            <p style={{ color: '#64748b', marginBottom: '20px', marginTop: '-16px' }}>{selectedBoard.name}</p>
              <div className="form-group">
                <label>Selecciona los miembros que quieres agregar al tablero</label>
                <div className="members-select">
                  {users.map(user => {
                    const isMember = optimisticMembers.includes(user.id)
                    return (
                      <label key={user.id} className="member-checkbox">
                        <div className="left-section">
                          <input
                            type="checkbox"
                            checked={isMember}
                            onChange={async (e) => {
                              const newOptimisticMembers = e.target.checked
                                ? [...optimisticMembers, user.id]
                                : optimisticMembers.filter(id => id !== user.id)
                              setOptimisticMembers(newOptimisticMembers)
                              try {
                                await boardService.update(selectedBoard.id, { member_ids: newOptimisticMembers })
                                const boardsRes = await boardService.getAll()
                                setBoards(boardsRes)
                                const updated = boardsRes.find(b => b.id === selectedBoard.id)
                                if (updated) setSelectedBoard(updated)
                              } catch (error) {
                                console.error('Error updating board members:', error)
                                setOptimisticMembers(selectedBoard.members?.map(m => m.id) || [])
                              }
                            }}
                          />
                          <span className="checkbox-custom"></span>
                          <div className="member-avatar">
                            {user.name.split(' ').map(n => n[0]).join('').substring(0, 2).toUpperCase()}
                          </div>
                          <div className="member-info">
                            <span className="member-name">{user.name}</span>
                            <span className="member-role">{user.user_type === 'empleado' ? 'Profesional' : user.user_type}</span>
                          </div>
                        </div>
                        <span className={`member-status ${isMember ? 'active' : 'inactive'}`}>
                          {isMember ? 'En tablero' : 'No asignado'}
                        </span>
                      </label>
                    )
                  })}
                </div>
              </div>
            <div className="modal-actions">
              <button type="button" className="btn-primary" onClick={() => { setShowBoardMembersModal(false); setOptimisticMembers([]) }}>Listo</button>
            </div>
          </div>
        </div>
      )}

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
              <button type="button" onClick={() => setShowNewTaskModal(false)} className="btn-cancel" disabled={isCreatingTask}>
                Cancelar
              </button>
              <button type="submit" form="create-task-form" className="btn-create" disabled={isCreatingTask}>
                {isCreatingTask ? 'Creando...' : 'Crear tarea'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
