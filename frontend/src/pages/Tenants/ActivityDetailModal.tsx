import { ArrowRight, Pin } from 'lucide-react'
import { Modal, Button } from '../../components/ui'
import { ACTIVITY_STYLE, ACTIVITY_LABEL, ACTIVITY_FALLBACK, CONTACT_STYLE } from './activityStyle'
import type { TenantActivity } from '../../hooks'
import styles from './ActivityDetail.module.css'

interface Props {
  activity: TenantActivity | null
  onClose: () => void
  /** Enlace al módulo que originó la entrada (hoy solo el testimonio). */
  onOpenSource?: (refId: number) => void
  /** El hilo de comentarios y archivos, si la entrada existe como registro. */
  children?: React.ReactNode
}

/**
 * Detalle de un movimiento del expediente de una empresa.
 *
 * La cronología es una lista para ojear: cada fila cabe en una línea y el texto
 * largo se corta. Aquí es al revés — el movimiento se ve entero, con quién,
 * cuándo y de qué tipo, y con sitio para su hilo de comentarios y archivos, que
 * en la fila quedaba apretado contra el borde.
 */
export function ActivityDetailModal({ activity, onClose, onOpenSource, children }: Props) {
  if (!activity) return null

  const st = ACTIVITY_STYLE[activity.type] || ACTIVITY_FALLBACK
  const Icon = st.icon
  const date = new Date(activity.timestamp)
  const validDate = !isNaN(date.getTime())
  const contact = activity.channel ? CONTACT_STYLE[activity.channel] : null

  return (
    <Modal
      isOpen
      onClose={onClose}
      size="lg"
      title="Detalle del movimiento"
      footer={
        <>
          {/* Un testimonio es más que su cita: tiene firma, permisos y
              constancia. Desde aquí se llega al registro completo. */}
          {activity.type === 'company_testimonial' && !!activity.ref_id && onOpenSource && (
            <Button
              variant="secondary"
              rightIcon={<ArrowRight size={14} />}
              onClick={() => onOpenSource(activity.ref_id!)}
            >
              Ver testimonio
            </Button>
          )}
          <Button onClick={onClose}>Cerrar</Button>
        </>
      }
    >
      <div className={styles.wrap}>
        <header className={styles.head}>
          <span className={styles.icon} style={{ color: st.color }}>
            <Icon size={18} />
          </span>
          <div className={styles.headText}>
            <span className={styles.kind} style={{ color: st.color }}>
              {ACTIVITY_LABEL[activity.type] || activity.type}
            </span>
            {activity.pinned && (
              <span className={styles.pinned}><Pin size={11} /> fijada</span>
            )}
          </div>
        </header>

        {/* El texto va entero y respeta los saltos de línea: una nota de tres
            párrafos en la fila se veía como un renglón cortado. */}
        <p className={styles.body}>{activity.details}</p>

        <dl className={styles.facts}>
          <div className={styles.fact}>
            <dt>Quién</dt>
            <dd>{activity.user || 'Sistema'}</dd>
          </div>
          <div className={styles.fact}>
            <dt>Cuándo</dt>
            <dd>
              {validDate
                ? date.toLocaleString('es-ES', {
                    day: '2-digit', month: 'long', year: 'numeric',
                    hour: '2-digit', minute: '2-digit',
                  })
                : '—'}
            </dd>
          </div>
          {contact && (
            <div className={styles.fact}>
              <dt>Vía</dt>
              <dd>{contact.label}</dd>
            </div>
          )}
          {activity.edited_at && (
            <div className={styles.fact}>
              <dt>Editada</dt>
              <dd>{new Date(activity.edited_at).toLocaleString('es-ES')}</dd>
            </div>
          )}
        </dl>

        {children && <div className={styles.thread}>{children}</div>}
      </div>
    </Modal>
  )
}

export default ActivityDetailModal
