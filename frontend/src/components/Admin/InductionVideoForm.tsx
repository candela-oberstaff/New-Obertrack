import { useState } from 'react'
import { Play, Save } from 'lucide-react'

import { useNotification } from '../../context/NotificationContext'
import { inductionService, type InductionVideo } from '../../services/induction.service'
import { getProviderLabel, parseVideoUrl } from '../Tutorials/utils'
import styles from './InductionSettings.module.css'

interface Props {
  /** null = video nuevo. */
  video: InductionVideo | null
  onSaved: (video: InductionVideo) => void
  onCancel: () => void
}

/**
 * Alta o edición de un video de la biblioteca de la inducción: título, enlace
 * de Drive o YouTube (con su reproductor al lado para confirmar que es el
 * correcto), duración y descripción.
 */
export default function InductionVideoForm({ video, onSaved, onCancel }: Props) {
  const { success, error: showError } = useNotification()
  const [title, setTitle] = useState(video?.title ?? '')
  const [url, setUrl] = useState(video?.video_url ?? '')
  const [duration, setDuration] = useState<string>(video?.duration_min ? String(video.duration_min) : '')
  const [description, setDescription] = useState(video?.description ?? '')
  const [saving, setSaving] = useState(false)

  const info = parseVideoUrl(url)

  const handleSave = async () => {
    if (!title.trim()) {
      showError('El video necesita un título.')
      return
    }
    if (!info) {
      showError('Pega un enlace de Google Drive (archivo) o de YouTube.')
      return
    }
    setSaving(true)
    try {
      const input = {
        title: title.trim(),
        description: description.trim(),
        video_url: url.trim(),
        duration_min: Number(duration) || 0,
      }
      const saved = video
        ? await inductionService.updateVideo(video.id, input)
        : await inductionService.createVideo(input)
      success(video ? 'Video guardado.' : 'Video agregado a la biblioteca.')
      onSaved(saved)
    } catch (err: any) {
      showError(err?.response?.data?.error ?? 'No se pudo guardar el video.')
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className={styles.videoFormSplit}>
      <div className={styles.videoFormFields}>
        <div className={styles.field}>
          <label htmlFor="video-title">Título</label>
          <input
            id="video-title"
            type="text"
            placeholder="Ej. Bienvenida a Oberstaff"
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            autoFocus
          />
        </div>
        <div className={styles.field}>
          <label htmlFor="video-url">Enlace del video</label>
          <input
            id="video-url"
            type="url"
            placeholder="https://youtu.be/… o https://drive.google.com/file/d/…"
            value={url}
            onChange={(e) => setUrl(e.target.value)}
          />
          <span className={styles.hint} style={{ margin: '6px 0 0' }}>
            {url.trim() === ''
              ? 'YouTube o un archivo de Google Drive compartido con «cualquiera con el enlace».'
              : info
                ? `${getProviderLabel(info.provider)} · se ve a la derecha.`
                : 'Ese enlace no es de YouTube ni de un archivo de Drive.'}
          </span>
        </div>
        <div className={styles.field} style={{ maxWidth: 200 }}>
          <label htmlFor="video-duration">Duración (min)</label>
          <input
            id="video-duration"
            type="number"
            min={0}
            max={600}
            placeholder="Opcional"
            value={duration}
            onChange={(e) => setDuration(e.target.value)}
          />
        </div>
        <div className={styles.field}>
          <label htmlFor="video-description">Descripción (opcional)</label>
          <textarea
            id="video-description"
            placeholder="De qué trata, para encontrarlo luego."
            value={description}
            onChange={(e) => setDescription(e.target.value)}
          />
        </div>
        <div className={styles.videoFormActions}>
          <button type="button" className={styles.ghostBtn} onClick={onCancel} disabled={saving}>
            Cancelar
          </button>
          <button type="button" className={styles.saveBtn} onClick={handleSave} disabled={saving}>
            <Save size={16} /> {saving ? 'Guardando...' : video ? 'Guardar video' : 'Agregar video'}
          </button>
        </div>
      </div>

      <div className={styles.videoPreview} style={{ borderRadius: 14, overflow: 'hidden' }}>
        {info ? (
          <iframe src={info.embedUrl} title={title || 'Vista previa'} allow="autoplay; encrypted-media" allowFullScreen />
        ) : (
          <div className={styles.videoThumb} style={{ height: '100%', margin: 0, borderRadius: 0 }}>
            <Play size={28} />
          </div>
        )}
      </div>
    </div>
  )
}
