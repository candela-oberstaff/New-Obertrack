import { useState, memo } from 'react'
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

function MessageItemInner({
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

  // M-7: trust the real `file_type` persisted by the backend (Sprint 4 / M-8).
  // Voice notes are recorded and sent as 'audio/webm' (see useSlackChat.sendRecording),
  // so `file_type` starting with 'audio/' is the authoritative signal.
  // - Removed the `file_name?.includes('voice')` branch: voice notes are named
  //   'Nota de voz', so it never matched.
  // - Removed the `.mp3` extension branch: a legitimately-attached .mp3 is NOT a
  //   voice note and must not get the voice-note UI (it can still render as audio
  //   via file_type 'audio/...').
  // - Kept only a `.webm` extension fallback for legacy messages whose file_type
  //   wasn't persisted, since .webm is the actual recording format we produce.
  // Una nota de voz es un audio grabado en la app (file_name 'Nota de voz', enviado
  // como 'audio/webm'). Distinguimos las notas de voz (UI propia) de otros audios
  // subidos como archivo (mp3/wav/ogg/m4a/...), que ahora también se reproducen inline.
  const isAudioAttachment = message.file_type?.startsWith('audio/') ||
                     /\.(mp3|wav|ogg|m4a|webm|aac|flac)([?#]|$)/i.test(message.file_name || '') ||
                     /\.(mp3|wav|ogg|m4a|webm|aac|flac)([?#]|$)/i.test(message.attachment || '')
  // Solo las grabaciones de la app llevan la UI específica de "Nota de voz".
  const isVoiceNote = isAudioAttachment && message.file_name === 'Nota de voz'

  // file_type isn't always persisted (e.g. GIFs from Giphy), so fall back to the URL extension.
  const isImageAttachment = !isAudioAttachment && (
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
              ) : isAudioAttachment ? (
                <div className={styles['audio-attachment']} style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
                  <audio controls src={message.attachment} style={{ maxWidth: '100%' }} />
                  <a href={message.attachment} target="_blank" rel="noopener noreferrer" className={styles['attachment-file'] || 'attachment-file'} download>
                    📁 {message.file_name || 'Audio'}
                  </a>
                </div>
              ) : isImageAttachment ? (
                <img src={message.attachment} alt="Adjunto" className={styles['attachment-image']} onClick={() => window.open(message.attachment, '_blank', 'noopener,noreferrer')} />
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

// Memoized so a message doesn't re-render unless its own props change. This
// avoids the O(messages x users) cost of highlightMentions running for every
// message on every parent render; relies on the parent passing stable props
// (notably a useCallback'd highlightMentions).
export const MessageItem = memo(MessageItemInner)
