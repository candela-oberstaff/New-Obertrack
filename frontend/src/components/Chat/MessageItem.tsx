import React from 'react'
import { Message, MessageReaction } from '../../types/chat'
import { User } from '../../types'

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
  onReactionAdd: (id: number, emoji: string) => void
  onReactionRemove: (id: number, emoji: string) => void
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
  onReactionAdd,
  onReactionRemove,
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

  const renderReactions = (reactions: MessageReaction[]) => {
    const grouped = (reactions || []).reduce((acc, r) => {
      acc[r.emoji] = (acc[r.emoji] || 0) + 1
      return acc
    }, {} as Record<string, number>)

    return Object.entries(grouped).map(([emoji, count]) => {
      const hasReacted = (reactions || []).some(r => r.user_id === currentUser?.id && r.emoji === emoji)
      return (
        <span 
          key={emoji} 
          className={`reaction-badge ${hasReacted ? 'active' : ''}`}
          onClick={() => hasReacted ? onReactionRemove(message.id, emoji) : onReactionAdd(message.id, emoji)}
        >
          {emoji} {count}
        </span>
      )
    })
  }

  return (
    <div className={`message-item ${isOwnMessage ? 'own-message' : ''} ${message.is_pinned ? 'pinned' : ''}`}>
      <div className="message-avatar">
        {message.user?.name?.charAt(0).toUpperCase() || '?'}
      </div>
      <div className="message-content-wrapper">
        <div className="message-header">
          <span className="user-name">{message.user?.name || 'Usuario'}</span>
          <span className="message-time">{formatTime(message.created_at)}</span>
          {message.is_pinned && <span className="pin-indicator">📍 Fijado</span>}
        </div>

        {editingMessageId === message.id ? (
          <div className="message-edit-box">
            <textarea 
              value={editContent} 
              onChange={(e) => setEditContent(e.target.value)}
              rows={3}
            />
            <div className="edit-actions">
              <button onClick={onCancelEdit}>Cancelar</button>
              <button className="btn-save" onClick={onSaveEdit}>Guardar</button>
            </div>
          </div>
        ) : (
          <div className="message-text">
            {message.is_deleted ? (
              <span className="deleted-text">[Mensaje eliminado]</span>
            ) : (
              highlightMentions(message.content)
            )}
            {message.is_edited && !message.is_deleted && <span className="edited-tag">(editado)</span>}
          </div>
        )}

        {message.attachment && !message.is_deleted && (
          <div className="message-attachment">
            {isVoiceNote ? (
              <div className="voice-note" onClick={() => togglePlayAudio(message.attachment!)}>
                <span className="play-icon">{playingAudio === message.attachment ? '⏸' : '▶'}</span>
                <span className="voice-label">Nota de voz</span>
              </div>
            ) : message.file_type?.startsWith('image/') ? (
              <img src={message.attachment} alt="Adjunto" className="attachment-image" onClick={() => window.open(message.attachment, '_blank')} />
            ) : (
              <a href={message.attachment} target="_blank" rel="noopener noreferrer" className="attachment-file">
                📁 {message.file_name || 'Archivo'}
              </a>
            )}
          </div>
        )}

        {!message.is_deleted && (
          <div className="message-reactions">
            {renderReactions(message.reactions || [])}
            <button className="add-reaction-btn" onClick={() => onReactionAdd(message.id, '👍')}>+</button>
          </div>
        )}

        <div className="message-actions">
          <button onClick={() => onReply(message)}>Reply</button>
          {!message.is_deleted && isOwnMessage && (
            <>
              <button onClick={() => onStartEdit(message)}>Edit</button>
              <button onClick={() => onDelete(message.id)}>Delete</button>
            </>
          )}
          {!message.is_deleted && (
            <button onClick={() => message.is_pinned ? onUnpin(message.id) : onPin(message.id)}>
              {message.is_pinned ? 'Unpin' : 'Pin'}
            </button>
          )}
        </div>
      </div>
    </div>
  )
}
