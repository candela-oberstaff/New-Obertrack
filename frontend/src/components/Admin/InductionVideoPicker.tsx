import { useMemo, useState } from 'react'
import { Ban, ListChecks, Play, Plus, Search } from 'lucide-react'

import type { InductionVideo } from '../../services/induction.service'
import { getProviderLabel, parseVideoUrl, videoThumbnailUrl } from '../Tutorials/utils'
import InductionVideoForm from './InductionVideoForm'
import styles from './InductionSettings.module.css'

interface Props {
  /** Videos de la biblioteca de la inducción. */
  videos: InductionVideo[]
  /** 0 = sin video. */
  value: number
  onChange: (id: number) => void
  /** Un video recién agregado a la biblioteca desde aquí. */
  onCreated: (video: InductionVideo) => void
}

function Thumb({ video }: { video: InductionVideo }) {
  const src = videoThumbnailUrl(video.video_url)
  const [failed, setFailed] = useState(false)
  return (
    <div className={styles.videoThumb}>
      {src && !failed ? <img src={src} alt="" loading="lazy" onError={() => setFailed(true)} /> : <Play size={22} />}
    </div>
  )
}

/**
 * Elegir el video del bloque desde la biblioteca de la inducción: la lista con
 * miniaturas a la izquierda y el elegido con su reproductor a la derecha. Si
 * el video aún no está, se agrega aquí mismo sin salir del bloque.
 */
export default function InductionVideoPicker({ videos, value, onChange, onCreated }: Props) {
  const selected = videos.find((v) => v.id === value) ?? null
  const [query, setQuery] = useState('')
  const [adding, setAdding] = useState(false)

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase()
    return q ? videos.filter((v) => v.title.toLowerCase().includes(q)) : videos
  }, [videos, query])

  if (adding) {
    return (
      <div className={styles.pickPanel}>
        <span className={styles.blockPreviewLabel}>Nuevo video para la biblioteca</span>
        <div style={{ marginTop: 10 }}>
          <InductionVideoForm
            video={null}
            onSaved={(video) => {
              setAdding(false)
              onCreated(video)
              onChange(video.id)
            }}
            onCancel={() => setAdding(false)}
          />
        </div>
      </div>
    )
  }

  const info = selected ? parseVideoUrl(selected.video_url) : null

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
        <div className={styles.videoGrid} role="listbox" aria-label="Videos de la biblioteca">
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
                {v.duration_min > 0 ? `${v.duration_min} min` : 'Video'}
                {v.block_names.length > 0 && ` · en ${v.block_names.length} ${v.block_names.length === 1 ? 'bloque' : 'bloques'}`}
              </span>
            </button>
          ))}
          <button type="button" className={styles.videoOptionNew} onClick={() => setAdding(true)}>
            <div className={styles.videoThumb}>
              <Plus size={22} />
            </div>
            <span className={styles.videoTitle}>Nuevo video</span>
            <span className={styles.videoMeta}>Agregarlo a la biblioteca</span>
          </button>
        </div>
        {filtered.length === 0 && query && <p className={styles.hint}>Ningún video coincide con «{query}».</p>}
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
              {selected.description && <p className={styles.hint} style={{ margin: '4px 0 0' }}>{selected.description}</p>}
            </div>
          </div>
        ) : (
          <div className={styles.videoNone}>
            <ListChecks size={26} />
            <strong>Sin video</strong>
            <span>
              {videos.length === 0
                ? 'La biblioteca está vacía. Agrega el primer video con «Nuevo video».'
                : 'El profesional pasa directo al cuestionario. Elige una tarjeta para agregar uno.'}
            </span>
          </div>
        )}
      </aside>
    </div>
  )
}
