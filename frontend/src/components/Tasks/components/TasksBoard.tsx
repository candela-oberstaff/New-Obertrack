import { useState, useCallback, useEffect } from 'react'
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
import { snapCenterToCursor } from '@dnd-kit/modifiers'
import { arrayMove, sortableKeyboardCoordinates } from '@dnd-kit/sortable'
import { Plus, X } from 'lucide-react'
import { useNotification } from '../../../context/NotificationContext'
import { Column } from '../Column'
import { TaskCard } from '../TaskCard'
import type { Task, Board } from '../../../types'
import type { ColumnType, Phase } from '../types'
import { phaseStatusId } from '../phaseStatus'
import { compareTasks } from '../taskSort'
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
    return tasks.filter((task) => task.status === status && task.board_id === selectedBoard?.id)
  }, [tasks, selectedBoard])

  const handleDragStart = useCallback((event: DragStartEvent) => {
    const { active } = event
    const task = tasks.find((t) => t.id === active.id)
    if (task) {
      setActiveTask(task)
    }
  }, [tasks])

  const handleDragOver = useCallback((event: { active: { id: unknown }; over: { id: unknown } | null }) => {
    const { active, over } = event
    if (!over) return

    const activeId = Number(active.id)
    const overId = over.id

    if (typeof overId !== 'string') return

    const column = getCurrentColumns().find(c => c.id === overId)
    if (!column) return

    const activeTaskObj = tasks.find((t) => t.id === activeId)
    if (!activeTaskObj || activeTaskObj.status === column.id) return

    // Visual feedback only, no backend call here
  }, [tasks, getCurrentColumns])

  const handleDragEnd = useCallback(async (event: DragEndEvent) => {
    const { active, over } = event
    setActiveTask(null)

    if (!over) return

    const activeId = Number(active.id)
    const overId = over.id

    const activeTaskObj = tasks.find((t) => t.id === activeId)
    if (!activeTaskObj) return

    // Destino: una columna (id string) o una tarjeta concreta (id numérico).
    let targetStatus: string | undefined
    let overTaskId: number | null = null

    if (typeof overId === 'string') {
      const column = getCurrentColumns().find(c => c.id === overId)
      if (column) targetStatus = column.id
    } else {
      const overTask = tasks.find((t) => t.id === Number(overId))
      if (overTask) {
        targetStatus = overTask.status
        overTaskId = overTask.id
      }
    }
    if (!targetStatus) return

    const boardId = selectedBoard?.id
    if (!boardId) return

    // Lista ordenada de la columna destino con el MISMO criterio que pinta
    // Column (compareTasks); si divergen, la tarjeta salta al soltarla. Se
    // calcula sobre TODAS las tareas del tablero (allTasks), no sobre las del
    // scope visible: el orden persistido es compartido por todos los miembros.
    const targetColumnTasks = (allTasks ?? tasks)
      .filter((t) => t.status === targetStatus && t.board_id === boardId)
      .sort(compareTasks)

    try {
      if (targetStatus === activeTaskObj.status) {
        // Reorden dentro de la misma columna (semántica arrayMove de dnd-kit).
        if (overTaskId === null || overTaskId === activeId || !onReorderColumn) return
        const ids = targetColumnTasks.map((t) => t.id)
        const oldIdx = ids.indexOf(activeId)
        const newIdx = ids.indexOf(overTaskId)
        if (oldIdx === -1 || newIdx === -1 || oldIdx === newIdx) return
        await onReorderColumn(boardId, targetStatus, arrayMove(ids, oldIdx, newIdx))
      } else if (onMoveTask) {
        // Movimiento entre columnas: se inserta antes de la tarjeta sobre la
        // que se soltó, o al final si se soltó sobre la columna.
        const ids = targetColumnTasks.map((t) => t.id).filter((id) => id !== activeId)
        let insertIdx = ids.length
        if (overTaskId !== null) {
          const i = ids.indexOf(overTaskId)
          if (i >= 0) insertIdx = i
        }
        ids.splice(insertIdx, 0, activeId)
        await onMoveTask(activeId, boardId, targetStatus, ids)
      } else {
        // Fallback si el padre no cablea el reorden: solo cambio de status.
        await onUpdateTask(activeId, { status: targetStatus as Task['status'] })
      }
    } catch (error: any) {
      // useTasks ya revirtió el update optimista; sin este aviso la tarjeta
      // "vuelve sola" y el usuario no sabe por qué.
      const backendMsg = error?.response?.data?.error
      if (backendMsg === 'Access denied') {
        showError('No tienes permiso para mover esta tarea: solo pueden hacerlo quien la creó, sus asignados o un responsable.')
      } else {
        showError(backendMsg || 'No se pudo mover la tarea. Inténtalo de nuevo.')
      }
    }
  }, [tasks, allTasks, getCurrentColumns, selectedBoard, onUpdateTask, onMoveTask, onReorderColumn, showError])

  return (
    <DndContext
      sensors={sensors}
      collisionDetection={closestCorners}
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
