import { sanitizeRichHtml } from '../../utils/sanitize'
import { buildEmbedUrl } from './utils'
import type { Tutorial } from '../../types'
import styles from './TutorialContent.module.css'

interface TutorialContentProps {
  tutorial: Tutorial
  /**
   * 'light' es sobre fondo blanco (la ficha de la sección); 'dark' es sobre el
   * aviso a pantalla completa. Solo cambia la paleta del texto con formato: el
   * video y la imagen se ven igual en los dos.
   */
  tone?: 'light' | 'dark'
}

/**
 * Pinta el contenido de una novedad según su tipo. Vive aparte porque lo
 * comparten el reproductor de la sección y el aviso a pantalla completa, y
 * cuando aparezca un cuarto tipo hay un solo sitio donde añadirlo.
 */
export function TutorialContent({ tutorial, tone = 'light' }: TutorialContentProps) {
  const toneClass = tone === 'dark' ? styles['tone-dark'] : styles['tone-light']

  if (tutorial.content_type === 'imagen') {
    if (!tutorial.image_url) {
      return <div className={styles['error']}>Esta novedad no tiene imagen cargada.</div>
    }
    return (
      <div className={styles['image-frame']}>
        <img src={tutorial.image_url} alt={tutorial.title} className={styles['image']} />
      </div>
    )
  }

  if (tutorial.content_type === 'texto') {
    return (
      <div
        className={`${styles['body']} ${toneClass}`}
        // El HTML viene del editor enriquecido: el backend lo limpia al
        // guardarlo y DOMPurify lo vuelve a limpiar aquí antes de pintarlo.
        dangerouslySetInnerHTML={{ __html: sanitizeRichHtml(tutorial.body) }}
      />
    )
  }

  const embedUrl = buildEmbedUrl(tutorial.google_drive_url)
  if (!embedUrl) {
    return <div className={styles['error']}>No se pudo cargar el video. El link no es de Google Drive ni YouTube.</div>
  }

  return (
    <div className={styles['video-frame']}>
      <iframe
        src={embedUrl}
        title={tutorial.title}
        allow="autoplay; encrypted-media"
        allowFullScreen
        className={styles['video']}
      />
    </div>
  )
}
