import { useState } from 'react'
import { Message } from '../../types/chat'
import { User } from '../../types'
import { ReplyIcon, EditIcon, TrashIcon, PinIcon, SmileIcon, StarIcon } from './Icons'
import styles from '../../pages/SlackChat.module.css'
import { getUserColor, mentionsUser } from './ChatUtils'

const REACTION_EMOJIS = ['👍', '❤️', '😂', '🎉', '😮', '😢', '🙏', '🔥', '👀', '✅']

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
  onToggleReaction: (msg: Message, emoji: string) => void
  isHighlighted?: boolean
  isStarred: boolean
  onStar: (id: number) => void
  onUnstar: (id: number) => void

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
  onToggleReaction,
  isHighlighted,
  isStarred,
  onStar,
  onUnstar,

  playingAudio,
  togglePlayAudio,
  formatTime,
  highlightMentions
}: MessageItemProps) {
  const [showEmojiPicker, setShowEmojiPicker] = useState(false)

  const isVoiceNote = message.file_type?.startsWith('audio/') ||
                     message.file_name?.includes('voice') ||
                     message.attachment?.includes('.webm') ||
                     message.attachment?.includes('.mp3')

  // file_type isn't always persisted (e.g. GIFs from Giphy), so fall back to the URL extension.
  const isImageAttachment = !isVoiceNote && (
    message.file_type?.startsWith('image/') ||
    /\.(gif|png|jpe?g|webp|avif)([?#]|$)/i.test(message.attachment || '')
  )

  const isOwnMessage = message.user_id === currentUser?.id
  const mentionsMe = !isOwnMessage && !message.is_deleted && mentionsUser(message.content, currentUser?.name)

  // Group reactions by emoji: count, who reacted, and whether the current user did.
  const reactionGroups = Object.entries(
    (message.reactions || []).reduce<Record<string, { count: number; mine: boolean; names: string[] }>>((acc, r) => {
      const group = acc[r.emoji] || { count: 0, mine: false, names: [] }
      group.count++
      if (r.user_id === currentUser?.id) group.mine = true
      if (r.user?.name) group.names.push(r.user.name)
      acc[r.emoji] = group
      return acc
    }, {})
  )



  return (
    <div
      id={`message-${message.id}`}
      className={`${styles['message-item']} ${isOwnMessage ? styles['own-message'] : ''} ${message.is_pinned ? styles['pinned'] : ''} ${mentionsMe ? styles['mentions-me'] : ''} ${isHighlighted ? styles['highlighted'] : ''}`}
    >
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
              ) : isImageAttachment ? (
                <img src={message.attachment} alt="Adjunto" className={styles['attachment-image']} onClick={() => window.open(message.attachment, '_blank')} />
              ) : (
                <a href={message.attachment} target="_blank" rel="noopener noreferrer" className={styles['attachment-file'] || 'attachment-file'}>
                  📁 {message.file_name || 'Archivo'}
                </a>
              )}
            </div>
          )}
        </div>

        {reactionGroups.length > 0 && !message.is_deleted && (
          <div className={styles['reactions-display']}>
            {reactionGroups.map(([emoji, group]) => (
              <button
                key={emoji}
                className={`${styles['reaction-badge']} ${group.mine ? styles['mine'] : ''}`}
                title={group.names.join(', ')}
                onClick={() => onToggleReaction(message, emoji)}
              >
                {emoji}{group.count > 1 && <span className={styles['reaction-count']}>{group.count}</span>}
              </button>
            ))}
          </div>
        )}

        {message.is_pinned && <span className={styles['pinned-badge-bubble']}>📍 Fijado</span>}

        {message.reply_count && message.reply_count > 0 ? (
          <button className={styles['thread-replies-link']} onClick={() => onReply(message)}>
            💬 {message.reply_count === 1 ? '1 respuesta' : `${message.reply_count} respuestas`}
          </button>
        ) : null}




        {!message.is_deleted && (
          <div className={styles['message-actions']}>

            <div className={styles['reaction-picker-wrapper']}>
              <button className={styles['action-btn']} onClick={() => setShowEmojiPicker(v => !v)} title="Añadir reacción">
                <SmileIcon />
              </button>
              {showEmojiPicker && (
                <div className={styles['emoji-picker']}>
                  {REACTION_EMOJIS.map(emoji => (
                    <button key={emoji} onClick={() => { onToggleReaction(message, emoji); setShowEmojiPicker(false) }}>
                      {emoji}
                    </button>
                  ))}
                </div>
              )}
            </div>
            <button
              className={`${styles['action-btn']} ${isStarred ? styles['starred-active'] : ''}`}
              onClick={() => isStarred ? onUnstar(message.id) : onStar(message.id)}
              title={isStarred ? 'Quitar de destacados' : 'Destacar mensaje'}
            >
              <StarIcon />
            </button>
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
