import { useEffect } from 'react'
import { createPortal } from 'react-dom'
import { Sparkles, X, ArrowRight, Check, ExternalLink, ShieldCheck, Lock } from 'lucide-react'
import { TutorialContent } from './TutorialContent'
import type { Tutorial } from '../../types'
import styles from './NovedadAnnouncer.module.css'

interface NovedadOverlayProps {
  tutorial: Tutorial
  /** Posición dentro de la tanda; se oculta con una sola novedad. */
  position?: number
  total?: number
  /** Se pulsó Entendido / Confirmo que lo leí, la X o Escape. */
  onDismiss: () => void
  /** Se pulsó el botón de acción de la novedad. */
  onCTA?: () => void
  /** Se pidió ir a la sección. Sin él, ese botón no se muestra. */
  onSeeAll?: () => void
  /** Previsualización: no registra nada y siempre se puede cerrar. */
  preview?: boolean
}

/**
 * La novedad a pantalla completa. Es solo presentación: quién la muestra y qué
 * se registra al cerrarla vive en NovedadAnnouncer, y así el formulario puede
 * reusarla para previsualizar sin tocar ningún dato.
 */
export function NovedadOverlay({
  tutorial,
  position = 1,
  total = 1,
  onDismiss,
  onCTA,
  onSeeAll,
  preview = false,
}: NovedadOverlayProps) {
  // Una novedad que exige confirmación no se puede esquivar: sin X ni Escape,
  // porque el acuse es la evidencia de que se leyó. En previsualización sí,
  // que si no no habría forma de salir.
  const mustAcknowledge = tutorial.require_ack && !preview
  const isProse = tutorial.content_type === 'texto'
  const isLast = position >= total
  const hasCTA = !!tutorial.cta_label && !!tutorial.cta_url

  useEffect(() => {
    const previousOverflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && !mustAcknowledge) onDismiss()
    }
    document.addEventListener('keydown', onKey)
    return () => {
      document.body.style.overflow = previousOverflow
      document.removeEventListener('keydown', onKey)
    }
  }, [mustAcknowledge, onDismiss])

  const header = (
    <>
      <div className={styles['badge-row']}>
        <span className={styles['badge']}>
          <Sparkles size={13} /> {preview ? 'Previsualización' : 'Nueva novedad'}
        </span>
        {total > 1 && <span className={styles['counter']}>{position} de {total}</span>}
        {mustAcknowledge && (
          <span className={styles['ack-badge']}>
            <ShieldCheck size={13} /> Requiere confirmación
          </span>
        )}
      </div>

      <h1 className={styles['title']}>{tutorial.title || 'Sin título'}</h1>

      <div className={styles['meta']}>
        <span>{tutorial.category}</span>
        {tutorial.duration_min > 0 && (
          <>
            <span className={styles['meta-dot']} />
            <span>{tutorial.duration_min} min</span>
          </>
        )}
      </div>
    </>
  )

  // Con acuse pendiente, confirmar es lo que desbloquea el aviso: va primero y
  // con su propio peso. Sin acuse manda el botón de acción, que es la razón por
  // la que se publicó la novedad.
  const dismissLabel = mustAcknowledge
    ? <><ShieldCheck size={17} /> Confirmo que lo leí</>
    : isLast ? <><Check size={16} /> Entendido</> : <>Siguiente <ArrowRight size={16} /></>

  const dismissClass = mustAcknowledge
    ? styles['ack-btn']
    : hasCTA ? styles['secondary-btn'] : styles['primary-btn']

  const dismissButton = (
    <button type="button" className={dismissClass} onClick={onDismiss} autoFocus={mustAcknowledge}>
      {dismissLabel}
    </button>
  )

  const ctaButton = hasCTA ? (
    <button type="button" className={styles['cta-btn']} onClick={onCTA}>
      {tutorial.cta_label} <ExternalLink size={15} />
    </button>
  ) : null

  const actions = (
    <>
      <div className={styles['actions']}>
        {mustAcknowledge ? <>{dismissButton}{ctaButton}</> : <>{ctaButton}{dismissButton}</>}
        {onSeeAll && !mustAcknowledge && (
          <button type="button" className={styles['secondary-btn']} onClick={onSeeAll}>
            Ver todas las novedades
          </button>
        )}
      </div>
      {mustAcknowledge && (
        <p className={styles['ack-note']}>
          <Lock size={13} /> Este aviso solo se cierra al confirmar. Queda registrado quién lo hizo y cuándo.
        </p>
      )}
    </>
  )

  return createPortal(
    <div className={styles['overlay']} role="dialog" aria-modal="true" aria-label={`Novedad: ${tutorial.title}`}>
      {!mustAcknowledge && (
        <button
          type="button"
          className={styles['close-btn']}
          onClick={onDismiss}
          aria-label="Cerrar novedad"
        >
          <X size={20} />
        </button>
      )}

      {isProse ? (
        <div className={styles['prose-layout']}>
          <div className={styles['panel']}>{header}</div>
          <div className={styles['prose-body']}>
            <TutorialContent tutorial={tutorial} tone="dark" />
          </div>
          {actions}
        </div>
      ) : (
        <div className={styles['content']}>
          <div className={styles['media']}>
            <TutorialContent tutorial={tutorial} tone="dark" />
          </div>

          <div className={styles['panel']}>
            {header}
            {tutorial.description && <p className={styles['description']}>{tutorial.description}</p>}
            {actions}
          </div>
        </div>
      )}
    </div>,
    document.body,
  )
}
