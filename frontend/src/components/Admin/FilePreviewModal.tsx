import { useEffect, useState } from 'react'
import { Download, FileText, AlertCircle } from 'lucide-react'
import { Modal, Button } from '../ui'
import styles from './FilePreview.module.css'

export interface PreviewFile {
  id: number
  title?: string
  file_name: string
  mime_type?: string
  file_size?: number
}

interface Props {
  file: PreviewFile | null
  /** Ruta autorizada del archivo. La misma que usa la descarga. */
  href: string
  onClose: () => void
}

/**
 * Tipo de vista previa que admite un archivo.
 *
 * Se decide por el mime que guardamos y, si viene vacío, por la extensión: los
 * documentos subidos antes de que se registrara el mime siguen ahí y merecen
 * verse igual.
 */
type PreviewKind = 'image' | 'pdf' | 'video' | 'audio' | 'none'

const EXT_KIND: Record<string, PreviewKind> = {
  png: 'image', jpg: 'image', jpeg: 'image', gif: 'image', webp: 'image', svg: 'image', avif: 'image', bmp: 'image',
  pdf: 'pdf',
  mp4: 'video', webm: 'video', mov: 'video',
  mp3: 'audio', wav: 'audio', ogg: 'audio', m4a: 'audio',
}

export function previewKind(file: PreviewFile): PreviewKind {
  const mime = (file.mime_type || '').toLowerCase()
  if (mime.startsWith('image/')) return 'image'
  if (mime === 'application/pdf') return 'pdf'
  if (mime.startsWith('video/')) return 'video'
  if (mime.startsWith('audio/')) return 'audio'

  const ext = file.file_name.split('.').pop()?.toLowerCase() ?? ''
  return EXT_KIND[ext] ?? 'none'
}

/** Tamaño legible. Un número de bytes crudo no le dice nada a quien revisa. */
function formatSize(bytes?: number): string {
  if (!bytes || bytes <= 0) return ''
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${Math.round(bytes / 1024)} KB`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}

/**
 * Visor de un adjunto del expediente, sin salir de la ficha.
 *
 * Antes cada archivo abría una pestaña nueva: para comprobar una captura había
 * que salir de la ficha, mirar, y volver a buscar dónde se estaba. Con
 * evaluaciones que se revisan una detrás de otra, eso es la mitad del trabajo.
 *
 * La imagen se pide a la MISMA ruta autorizada que la descarga —nunca al
 * archivo crudo—, así que el backend sigue comprobando visibilidad y propiedad
 * en cada vista. Lo que no se puede previsualizar (un .docx, un .zip) no se
 * intenta: se dice claro y se ofrece bajarlo.
 */
export function FilePreviewModal({ file, href, onClose }: Props) {
  const [failed, setFailed] = useState(false)

  // Escape cierra SOLO el visor.
  //
  // Modal escucha la tecla en `document` en fase de burbujeo, así que con el
  // expediente abierto detrás las dos ventanas la recibirían y un solo Escape
  // cerraría ambas —perdiendo de paso la nota a medio escribir—. Este oyente va
  // en fase de captura, que corre antes, y corta el reparto ahí mismo.
  // Cada archivo empieza limpio: sin esto, uno que falló dejaba al siguiente
  // mostrando el aviso de error aunque se viera perfectamente.
  useEffect(() => { setFailed(false) }, [file?.id])

  useEffect(() => {
    if (!file) return
    const onKey = (e: KeyboardEvent) => {
      if (e.key !== 'Escape') return
      e.stopImmediatePropagation()
      onClose()
    }
    document.addEventListener('keydown', onKey, true)
    return () => document.removeEventListener('keydown', onKey, true)
  }, [file, onClose])

  if (!file) return null

  const kind = previewKind(file)
  const name = file.title || file.file_name
  const size = formatSize(file.file_size)

  return (
    <Modal
      isOpen
      onClose={onClose}
      size="xl"
      title={name}
      footer={
        <>
          <span className={styles.meta}>
            {file.file_name}{size ? ` · ${size}` : ''}
          </span>
          {/* El atributo download fuerza la descarga aunque el archivo se
              pueda mostrar: sin él, un PDF se abriría en otra pestaña y el
              botón no haría lo que dice. */}
          <a href={href} download={file.file_name} className={styles.downloadLink}>
            <Button variant="secondary" leftIcon={<Download size={15} />}>Descargar</Button>
          </a>
        </>
      }
    >
      <div className={styles.viewer}>
        {failed || kind === 'none' ? (
          <div className={styles.fallback}>
            {failed ? <AlertCircle size={34} /> : <FileText size={34} />}
            <p className={styles.fallbackTitle}>
              {failed ? 'No se pudo mostrar el archivo' : 'Este tipo de archivo no se puede previsualizar'}
            </p>
            <p className={styles.fallbackHint}>
              {failed
                ? 'Puede que se haya movido o que ya no esté disponible. Prueba a descargarlo.'
                : 'Descárgalo para abrirlo con el programa que le corresponda.'}
            </p>
          </div>
        ) : kind === 'image' ? (
          <img src={href} alt={name} className={styles.image} onError={() => setFailed(true)} />
        ) : kind === 'pdf' ? (
          // title es obligatorio para que un lector de pantalla sepa qué hay
          // dentro del marco.
          <iframe src={href} title={name} className={styles.frame} />
        ) : kind === 'video' ? (
          <video src={href} controls className={styles.media} onError={() => setFailed(true)} />
        ) : (
          <audio src={href} controls className={styles.audio} onError={() => setFailed(true)} />
        )}
      </div>
    </Modal>
  )
}

export default FilePreviewModal
