import { useMemo, useState } from 'react'
import { Ban, Eye, EyeOff, Play, RefreshCw, Search, X } from 'lucide-react'

import type { Tutorial } from '../../types/tutorials'
import { getProviderLabel, parseVideoUrl } from '../Tutorials/utils'
import styles from './InductionSettings.module.css'

interface Props {
  /** Novedades de tipo video. */
  videos: Tutorial[]
  /** 0 = sin video. */
  value: number
  onChange: (id: number) => void
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

function VisibilityTag({ video }: { video: Tutorial }) {
  return video.is_active ? (
    <span className={styles.tagWarn} title="Se anuncia a toda su audiencia en Novedades">
      <Eye size={12} /> Visible en Novedades
    </span>
  ) : (
    <span className={styles.tag}>
      <EyeOff size={12} /> Oculta
    </span>
  )
}

/**
 * Elegir el video del bloque viéndolo: tarjetas con miniatura y, una vez
 * elegido, el reproductor para confirmar que es el correcto.
 */
export default function InductionVideoPicker({ videos, value, onChange }: Props) {
  const selected = videos.find((v) => v.id === value) ?? null
  const [picking, setPicking] = useState(!selected)
  const [query, setQuery] = useState('')

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase()
    return q ? videos.filter((v) => v.title.toLowerCase().includes(q)) : videos
  }, [videos, query])

  const choose = (id: number) => {
    onChange(id)
    setPicking(id === 0)
    setQuery('')
  }

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

  if (selected && !picking) {
    const info = parseVideoUrl(selected.google_drive_url)
    return (
      <div className={styles.videoSelected}>
        <div className={styles.videoPreview}>
          {info ? (
            <iframe src={info.embedUrl} title={selected.title} allow="autoplay; encrypted-media" allowFullScreen />
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
            <VisibilityTag video={selected} />
          </div>
          {selected.is_active && (
            <p className={styles.hint} style={{ margin: '6px 0 0' }}>
              Al estar visible se anuncia a toda su audiencia. Para inducción conviene ocultarla
              desde Novedades: el bloque la reproduce igual.
            </p>
          )}
          <div className={styles.videoActions}>
            <button type="button" className={styles.ghostBtnSm} onClick={() => setPicking(true)}>
              <RefreshCw size={14} /> Cambiar video
            </button>
            <button type="button" className={styles.ghostBtnSm} onClick={() => choose(0)}>
              <X size={14} /> Quitar
            </button>
          </div>
        </div>
      </div>
    )
  }

  return (
    <div className={styles.videoPicker}>
      {videos.length > 4 && (
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
          onClick={() => choose(0)}
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
            onClick={() => choose(v.id)}
            title={v.title}
          >
            <Thumb video={v} />
            <span className={styles.videoTitle}>{v.title}</span>
            <span className={styles.videoMeta}>
              {v.is_active ? (
                <>
                  <Eye size={12} /> Visible
                </>
              ) : (
                <>
                  <EyeOff size={12} /> Oculta
                </>
              )}
              {v.duration_min > 0 && ` · ${v.duration_min} min`}
            </span>
          </button>
        ))}
      </div>
      {filtered.length === 0 && <p className={styles.hint}>Ningún video coincide con «{query}».</p>}
      {selected && (
        <button type="button" className={styles.linkBtn} style={{ marginTop: 10 }} onClick={() => setPicking(false)}>
          Volver al video elegido
        </button>
      )}
    </div>
  )
}
