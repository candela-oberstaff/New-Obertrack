import { useCallback, useEffect, useMemo, useState } from 'react'
import { ArrowLeft, Clapperboard, Layers, Pencil, Play, Plus, Search, Trash2 } from 'lucide-react'

import { useConfirm } from '../ui/ConfirmProvider'
import { useNotification } from '../../context/NotificationContext'
import { inductionService, type InductionVideo } from '../../services/induction.service'
import { videoThumbnailUrl } from '../Tutorials/utils'
import InductionVideoForm from './InductionVideoForm'
import styles from './InductionSettings.module.css'

interface Props {
  onBack: () => void
}

function Thumb({ url }: { url: string }) {
  const src = videoThumbnailUrl(url)
  const [failed, setFailed] = useState(false)
  return (
    <div className={styles.videoThumb} style={{ margin: 0, borderRadius: 0 }}>
      {src && !failed ? <img src={src} alt="" loading="lazy" onError={() => setFailed(true)} /> : <Play size={26} />}
    </div>
  )
}

/**
 * Biblioteca de videos de la inducción. Separada de Novedades: aquí viven los
 * videos que reproducen los bloques, sin anunciarse a nadie.
 */
export default function InductionVideoLibrary({ onBack }: Props) {
  const { success, error: showError } = useNotification()
  const confirm = useConfirm()

  const [videos, setVideos] = useState<InductionVideo[]>([])
  const [loading, setLoading] = useState(true)
  const [query, setQuery] = useState('')
  // null = galería; 'new' = agregando; video = editando.
  const [editing, setEditing] = useState<InductionVideo | 'new' | null>(null)

  const load = useCallback(async () => {
    try {
      setVideos(await inductionService.listVideos())
    } catch {
      setVideos([])
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void load()
  }, [load])

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase()
    return q ? videos.filter((v) => `${v.title} ${v.description}`.toLowerCase().includes(q)) : videos
  }, [videos, query])

  const handleDelete = async (v: InductionVideo) => {
    if (v.block_names.length > 0) {
      showError(`Este video está en uso en: ${v.block_names.join(', ')}. Cámbialo en esos bloques antes de borrarlo.`)
      return
    }
    const ok = await confirm({
      title: 'Borrar video',
      message: `Se borrará «${v.title}» de la biblioteca. El archivo en Drive o YouTube no se toca.`,
      confirmLabel: 'Borrar',
    })
    if (!ok) return
    try {
      await inductionService.deleteVideo(v.id)
      success('Video borrado.')
      await load()
    } catch (err: any) {
      showError(err?.response?.data?.error ?? 'No se pudo borrar el video.')
    }
  }

  return (
    <div className={styles.panel}>
      <div className={styles.editorHead}>
        <button type="button" className={styles.backBtn} onClick={editing ? () => setEditing(null) : onBack}>
          <ArrowLeft size={14} /> {editing ? 'Biblioteca' : 'Inducción'}
        </button>
        <h3 className={styles.keyTitle}>
          {editing === 'new' ? 'Nuevo video' : editing ? editing.title : 'Biblioteca de videos'}
        </h3>
      </div>

      {editing ? (
        <div className={styles.section}>
          <InductionVideoForm
            video={editing === 'new' ? null : editing}
            onSaved={async () => {
              setEditing(null)
              await load()
            }}
            onCancel={() => setEditing(null)}
          />
        </div>
      ) : (
        <>
          <div className={styles.toolbar}>
            <p className={styles.intro}>
              Los videos que reproducen los bloques de la inducción. Están separados de Novedades: no
              se anuncian a nadie y solo los ve quien recorre el bloque que los usa.
            </p>
            <button type="button" className={styles.saveBtn} onClick={() => setEditing('new')}>
              <Plus size={16} /> Nuevo video
            </button>
          </div>

          {videos.length > 6 && (
            <div className={styles.videoSearch} style={{ maxWidth: 420 }}>
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

          {loading ? (
            <p className={styles.muted}>Cargando videos...</p>
          ) : videos.length === 0 ? (
            <div className={styles.empty}>
              <Clapperboard size={26} style={{ color: '#94a3b8', marginBottom: 8 }} />
              <div>Todavía no hay videos. Agrega el primero para usarlo en un bloque.</div>
              <div style={{ marginTop: 12 }}>
                <button type="button" className={styles.ghostBtn} onClick={() => setEditing('new')}>
                  <Plus size={16} /> Agregar el primer video
                </button>
              </div>
            </div>
          ) : (
            <div className={styles.certGallery}>
              {filtered.map((v) => (
                <div key={v.id} className={styles.certTile}>
                  <button
                    type="button"
                    className={styles.libVideoThumb}
                    onClick={() => setEditing(v)}
                    aria-label={`Editar ${v.title}`}
                  >
                    <Thumb url={v.video_url} />
                    {v.duration_min > 0 && <span className={styles.libVideoDuration}>{v.duration_min} min</span>}
                  </button>
                  <div className={styles.certTileBody}>
                    <div className={styles.rowTitle}>{v.title}</div>
                    <div className={styles.rowMeta}>
                      {v.block_names.length > 0 ? (
                        <span className={styles.tagOk}>
                          <Layers size={12} /> En {v.block_names.join(', ')}
                        </span>
                      ) : (
                        <span>Sin usar todavía</span>
                      )}
                    </div>
                    <div className={styles.certTileActions}>
                      <button type="button" className={styles.ghostBtnSm} onClick={() => setEditing(v)}>
                        <Pencil size={14} /> Editar
                      </button>
                      <button
                        type="button"
                        className={styles.iconBtn}
                        title={v.block_names.length > 0 ? 'En uso: cámbialo en sus bloques para borrarlo' : 'Borrar'}
                        aria-label={`Borrar ${v.title}`}
                        disabled={v.block_names.length > 0}
                        onClick={() => handleDelete(v)}
                      >
                        <Trash2 size={16} />
                      </button>
                    </div>
                  </div>
                </div>
              ))}
              <button type="button" className={styles.certTileNew} onClick={() => setEditing('new')}>
                <span className={styles.dropIcon}>
                  <Plus size={22} />
                </span>
                <strong>Nuevo video</strong>
                <span>YouTube o Google Drive</span>
              </button>
            </div>
          )}
        </>
      )}
    </div>
  )
}
