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
import { sortableKeyboardCoordinates } from '@dnd-kit/sortable'
import { Plus, X } from 'lucide-react'
import { Column } from '../Column'
import { TaskCard } from '../TaskCard'
import type { Task, Board } from '../../../types'
import type { ColumnType, Phase } from '../types'
import styles from '../../../pages/Tasks.module.css'

const DEFAULT_PHASE_COLOR = '#6b7280'

interface TasksBoardProps {
  tasks: Task[]
  selectedBoard: Board | null
  onTaskClick: (task: Task) => void
  onUpdateTask: (id: number, data: Partial<Task>) => Promise<void>
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
  selectedBoard,
  onTaskClick,
  onUpdateTask,
  onReorderPhase,
  onAddPhase,
  isSavingPhase = false,
}: TasksBoardProps) {
  const [activeTask, setActiveTask] = useState<Task | null>(null)
  const [localColumnOrder] = useState<string[] | null>(() => {
    const saved = localStorage.getItem('columnOrder')
    return saved ? JSON.parse(saved) : null
  })

  // Column (phase) drag-and-drop state
  const [draggedColIdx, setDraggedColIdx] = useState<number | null>(null)
  const [dragOverColIdx, setDragOverColIdx] = useState<number | null>(null)

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

  useEffect(() => {
    if (localColumnOrder) {
      localStorage.setItem('columnOrder', JSON.stringify(localColumnOrder))
    }
  }, [localColumnOrder])

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

    if (newStatus && newStatus !== activeTaskObj.status) {
      await onUpdateTask(activeId, { status: newStatus as Task['status'] })
    }
  }, [tasks, getCurrentColumns, onUpdateTask])

  return (
    <DndContext
      sensors={sensors}
      collisionDetection={closestCorners}
      onDragStart={handleDragStart}
      onDragOver={handleDragOver}
      onDragEnd={handleDragEnd}
    >
      <div className={styles['kanban-board']}>
        {(selectedBoard?.phases?.length
          ? selectedBoard.phases
          : localColumnOrder
            ? localColumnOrder.map(id => DEFAULT_COLUMNS.find(c => c.id === id) || DEFAULT_COLUMNS[0])
            : DEFAULT_COLUMNS
        ).map((p: Phase | ColumnType, idx: number) => {
          const isPhase = !!(p as Phase).name
          const column = {
            id: isPhase ? ((p as Phase).status || (p as Phase).name.toLowerCase().replace(/\s+/g, '_')) : (p as ColumnType).id,
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

      <DragOverlay>
        {activeTask ? (
          <TaskCard task={activeTask} onClick={() => { }} />
        ) : null}
      </DragOverlay>
    </DndContext>
  )
}
