import type { DragEvent } from 'react'
import { useDroppable } from '@dnd-kit/core'
import { SortableContext, verticalListSortingStrategy } from '@dnd-kit/sortable'
import { GripVertical } from 'lucide-react'
import { SortableTaskCard } from './TaskCard'
import type { Task } from '../../types'
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
        <span className={styles['column-count']}>{tasks.length}</span>
      </div>
      <SortableContext items={tasks.map(t => t.id)} strategy={verticalListSortingStrategy}>
        <div className={styles['column-content']} ref={setNodeRef}>
          {tasks.map((task) => (
            <SortableTaskCard key={task.id} task={task} onClick={() => onTaskClick(task)} />
          ))}
        </div>
      </SortableContext>
    </div>
  )
}
