import React from 'react'
import { Message } from '../../types/chat'
import { User } from '../../types'
import { ReplyIcon, EditIcon, TrashIcon, PinIcon } from './Icons'

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
              autoFocus
            />
            <div className="edit-actions">
              <button className="btn-cancel" onClick={onCancelEdit}>Cancelar</button>
              <button className="btn-save" onClick={onSaveEdit}>Guardar cambios</button>
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
          <div className="message-actions">

            <button className="action-btn" onClick={() => onReply(message)} title="Reply in thread">
              <ReplyIcon />
            </button>
            {isOwnMessage && (
              <>
                <button className="action-btn" onClick={() => onStartEdit(message)} title="Edit message">
                  <EditIcon />
                </button>
                <button className="action-btn delete" onClick={() => onDelete(message.id)} title="Delete message">
                  <TrashIcon />
                </button>
              </>
            )}
            <button className="action-btn" onClick={() => message.is_pinned ? onUnpin(message.id) : onPin(message.id)} title={message.is_pinned ? "Unpin message" : "Pin message"}>
              <PinIcon />
            </button>
          </div>
        )}
      </div>
    </div>
  )
}
