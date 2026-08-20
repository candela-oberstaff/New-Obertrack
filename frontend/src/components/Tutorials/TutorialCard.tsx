import { Pencil, Trash2, Check, GripVertical, BarChart3, Video, Image as ImageIcon, FileText, Crosshair, Radio, CalendarClock, ShieldCheck } from 'lucide-react'
import { useSortable } from '@dnd-kit/sortable'
import { CSS } from '@dnd-kit/utilities'
import { TutorialIcon } from './icons'
import { isEmptyTarget } from '../../types'
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
  onMetrics: (tutorial: Tutorial) => void
}

/** Icono y nombre de cada formato, para la chapa de la tarjeta. */
const CONTENT_BADGES = {
  video: { icon: Video, label: 'Video' },
  imagen: { icon: ImageIcon, label: 'Imagen' },
  texto: { icon: FileText, label: 'Texto' },
} as const

/** El aviso a pantalla completa sigue emergiendo para quien no la ha visto. */
function isAnnouncementOpen(tutorial: Tutorial): boolean {
  if (!tutorial.announced_at || !tutorial.announce_days) return false
  const closesAt = new Date(tutorial.announced_at).getTime() + tutorial.announce_days * 86_400_000
  return closesAt > Date.now()
}

export function TutorialCard({ tutorial, isAdmin, isViewed, sortable, onOpen, onEdit, onDelete, onMetrics }: TutorialCardProps) {
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

  const handleMetrics = (e: React.MouseEvent) => {
    e.stopPropagation()
    onMetrics(tutorial)
  }

  const handleCardClick = () => {
    if (!isDragging) onOpen(tutorial)
  }

  const contentBadge = CONTENT_BADGES[tutorial.content_type] ?? CONTENT_BADGES.video
  const ContentIcon = contentBadge.icon
  const isTargeted = !isEmptyTarget(tutorial.target)
  const announcing = isAdmin && isAnnouncementOpen(tutorial)
  // Programada: tiene hora futura y todavía no se ha publicado.
  const scheduled = isAdmin && !!tutorial.publish_at && !tutorial.announced_at
    && new Date(tutorial.publish_at).getTime() > Date.now()

  return (
    <article
      ref={setNodeRef}
      style={style}
      className={`${styles['tutorial-card']} ${!tutorial.is_active ? styles['inactive'] : ''} ${isDragging ? styles['dragging'] : ''}`}
      onClick={handleCardClick}
      data-tour="tutorial-card"
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
            <div className={styles['tutorial-card-actions']} data-tour="tutorial-card-actions">
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
                onClick={handleMetrics}
                title="Métricas"
              >
                <BarChart3 size={15} />
              </button>
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
      {/* La novedad de imagen se presenta con la imagen: sin portada, la
          tarjeta quedaría igual que una de texto y no se distinguirían. */}
      {tutorial.content_type === 'imagen' && tutorial.image_url && (
        <div className={styles['tutorial-card-cover']}>
          <img src={tutorial.image_url} alt="" />
        </div>
      )}
      {tutorial.category && tutorial.category !== 'General' && (
        <span className={styles['tutorial-card-category']}>{tutorial.category}</span>
      )}
      <h3 className={styles['tutorial-card-title']}>{tutorial.title}</h3>
      <p className={styles['tutorial-card-description']}>{tutorial.description}</p>
      <div className={styles['tutorial-card-footer']}>
        {/* El formato manda: es lo que dice si esto es un video que hay que
            mirar o un texto que se lee en diez segundos. */}
        <span className={styles['tutorial-card-format']}>
          <ContentIcon size={12} /> {contentBadge.label}
        </span>
        {tutorial.duration_min > 0 && (
          <span className={styles['tutorial-card-duration']}>{tutorial.duration_min} min</span>
        )}
        {scheduled && (
          <span className={`${styles['tutorial-card-badge']} ${styles['scheduled']}`} title="Se publicará sola a esa hora">
            <CalendarClock size={11} /> {new Date(tutorial.publish_at!).toLocaleString('es-ES', {
              day: '2-digit', month: '2-digit', hour: '2-digit', minute: '2-digit',
            })}
          </span>
        )}
        {isAdmin && tutorial.require_ack && (
          <span className={`${styles['tutorial-card-badge']} ${styles['ack']}`} title="Exige confirmar la lectura">
            <ShieldCheck size={11} /> Confirmación
          </span>
        )}
        {announcing && (
          <span className={`${styles['tutorial-card-badge']} ${styles['announcing']}`} title="El aviso a pantalla completa sigue activo">
            <Radio size={11} /> Anunciando
          </span>
        )}
        {isAdmin && isTargeted && (
          <span className={`${styles['tutorial-card-badge']} ${styles['targeted']}`} title="Esta novedad va a un público acotado">
            <Crosshair size={11} /> Público acotado
          </span>
        )}
        {!tutorial.is_active && (
          <span className={styles['tutorial-card-badge']}>Oculto</span>
        )}
        {isAdmin && tutorial.audience === 'empleador' && (
          <span className={`${styles['tutorial-card-badge']} ${styles['audience-empleador']}`}>Empresas</span>
        )}
        {isAdmin && tutorial.audience === 'profesional' && (
          <span className={`${styles['tutorial-card-badge']} ${styles['audience-profesional']}`}>Profesionales</span>
        )}
        {isAdmin && tutorial.audience === 'manager' && (
          <span className={`${styles['tutorial-card-badge']} ${styles['audience-manager']}`}>Managers</span>
        )}
      </div>
    </article>
  )
}
