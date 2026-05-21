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
import { Column } from '../Column'
import { TaskCard } from '../TaskCard'
import type { Task, Board } from '../../../types'
import type { ColumnType, Phase } from '../types'
import styles from '../../../pages/Tasks.module.css'

interface TasksBoardProps {
  tasks: Task[]
  selectedBoard: Board | null
  onTaskClick: (task: Task) => void
  onUpdateTask: (id: number, data: Partial<Task>) => Promise<void>
  onMovePhaseLeft?: (idx: number) => void
  onMovePhaseRight?: (idx: number) => void
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
}: TasksBoardProps) {
  const [activeTask, setActiveTask] = useState<Task | null>(null)
  const [localColumnOrder] = useState<string[] | null>(() => {
    const saved = localStorage.getItem('columnOrder')
    return saved ? JSON.parse(saved) : null
  })

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
        ).map((p: Phase | ColumnType) => {
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
            />
          )
        })}
      </div>

      <DragOverlay>
        {activeTask ? (
          <TaskCard task={activeTask} onClick={() => { }} />
        ) : null}
      </DragOverlay>
    </DndContext>
  )
}
