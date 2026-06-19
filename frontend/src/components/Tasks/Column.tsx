import type { DragEvent } from 'react'
import { useMemo, useState } from 'react'
import { useDroppable } from '@dnd-kit/core'
import { SortableContext, verticalListSortingStrategy } from '@dnd-kit/sortable'
import { GripVertical } from 'lucide-react'
import { SortableTaskCard } from './TaskCard'
import { TaskFilters, DEFAULT_FILTERS, type TaskFiltersState } from './components/TaskFilters'
import type { Task } from '../../types'
import { parseDateOnly } from '../../utils/date'
import { ColumnType } from './types'
import styles from '../../pages/Tasks.module.css'

interface ColumnProps {
  column: ColumnType
  tasks: Task[]
  onTaskClick: (task: Task) => void
  index?: number
  columnDraggable?: boolean
  isColumnDragging?: boolean
  isColumnDragOver?: boolean
  onColumnDragStart?: (index: number) => void
  onColumnDragEnter?: (index: number) => void
  onColumnDrop?: (index: number) => void
  onColumnDragEnd?: () => void
}

function sortByEndDate(a: Task, b: Task) {
  if (!a.end_date && !b.end_date) return 0
  if (!a.end_date) return 1
  if (!b.end_date) return -1
  return parseDateOnly(a.end_date).getTime() - parseDateOnly(b.end_date).getTime()
}

function filterByPriority(task: Task, priority: string) {
  if (!priority) return true
  return task.priority === priority
}

function filterByDateRange(task: Task, dateFrom: string, dateTo: string) {
  if (!dateFrom && !dateTo) return true
  if (!task.end_date) return !dateFrom && !dateTo
  const d = parseDateOnly(task.end_date).getTime()
  if (dateFrom && d < parseDateOnly(dateFrom).getTime()) return false
  if (dateTo && d > new Date(dateTo + 'T23:59:59').getTime()) return false
  return true
}

function filterByDateStatus(task: Task, dateStatus: string) {
  if (!dateStatus) return true
  if (!task.end_date) return false
  const now = new Date()
  const today = new Date(now.getFullYear(), now.getMonth(), now.getDate()).getTime()
  const due = parseDateOnly(task.end_date).getTime()
  if (dateStatus === 'overdue') return due < today && task.status !== 'finalizado'
  if (dateStatus === 'today') return due >= today && due < today + 86400000
  if (dateStatus === 'week') return due >= today && due < today + 7 * 86400000
  return true
}

export function Column({
  column,
  tasks,
  onTaskClick,
  index,
  columnDraggable = false,
  isColumnDragging = false,
  isColumnDragOver = false,
  onColumnDragStart,
  onColumnDragEnter,
  onColumnDrop,
  onColumnDragEnd,
}: ColumnProps) {
  const { setNodeRef, isOver } = useDroppable({
    id: column.id,
  })

  const [filters, setFilters] = useState<TaskFiltersState>(DEFAULT_FILTERS)

  const filteredSorted = useMemo(() => {
    return tasks
      .filter((t) => filterByPriority(t, filters.priority) && filterByDateRange(t, filters.dateFrom, filters.dateTo) && filterByDateStatus(t, filters.dateStatus))
      .sort(sortByEndDate)
  }, [tasks, filters])

  const colDragHandlers = columnDraggable && typeof index === 'number'
    ? {
        onDragOver: (e: DragEvent) => { e.preventDefault(); onColumnDragEnter?.(index) },
        onDrop: (e: DragEvent) => { e.preventDefault(); onColumnDrop?.(index) },
      }
    : {}

  return (
    <div
      className={`${styles['kanban-column']} ${isOver ? (styles['drag-over'] || 'drag-over') : ''} ${isColumnDragging ? (styles['column-dragging'] || 'column-dragging') : ''} ${isColumnDragOver ? (styles['column-drag-over'] || 'column-drag-over') : ''}`}
      {...colDragHandlers}
    >
      <div
        className={styles['column-header']}
        draggable={columnDraggable}
        onDragStart={columnDraggable && typeof index === 'number' ? () => onColumnDragStart?.(index) : undefined}
        onDragEnd={columnDraggable ? () => onColumnDragEnd?.() : undefined}
        style={columnDraggable ? { cursor: 'grab' } : undefined}
      >
        <div className={styles['column-title']}>
          {columnDraggable && (
            <span className={styles['column-grip'] || 'column-grip'} style={{ color: '#94a3b8', display: 'flex', cursor: 'grab' }}>
              <GripVertical size={16} />
            </span>
          )}
          <span className={styles['column-dot']} style={{ backgroundColor: column.color }} />
          <span>{column.title}</span>
        </div>
        <div className={styles['column-header-actions']}>
          <TaskFilters filters={filters} onChange={setFilters} />
          <span className={styles['column-count']}>{filteredSorted.length}</span>
        </div>
      </div>
      <SortableContext items={filteredSorted.map(t => t.id)} strategy={verticalListSortingStrategy}>
        <div className={styles['column-content']} ref={setNodeRef}>
          {filteredSorted.map((task) => (
            <SortableTaskCard key={task.id} task={task} onClick={() => onTaskClick(task)} />
          ))}
          {filteredSorted.length === 0 && tasks.length > 0 && (
            <div className={styles['column-empty-filter']}>Ninguna tarea coincide con los filtros</div>
          )}
        </div>
      </SortableContext>
    </div>
  )
}
