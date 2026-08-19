import { useEffect, useRef, useState } from 'react'
import { Paperclip, Send, Trash2, Pencil, FileText, Image as ImageIcon, Loader2, MessageSquare, Download, X } from 'lucide-react'
import { Button } from '../../components/ui'
import Avatar from '../../components/Common/Avatar'
import { useConfirm } from '../../components/ui/ConfirmProvider'
import { useNotification } from '../../context/NotificationContext'
import { useImagePaste } from '../../hooks/useImagePaste'
import { tenantAttachmentUrl, type CompanyAttachment, type CompanyComment, type EventThread as Thread } from '../../services/admin.service'
import styles from './Tenants.module.css'

const COMMENT_MAX_LENGTH = 4000

/** Extensiones que el backend acepta hoy (service/upload_service.go). */
const ACCEPTED_FILES = '.pdf,.doc,.docx,.xls,.xlsx,.jpg,.jpeg,.png,.gif,.webp,.mp3,.wav,.ogg'

function formatSize(bytes: number): string {
  if (!bytes) return ''
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${Math.round(bytes / 1024)} KB`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}

function isImage(mime: string): boolean {
  return (mime || '').toLowerCase().startsWith('image/')
}

function timeLabel(iso: string): string {
  const d = new Date(iso)
  if (isNaN(d.getTime())) return ''
  return d.toLocaleString('es-ES', { day: '2-digit', month: '2-digit', year: 'numeric', hour: '2-digit', minute: '2-digit' })
}

interface AttachmentChipProps {
  tenantId: number
  attachment: CompanyAttachment
  canEdit: boolean
  onDelete: (a: CompanyAttachment) => void
}

/**
 * Un adjunto ya guardado. Las imágenes se ven; el resto se identifica por
 * nombre.
 *
 * Enseñar la miniatura no es adorno: en soporte casi todos los adjuntos son
 * capturas, y tener que abrir cada una para saber cuál buscas convierte el
 * expediente en una lista de "image.png".
 */
function AttachmentChip({ tenantId, attachment, canEdit, onDelete }: AttachmentChipProps) {
  const href = tenantAttachmentUrl(tenantId, attachment.id)
  const img = isImage(attachment.mime_type)

  return (
    <div className={styles.attachment}>
      <a
        href={href}
        target="_blank"
        rel="noopener noreferrer"
        className={styles.attachmentLink}
        title={`${attachment.file_name}${attachment.file_size ? ` · ${formatSize(attachment.file_size)}` : ''}`}
      >
        {img ? (
          <img src={href} alt={attachment.file_name} className={styles.attachmentThumb} loading="lazy" />
        ) : (
          <span className={styles.attachmentIcon}><FileText size={18} /></span>
        )}
        <span className={styles.attachmentMeta}>
          <span className={styles.attachmentName}>{attachment.file_name}</span>
          <span className={styles.attachmentSize}>{formatSize(attachment.file_size)}</span>
        </span>
      </a>
      <div className={styles.attachmentActions}>
        <a href={href} download={attachment.file_name} className={styles.iconBtn} title="Descargar" aria-label={`Descargar ${attachment.file_name}`}>
          <Download size={14} />
        </a>
        {canEdit && (
          <button
            type="button"
            className={`${styles.iconBtn} ${styles.danger}`}
            onClick={() => onDelete(attachment)}
            title="Eliminar archivo"
            aria-label={`Eliminar ${attachment.file_name}`}
          >
            <Trash2 size={14} />
          </button>
        )}
      </div>
    </div>
  )
}

/**
 * Un archivo EN COLA: elegido pero todavía no subido, porque va a viajar con el
 * comentario que se está escribiendo. Se puede quitar antes de enviar.
 */
function PendingChip({ file, onRemove }: { file: File; onRemove: () => void }) {
  const img = isImage(file.type)
  // La vista previa sale de memoria para poder confirmar que se pegó la captura
  // correcta antes de publicar nada.
  //
  // La URL se crea UNA vez por archivo y se libera al desmontar: hacerlo en el
  // cuerpo del componente generaba una nueva en cada render y ninguna se
  // liberaba, así que el navegador se quedaba con todas las capturas en memoria
  // mientras la ficha estuviera abierta.
  const [preview, setPreview] = useState('')
  useEffect(() => {
    if (!img || typeof URL.createObjectURL !== 'function') return
    const url = URL.createObjectURL(file)
    setPreview(url)
    return () => URL.revokeObjectURL(url)
  }, [file, img])

  return (
    <div className={`${styles.attachment} ${styles.attachmentPending}`}>
      {img && preview ? (
        <img src={preview} alt={file.name} className={styles.attachmentThumb} />
      ) : (
        <span className={styles.attachmentIcon}><FileText size={18} /></span>
      )}
      <span className={styles.attachmentMeta}>
        <span className={styles.attachmentName} title={file.name}>{file.name}</span>
        <span className={styles.attachmentSize}>{formatSize(file.size)}</span>
      </span>
      <button
        type="button"
        className={styles.iconBtn}
        onClick={onRemove}
        title="Quitar"
        aria-label={`Quitar ${file.name} del comentario`}
      >
        <X size={14} />
      </button>
    </div>
  )
}

export interface EventThreadProps {
  tenantId: number
  eventId: number
  thread?: Thread
  canEdit: boolean
  addComment: (eventId: number, content: string) => Promise<number>
  updateComment: (commentId: number, content: string) => Promise<void>
  deleteComment: (commentId: number) => Promise<void>
  addAttachment: (eventId: number, file: File, commentId?: number) => Promise<void>
  deleteAttachment: (attachmentId: number) => Promise<void>
}

/**
 * El hilo de una entrada del expediente: sus archivos y su conversación.
 *
 * Los archivos que se eligen mientras se escribe NO se suben al momento: quedan
 * en cola y se publican con el comentario, porque un mensaje y su captura son
 * una sola cosa. Subirlos por separado dejaba el archivo suelto arriba y el
 * texto abajo, sin nada que los relacionara.
 *
 * Arranca plegado y solo enseña el resumen. El expediente es una cronología que
 * se recorre de un vistazo, y desplegar cada conversación por defecto lo
 * convertiría en un muro por el que hay que bajar para llegar a lo de ayer.
 */
export function EventThread(props: EventThreadProps) {
  const { tenantId, eventId, thread, canEdit } = props
  const confirm = useConfirm()
  const notify = useNotification()

  const comments = thread?.comments ?? []
  const attachments = thread?.attachments ?? []
  const total = comments.length + attachments.length

  const [open, setOpen] = useState(false)
  const [text, setText] = useState('')
  const [sending, setSending] = useState(false)
  const [pending, setPending] = useState<File[]>([])
  const [editingId, setEditingId] = useState<number | null>(null)
  const [editText, setEditText] = useState('')
  const fileRef = useRef<HTMLInputElement>(null)

  const stage = (files: File[] | FileList | null) => {
    if (!files) return
    const list = Array.from(files)
    if (list.length > 0) {
      setPending(p => [...p, ...list])
      setOpen(true)
    }
  }

  // Pegar o arrastrar encola la imagen igual que el botón de adjuntar: pegar una
  // captura y que se publique sola antes de escribir el mensaje sería justo el
  // problema que se está arreglando.
  const { onPaste, onDrop } = useImagePaste(files => { stage(files) })

  const removePending = (i: number) => setPending(p => p.filter((_, idx) => idx !== i))

  const submit = async () => {
    const content = text.trim()
    if (!content && pending.length === 0) return
    setSending(true)
    try {
      if (content) {
        // El comentario primero: los archivos necesitan un id al que colgarse.
        const commentId = await props.addComment(eventId, content)
        for (const f of pending) {
          await props.addAttachment(eventId, f, commentId)
        }
      } else {
        // Solo archivos: se cuelgan de la entrada. Es el caso de "dejo aquí el
        // contrato" sin nada que añadir.
        for (const f of pending) {
          await props.addAttachment(eventId, f)
        }
      }
      setText('')
      setPending([])
      setOpen(true)
    } catch (err: any) {
      // La cola NO se vacía: si falló una subida, los archivos siguen ahí para
      // reintentarlo sin tener que volver a buscarlos.
      notify.error(err?.response?.data?.error || 'No se pudo publicar. Los archivos siguen preparados.')
    } finally {
      setSending(false)
    }
  }

  const saveEdit = async () => {
    if (editingId === null) return
    const content = editText.trim()
    if (!content) return
    try {
      await props.updateComment(editingId, content)
      setEditingId(null)
      setEditText('')
    } catch (err: any) {
      notify.error(err?.response?.data?.error || 'No se pudo guardar el comentario.')
    }
  }

  const removeComment = async (c: CompanyComment) => {
    const n = c.attachments?.length ?? 0
    const ok = await confirm({
      title: '¿Eliminar este comentario?',
      message: n > 0
        ? `Se eliminará también ${n === 1 ? 'el archivo adjunto' : `los ${n} archivos adjuntos`}.`
        : 'Desaparecerá del expediente.',
      confirmLabel: 'Eliminar',
      variant: 'danger',
    })
    if (ok) await props.deleteComment(c.id)
  }

  const removeAttachment = async (a: CompanyAttachment) => {
    const ok = await confirm({
      title: `¿Eliminar ${a.file_name}?`,
      message: 'Se quitará del expediente.',
      confirmLabel: 'Eliminar archivo',
      variant: 'danger',
    })
    if (ok) await props.deleteAttachment(a.id)
  }

  const canSubmit = !!text.trim() || pending.length > 0

  return (
    <div className={styles.thread}>
      {(total > 0 || canEdit) && (
        <button type="button" className={styles.threadToggle} onClick={() => setOpen(o => !o)} aria-expanded={open}>
          <MessageSquare size={13} />
          {total === 0
            ? 'Comentar o adjuntar'
            : [
                comments.length > 0 ? `${comments.length} ${comments.length === 1 ? 'comentario' : 'comentarios'}` : '',
                attachments.length > 0 ? `${attachments.length} ${attachments.length === 1 ? 'archivo' : 'archivos'}` : '',
              ].filter(Boolean).join(' · ')}
        </button>
      )}

      {open && (
        <div className={styles.threadBody}>
          {/* Archivos de la nota, no de un comentario. Van etiquetados para que
              se entienda por qué están sueltos y no dentro de un mensaje. */}
          {attachments.length > 0 && (
            <div className={styles.looseFiles}>
              <span className={styles.looseFilesLabel}>Archivos de esta entrada</span>
              <div className={styles.attachmentGrid}>
                {attachments.map(a => (
                  <AttachmentChip key={`att-${a.id}`} tenantId={tenantId} attachment={a} canEdit={canEdit} onDelete={removeAttachment} />
                ))}
              </div>
            </div>
          )}

          {comments.map(c => (
            <div key={`c-${c.id}`} className={styles.comment}>
              <Avatar name={c.author || 'Sistema'} size="sm" />
              <div className={styles.commentMain}>
                <div className={styles.commentHead}>
                  <strong>{c.author || 'Sistema'}</strong>
                  <span>{timeLabel(c.created_at)}</span>
                  {c.edited_at && <span className={styles.noteEdited}>editado</span>}
                  {canEdit && editingId !== c.id && (
                    <div className={styles.commentActions}>
                      <button
                        type="button"
                        className={styles.iconBtn}
                        onClick={() => { setEditingId(c.id); setEditText(c.content) }}
                        title="Editar comentario"
                        aria-label={`Editar el comentario de ${c.author || 'Sistema'}`}
                      >
                        <Pencil size={13} />
                      </button>
                      <button
                        type="button"
                        className={`${styles.iconBtn} ${styles.danger}`}
                        onClick={() => removeComment(c)}
                        title="Eliminar comentario"
                        aria-label={`Eliminar el comentario de ${c.author || 'Sistema'}`}
                      >
                        <Trash2 size={13} />
                      </button>
                    </div>
                  )}
                </div>

                {editingId === c.id ? (
                  <div className={styles.commentEdit}>
                    <textarea
                      value={editText}
                      onChange={e => setEditText(e.target.value.slice(0, COMMENT_MAX_LENGTH))}
                      rows={3}
                      autoFocus
                    />
                    <div className={styles.commentEditActions}>
                      <Button size="sm" variant="secondary" onClick={() => { setEditingId(null); setEditText('') }}>Cancelar</Button>
                      <Button size="sm" onClick={saveEdit} disabled={!editText.trim()}>Guardar</Button>
                    </div>
                  </div>
                ) : (
                  <p className={styles.commentBody}>{c.content}</p>
                )}

                {(c.attachments?.length ?? 0) > 0 && (
                  <div className={styles.attachmentGrid}>
                    {c.attachments!.map(a => (
                      <AttachmentChip key={`catt-${a.id}`} tenantId={tenantId} attachment={a} canEdit={canEdit} onDelete={removeAttachment} />
                    ))}
                  </div>
                )}
              </div>
            </div>
          ))}

          {canEdit && (
            <div className={styles.commentComposer} onDrop={onDrop} onDragOver={e => e.preventDefault()}>
              <textarea
                value={text}
                onChange={e => setText(e.target.value.slice(0, COMMENT_MAX_LENGTH))}
                onPaste={onPaste}
                placeholder="Escribe un comentario, o pega una captura directamente aquí…"
                rows={2}
                aria-label="Nuevo comentario"
              />

              {pending.length > 0 && (
                <div className={styles.pendingBox}>
                  <span className={styles.pendingLabel}>
                    {pending.length === 1 ? 'Se enviará con tu comentario' : `${pending.length} archivos se enviarán con tu comentario`}
                  </span>
                  <div className={styles.attachmentGrid}>
                    {pending.map((f, i) => (
                      <PendingChip key={`p-${i}-${f.name}`} file={f} onRemove={() => removePending(i)} />
                    ))}
                  </div>
                </div>
              )}

              <div className={styles.composerActions}>
                <input
                  ref={fileRef}
                  type="file"
                  multiple
                  accept={ACCEPTED_FILES}
                  style={{ display: 'none' }}
                  onChange={e => { stage(e.target.files); e.target.value = '' }}
                />
                <button
                  type="button"
                  className={styles.composerAttach}
                  onClick={() => fileRef.current?.click()}
                  disabled={sending}
                  title="Adjuntar archivo al comentario"
                >
                  <Paperclip size={15} /> Adjuntar
                </button>
                <span className={styles.composerHint}>
                  <ImageIcon size={12} /> pega o arrastra imágenes
                </span>
                <Button
                  size="sm"
                  onClick={submit}
                  loading={sending}
                  disabled={!canSubmit}
                  leftIcon={sending ? <Loader2 size={13} className={styles.spin} /> : <Send size={13} />}
                >
                  {/* La etiqueta dice qué va a pasar: sin texto los archivos se
                      cuelgan de la entrada, no de un mensaje que no existe. */}
                  {!text.trim() && pending.length > 0 ? 'Adjuntar a la entrada' : 'Comentar'}
                </Button>
              </div>
            </div>
          )}

          {!canEdit && total === 0 && (
            <p className={styles.threadEmpty}>Sin comentarios ni archivos.</p>
          )}
        </div>
      )}
    </div>
  )
}
