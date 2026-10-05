import { useMemo, useState } from 'react'
import { Ban, EyeOff, Eye, ListChecks, Play, Search } from 'lucide-react'

import type { Tutorial } from '../../types/tutorials'
import { getProviderLabel, parseVideoUrl } from '../Tutorials/utils'
import styles from './InductionSettings.module.css'

interface Props {
  /** Novedades de tipo video. */
  videos: Tutorial[]
  /** 0 = sin video. */
  value: number
  onChange: (id: number) => void
  /**
   * Oculta el video en Novedades sin salir del bloque. Ausente = no se ofrece.
   * Devuelve cuando terminó (o lanza si falló).
   */
  onHide?: (id: number) => Promise<void>
}

/** Miniatura del video: YouTube y Drive la publican por id. */
function thumbnailUrl(url: string): string | null {
  const info = parseVideoUrl(url)
  if (!info) return null
  return info.provider === 'youtube'
    ? `https://img.youtube.com/vi/${info.videoId}/mqdefault.jpg`
    : `https://drive.google.com/thumbnail?id=${info.videoId}&sz=w320`
}

function Thumb({ video }: { video: Tutorial }) {
  const src = thumbnailUrl(video.google_drive_url)
  const [failed, setFailed] = useState(false)
  return (
    <div className={styles.videoThumb}>
      {src && !failed ? (
        <img src={src} alt="" loading="lazy" onError={() => setFailed(true)} />
      ) : (
        <Play size={22} />
      )}
    </div>
  )
}

/**
 * Elegir el video del bloque: la lista con miniaturas a la izquierda y, a la
 * derecha, el elegido con su reproductor para confirmar que es el correcto.
 * Cambiar de video es un clic en otra tarjeta.
 */
export default function InductionVideoPicker({ videos, value, onChange, onHide }: Props) {
  const selected = videos.find((v) => v.id === value) ?? null
  const [query, setQuery] = useState('')
  const [hiding, setHiding] = useState(false)

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase()
    return q ? videos.filter((v) => v.title.toLowerCase().includes(q)) : videos
  }, [videos, query])

  if (videos.length === 0) {
    return (
      <div className={styles.videoEmpty}>
        <Play size={22} />
        <span>
          No hay novedades de tipo video todavía. Crea una desde la pestaña Novedades (puede quedar
          oculta) y vuelve aquí. Mientras tanto, el bloque puede ser solo cuestionario.
        </span>
      </div>
    )
  }

  const info = selected ? parseVideoUrl(selected.google_drive_url) : null

  const hide = async () => {
    if (!selected || !onHide) return
    setHiding(true)
    try {
      await onHide(selected.id)
    } finally {
      setHiding(false)
    }
  }

  return (
    <div className={styles.videoSplit}>
      <div className={styles.videoPicker}>
        {videos.length > 6 && (
          <div className={styles.videoSearch}>
            <Search size={15} />
            <input
              type="text"
              placeholder="Buscar video..."
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              aria-label="Buscar video"
            />
          </div>
        )}
        <div className={styles.videoGrid} role="listbox" aria-label="Videos de Novedades">
          <button
            type="button"
            role="option"
            aria-selected={value === 0}
            className={value === 0 ? styles.videoOptionActive : styles.videoOption}
            onClick={() => onChange(0)}
          >
            <div className={styles.videoThumb}>
              <Ban size={20} />
            </div>
            <span className={styles.videoTitle}>Sin video</span>
            <span className={styles.videoMeta}>Solo cuestionario</span>
          </button>
          {filtered.map((v) => (
            <button
              key={v.id}
              type="button"
              role="option"
              aria-selected={value === v.id}
              className={value === v.id ? styles.videoOptionActive : styles.videoOption}
              onClick={() => onChange(v.id)}
              title={v.title}
            >
              <Thumb video={v} />
              <span className={styles.videoTitle}>{v.title}</span>
              <span className={styles.videoMeta}>
                {v.is_active ? <Eye size={12} /> : <EyeOff size={12} />}
                {v.is_active ? 'Visible' : 'Oculta'}
                {v.duration_min > 0 && ` · ${v.duration_min} min`}
              </span>
            </button>
          ))}
        </div>
        {filtered.length === 0 && <p className={styles.hint}>Ningún video coincide con «{query}».</p>}
      </div>

      {/* --- El elegido --- */}
      <aside className={styles.videoAside} aria-label="Video elegido">
        <span className={styles.blockPreviewLabel}>Video elegido</span>
        {selected ? (
          <div className={styles.videoSelected}>
            <div className={styles.videoPreview}>
              {info ? (
                <iframe
                  key={selected.id}
                  src={info.embedUrl}
                  title={selected.title}
                  allow="autoplay; encrypted-media"
                  allowFullScreen
                />
              ) : (
                <div className={styles.videoThumb}>
                  <Play size={26} />
                </div>
              )}
            </div>
            <div className={styles.videoSelectedBody}>
              <div className={styles.videoTitle}>{selected.title}</div>
              <div className={styles.videoMeta}>
                {info && <span>{getProviderLabel(info.provider)}</span>}
                {selected.duration_min > 0 && <span>· {selected.duration_min} min</span>}
              </div>
              {selected.is_active ? (
                <div className={styles.videoNotice}>
                  <span>
                    <strong>Visible en Novedades:</strong> se anuncia a toda su audiencia. Para
                    inducción conviene ocultarlo; el bloque lo reproduce igual.
                  </span>
                  {onHide && (
                    <button type="button" className={styles.ghostBtnSm} disabled={hiding} onClick={hide}>
                      <EyeOff size={14} /> {hiding ? 'Ocultando...' : 'Ocultar de Novedades'}
                    </button>
                  )}
                </div>
              ) : (
                <span className={styles.videoOk}>
                  <EyeOff size={13} /> Oculto en Novedades: solo lo ve quien hace la inducción.
                </span>
              )}
            </div>
          </div>
        ) : (
          <div className={styles.videoNone}>
            <ListChecks size={26} />
            <strong>Sin video</strong>
            <span>El profesional pasa directo al cuestionario. Elige una tarjeta para agregar uno.</span>
          </div>
        )}
      </aside>
    </div>
  )
}
