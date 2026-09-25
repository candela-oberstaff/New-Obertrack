import { useEffect, useState } from 'react'
import { Award } from 'lucide-react'

import { badgeService, type BadgeOverview } from '../../services/badge.service'
import { BadgeMedallion } from './BadgeMedallion'
import styles from './Badges.module.css'

interface Props {
  /** Sin userId = las del usuario de la sesión. */
  userId?: number
  /** Clase del contenedor, para adoptar la tarjeta de cada página. */
  className?: string
  style?: React.CSSProperties
  /** Estilo del título h3, para adoptar el de cada página. */
  titleStyle?: React.CSSProperties
  /** Texto del estado vacío. */
  emptyText?: string
  /** Muestra también las que faltan por ganar (solo tiene sentido para uno mismo o para Soporte). */
  showPending?: boolean
  /** Cambia para volver a cargar (por ejemplo, tras reiniciar la inducción). */
  refreshKey?: unknown
}

function formatDate(iso: string) {
  return new Date(iso).toLocaleDateString('es-ES', { day: '2-digit', month: 'short', year: 'numeric' })
}

/**
 * Tarjeta de insignias de una persona: las ganadas como medallones y, si se
 * pide, las que todavía puede ganar en su inducción en curso, en gris.
 *
 * Se oculta sola cuando no hay nada que mostrar y no se pidieron pendientes:
 * una tarjeta vacía en el expediente de alguien que nunca pasó por la
 * inducción solo ensuciaría.
 */
export function UserBadges({
  userId,
  className,
  style,
  titleStyle,
  emptyText = 'Todavía no hay insignias. Se ganan completando la inducción.',
  showPending = false,
  refreshKey,
}: Props) {
  const [data, setData] = useState<BadgeOverview | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    let cancelled = false
    setLoading(true)
    const req = userId ? badgeService.forUser(userId) : badgeService.mine()
    req
      .then((d) => {
        if (!cancelled) setData(d)
      })
      .catch(() => {
        if (!cancelled) setData(null)
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [userId, refreshKey])

  if (loading || !data) return null
  const earned = data.earned ?? []
  const pending = showPending ? data.pending ?? [] : []
  if (earned.length === 0 && pending.length === 0 && !showPending) return null

  return (
    <div className={className} style={style}>
      <h3 style={{ display: 'flex', alignItems: 'center', gap: 8, ...titleStyle }}>
        <Award size={18} /> Insignias
        <span className={styles.count}>{earned.length}</span>
      </h3>

      {earned.length === 0 ? (
        <p className={styles.empty}>{emptyText}</p>
      ) : (
        <div className={styles.grid}>
          {earned.map((b) => (
            <div key={b.id} className={styles.item} title={b.description}>
              <BadgeMedallion icon={b.icon} color={b.color} size="md" title={b.title} />
              <span className={styles.itemTitle}>{b.title}</span>
              <span className={styles.itemMeta}>{formatDate(b.earned_at)}</span>
            </div>
          ))}
        </div>
      )}

      {pending.length > 0 && (
        <>
          <div className={styles.sectionLabel}>Por ganar</div>
          <div className={styles.grid}>
            {pending.map((p, i) => (
              <div key={`${p.kind}-${i}`} className={`${styles.item} ${styles.itemLocked}`} title="Se gana al aprobar">
                <BadgeMedallion icon={p.icon} color={p.color} size="md" locked title={p.title} />
                <span className={styles.itemTitle}>{p.title}</span>
              </div>
            ))}
          </div>
        </>
      )}
    </div>
  )
}
