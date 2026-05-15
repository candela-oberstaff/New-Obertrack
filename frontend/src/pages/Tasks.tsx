import { useState, useCallback, useRef, useEffect } from 'react'
import { useAuth } from '../context/AuthContext'
import { useNotification } from '../context/NotificationContext'
import { userService, taskService } from '../services/api'
import type { User, Board, Task, CreateTaskInput, Phase } from '../types'

import { TasksBoard } from '../components/Tasks/components/TasksBoard'
import { TaskDetailPanel } from '../components/Tasks/TaskDetailPanel'
import { NewTaskModal } from '../components/Tasks/Modals/NewTaskModal'
import { BoardModal } from '../components/Tasks/Modals/BoardModal'
import { BoardMembersModal } from '../components/Tasks/Modals/BoardMembersModal'
import { PhasesModal } from '../components/Tasks/Modals/PhasesModal'
import { JoinBoardModal } from '../components/Tasks/Modals/JoinBoardModal'
import { ColumnType } from '../components/Tasks/types'
import { useTasks } from '../components/Tasks/hooks/useTasks'
import { useBoards } from '../components/Tasks/hooks/useBoards'
import {
  Plus,
  Settings2,
  UserPlus,
  Trash2,
  CheckSquare
} from 'lucide-react'
import styles from './Tasks.module.css'

interface PhaseFormInput {
  name: string
  color: string
}

interface BoardFormData {
  name: string
  description: string
  color: string
  member_ids: number[]
  phases: PhaseFormInput[]
}

const DEFAULT_COLUMNS: ColumnType[] = [
  { id: 'por_hacer', title: 'Por hacer', color: '#6b7280' },
  { id: 'en_proceso', title: 'En proceso', color: 'var(--primary)' },
  { id: 'finalizado', title: 'Finalizado', color: '#22c55e' },
]

export default function Tasks() {
  const { user } = useAuth()
  const { error: showError } = useNotification()

  // Users state
  const [users, setUsers] = useState<User[]>([])
  const [assigneeSearch, setAssigneeSearch] = useState('')

  // UI State
  const [showAllTasks, setShowAllTasks] = useState(false)
  const [showNewTaskModal, setShowNewTaskModal] = useState(false)
  const [showBoardModal, setShowBoardModal] = useState(false)
  const [showBoardMembersModal, setShowBoardMembersModal] = useState(false)
  const [showPhasesModal, setShowPhasesModal] = useState(false)
  const [showJoinBoardModal, setShowJoinBoardModal] = useState(false)
  const [optimisticMembers, setOptimisticMembers] = useState<number[]>([])
  const [isCreatingTask, setIsCreatingTask] = useState(false)
  const [isDeletingBoard, setIsDeletingBoard] = useState(false)
  const [isJoiningBoard, setIsJoiningBoard] = useState(false)

  // Phases drag state
  const [draggingPhase, setDraggingPhase] = useState<number | null>(null)
  const [dragOverIdx, setDragOverIdx] = useState<number | null>(null)
  const isDraggingPhasesRef = useRef(false)
  const phasesOrderRef = useRef<Board['phases']>([])

  // New task form state
  const [newTaskData, setNewTaskData] = useState<CreateTaskInput & { attachments: File[] }>({
    title: '',
    description: '',
    priority: 'medium',
    end_date: '',
    assignees: [],
    attachments: [],
  })

  // Board form state for modal
  const [boardFormData, setBoardFormData] = useState<BoardFormData>({
    name: '',
    description: '',
    color: 'var(--primary)',
    member_ids: [],
    phases: [
      { name: 'Por hacer', color: '#6b7280' },
      { name: 'En proceso', color: 'var(--primary)' },
      { name: 'Finalizado', color: '#22c55e' },
    ],
  })
  const [newBoardPhaseSearch, setNewBoardPhaseSearch] = useState('')

  // Boards hook
  const {
    boards,
    selectedBoard,
    setSelectedBoard,
    publicBoards,
    isLoading: isLoadingBoards,
    isCreatingBoard,
    createBoard,
    deleteBoard,
    joinBoard,
    fetchPublicBoards,
    updateBoardMembers,
    reorderPhases,
  } = useBoards()

  // Tasks hook
  const {
    tasks,
    selectedTask,
    setSelectedTask,
    createTask,
    updateTask,
    deleteTask,
    fetchTasks,
  } = useTasks({
    boardId: selectedBoard?.id,
    showAllTasks,
  })

  // Fetch users on mount
  useEffect(() => {
    fetchUsers()
  }, [])

  const fetchUsers = useCallback(async () => {
    const usersRes = await userService.getAll({ limit: 1000 })
    setUsers(usersRes.data || [])
  }, [])

  // 1. All users that the current user is ALLOWED to see/assign
  const visibleUsers = users.filter((u: User) => {
    // ROLE RESTRICTION: Professionals cannot see/assign Superadmins
    if (user?.user_type === 'profesional' && u.user_type === 'superadmin') return false
    return true
  })

  // 2. Search filtering applied to visible users
  const potentialMemberUsers = visibleUsers.filter((u: User) => 
    u.name?.toLowerCase().includes(assigneeSearch.toLowerCase()) ||
    u.email?.toLowerCase().includes(assigneeSearch.toLowerCase())
  )

  // 3. Users available for task assignment (Restricted to board members, unless superadmin)
  const assignableUsers = potentialMemberUsers.filter((u: User) => {
    // Superadmin can assign any user (who passed the base filter)
    if (user?.user_type === 'superadmin') return true

    const currentBoardId = selectedBoard?.id || (boards.length > 0 ? boards[0].id : null)
    if (!currentBoardId) return true

    const board = boards.find((b: Board) => b.id === currentBoardId)
    return board?.members?.some((m: User) => m.id === u.id)
  })

  // Board actions
  const handleDeleteBoard = useCallback(async (boardId: number) => {
    if (!confirm('¿Eliminar este tablero? Esta acción no se puede deshacer.')) return
    setIsDeletingBoard(true)
    try {
      await deleteBoard(boardId)
      if (selectedBoard?.id === boardId) {
        setSelectedTask(null)
      }
    } catch (error) {
      console.error('Error deleting board:', error)
    } finally {
      setIsDeletingBoard(false)
    }
  }, [deleteBoard, selectedBoard, setSelectedTask])

  // Handle board form submission - transform BoardFormData to CreateBoardInput
  const handleBoardSubmit = useCallback(async (data: BoardFormData) => {
    try {
      await createBoard({
        name: data.name,
        description: data.description,
        color: data.color,
        member_ids: data.member_ids,
        phases: data.phases,
      })
      setShowBoardModal(false)
    } catch (error) {
      console.error('Error creating board:', error)
    }
  }, [createBoard])

  // Phase drag handlers
  const handlePhaseDragStart = useCallback((phaseId: number) => {
    isDraggingPhasesRef.current = true
    setDraggingPhase(phaseId)
    if (selectedBoard?.phases) {
      phasesOrderRef.current = [...selectedBoard.phases]
    }
  }, [selectedBoard])

  const handlePhaseDragEnter = useCallback((targetIdx: number) => {
    if (!isDraggingPhasesRef.current || !selectedBoard?.phases) return

    const phases = [...selectedBoard.phases]
    const dragIdx = phases.findIndex(p => p.id === draggingPhase)
    if (dragIdx === -1 || dragIdx === targetIdx) return

    const [dragged] = phases.splice(dragIdx, 1)
    phases.splice(targetIdx, 0, dragged)

    phasesOrderRef.current = phases
    setDragOverIdx(targetIdx)
    setSelectedBoard({ ...selectedBoard, phases })
  }, [draggingPhase, selectedBoard, setSelectedBoard])

  const handlePhaseDragEnd = useCallback(async () => {
    isDraggingPhasesRef.current = false
    setDraggingPhase(null)
    setDragOverIdx(null)

    if (!selectedBoard?.phases || !selectedBoard?.id) return

    const phaseIds = selectedBoard.phases.map((p: Phase) => p.id)
    await reorderPhases(selectedBoard.id, phaseIds)
  }, [selectedBoard, reorderPhases])

  // Phase move handlers
  const handleMovePhaseLeft = useCallback((idx: number) => {
    if (!selectedBoard?.phases || !selectedBoard.id || idx <= 0) return
    const newPhases = [...selectedBoard.phases]
    const temp = newPhases[idx]
    newPhases[idx] = newPhases[idx - 1]
    newPhases[idx - 1] = temp
    setSelectedBoard({ ...selectedBoard, phases: newPhases })
    reorderPhases(selectedBoard.id, newPhases.map((p: Phase) => p.id))
  }, [selectedBoard, setSelectedBoard, reorderPhases])

  const handleMovePhaseRight = useCallback((idx: number) => {
    if (!selectedBoard?.phases || !selectedBoard.id || idx >= selectedBoard.phases.length - 1) return
    const newPhases = [...selectedBoard.phases]
    const temp = newPhases[idx]
    newPhases[idx] = newPhases[idx + 1]
    newPhases[idx + 1] = temp
    setSelectedBoard({ ...selectedBoard, phases: newPhases })
    reorderPhases(selectedBoard.id, newPhases.map((p: Phase) => p.id))
  }, [selectedBoard, setSelectedBoard, reorderPhases])

  const handleOpenNewTaskModal = () => {
    if (!selectedBoard) {
      showError('Debes seleccionar un tablero específico antes de crear una tarea')
      return
    }
    setShowNewTaskModal(true)
  }

  // Task actions
  const handleCreateTask = useCallback(async (e: React.FormEvent) => {
    e.preventDefault()
    try {
      const boardId = selectedBoard?.id || (boards.length > 0 ? boards[0].id : 0)

      if (!boardId) {
        showError('Debes seleccionar o crear un tablero antes de añadir una tarea')
        return
      }

      const taskData: CreateTaskInput = {
        title: newTaskData.title,
        description: newTaskData.description || undefined,
        priority: newTaskData.priority || undefined,
        board_id: boardId,
        end_date: newTaskData.end_date || undefined,
        assignees: newTaskData.assignees?.length ? newTaskData.assignees : undefined,
      }

      setIsCreatingTask(true)
      const createdTask = await createTask(taskData)

      if (createdTask && newTaskData.attachments?.length > 0) {
        for (const file of newTaskData.attachments) {
          try {
            await taskService.addAttachment(createdTask.id, file)
          } catch (uploadErr) {
            console.error('Error uploading attachment:', uploadErr)
          }
        }
      }

      setShowNewTaskModal(false)
      setNewTaskData({
        title: '',
        description: '',
        priority: 'medium',
        end_date: '',
        assignees: [],
        attachments: [],
      })
      fetchTasks()
    } catch (error: any) {
      console.error('Error creating task:', error)
      const errorMsg = error?.response?.data?.error || 'Error al crear la tarea'
      showError(errorMsg)
    } finally {
      setIsCreatingTask(false)
    }
  }, [selectedBoard, boards, newTaskData, createTask, fetchTasks, showError])

  const handleUpdateTask = useCallback(async (id: number, data: Partial<User | Task | any>) => {
    await updateTask(id, data)
  }, [updateTask])

  const handleDeleteTask = useCallback(async (id: number) => {
    await deleteTask(id)
  }, [deleteTask])

  const handleJoinBoard = useCallback(async (boardId: number) => {
    setIsJoiningBoard(true)
    try {
      const success = await joinBoard(boardId)
      if (!success) {
        // useBoards already logs the error, but we can show a specific message if needed
        showError('No se pudo unir al tablero. Es posible que ya seas miembro.')
      }
      setShowJoinBoardModal(false)
    } finally {
      setIsJoiningBoard(false)
    }
  }, [joinBoard, showError])

  const getCurrentColumns = useCallback((): ColumnType[] => {
    if (selectedBoard?.phases?.length) {
      return selectedBoard.phases.map((p: Phase) => ({
        id: p.status || p.name.toLowerCase().replace(/\s+/g, '_'),
        title: p.name,
        color: p.color
      }))
    }
    return DEFAULT_COLUMNS
  }, [selectedBoard])

  // Reset board form when opening modal
  const openBoardModal = useCallback(() => {
    setBoardFormData({
      name: '',
      description: '',
      color: 'var(--primary)',
      member_ids: [],
      phases: [
        { name: 'Por hacer', color: '#6b7280' },
        { name: 'En proceso', color: 'var(--primary)' },
        { name: 'Finalizado', color: '#22c55e' },
      ],
    })
    setAssigneeSearch('')
    setNewBoardPhaseSearch('')
    setShowBoardModal(true)
  }, [])

  // Loading state
  if (isLoadingBoards && boards.length === 0) {
    return (
      <div className={styles['tasks-page']}>
        <div className={styles['tasks-loading']}>
          <div className={styles['spinner']} />
          <p>Cargando...</p>
        </div>
      </div>
    )
  }

  // Empty state
  if (boards.length === 0) {
    return (
      <div className={styles['tasks-page']}>
        <div className={styles['page-header']}>
          <h1>Tareas</h1>
        </div>
        <div className={styles['tasks-loading']}>
          <h2>No tienes tableros</h2>
          <p>Crea tu primer tablero para organizar tus tareas</p>
          <div style={{ display: 'flex', gap: '12px' }}>
            <button className={styles['btn-primary']} onClick={openBoardModal}>
              <Plus size={18} /> Crear Tablero
            </button>
            <button 
              className={styles['btn-secondary'] || 'btn-secondary'} 
              onClick={() => {
                fetchPublicBoards()
                setShowJoinBoardModal(true)
              }}
            >
              Unirse a Tablero
            </button>
          </div>
        </div>
        <BoardModal
          isOpen={showBoardModal}
          onClose={() => setShowBoardModal(false)}
          newBoardData={boardFormData}
          setNewBoardData={setBoardFormData}
          assigneeSearch={assigneeSearch}
          setAssigneeSearch={setAssigneeSearch}
          filteredUsers={potentialMemberUsers}
          newBoardPhaseSearch={newBoardPhaseSearch}
          setNewBoardPhaseSearch={setNewBoardPhaseSearch}
          onSubmit={handleBoardSubmit}
          isCreatingBoard={isCreatingBoard}
        />
      </div>
    )
  }

  return (
    <div className={styles['tasks-page']}>
      <div className={styles['page-header']}>
        <div className={styles['header-left']}>
          <h1>Tareas</h1>
          <div className={styles['board-selector']}>
            {boards.length > 0 && (
              <>
                <div className={styles['board-select-container']}>
                  <select
                    value={showAllTasks ? 'all' : (selectedBoard?.id || '')}
                    onChange={(e) => {
                      if (e.target.value === '') {
                        setShowAllTasks(false)
                        setSelectedBoard(null)
                        return
                      }
                      if (e.target.value === 'all') {
                        setShowAllTasks(true)
                        setSelectedBoard(null)
                        return
                      }
                      setShowAllTasks(false)
                      const board = boards.find((b: Board) => b.id === Number(e.target.value))
                      if (board) setSelectedBoard(board)
                    }}
                    style={{ borderLeftColor: showAllTasks ? '#f472b6' : (selectedBoard?.color || 'transparent') }}
                  >
                    <option value="">Seleccione un tablero...</option>
                    <option value="all">Todos los tableros</option>
                    {boards.map((board: Board) => (
                      <option key={board.id} value={board.id}>{board.name}</option>
                    ))}
                  </select>
                </div>
                <button className={styles['btn-icon']} onClick={openBoardModal} title="Crear tablero">
                  <Plus size={18} />
                </button>
                <button
                  className={`${styles['btn-secondary'] || 'btn-secondary'} ${styles['btn-sm'] || 'btn-sm'}`}
                  onClick={() => {
                    fetchPublicBoards()
                    setShowJoinBoardModal(true)
                  }}
                  style={{ marginLeft: '8px', fontSize: '12px', padding: '6px 10px' }}
                >
                  Unirse a Tablero
                </button>
                {selectedBoard && (
                  <>
                    <button
                      className={`${styles['btn-icon']} ${styles['phases-btn'] || 'phases-btn'}`}
                      onClick={() => setShowPhasesModal(true)}
                      title="Gestionar fases"
                      style={{ marginLeft: '4px' }}
                    >
                      <Settings2 size={18} />
                    </button>
                    <button
                      className={`${styles['btn-icon']} ${styles['members-btn'] || 'members-btn'}`}
                      onClick={() => {
                        setOptimisticMembers(selectedBoard.members?.map((m: User) => m.id) || [])
                        setShowBoardMembersModal(true)
                      }}
                      title="Gestionar miembros"
                      style={{ marginLeft: '4px' }}
                    >
                      <UserPlus size={18} />
                    </button>
                    {user?.id === selectedBoard.created_by && (
                      <button
                        className={`${styles['btn-icon']} ${styles['delete-board-btn'] || 'delete-board-btn'}`}
                        onClick={() => handleDeleteBoard(selectedBoard.id)}
                        title={isDeletingBoard ? "Eliminando..." : "Eliminar tablero"}
                        disabled={isDeletingBoard}
                        style={{ marginLeft: '4px', color: '#ef4444' }}
                      >
                        {isDeletingBoard ? '...' : <Trash2 size={18} />}
                      </button>
                    )}
                  </>
                )}
              </>
            )}
          </div>
        </div>
        <button className={styles['btn-primary']} onClick={handleOpenNewTaskModal}>
          + Nueva Tarea
        </button>
      </div>

      {!selectedBoard && !showAllTasks ? (
        <div className={styles['tasks-loading']} style={{ background: 'transparent' }}>
          <div className={styles['empty-state-glass'] || styles['dashboard-card']}>
            <CheckSquare size={64} style={{ color: 'var(--primary)', marginBottom: '24px', opacity: 0.6 }} />
            <h2 style={{ fontSize: '24px', fontWeight: '700', color: 'var(--black)', marginBottom: '12px' }}>
              Comienza seleccionando un tablero
            </h2>
            <p style={{ color: '#64748b', fontSize: '16px', maxWidth: '400px', margin: '0 auto 32px' }}>
              Elige uno de tus tableros en el menú superior o crea uno nuevo para empezar a gestionar tus tareas.
            </p>
            <div style={{ display: 'flex', gap: '12px', justifyContent: 'center' }}>
              <button className={styles['btn-primary']} onClick={openBoardModal}>
                <Plus size={18} /> Crear Nuevo Tablero
              </button>
              <button 
                className={styles['btn-secondary'] || 'btn-secondary'} 
                onClick={() => {
                  fetchPublicBoards()
                  setShowJoinBoardModal(true)
                }}
              >
                Unirse a Tablero
              </button>
            </div>
          </div>
        </div>
      ) : (
        <TasksBoard
          tasks={tasks}
          selectedBoard={selectedBoard}
          onTaskClick={setSelectedTask}
          onUpdateTask={handleUpdateTask}
          onMovePhaseLeft={handleMovePhaseLeft}
          onMovePhaseRight={handleMovePhaseRight}
        />
      )}

      {selectedTask && (
        <TaskDetailPanel
          task={selectedTask}
          users={visibleUsers}
          onClose={() => setSelectedTask(null)}
          onUpdate={handleUpdateTask}
          onDelete={handleDeleteTask}
          columns={getCurrentColumns()}
        />
      )}

      <NewTaskModal
        isOpen={showNewTaskModal}
        onClose={() => {
          setShowNewTaskModal(false)
          setNewTaskData({
            title: '',
            description: '',
            priority: 'medium',
            end_date: '',
            assignees: [],
            attachments: [],
          })
        }}
        newTaskData={newTaskData}
        setNewTaskData={setNewTaskData}
        isCreatingTask={isCreatingTask}
        onSubmit={handleCreateTask}
        assigneeSearch={assigneeSearch}
        setAssigneeSearch={setAssigneeSearch}
        filteredUsers={assignableUsers}
      />

      <BoardModal
        isOpen={showBoardModal}
        onClose={() => {
          setShowBoardModal(false)
          setAssigneeSearch('')
          setNewBoardPhaseSearch('')
        }}
        newBoardData={boardFormData}
        setNewBoardData={setBoardFormData}
        assigneeSearch={assigneeSearch}
        setAssigneeSearch={setAssigneeSearch}
        filteredUsers={potentialMemberUsers}
        newBoardPhaseSearch={newBoardPhaseSearch}
        setNewBoardPhaseSearch={setNewBoardPhaseSearch}
        onSubmit={handleBoardSubmit}
        isCreatingBoard={isCreatingBoard}
      />

      <BoardMembersModal
        isOpen={showBoardMembersModal}
        onClose={() => {
          setShowBoardMembersModal(false)
          setOptimisticMembers(selectedBoard?.members?.map((m: User) => m.id) || [])
        }}
        selectedBoard={selectedBoard}
        optimisticMembers={optimisticMembers}
        setOptimisticMembers={setOptimisticMembers}
        users={visibleUsers}
        onUpdateMembers={(newMembers: number[]) => selectedBoard ? updateBoardMembers(selectedBoard.id, newMembers) : Promise.resolve()}
      />

      <PhasesModal
        isOpen={showPhasesModal}
        onClose={() => setShowPhasesModal(false)}
        selectedBoard={selectedBoard}
        setBoards={() => {}}
        setSelectedBoard={setSelectedBoard}
        draggingPhase={draggingPhase}
        dragOverIdx={dragOverIdx}
        handlePhaseDragStart={handlePhaseDragStart}
        handlePhaseDragEnter={handlePhaseDragEnter}
        handlePhaseDragEnd={handlePhaseDragEnd}
        isDraggingPhasesRef={isDraggingPhasesRef}
      />

      <JoinBoardModal
        isOpen={showJoinBoardModal}
        onClose={() => setShowJoinBoardModal(false)}
        publicBoards={publicBoards}
        handleJoinBoard={handleJoinBoard}
        isJoiningBoard={isJoiningBoard}
      />
    </div>
  )
}
