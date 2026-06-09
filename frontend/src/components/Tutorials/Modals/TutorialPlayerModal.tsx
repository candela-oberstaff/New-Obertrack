import { buildEmbedUrl } from '../utils'
import { Modal } from '../../ui'
import type { Tutorial } from '../../../types'
import styles from '../../../pages/Tutoriales.module.css'

interface TutorialPlayerModalProps {
  tutorial: Tutorial | null
  onClose: () => void
}

export function TutorialPlayerModal({ tutorial, onClose }: TutorialPlayerModalProps) {
  if (!tutorial) return null

  const embedUrl = buildEmbedUrl(tutorial.google_drive_url)

  return (
    <Modal
      isOpen
      onClose={onClose}
      size="lg"
      title={
        <span style={{ display: 'inline-flex', alignItems: 'baseline', gap: '10px' }}>
          {tutorial.title}
          {tutorial.duration_min > 0 && (
            <span className={styles['tutorial-player-duration']}>{tutorial.duration_min} min</span>
          )}
        </span>
      }
    >
      <div className={styles['tutorial-player-body']}>
        {embedUrl ? (
          <div className={styles['tutorial-player-iframe-wrapper']}>
            <iframe
              src={embedUrl}
              title={tutorial.title}
              allow="autoplay; encrypted-media"
              allowFullScreen
              className={styles['tutorial-player-iframe']}
            />
          </div>
        ) : (
          <div className={styles['tutorial-player-error']}>
            No se pudo cargar el video. El link no es de Google Drive ni YouTube.
          </div>
        )}
        {tutorial.description && (
          <p className={styles['tutorial-player-description']}>{tutorial.description}</p>
        )}
      </div>
    </Modal>
  )
}
