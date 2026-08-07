import { useState, useCallback, useEffect } from 'react'
import {
  DndContext,
  DragOverlay,
  closestCorners,
  KeyboardSensor,
  PointerSensor,
  useSensor,
  useSensors,
  pointerWithin,
  rectIntersection,
  type DragStartEvent,
  type DragEndEvent,
  type DragOverEvent,
  type CollisionDetection,
} from '@dnd-kit/core'
import { snapCenterToCursor } from '@dnd-kit/modifiers'
import { arrayMove, sortableKeyboardCoordinates } from '@dnd-kit/sortable'
import { Plus, X } from 'lucide-react'
import { useNotification } from '../../../context/NotificationContext'
import { Column } from '../Column'
import { TaskCard } from '../TaskCard'
import type { Task, Board } from '../../../types'
import type { ColumnType, Phase } from '../types'
import { phaseStatusId } from '../phaseStatus'
import styles from '../../../pages/Tasks.module.css'

const DEFAULT_PHASE_COLOR = '#6b7280'

interface TasksBoardProps {
  tasks: Task[]
  /**
   * TODAS las tareas del tablero, sin el filtro de scope ("Mis tareas"). El
   * reorden persiste posiciones para todo el mundo: calcularlo solo sobre las
   * visibles renumeraría un subconjunto y desordenaría la columna para el resto.
   */
  allTasks?: Task[]
  selectedBoard: Board | null
  onTaskClick: (task: Task) => void
  onUpdateTask: (id: number, data: Partial<Task>) => Promise<void>
  /** Mueve una tarjeta a otra columna dejándola en la posición indicada. */
  onMoveTask?: (id: number, boardId: number, newStatus: string, orderedIds: number[]) => Promise<void>
  /** Persiste el orden manual dentro de una columna. */
  onReorderColumn?: (boardId: number, status: string, orderedIds: number[]) => Promise<void>
  onMovePhaseLeft?: (idx: number) => void
  onMovePhaseRight?: (idx: number) => void
  onReorderPhase?: (fromIdx: number, toIdx: number) => void
  onAddPhase?: (phase: { name: string; color: string }) => Promise<void>
  isSavingPhase?: boolean
}

const DEFAULT_COLUMNS: ColumnType[] = [
  { id: 'por_hacer', title: 'Por hacer', color: '#6b7280' },
  { id: 'en_proceso', title: 'En proceso', color: 'var(--primary)' },
  { id: 'finalizado', title: 'Finalizado', color: '#22c55e' },
]

export function TasksBoard({
  tasks,
  allTasks,
  selectedBoard,
  onTaskClick,
  onUpdateTask,
  onMoveTask,
  onReorderColumn,
  onReorderPhase,
  onAddPhase,
  isSavingPhase = false,
}: TasksBoardProps) {
  const { error: showError } = useNotification()
  const [activeTask, setActiveTask] = useState<Task | null>(null)

  // Local optimistic state for real-time sorting and drop indicators
  const [localTasks, setLocalTasks] = useState<Task[]>(tasks)
  const [localAllTasks, setLocalAllTasks] = useState<Task[]>(allTasks || [])

  useEffect(() => {
    setLocalTasks(tasks)
  }, [tasks])

  useEffect(() => {
    setLocalAllTasks(allTasks || [])
  }, [allTasks])

  // Column (phase) drag-and-drop state
  const [draggedColIdx, setDraggedColIdx] = useState<number | null>(null)
  const [dragOverColIdx, setDragOverColIdx] = useState<number | null>(null)

  // Columnas colapsadas, persistidas por tablero.
  const collapseStorageKey = `collapsedColumns:${selectedBoard?.id ?? 'default'}`
  const [collapsedColumns, setCollapsedColumns] = useState<string[]>([])

  useEffect(() => {
    try {
      setCollapsedColumns(JSON.parse(localStorage.getItem(collapseStorageKey) || '[]'))
    } catch {
      setCollapsedColumns([])
    }
  }, [collapseStorageKey])

  const toggleCollapse = useCallback((columnId: string) => {
    setCollapsedColumns((prev) => {
      const next = prev.includes(columnId) ? prev.filter((c) => c !== columnId) : [...prev, columnId]
      localStorage.setItem(collapseStorageKey, JSON.stringify(next))
      return next
    })
  }, [collapseStorageKey])

  // Inline "add phase" state
  const [addingPhase, setAddingPhase] = useState(false)
  const [newPhaseName, setNewPhaseName] = useState('')
  const [newPhaseColor, setNewPhaseColor] = useState(DEFAULT_PHASE_COLOR)

  const canManagePhases = !!(selectedBoard?.phases?.length && onReorderPhase)

  const handleColDragStart = useCallback((idx: number) => {
    setDraggedColIdx(idx)
  }, [])

  const handleColDragEnter = useCallback((idx: number) => {
    setDragOverColIdx((prev) => (prev === idx ? prev : idx))
  }, [])

  const handleColDrop = useCallback((idx: number) => {
    if (draggedColIdx !== null && draggedColIdx !== idx) {
      onReorderPhase?.(draggedColIdx, idx)
    }
    setDraggedColIdx(null)
    setDragOverColIdx(null)
  }, [draggedColIdx, onReorderPhase])

  const handleColDragEnd = useCallback(() => {
    setDraggedColIdx(null)
    setDragOverColIdx(null)
  }, [])

  const submitNewPhase = useCallback(async () => {
    if (!newPhaseName.trim() || isSavingPhase || !onAddPhase) return
    await onAddPhase({ name: newPhaseName.trim(), color: newPhaseColor })
    setNewPhaseName('')
    setNewPhaseColor(DEFAULT_PHASE_COLOR)
    setAddingPhase(false)
  }, [newPhaseName, newPhaseColor, isSavingPhase, onAddPhase])

  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 5 } }),
    useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates })
  )

  const collisionDetectionStrategy = useCallback<CollisionDetection>((args) => {
    const pointerCollisions = pointerWithin(args)
    let collisions = pointerCollisions.length > 0 ? pointerCollisions : rectIntersection(args)
    
    if (collisions.length === 0) {
      collisions = closestCorners(args)
    }

    return collisions
  }, [])

  const getCurrentColumns = useCallback((): ColumnType[] => {
    if (selectedBoard?.phases?.length) {
      return selectedBoard.phases.map((p: Phase) => ({
        id: phaseStatusId(p),
        title: p.name,
        color: p.color
      }))
    }
    return DEFAULT_COLUMNS
  }, [selectedBoard])

  const getTasksByStatus = useCallback((status: string) => {
    return localTasks.filter((task) => task.status === status && task.board_id === selectedBoard?.id)
  }, [localTasks, selectedBoard])

  const handleDragStart = useCallback((event: DragStartEvent) => {
    const { active } = event
    const task = localTasks.find((t) => t.id === active.id)
    if (task) {
      setActiveTask(task)
    }
  }, [localTasks])

  const handleDragOver = useCallback((event: DragOverEvent) => {
    const { active, over } = event
    if (!over) return

    const activeId = Number(active.id)
    const overId = over.id

    if (activeId === overId) return

    // Find the active task in local state
    let activeTaskObj = localTasks.find((t) => t.id === activeId)
    if (!activeTaskObj) {
      activeTaskObj = localAllTasks.find((t) => t.id === activeId)
    }
    if (!activeTaskObj) return

    // Determine target column status
    let targetStatus: string | undefined
    if (typeof overId === 'string') {
      const column = getCurrentColumns().find(c => c.id === overId)
      if (column) targetStatus = column.id
    } else {
      const overTask = localAllTasks.find((t) => t.id === Number(overId))
      if (overTask) targetStatus = overTask.status
    }

    if (!targetStatus) return

    // Core helper to move task position in state optimistically
    const updateList = (list: Task[]) => {
      const activeObj = list.find((t) => t.id === activeId)
      if (!activeObj) return list

      if (activeObj.status !== targetStatus) {
        // Dragging into a different column
        const filtered = list.filter((t) => t.id !== activeId)
        let insertIndex = filtered.length

        if (typeof overId !== 'string') {
          const overIndex = filtered.findIndex((t) => t.id === Number(overId))
          if (overIndex !== -1) {
            insertIndex = overIndex
          }
        }

        const updated = { ...activeObj, status: targetStatus as Task['status'] }
        const result = [...filtered]
        result.splice(insertIndex, 0, updated)
        return result
      } else {
        // Dragging/sorting within the same column
        if (typeof overId !== 'string') {
          const oldIndex = list.findIndex((t) => t.id === activeId)
          const newIndex = list.findIndex((t) => t.id === Number(overId))
          if (oldIndex !== -1 && newIndex !== -1 && oldIndex !== newIndex) {
            return arrayMove(list, oldIndex, newIndex)
          }
        }
        return list
      }
    }

    // Trigger state changes
    setLocalTasks((prev) => updateList(prev))
    setLocalAllTasks((prev) => updateList(prev))
  }, [localTasks, localAllTasks, getCurrentColumns])

  const handleDragEnd = useCallback(async (event: DragEndEvent) => {
    const { active, over } = event
    setActiveTask(null)

    if (!over) {
      // Revert drag changes if dropped outside a valid target
      setLocalTasks(tasks)
      setLocalAllTasks(allTasks || [])
      return
    }

    const activeId = Number(active.id)
    const activeTaskObj = localAllTasks.find((t) => t.id === activeId)
    if (!activeTaskObj) {
      setLocalTasks(tasks)
      setLocalAllTasks(allTasks || [])
      return
    }

    const targetStatus = activeTaskObj.status
    const boardId = selectedBoard?.id
    if (!boardId) return

    // Get the final order of task IDs in the target column from localAllTasks
    const targetColumnTasks = localAllTasks
      .filter((t) => t.status === targetStatus && t.board_id === boardId)
    const ids = targetColumnTasks.map((t) => t.id)

    try {
      const originalTaskObj = tasks.find((t) => t.id === activeId)
      const wasSameColumn = originalTaskObj && originalTaskObj.status === targetStatus

      if (wasSameColumn) {
        if (!onReorderColumn) return
        await onReorderColumn(boardId, targetStatus, ids)
      } else if (onMoveTask) {
        await onMoveTask(activeId, boardId, targetStatus, ids)
      } else {
        await onUpdateTask(activeId, { status: targetStatus as Task['status'] })
      }
    } catch (error: any) {
      // Revert local state to matches props in case of request error
      setLocalTasks(tasks)
      setLocalAllTasks(allTasks || [])

      const backendMsg = error?.response?.data?.error
      if (backendMsg === 'Access denied') {
        showError('No tienes permiso para mover esta tarea: solo pueden hacerlo quien la creó, sus asignados o un responsable.')
      } else {
        showError(backendMsg || 'No se pudo mover la tarea. Inténtalo de nuevo.')
      }
    }
  }, [tasks, allTasks, localAllTasks, selectedBoard, onUpdateTask, onMoveTask, onReorderColumn, showError])

  return (
    <DndContext
      sensors={sensors}
      collisionDetection={collisionDetectionStrategy}
      onDragStart={handleDragStart}
      onDragOver={handleDragOver}
      onDragEnd={handleDragEnd}
    >
      <div className={styles['kanban-board']} data-tour="tasks-board">
        {(selectedBoard?.phases?.length
          ? selectedBoard.phases
          : DEFAULT_COLUMNS
        ).map((p: Phase | ColumnType, idx: number) => {
          const isPhase = !!(p as Phase).name
          const column = {
            id: isPhase ? phaseStatusId(p as Phase) : (p as ColumnType).id,
            title: isPhase ? (p as Phase).name : (p as ColumnType).title,
            color: (p as Phase).color || (p as ColumnType).color
          }
          return (
            <Column
              key={column.id}
              column={column}
              tasks={getTasksByStatus(column.id)}
              onTaskClick={onTaskClick}
              index={idx}
              columnDraggable={canManagePhases}
              isColumnDragging={draggedColIdx === idx}
              isColumnDragOver={dragOverColIdx === idx && draggedColIdx !== idx}
              isCollapsed={collapsedColumns.includes(column.id)}
              onToggleCollapse={toggleCollapse}
              onColumnDragStart={handleColDragStart}
              onColumnDragEnter={handleColDragEnter}
              onColumnDrop={handleColDrop}
              onColumnDragEnd={handleColDragEnd}
            />
          )
        })}

        {canManagePhases && onAddPhase && (
          <div className={styles['add-column']}>
            {addingPhase ? (
              <div className={styles['add-column-form']}>
                <div className={styles['add-column-row']}>
                  <input
                    type="color"
                    value={newPhaseColor.startsWith('#') ? newPhaseColor : DEFAULT_PHASE_COLOR}
                    onChange={(e) => setNewPhaseColor(e.target.value)}
                    title="Color de la fase"
                    style={{ width: '36px', height: '36px', padding: 0, border: 'none', background: 'none', cursor: 'pointer', flexShrink: 0 }}
                  />
                  <input
                    type="text"
                    autoFocus
                    value={newPhaseName}
                    onChange={(e) => setNewPhaseName(e.target.value)}
                    onKeyDown={(e) => {
                      if (e.key === 'Enter') { e.preventDefault(); submitNewPhase() }
                      if (e.key === 'Escape') { setAddingPhase(false); setNewPhaseName('') }
                    }}
                    placeholder="Nombre de la fase..."
                    style={{ flex: 1, border: '1px solid #e2e8f0', borderRadius: '8px', padding: '8px 12px', minWidth: 0 }}
                  />
                </div>
                <div className={styles['add-column-actions']}>
                  <button
                    type="button"
                    className={styles['btn-primary']}
                    onClick={submitNewPhase}
                    disabled={isSavingPhase || !newPhaseName.trim()}
                    style={{ flex: 1 }}
                  >
                    {isSavingPhase ? 'Guardando...' : 'Agregar'}
                  </button>
                  <button
                    type="button"
                    className={styles['btn-icon']}
                    onClick={() => { setAddingPhase(false); setNewPhaseName('') }}
                    title="Cancelar"
                  >
                    <X size={18} />
                  </button>
                </div>
              </div>
            ) : (
              <button type="button" className={styles['add-column-btn']} onClick={() => setAddingPhase(true)}>
                <Plus size={20} />
                <span>Agregar fase</span>
              </button>
            )}
          </div>
        )}
      </div>

      <DragOverlay modifiers={[snapCenterToCursor]}>
        {activeTask ? (
          <TaskCard task={activeTask} isDragging onClick={() => { }} />
        ) : null}
      </DragOverlay>
    </DndContext>
  )
}
