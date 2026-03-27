import { ChevronLeft, ChevronRight } from 'lucide-react'
import { useDroppable } from '@dnd-kit/core'
import { SortableContext, verticalListSortingStrategy } from '@dnd-kit/sortable'
import { SortableTaskCard } from './TaskCard'
import type { Task } from '../../types'
import { ColumnType } from './types'

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
      className={`kanban-column ${isOver ? 'drag-over' : ''}`}
    >
      <div className="column-header">
        <div className="column-title">
          <span className="column-dot" style={{ backgroundColor: column.color }} />
          <span>{column.title}</span>
        </div>
        <div className="column-actions">
          <button 
            className="column-move-btn" 
            onClick={onMoveLeft}
            disabled={!canMoveLeft}
            title="Mover a la izquierda"
          >
            <ChevronLeft size={16} />
          </button>
          <button 
            className="column-move-btn" 
            onClick={onMoveRight}
            disabled={!canMoveRight}
            title="Mover a la derecha"
          >
            <ChevronRight size={16} />
          </button>
          <span className="column-count">{tasks.length}</span>
        </div>
      </div>
      <SortableContext items={tasks.map(t => t.id)} strategy={verticalListSortingStrategy}>
        <div className="column-content" ref={setNodeRef}>
          {tasks.map((task) => (
            <SortableTaskCard key={task.id} task={task} onClick={() => onTaskClick(task)} />
          ))}
        </div>
      </SortableContext>
    </div>
  )
}
