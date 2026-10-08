import { useNavigate } from 'react-router-dom'
import { ChevronRight, Megaphone } from 'lucide-react'

import { TutorialIcon } from './icons'
import { publishedAt, useNovedadesFeed } from './hooks/useNovedadesFeed'
import styles from '../../pages/Dashboard.module.css'

const SHOWN = 4

function ago(ms: number): string {
  const days = Math.floor((Date.now() - ms) / 86_400_000)
  if (days <= 0) return 'Hoy'
  if (days === 1) return 'Ayer'
  if (days < 7) return `Hace ${days} días`
  return new Date(ms).toLocaleDateString('es-ES', { day: 'numeric', month: 'short' })
}

/**
 * Novedades en el dashboard: las más recientes, con las que la persona no ha
 * abierto marcadas como «Nueva» y un contador que late en la cabecera para que
 * no pasen desapercibidas. Cada una abre directo en Novedades.
 */
export default function NovedadesCard() {
  const navigate = useNavigate()
  const { items, unseen, viewedIds, isLoading } = useNovedadesFeed()

  // Las nuevas primero; después las demás, de la más reciente a la más antigua.
  const shown = [...unseen, ...items.filter((t) => viewedIds.has(t.id))].slice(0, SHOWN)
  const hasNew = unseen.length > 0

  return (
    <div
      className={`${styles['dashboard-card']} ${styles['news-card']} ${hasNew ? styles['news-card-new'] : ''}`}
      data-tour="dashboard-news-card"
    >
      <div className={styles['card-header']}>
        <h3 className={styles['news-title']}>
          <span className={styles['news-icon']}>
            <Megaphone size={16} />
          </span>
          Novedades
          {hasNew && (
            <span className={styles['news-count']}>
              <span className={styles['news-pulse']} aria-hidden />
              {unseen.length} {unseen.length === 1 ? 'nueva' : 'nuevas'}
            </span>
          )}
        </h3>
        <button className={styles['btn-link']} onClick={() => navigate('/novedades')}>
          Ver todas
        </button>
      </div>

      <div className={styles['news-list']}>
        {isLoading ? (
          <p className={styles['news-empty']}>Cargando novedades...</p>
        ) : shown.length === 0 ? (
          <div className={styles['empty-card']}>
            <Megaphone size={36} style={{ color: '#94a3b8', marginBottom: '12px' }} />
            <p>Todavía no hay novedades</p>
          </div>
        ) : (
          shown.map((t) => {
            const isNew = !viewedIds.has(t.id)
            return (
              <button
                key={t.id}
                type="button"
                className={`${styles['news-row']} ${isNew ? styles['news-row-new'] : ''}`}
                onClick={() => navigate(`/novedades?ver=${t.id}`)}
              >
                <span className={styles['news-row-icon']}>
                  <TutorialIcon name={t.icon_name} size={18} />
                </span>
                <span className={styles['news-row-text']}>
                  <span className={styles['news-row-title']}>
                    {t.title}
                    {isNew && <span className={styles['news-new-tag']}>Nueva</span>}
                  </span>
                  <span className={styles['news-row-meta']}>
                    {t.category || 'General'} · {ago(publishedAt(t))}
                  </span>
                </span>
                <ChevronRight size={16} className={styles['news-row-arrow']} />
              </button>
            )
          })
        )}
      </div>

      {hasNew && (
        <button type="button" className={styles['news-cta']} onClick={() => navigate(`/novedades?ver=${unseen[0].id}`)}>
          Tienes {unseen.length === 1 ? 'una novedad sin ver' : `${unseen.length} novedades sin ver`}: ábrela ahora
        </button>
      )}
    </div>
  )
}
