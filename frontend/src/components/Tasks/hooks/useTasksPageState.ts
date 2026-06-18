import { useState, useCallback, useRef, useEffect } from 'react'
import { useAuth } from '../../../context/AuthContext'
import { useNotification } from '../../../context/NotificationContext'
import { useConfirm } from '../../ui/ConfirmProvider'
import { userService, taskService, adminService } from '../../../services/api'
import type { User, Board, Task, CreateTaskInput, Phase } from '../../../types'
import { ColumnType } from '../types'
import { useBoards } from './useBoards'
import { useTasks } from './useTasks'

export interface CompanyOption {
  id: number
  company_name: string
}

export interface PhaseFormInput {
  name: string
  color: string
}

export interface BoardFormData {
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

export function useTasksPageState() {
  const { user } = useAuth()
  const { error: showError } = useNotification()
  const confirm = useConfirm()

  const isSuperadmin = user?.user_type === 'superadmin'

  // Company scope (superadmin only). Persisted so the selection survives reloads.
  const [companies, setCompanies] = useState<CompanyOption[]>([])
  const [selectedCompanyId, setSelectedCompanyIdState] = useState<number | null>(() => {
    const stored = localStorage.getItem('preferred_company_id')
    return stored ? Number(stored) : null
  })

  const setSelectedCompanyId = useCallback((id: number | null) => {
    setSelectedCompanyIdState(id)
    if (id) {
      localStorage.setItem('preferred_company_id', String(id))
    } else {
      localStorage.removeItem('preferred_company_id')
    }
  }, [])

  // Users state
  const [users, setUsers] = useState<User[]>([])
  const [assigneeSearch, setAssigneeSearch] = useState('')

  // UI State
  const [showNewTaskModal, setShowNewTaskModal] = useState(false)
  const [showBoardModal, setShowBoardModal] = useState(false)
  const [showBoardMembersModal, setShowBoardMembersModal] = useState(false)
  const [showPhasesModal, setShowPhasesModal] = useState(false)
  const [showJoinBoardModal, setShowJoinBoardModal] = useState(false)
  const [isSavingPhase, setIsSavingPhase] = useState(false)
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
    addPhase,
    removePhase,
  } = useBoards({
    companyId: isSuperadmin ? selectedCompanyId : null,
    requireCompany: isSuperadmin,
    // Superadmin lands on a board picker after choosing a company instead of
    // jumping straight into a board.
    autoSelectFirst: !isSuperadmin,
  })

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
    companyId: isSuperadmin ? selectedCompanyId : null,
  })

  // Per-board task counts grouped by status, used to show a breakdown on the
  // board picker cards (Por hacer / En proceso / Finalizado + custom phases).
  const [boardTaskCounts, setBoardTaskCounts] = useState<Record<number, Record<string, number>>>({})

  useEffect(() => {
    // Only needed for the board picker (no board selected yet).
    if (selectedBoard || boards.length === 0) return
    if (isSuperadmin && !selectedCompanyId) return
    let active = true
    // Server-side aggregation: counts come grouped by board+status (no full
    // task list download, scales past any task count).
    taskService.getBoardStatusCounts(isSuperadmin ? selectedCompanyId : null)
      .then((counts) => {
        if (!active) return
        setBoardTaskCounts(counts || {})
      })
      .catch((e) => console.error('Error fetching task counts:', e))
    return () => { active = false }
  }, [selectedBoard, boards, isSuperadmin, selectedCompanyId])

  // Fetch the company list for the superadmin company selector
  useEffect(() => {
    if (!isSuperadmin) return
    let active = true
    adminService.getTenants()
      .then((res: any) => {
        if (!active) return
        const list: CompanyOption[] = (res || []).map((t: any) => ({
          id: t.id,
          company_name: t.company_name || t.owner_name || `Empresa ${t.id}`,
        }))
        setCompanies(list)
      })
      .catch((err) => console.error('Error fetching companies:', err))
    return () => { active = false }
  }, [isSuperadmin])

  const fetchUsers = useCallback(async () => {
    // Superadmin: scope the user fetch to the selected company (server-side) instead
    // of downloading every user across all tenants. Without a company, fetch nothing.
    if (isSuperadmin && !selectedCompanyId) {
      setUsers([])
      return
    }
    const params: { limit: number; company_id?: number } = { limit: 1000 }
    if (isSuperadmin && selectedCompanyId) params.company_id = selectedCompanyId
    const usersRes = await userService.getAll(params)
    setUsers(usersRes.data || [])
  }, [isSuperadmin, selectedCompanyId])

  // Fetch users on mount and whenever the company scope changes (superadmin).
  useEffect(() => {
    fetchUsers()
  }, [fetchUsers])

  // 1. All users that the current user is ALLOWED to see/assign (strictly linked to the company)
  const visibleUsers = users.filter((u: User) => {
    // Exclude superadmins from candidate lists
    if (u.user_type === 'superadmin') return false

    // Identify the company ID
    let companyID = 0
    if (isSuperadmin) {
      // Scope strictly to the company the superadmin has selected. Without a
      // selected company we show no users to avoid mixing tenants.
      if (!selectedCompanyId) return false
      companyID = selectedCompanyId
    } else {
      companyID = user?.user_type === 'empleador' ? user.id : (user?.empleador_id || 0)
    }

    if (!companyID) return false

    if (u.user_type === 'empleador') {
      return u.id === companyID
    }
    if (u.user_type === 'profesional' || u.user_type === 'empleado') {
      return u.empleador_id === companyID
    }

    return false
  })

  // 2. Search filtering applied to visible users
  const potentialMemberUsers = visibleUsers.filter((u: User) => 
    u.name?.toLowerCase().includes(assigneeSearch.toLowerCase()) ||
    u.email?.toLowerCase().includes(assigneeSearch.toLowerCase())
  )

  // 3. Users available for task assignment (Restricted to board members, unless superadmin)
  const assignableUsers = potentialMemberUsers.filter((u: User) => {
    if (user?.user_type === 'superadmin') return true

    const currentBoardId = selectedBoard?.id || (boards.length > 0 ? boards[0].id : null)
    if (!currentBoardId) return true

    const board = boards.find((b: Board) => b.id === currentBoardId)
    return board?.members?.some((m: User) => m.id === u.id)
  })

  // Board actions
  const handleDeleteBoard = useCallback(async (boardId: number) => {
    const ok = await confirm({
      title: 'Eliminar tablero',
      message: '¿Eliminar este tablero? Esta acción no se puede deshacer.',
      confirmLabel: 'Eliminar',
      variant: 'danger',
    })
    if (!ok) return
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
  }, [deleteBoard, selectedBoard, setSelectedTask, confirm])

  // Handle board form submission - transform BoardFormData to CreateBoardInput
  const handleBoardSubmit = useCallback(async (data: BoardFormData) => {
    try {
      const board = await createBoard({
        name: data.name,
        description: data.description,
        color: data.color,
        member_ids: data.member_ids,
        phases: data.phases,
      })
      if (!board) {
        showError('No se pudo crear el tablero. Revisa los datos e inténtalo de nuevo.')
        return
      }
      setShowBoardModal(false)
    } catch (error: any) {
      showError(error?.response?.data?.error || 'Error al crear el tablero')
    }
  }, [createBoard, showError])

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

  // Move a phase from one position to another (drag & drop on the board)
  const handleReorderPhaseByIndex = useCallback((fromIdx: number, toIdx: number) => {
    if (!selectedBoard?.phases || !selectedBoard.id) return
    if (fromIdx === toIdx || fromIdx < 0 || toIdx < 0) return
    const newPhases = [...selectedBoard.phases]
    if (fromIdx >= newPhases.length || toIdx >= newPhases.length) return
    const [moved] = newPhases.splice(fromIdx, 1)
    newPhases.splice(toIdx, 0, moved)
    setSelectedBoard({ ...selectedBoard, phases: newPhases })
    reorderPhases(selectedBoard.id, newPhases.map((p: Phase) => p.id))
  }, [selectedBoard, setSelectedBoard, reorderPhases])

  // Add / remove phases on an existing board
  const handleAddPhase = useCallback(async (phase: { name: string; color: string }) => {
    if (!selectedBoard?.id || !phase.name.trim()) return
    setIsSavingPhase(true)
    try {
      await addPhase(selectedBoard.id, { name: phase.name.trim(), color: phase.color })
    } catch (error) {
      console.error('Error adding phase:', error)
      showError('No se pudo agregar la fase')
    } finally {
      setIsSavingPhase(false)
    }
  }, [selectedBoard, addPhase, showError])

  const handleRemovePhase = useCallback(async (phaseId: number) => {
    if (!selectedBoard?.id) return
    const ok = await confirm({
      title: 'Eliminar fase',
      message: '¿Eliminar esta fase? Las tareas en esta fase podrían verse afectadas.',
      confirmLabel: 'Eliminar',
      variant: 'danger',
    })
    if (!ok) return
    setIsSavingPhase(true)
    try {
      await removePhase(selectedBoard.id, phaseId)
    } catch (error: any) {
      console.error('Error removing phase:', error)
      showError(error?.response?.data?.error || 'No se pudo eliminar la fase')
    } finally {
      setIsSavingPhase(false)
    }
  }, [selectedBoard, removePhase, showError, confirm])

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
    if (isCreatingTask) return
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
  }, [selectedBoard, boards, newTaskData, createTask, fetchTasks, showError, isCreatingTask])

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

  return {
    user,
    isSuperadmin,
    companies,
    selectedCompanyId,
    setSelectedCompanyId,
    users,
    visibleUsers,
    assigneeSearch,
    setAssigneeSearch,
    showNewTaskModal,
    setShowNewTaskModal,
    showBoardModal,
    setShowBoardModal,
    showBoardMembersModal,
    setShowBoardMembersModal,
    showPhasesModal,
    setShowPhasesModal,
    isSavingPhase,
    showJoinBoardModal,
    setShowJoinBoardModal,
    optimisticMembers,
    setOptimisticMembers,
    isCreatingTask,
    isDeletingBoard,
    isJoiningBoard,
    draggingPhase,
    dragOverIdx,
    isDraggingPhasesRef,
    newTaskData,
    setNewTaskData,
    boardFormData,
    setBoardFormData,
    newBoardPhaseSearch,
    setNewBoardPhaseSearch,
    boards,
    boardTaskCounts,
    selectedBoard,
    setSelectedBoard,
    publicBoards,
    isLoadingBoards,
    isCreatingBoard,
    tasks,
    selectedTask,
    setSelectedTask,
    potentialMemberUsers,
    assignableUsers,
    handleDeleteBoard,
    handleBoardSubmit,
    handlePhaseDragStart,
    handlePhaseDragEnter,
    handlePhaseDragEnd,
    handleMovePhaseLeft,
    handleMovePhaseRight,
    handleReorderPhaseByIndex,
    handleAddPhase,
    handleRemovePhase,
    handleOpenNewTaskModal,
    handleCreateTask,
    handleUpdateTask,
    handleDeleteTask,
    handleJoinBoard,
    getCurrentColumns,
    openBoardModal,
    fetchPublicBoards,
    updateBoardMembers,
  }
}
