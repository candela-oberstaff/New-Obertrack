import { Pencil, Trash2, Check, BarChart3, Video, Image as ImageIcon, FileText, Crosshair, Radio, CalendarClock, ShieldCheck, MousePointerClick } from 'lucide-react'
import { TutorialIcon } from './icons'
import { isEmptyTarget } from '../../types'
import type { Tutorial } from '../../types'
import styles from './TutorialTable.module.css'

interface TutorialTableProps {
  tutorials: Tutorial[]
  isAdmin: boolean
  viewedIds: Set<number>
  onOpen: (tutorial: Tutorial) => void
  onEdit: (tutorial: Tutorial) => void
  onDelete: (tutorial: Tutorial) => void
  onMetrics: (tutorial: Tutorial) => void
}

const CONTENT_LABELS = {
  video: { icon: Video, label: 'Video' },
  imagen: { icon: ImageIcon, label: 'Imagen' },
  texto: { icon: FileText, label: 'Texto' },
} as const

const AUDIENCE_LABELS: Record<string, string> = {
  all: 'Todos',
  empleador: 'Empresas',
  profesional: 'Profesionales',
}

function isAnnouncementOpen(tutorial: Tutorial): boolean {
  if (!tutorial.announced_at || !tutorial.announce_days) return false
  return new Date(tutorial.announced_at).getTime() + tutorial.announce_days * 86_400_000 > Date.now()
}

function formatDate(value?: string | null): string {
  if (!value) return '—'
  return new Date(value).toLocaleDateString('es-ES', { day: '2-digit', month: '2-digit', year: '2-digit' })
}

/**
 * Vista de tabla de las novedades. Responde otra pregunta que las tarjetas:
 * ahí se mira una novedad, aquí se comparan todas —qué formato tiene cada una,
 * a quién va, cuál sigue anunciándose— y por eso el estado va en columnas y no
 * en chapas sueltas.
 */
export function TutorialTable({ tutorials, isAdmin, viewedIds, onOpen, onEdit, onDelete, onMetrics }: TutorialTableProps) {
  return (
    <div className={styles['wrapper']} data-tour="tutoriales-grid">
      <table className={styles['table']}>
        <thead>
          <tr>
            <th className={styles['col-check']}><span className={styles['sr-only']}>Vista</span></th>
            <th>Novedad</th>
            <th className={styles['col-format']}>Formato</th>
            <th className={styles['col-category']}>Categoría</th>
            {isAdmin && <th className={styles['col-audience']}>Dirigido a</th>}
            <th className={styles['col-state']}>Estado</th>
            {isAdmin && <th className={styles['col-actions']}><span className={styles['sr-only']}>Acciones</span></th>}
          </tr>
        </thead>
        <tbody>
          {tutorials.map((tutorial) => {
            const content = CONTENT_LABELS[tutorial.content_type] ?? CONTENT_LABELS.video
            const ContentIcon = content.icon
            const announcing = isAnnouncementOpen(tutorial)
            const scheduled = !!tutorial.publish_at && !tutorial.announced_at
              && new Date(tutorial.publish_at).getTime() > Date.now()

            return (
              <tr key={tutorial.id} onClick={() => onOpen(tutorial)} className={styles['row']}>
                <td className={styles['col-check']}>
                  {viewedIds.has(tutorial.id) && (
                    <span className={styles['viewed']} title="Ya vista"><Check size={13} /></span>
                  )}
                </td>

                <td>
                  <div className={styles['title-cell']}>
                    <span className={styles['title-icon']}>
                      <TutorialIcon name={tutorial.icon_name} size={17} />
                    </span>
                    <span className={styles['title-text']}>
                      <strong>{tutorial.title}</strong>
                      {tutorial.description && <small>{tutorial.description}</small>}
                    </span>
                  </div>
                </td>

                <td className={styles['col-format']}>
                  <span className={styles['chip']}>
                    <ContentIcon size={12} /> {content.label}
                  </span>
                  {tutorial.duration_min > 0 && (
                    <span className={styles['muted']}> · {tutorial.duration_min} min</span>
                  )}
                </td>

                <td className={styles['col-category']}>{tutorial.category || 'General'}</td>

                {isAdmin && (
                  <td className={styles['col-audience']}>
                    {AUDIENCE_LABELS[tutorial.audience] ?? tutorial.audience}
                    {!isEmptyTarget(tutorial.target) && (
                      <span className={styles['target']} title="Público acotado">
                        <Crosshair size={11} /> acotado
                      </span>
                    )}
                  </td>
                )}

                <td className={styles['col-state']}>
                  <div className={styles['states']}>
                    {!tutorial.is_active && !scheduled && <span className={styles['chip']}>Oculta</span>}
                    {scheduled && (
                      <span className={`${styles['chip']} ${styles['scheduled']}`} title="Se publicará sola">
                        <CalendarClock size={11} /> {formatDate(tutorial.publish_at)}
                      </span>
                    )}
                    {announcing && (
                      <span className={`${styles['chip']} ${styles['announcing']}`}>
                        <Radio size={11} /> Anunciando
                      </span>
                    )}
                    {tutorial.require_ack && (
                      <span className={`${styles['chip']} ${styles['ack']}`} title="Exige confirmar la lectura">
                        <ShieldCheck size={11} /> Confirmación
                      </span>
                    )}
                    {!!tutorial.cta_label && (
                      <span className={`${styles['chip']} ${styles['cta']}`} title={tutorial.cta_url}>
                        <MousePointerClick size={11} /> {tutorial.cta_label}
                      </span>
                    )}
                  </div>
                </td>

                {isAdmin && (
                  <td className={styles['col-actions']}>
                    {/* Las acciones no deben abrir la novedad al pulsarlas. */}
                    <div className={styles['actions']} onClick={(e) => e.stopPropagation()}>
                      <button type="button" onClick={() => onMetrics(tutorial)} title="Métricas">
                        <BarChart3 size={15} />
                      </button>
                      <button type="button" onClick={() => onEdit(tutorial)} title="Editar">
                        <Pencil size={15} />
                      </button>
                      <button type="button" className={styles['danger']} onClick={() => onDelete(tutorial)} title="Eliminar">
                        <Trash2 size={15} />
                      </button>
                    </div>
                  </td>
                )}
              </tr>
            )
          })}
        </tbody>
      </table>
    </div>
  )
}
