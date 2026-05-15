import { ChevronLeft, ChevronRight } from 'lucide-react'
import { useDroppable } from '@dnd-kit/core'
import { SortableContext, verticalListSortingStrategy } from '@dnd-kit/sortable'
import { SortableTaskCard } from './TaskCard'
import type { Task } from '../../types'
import { ColumnType } from './types'
import styles from '../../pages/Tasks.module.css'

interface ColumnProps {
  column: ColumnType
  tasks: Task[]
  onTaskClick: (task: Task) => void
  onMoveLeft?: () => void
  onMoveRight?: () => void
  canMoveLeft?: boolean
  canMoveRight?: boolean
}

export function Column({
  column,
  tasks,
  onTaskClick,
  onMoveLeft,
  onMoveRight,
  canMoveLeft,
  canMoveRight
}: ColumnProps) {
  const { setNodeRef, isOver } = useDroppable({
    id: column.id,
  })

  return (
    <div
      className={`${styles['kanban-column']} ${isOver ? (styles['drag-over'] || 'drag-over') : ''}`}
    >
      <div className={styles['column-header']}>
        <div className={styles['column-title']}>
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
