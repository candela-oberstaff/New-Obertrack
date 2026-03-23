import { useState, useEffect, useRef } from 'react'
import { useAuth } from '../context/AuthContext'
import { useNotification } from '../context/NotificationContext'
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
          Link
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

interface ColumnType {
  id: string
  title: string
  color: string
}

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
            {new Date(task.start_date).toLocaleDateString('es-ES', { day: 'numeric', month: 'short' })}
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
  column: ColumnType
  tasks: Task[]
  onTaskClick: (task: Task) => void
  isDragging?: boolean
}

function Column({ column, tasks, onTaskClick, onMoveLeft, onMoveRight, canMoveLeft, canMoveRight }: ColumnProps & { onMoveLeft?: () => void, onMoveRight?: () => void, canMoveLeft?: boolean, canMoveRight?: boolean }) {
  const { setNodeRef, isOver } = useDroppable({
    id: column.id,
  })

  return (
    <div 
      className={`kanban-column ${isOver ? 'drag-over' : ''}`}
    >
      <div className="column-header">
        <div className="column-title">
          <span className="column-dot" style={{ backgroundColor: column.color }} />
          <span>{column.title}</span>
        </div>
        <div className="column-actions">
          <button 
            className="column-move-btn" 
            onClick={onMoveLeft}
            disabled={!canMoveLeft}
            title="Mover a la izquierda"
          >←</button>
          <button 
            className="column-move-btn" 
            onClick={onMoveRight}
            disabled={!canMoveRight}
            title="Mover a la derecha"
          >→</button>
          <span className="column-count">{tasks.length}</span>
        </div>
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
  users: User[]
  onClose: () => void
  onUpdate: (id: number, data: Partial<Task>) => Promise<void>
  onDelete: (id: number) => Promise<void>
  columns: ColumnType[]
}

function TaskDetailPanel({ task, users, onClose, onUpdate, onDelete, columns }: TaskDetailPanelProps) {
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

export default function Tasks() {
  const { user } = useAuth()
  const { error: showError } = useNotification()
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
  const [showPhasesModal, setShowPhasesModal] = useState(false)
  const [optimisticMembers, setOptimisticMembers] = useState<number[]>([])
  const [isCreatingTask, setIsCreatingTask] = useState(false)
  const [isCreatingBoard, setIsCreatingBoard] = useState(false)
  const [isDeletingBoard, setIsDeletingBoard] = useState(false)
  const [isAddingPhase, setIsAddingPhase] = useState(false)
  const [isDeletingPhase, setIsDeletingPhase] = useState(false)
  const [draggingPhase, setDraggingPhase] = useState<number | null>(null)
  const [dragOverIdx, setDragOverIdx] = useState<number | null>(null)
  const phasesOrderRef = useRef<Board['phases']>([])
  const draggingPhaseRef = useRef<number | null>(null)
  const isDraggingPhasesRef = useRef(false)
  const dragOriginalStatusRef = useRef<string | null>(null)
  const [newBoardData, setNewBoardData] = useState({
    name: '',
    description: '',
    color: '#3b82f6',
    member_ids: [] as number[],
    phases: [
      { name: 'Por hacer', color: '#6b7280' },
      { name: 'En proceso', color: '#3b82f6' },
      { name: 'Finalizado', color: '#22c55e' },
    ],
  })
  const [newBoardPhaseSearch, setNewBoardPhaseSearch] = useState('')
  const [newTaskData, setNewTaskData] = useState({
    title: '',
    description: '',
    priority: 'medium',
    end_date: '',
    assignees: [] as number[],
  })
  const [assigneeSearch, setAssigneeSearch] = useState('')
  const [localColumnOrder, setLocalColumnOrder] = useState<string[] | null>(() => {
    const saved = localStorage.getItem('columnOrder')
    return saved ? JSON.parse(saved) : null
  })

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
    if (localColumnOrder) {
      localStorage.setItem('columnOrder', JSON.stringify(localColumnOrder))
    }
  }, [localColumnOrder])

  useEffect(() => {
    const handleTaskAssigned = () => {
      fetchTasks()
    }
    window.addEventListener('task-assigned', handleTaskAssigned)
    return () => window.removeEventListener('task-assigned', handleTaskAssigned)
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
      setUsers(usersRes.data || [])
      setBoards(boardsRes || [])
      if (boardsRes && boardsRes.length > 0 && !selectedBoard) {
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
    setIsCreatingBoard(true)
    try {
      const newBoard = await boardService.create({
        name: newBoardData.name,
        description: newBoardData.description,
        color: newBoardData.color,
        member_ids: newBoardData.member_ids,
        phases: newBoardData.phases,
      })
      setNewBoardData({ 
        name: '', 
        description: '', 
        color: '#3b82f6', 
        member_ids: [], 
        phases: [
          { name: 'Por hacer', color: '#6b7280' },
          { name: 'En proceso', color: '#3b82f6' },
          { name: 'Finalizado', color: '#22c55e' },
        ],
      })
      setShowBoardModal(false)
      
      const boardsRes = await boardService.getAll()
      setBoards(boardsRes)
      setSelectedBoard(newBoard)
    } catch (error) {
      console.error('Error creating board:', error)
    } finally {
      setIsCreatingBoard(false)
    }
  }

  const getTasksByStatus = (status: string) => {
    return tasks.filter((task) => task.status === status)
  }

  const getCurrentColumns = (): ColumnType[] => {
    if (selectedBoard?.phases?.length) {
      return selectedBoard.phases.map(p => ({
        id: p.status || p.name.toLowerCase().replace(/\s+/g, '_'),
        title: p.name,
        color: p.color
      }))
    }
    return COLUMNS
  }

  const handleDragStart = (event: DragStartEvent) => {
    const { active } = event
    const task = tasks.find((t) => t.id === active.id)
    if (task) {
      setActiveTask(task)
      dragOriginalStatusRef.current = task.status
    }
  }

  const handleDragOver = (event: { active: { id: unknown }; over: { id: unknown } | null }) => {
    const { active, over } = event
    if (!over) return

    const activeId = Number(active.id)
    const overId = over.id

    if (typeof overId !== 'string') return

    const column = getCurrentColumns().find(c => c.id === overId)
    if (!column) return

    const activeTask = tasks.find((t) => t.id === activeId)
    if (!activeTask || activeTask.status === column.id) return

    setTasks((prevTasks) =>
      prevTasks.map((t) =>
        t.id === activeId ? { ...t, status: column.id as TaskStatus } : t
      )
    )
  }

  const handleDragEnd = async (event: DragEndEvent) => {
    const { active, over } = event
    setActiveTask(null)

    const originalStatus = dragOriginalStatusRef.current
    dragOriginalStatusRef.current = null

    if (!over) return

    const activeId = Number(active.id)
    const overId = over.id

    const activeTask = tasks.find((t) => t.id === activeId)
    if (!activeTask) return

    let newStatus: string | undefined

    if (typeof overId === 'string') {
      const column = getCurrentColumns().find(c => c.id === overId)
      if (column) {
        newStatus = column.id
      }
    } else {
      const overTask = tasks.find((t) => t.id === Number(overId))
      if (overTask) {
        newStatus = overTask.status
      }
    }

    if (newStatus && newStatus !== originalStatus) {
      console.log('[DragEnd] Updating task:', activeId, 'to status:', newStatus)
      try {
        await taskService.update(activeId, { status: newStatus as TaskStatus })
        console.log('[DragEnd] Update successful')
      } catch (error: any) {
        console.error('[DragEnd] Error updating task status:', error?.response?.data || error)
        setTasks((prevTasks) =>
          prevTasks.map((t) =>
            t.id === activeId ? { ...t, status: activeTask.status } : t
          )
        )
      }
    }
  }

  const handlePhaseDragStart = (phaseId: number) => {
    isDraggingPhasesRef.current = true
    setDraggingPhase(phaseId)
    draggingPhaseRef.current = phaseId
    if (selectedBoard?.phases) {
      phasesOrderRef.current = [...selectedBoard.phases]
    }
  }

  const handlePhaseDragEnter = (targetIdx: number) => {
    if (!isDraggingPhasesRef.current || draggingPhaseRef.current === null || !selectedBoard?.phases) return
    
    const phases = [...selectedBoard.phases]
    const dragIdx = phases.findIndex(p => p.id === draggingPhaseRef.current)
    if (dragIdx === -1 || dragIdx === targetIdx) return
    
    const [dragged] = phases.splice(dragIdx, 1)
    phases.splice(targetIdx, 0, dragged)
    
    phasesOrderRef.current = phases
    setDragOverIdx(targetIdx)
    setSelectedBoard((prev: Board | null) => {
      if (!prev) return prev
      return { ...prev, phases }
    })
  }

const handlePhaseDragEnd = async () => {
    isDraggingPhasesRef.current = false
    setDraggingPhase(null)
    setDragOverIdx(null)
    draggingPhaseRef.current = null
    
    if (!selectedBoard?.phases || !selectedBoard?.id) return
    
    const phaseIds = selectedBoard.phases.map(p => p.id)
    
    try {
      await boardService.reorderPhases(selectedBoard.id, phaseIds)
      const boardsRes = await boardService.getAll()
      setBoards(boardsRes)
      const found = boardsRes.find((b: Board) => b.id === selectedBoard.id)
      if (found) setSelectedBoard(found)
    } catch (error) {
      console.error('Error reordering phases:', error)
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
    } catch (error: any) {
      console.error('Error creating task:', error)
      const errorMsg = error?.response?.data?.error || 'Error al crear la tarea'
      showError(errorMsg)
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

  const handleDeleteBoard = async (boardId: number) => {
    if (!confirm('¿Eliminar este tablero? Esta acción no se puede deshacer.')) return
    setIsDeletingBoard(true)
    try {
      await boardService.delete(boardId)
      const boardsRes = await boardService.getAll()
      setBoards(boardsRes)
      if (selectedBoard?.id === boardId) {
        setSelectedBoard(boardsRes.length > 0 ? boardsRes[0] : null)
      }
    } catch (error) {
      console.error('Error deleting board:', error)
    } finally {
      setIsDeletingBoard(false)
    }
  }

  if (isLoading && boards.length === 0) {
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
          <h1>Tareas</h1>
        </div>
        <div className="tasks-loading">
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
<button type="submit" className="btn-primary" disabled={isCreatingBoard}>
                      {isCreatingBoard ? 'Creando...' : 'Crear Tablero'}
                    </button>
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
          <h1>Tareas</h1>
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
                  <option value="all">Todos los tableros</option>
                  {boards.map(board => (
                    <option key={board.id} value={board.id}>{board.name}</option>
                  ))}
                </select>
                <button className="btn-icon" onClick={() => setShowBoardModal(true)} title="Crear tablero">
                  +
                </button>
                {selectedBoard && (
                  <>
                    <button 
                      className="btn-icon phases-btn" 
                      onClick={() => setShowPhasesModal(true)}
                      title="Gestionar fases"
                      style={{ marginLeft: '4px' }}
                    >
                      ≡
                    </button>
                    <button 
                      className="btn-icon members-btn" 
                      onClick={() => {
                        setOptimisticMembers(selectedBoard.members?.map(m => m.id) || [])
                        setShowBoardMembersModal(true)
                      }}
                      title="Gestionar miembros"
                      style={{ marginLeft: '4px' }}
                    >
                      ⊕
                    </button>
                    {user?.id === selectedBoard.created_by && (
                      <button 
                        className="btn-icon delete-board-btn" 
                        onClick={() => handleDeleteBoard(selectedBoard.id)}
                        title={isDeletingBoard ? "Eliminando..." : "Eliminar tablero"}
                        disabled={isDeletingBoard}
                        style={{ marginLeft: '4px', color: '#ef4444' }}
                      >
                        {isDeletingBoard ? '...' : '🗑'}
                      </button>
                    )}
                  </>
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
                <input
                  type="text"
                  placeholder="Buscar usuarios..."
                  value={assigneeSearch}
                  onChange={(e) => setAssigneeSearch(e.target.value)}
                  style={{ width: '100%', padding: '8px 12px', marginBottom: '8px', border: '1px solid #e2e8f0', borderRadius: '8px' }}
                />
                <div className="members-select" style={{ maxHeight: '200px', overflowY: 'auto' }}>
                  {filteredUsers
                    .slice()
                    .sort((a, b) => a.name.localeCompare(b.name))
                    .map(user => (
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

              <div className="form-group">
                <label>Fases del tablero</label>
                <div className="phases-list" style={{ maxHeight: '180px', overflowY: 'auto', marginBottom: '12px' }}>
                  {newBoardData.phases.map((phase, idx) => (
                    <div key={idx} className="phase-item">
                      <div className="phase-color" style={{ backgroundColor: phase.color }}></div>
                      <span className="phase-name">{phase.name}</span>
                      {newBoardData.phases.length > 1 && (
                        <button
                          type="button"
                          className="btn-icon phase-delete"
                          onClick={() => {
                            setNewBoardData({
                              ...newBoardData,
                              phases: newBoardData.phases.filter((_, i) => i !== idx)
                            })
                          }}
                          title="Eliminar fase"
                        >
                          ×
                        </button>
                      )}
                    </div>
                  ))}
                </div>
                <div style={{ display: 'flex', gap: '8px', alignItems: 'center' }}>
                  <input
                    type="text"
                    placeholder="Nueva fase..."
                    value={newBoardPhaseSearch}
                    onChange={(e) => setNewBoardPhaseSearch(e.target.value)}
                    onKeyDown={(e) => {
                      if (e.key === 'Enter' && newBoardPhaseSearch.trim()) {
                        e.preventDefault()
                        setNewBoardData({
                          ...newBoardData,
                          phases: [...newBoardData.phases, { name: newBoardPhaseSearch.trim(), color: '#6b7280' }]
                        })
                        setNewBoardPhaseSearch('')
                      }
                    }}
                    style={{ flex: 1, padding: '8px 12px', border: '1px solid #e2e8f0', borderRadius: '8px' }}
                  />
                  <input
                    type="color"
                    defaultValue="#6b7280"
                    style={{ width: '36px', height: '36px', border: 'none', cursor: 'pointer', padding: '2px' }}
                  />
                  <button
                    type="button"
                    className="btn-primary"
                    style={{ padding: '8px 12px' }}
                    onClick={() => {
                      if (newBoardPhaseSearch.trim()) {
                        setNewBoardData({
                          ...newBoardData,
                          phases: [...newBoardData.phases, { name: newBoardPhaseSearch.trim(), color: '#6b7280' }]
                        })
                        setNewBoardPhaseSearch('')
                      }
                    }}
                  >
                    +
                  </button>
                </div>
              </div>

              <div className="modal-actions">
                <button type="button" onClick={() => {
                  setShowBoardModal(false)
                  setNewBoardData({
                    name: '',
                    description: '',
                    color: '#3b82f6',
                    member_ids: [],
                    phases: [
                      { name: 'Por hacer', color: '#6b7280' },
                      { name: 'En proceso', color: '#3b82f6' },
                      { name: 'Finalizado', color: '#22c55e' },
                    ],
                  })
                  setAssigneeSearch('')
                  setNewBoardPhaseSearch('')
                }}>Cancelar</button>
                <button type="submit" className="btn-primary" disabled={isCreatingBoard}>
                  {isCreatingBoard ? 'Creando...' : 'Crear Tablero'}
                </button>
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

      {showPhasesModal && selectedBoard && (
        <div className="modal-overlay" onClick={() => setShowPhasesModal(false)} onMouseUp={handlePhaseDragEnd}>
          <div className="modal board-modal phases-modal" onClick={(e) => e.stopPropagation()} onMouseUp={() => handlePhaseDragEnd()}>
            <div className="board-modal-header">
              <h2>Gestionar Fases</h2>
              <button className="close-btn" onClick={() => setShowPhasesModal(false)}>×</button>
            </div>
            <p className="board-subtitle">{selectedBoard.name}</p>
            
            <div 
              className="phases-list" 
              onMouseMove={(e) => {
                if (!isDraggingPhasesRef.current) return
                const target = e.target as HTMLElement
                const phaseItem = target.closest('.phase-item')
                if (phaseItem) {
                  const idx = parseInt(phaseItem.getAttribute('data-idx') || '-1')
                  if (idx >= 0) handlePhaseDragEnter(idx)
                }
              }}
              onMouseUp={handlePhaseDragEnd}
              onMouseLeave={(e) => {
                if (isDraggingPhasesRef.current) {
                  const relatedTarget = e.relatedTarget as HTMLElement
                  if (!relatedTarget || !e.currentTarget.contains(relatedTarget)) {
                    handlePhaseDragEnd()
                  }
                }
              }}
            >
              {selectedBoard.phases?.map((phase, idx) => (
                <div 
                  key={phase.id} 
                  data-idx={idx}
                  className={`phase-item ${draggingPhase === phase.id ? 'dragging' : ''} ${dragOverIdx === idx ? 'drag-over' : ''}`}
                  onMouseDown={() => handlePhaseDragStart(phase.id)}
                >
                  <span className="drag-handle" style={{ cursor: 'grab' }}>⋮⋮</span>
                  <div className="phase-color" style={{ backgroundColor: phase.color }}></div>
                  <span className="phase-name">{phase.name}</span>
                  <button 
                    className="btn-icon phase-delete"
                    onClick={(e) => {
                      e.stopPropagation()
                      if (!confirm('¿Eliminar esta fase?')) return
                      setIsDeletingPhase(true)
                      boardService.removePhase(selectedBoard.id, phase.id)
                        .then(() => boardService.getAll())
                        .then((boardsRes) => {
                          setBoards(boardsRes)
                          const found = boardsRes.find((b: Board) => b.id === selectedBoard.id)
                          if (found) setSelectedBoard(found)
                        })
                        .catch((error: any) => {
                          alert(error.response?.data?.error || 'Error al eliminar fase')
                        })
                        .finally(() => setIsDeletingPhase(false))
                    }}
                    title={isDeletingPhase ? "Eliminando..." : "Eliminar fase"}
                    disabled={isDeletingPhase}
                  >
                    {isDeletingPhase ? '...' : '×'}
                  </button>
                </div>
              ))}
            </div>

            <div className="add-phase-form" onMouseUp={handlePhaseDragEnd}>
              <input
                type="text"
                placeholder="Nombre de la nueva fase"
                id="new-phase-name"
                style={{ flex: 1, padding: '10px', border: '1px solid #e2e8f0', borderRadius: '8px', marginRight: '8px' }}
              />
              <input
                type="color"
                id="new-phase-color"
                defaultValue="#6b7280"
                style={{ width: '40px', height: '40px', border: 'none', cursor: 'pointer' }}
              />
              <button 
                className="btn-primary"
                style={{ marginLeft: '8px' }}
                disabled={isAddingPhase}
                onClick={async () => {
                  const nameInput = document.getElementById('new-phase-name') as HTMLInputElement
                  const colorInput = document.getElementById('new-phase-color') as HTMLInputElement
                  const name = nameInput.value.trim()
                  const color = colorInput.value
                  if (!name) return
                  setIsAddingPhase(true)
                  try {
                    await boardService.addPhase(selectedBoard.id, { name, color })
                    const boardsRes = await boardService.getAll()
                    setBoards(boardsRes)
                    const found = boardsRes.find(b => b.id === selectedBoard.id)
                    if (found) setSelectedBoard(found)
                    nameInput.value = ''
                  } catch (error) {
                    console.error('Error adding phase:', error)
                  } finally {
                    setIsAddingPhase(false)
                  }
                }}
              >
                {isAddingPhase ? 'Agregando...' : '+'}
              </button>
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
          {(selectedBoard?.phases?.length ? selectedBoard.phases : localColumnOrder ? localColumnOrder.map(id => COLUMNS.find(c => c.id === id) || COLUMNS[0]) : COLUMNS).map((p: any, idx: number) => {
            const isPhase = !!p.name
            const column = {
              id: isPhase ? (p.status || p.name.toLowerCase().replace(/\s+/g, '_')) : p.id,
              title: isPhase ? p.name : p.title,
              color: p.color
            }
            const phasesLength = selectedBoard?.phases?.length || (localColumnOrder?.length || COLUMNS.length)
            return (
              <Column
                key={column.id}
                column={column}
                tasks={getTasksByStatus(column.id)}
                onTaskClick={setSelectedTask}
                canMoveLeft={idx > 0}
                canMoveRight={idx < phasesLength - 1}
                onMoveLeft={() => {
                  if (selectedBoard?.phases && selectedBoard.id) {
                    if (idx <= 0) return
                    const newPhases = [...selectedBoard.phases]
                    const temp = newPhases[idx]
                    newPhases[idx] = newPhases[idx - 1]
                    newPhases[idx - 1] = temp
                    setSelectedBoard((prev: Board | null) => {
                      if (!prev) return prev
                      return { ...prev, phases: newPhases }
                    })
                    boardService.reorderPhases(selectedBoard.id, newPhases.map((p: any) => p.id))
                      .then(() => boardService.getAll())
                      .then((boardsRes) => setBoards(boardsRes))
                      .catch(console.error)
                  } else {
                    const cols = localColumnOrder || COLUMNS.map(c => c.id)
                    if (cols.length <= 1) return
                    const newOrder = [...cols]
                    const temp = newOrder[idx]
                    newOrder[idx] = newOrder[idx - 1]
                    newOrder[idx - 1] = temp
                    setLocalColumnOrder(newOrder)
                  }
                }}
                onMoveRight={() => {
                  if (selectedBoard?.phases && selectedBoard.id) {
                    if (idx >= (selectedBoard.phases.length - 1)) return
                    const newPhases = [...selectedBoard.phases]
                    const temp = newPhases[idx]
                    newPhases[idx] = newPhases[idx + 1]
                    newPhases[idx + 1] = temp
                    setSelectedBoard((prev: Board | null) => {
                      if (!prev) return prev
                      return { ...prev, phases: newPhases }
                    })
                    boardService.reorderPhases(selectedBoard.id, newPhases.map((p: any) => p.id))
                      .then(() => boardService.getAll())
                      .then((boardsRes) => setBoards(boardsRes))
                      .catch(console.error)
                  } else {
                    const cols = localColumnOrder || COLUMNS.map(c => c.id)
                    if (cols.length <= 1) return
                    const newOrder = [...cols]
                    const temp = newOrder[idx]
                    newOrder[idx] = newOrder[idx + 1]
                    newOrder[idx + 1] = temp
                    setLocalColumnOrder(newOrder)
                  }
                }}
              />
            )
          })}
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
          users={users}
          onClose={() => setSelectedTask(null)}
          onUpdate={handleUpdateTask}
          onDelete={handleDeleteTask}
          columns={getCurrentColumns()}
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
                      <option value="low">Baja</option>
                      <option value="medium">Media</option>
                      <option value="high">Alta</option>
                      <option value="urgent">Urgente</option>
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
