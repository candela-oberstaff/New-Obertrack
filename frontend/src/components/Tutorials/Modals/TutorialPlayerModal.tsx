import { useNavigate } from 'react-router-dom'
import { ExternalLink } from 'lucide-react'
import { Modal, Button } from '../../ui'
import { TutorialContent } from '../TutorialContent'
import { tutorialService } from '../../../services/api'
import type { Tutorial } from '../../../types'
import styles from '../../../pages/Tutoriales.module.css'

interface TutorialPlayerModalProps {
  tutorial: Tutorial | null
  onClose: () => void
}

export function TutorialPlayerModal({ tutorial, onClose }: TutorialPlayerModalProps) {
  const navigate = useNavigate()

  if (!tutorial) return null

  const hasCTA = !!tutorial.cta_label && !!tutorial.cta_url

  // El mismo botón que en el aviso a pantalla completa: la novedad se puede
  // abrir por cualquiera de los dos sitios y el clic cuenta igual.
  const handleCTA = () => {
    void tutorialService.recordClick(tutorial.id).catch(() => {})
    if (/^https?:\/\//.test(tutorial.cta_url)) {
      window.open(tutorial.cta_url, '_blank', 'noopener,noreferrer')
      return
    }
    // Navegación del router, no recarga completa: la ruta es interna.
    onClose()
    navigate(tutorial.cta_url)
  }

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
      footer={hasCTA ? (
        <Button onClick={handleCTA}>
          {tutorial.cta_label} <ExternalLink size={15} />
        </Button>
      ) : undefined}
    >
      <div className={styles['tutorial-player-body']}>
        <TutorialContent tutorial={tutorial} />
        {/* En las novedades de texto la descripción ya es el resumen de la
            tarjeta: repetirla encima del contenido sobra. */}
        {tutorial.description && tutorial.content_type !== 'texto' && (
          <p className={styles['tutorial-player-description']}>{tutorial.description}</p>
        )}
      </div>
    </Modal>
  )
}
