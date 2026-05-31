import { Pencil, Trash2, Check, GripVertical } from 'lucide-react'
import { useSortable } from '@dnd-kit/sortable'
import { CSS } from '@dnd-kit/utilities'
import { TutorialIcon } from './icons'
import type { Tutorial } from '../../types'
import styles from '../../pages/Tutoriales.module.css'

interface TutorialCardProps {
  tutorial: Tutorial
  isAdmin: boolean
  isViewed: boolean
  sortable: boolean
  onOpen: (tutorial: Tutorial) => void
  onEdit: (tutorial: Tutorial) => void
  onDelete: (tutorial: Tutorial) => void
}

export function TutorialCard({ tutorial, isAdmin, isViewed, sortable, onOpen, onEdit, onDelete }: TutorialCardProps) {
  const sortableState = useSortable({ id: tutorial.id, disabled: !sortable })
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = sortableState

  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
    opacity: isDragging ? 0.5 : 1,
    zIndex: isDragging ? 10 : 'auto',
  } as React.CSSProperties

  const handleEdit = (e: React.MouseEvent) => {
    e.stopPropagation()
    onEdit(tutorial)
  }

  const handleDelete = (e: React.MouseEvent) => {
    e.stopPropagation()
    onDelete(tutorial)
  }

  const handleCardClick = () => {
    if (!isDragging) onOpen(tutorial)
  }

  return (
    <article
      ref={setNodeRef}
      style={style}
      className={`${styles['tutorial-card']} ${!tutorial.is_active ? styles['inactive'] : ''} ${isDragging ? styles['dragging'] : ''}`}
      onClick={handleCardClick}
    >
      <div className={styles['tutorial-card-header']}>
        <div className={styles['tutorial-card-icon']}>
          <TutorialIcon name={tutorial.icon_name} size={22} />
        </div>
        <div className={styles['tutorial-card-header-right']}>
          {isViewed && (
            <span className={styles['tutorial-card-viewed']} title="Ya visto">
              <Check size={12} />
            </span>
          )}
          {isAdmin && (
            <div className={styles['tutorial-card-actions']}>
              {sortable && (
                <button
                  type="button"
                  className={styles['tutorial-card-action-btn']}
                  onClick={(e) => e.stopPropagation()}
                  title="Arrastrar para reordenar"
                  {...attributes}
                  {...listeners}
                >
                  <GripVertical size={15} />
                </button>
              )}
              <button
                type="button"
                className={styles['tutorial-card-action-btn']}
                onClick={handleEdit}
                title="Editar"
              >
                <Pencil size={15} />
              </button>
              <button
                type="button"
                className={`${styles['tutorial-card-action-btn']} ${styles['danger']}`}
                onClick={handleDelete}
                title="Eliminar"
              >
                <Trash2 size={15} />
              </button>
            </div>
          )}
        </div>
      </div>
      {tutorial.category && tutorial.category !== 'General' && (
        <span className={styles['tutorial-card-category']}>{tutorial.category}</span>
      )}
      <h3 className={styles['tutorial-card-title']}>{tutorial.title}</h3>
      <p className={styles['tutorial-card-description']}>{tutorial.description}</p>
      <div className={styles['tutorial-card-footer']}>
        {tutorial.duration_min > 0 && (
          <span className={styles['tutorial-card-duration']}>{tutorial.duration_min} min</span>
        )}
        {!tutorial.is_active && (
          <span className={styles['tutorial-card-badge']}>Oculto</span>
        )}
      </div>
    </article>
  )
}
