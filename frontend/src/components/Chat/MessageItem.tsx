import { Message } from '../../types/chat'
import { User } from '../../types'
import { ReplyIcon, EditIcon, TrashIcon, PinIcon } from './Icons'
import styles from '../../pages/SlackChat.module.css'
import { getUserColor } from './ChatUtils'

interface MessageItemProps {
  message: Message
  currentUser: User | null
  editingMessageId: number | null
  editContent: string
  setEditContent: (content: string) => void
  onSaveEdit: () => void
  onCancelEdit: () => void
  onStartEdit: (msg: Message) => void
  onDelete: (id: number) => void
  onPin: (id: number) => void
  onUnpin: (id: number) => void
  onReply: (msg: Message) => void

  playingAudio: string | null
  togglePlayAudio: (url: string) => void
  formatTime: (date: string) => string
  highlightMentions: (content: string) => React.ReactNode
}

export function MessageItem({
  message,
  currentUser,
  editingMessageId,
  editContent,
  setEditContent,
  onSaveEdit,
  onCancelEdit,
  onStartEdit,
  onDelete,
  onPin,
  onUnpin,
  onReply,

  playingAudio,
  togglePlayAudio,
  formatTime,
  highlightMentions
}: MessageItemProps) {
  const isVoiceNote = message.file_type?.startsWith('audio/') || 
                     message.file_name?.includes('voice') || 
                     message.attachment?.includes('.webm') || 
                     message.attachment?.includes('.mp3')

  const isOwnMessage = message.user_id === currentUser?.id



  return (
    <div className={`${styles['message-item']} ${isOwnMessage ? styles['own-message'] : ''} ${message.is_pinned ? styles['pinned'] : ''}`}>
      <div 
        className={styles['message-avatar']} 
        style={{ backgroundColor: getUserColor(message.user?.name || '') }}
      >
        {message.user?.name?.charAt(0).toUpperCase() || '?'}
      </div>
      <div className={styles['message-main'] || 'message-content-wrapper'}>
        {!isOwnMessage && (
          <div className={styles['message-header']}>
            <span className={styles['sender-name'] || 'user-name'}>{message.user?.name || 'Usuario'}</span>
            <span className={styles['message-time']}>{formatTime(message.created_at)}</span>
          </div>
        )}

        <div className={styles['message-bubble']}>
          {isOwnMessage && (
            <div className={styles['message-header-own']}>
              <span className={styles['message-time']}>{formatTime(message.created_at)}</span>
            </div>
          )}

          {editingMessageId === message.id ? (
            <div className={styles['message-edit-box'] || 'message-edit-box'}>
              <textarea 
                value={editContent} 
                onChange={(e) => setEditContent(e.target.value)}
                rows={3}
                autoFocus
              />
              <div className={styles['edit-actions'] || 'edit-actions'}>
                <button className={styles['btn-cancel'] || 'btn-cancel'} onClick={onCancelEdit}>Cancelar</button>
                <button className={styles['btn-save'] || 'btn-save'} onClick={onSaveEdit}>Guardar</button>
              </div>
            </div>
          ) : (
            <div className={styles['message-text'] || 'message-body'}>
              {message.is_deleted ? (
                <span className={styles['deleted-text'] || 'deleted-text'}>[Mensaje eliminado]</span>
              ) : (
                highlightMentions(message.content)
              )}
              {message.is_edited && !message.is_deleted && <span className={styles['edited-tag'] || 'edited-tag'}>(editado)</span>}
            </div>
          )}
          
          {message.attachment && !message.is_deleted && (
            <div className={styles['message-attachment-bubble']}>
              {isVoiceNote ? (
                <div className={styles['voice-note']} onClick={() => togglePlayAudio(message.attachment!)}>
                  <span className={styles['play-btn'] || 'play-icon'}>{playingAudio === message.attachment ? '⏸' : '▶'}</span>
                  <span className={styles['voice-label'] || 'voice-label'}>Nota de voz</span>
                </div>
              ) : message.file_type?.startsWith('image/') ? (
                <img src={message.attachment} alt="Adjunto" className={styles['attachment-image']} onClick={() => window.open(message.attachment, '_blank')} />
              ) : (
                <a href={message.attachment} target="_blank" rel="noopener noreferrer" className={styles['attachment-file'] || 'attachment-file'}>
                  📁 {message.file_name || 'Archivo'}
                </a>
              )}
            </div>
          )}
        </div>

        {message.is_pinned && <span className={styles['pinned-badge-bubble']}>📍 Fijado</span>}

        {message.reply_count && message.reply_count > 0 ? (
          <button className={styles['thread-replies-link']} onClick={() => onReply(message)}>
            💬 {message.reply_count === 1 ? '1 respuesta' : `${message.reply_count} respuestas`}
          </button>
        ) : null}




        {!message.is_deleted && (
          <div className={styles['message-actions']}>

            <button className={styles['action-btn']} onClick={() => onReply(message)} title="Reply in thread">
              <ReplyIcon />
            </button>
            {isOwnMessage && (
              <>
                <button className={styles['action-btn']} onClick={() => onStartEdit(message)} title="Edit message">
                  <EditIcon />
                </button>
                <button className={`${styles['action-btn']} ${styles['delete']}`} onClick={() => onDelete(message.id)} title="Delete message">
                  <TrashIcon />
                </button>
              </>
            )}
            <button className={styles['action-btn']} onClick={() => message.is_pinned ? onUnpin(message.id) : onPin(message.id)} title={message.is_pinned ? "Unpin message" : "Pin message"}>
              <PinIcon />
            </button>
          </div>
        )}
      </div>
    </div>
  )
}
