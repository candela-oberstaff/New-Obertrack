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
  type DragStartEvent,
  type DragEndEvent,
} from '@dnd-kit/core'
import {
  sortableKeyboardCoordinates,
} from '@dnd-kit/sortable'
import { taskService, userService, boardService } from '../services/api'
import type { Task, User, TaskStatus, CreateTaskInput, Board } from '../types'
import { RichTextEditor } from '../components/Tasks/RichTextEditor'
import { TaskCard } from '../components/Tasks/TaskCard'
import { Column } from '../components/Tasks/Column'
import { TaskDetailPanel } from '../components/Tasks/TaskDetailPanel'
import { ColumnType } from '../components/Tasks/types'
import './Tasks.css'

const COLUMNS: ColumnType[] = [
  { id: 'por_hacer', title: 'Por hacer', color: '#6b7280' },
  { id: 'en_proceso', title: 'En proceso', color: '#3b82f6' },
  { id: 'finalizado', title: 'Finalizado', color: '#22c55e' },
]

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
      setTasks(tasksRes.data || [])
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
