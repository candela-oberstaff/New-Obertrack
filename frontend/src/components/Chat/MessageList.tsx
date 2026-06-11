import { useRef } from 'react'
import { Message } from '../../types/chat'
import { User } from '../../types'
import { MessageItem } from './MessageItem'
import styles from '../../pages/SlackChat.module.css'

interface MessageListProps {
  messages: Message[]
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
  starredIds: Set<number>
  onStar: (id: number) => void
  onUnstar: (id: number) => void

  playingAudio: string | null
  togglePlayAudio: (url: string) => void
  formatTime: (date: string) => string
  highlightMentions: (content: string) => React.ReactNode
  messagesEndRef: React.RefObject<HTMLDivElement | null>
  typingArray: string[]
  highlightedMessageId: number | null
  hasMoreMessages: boolean
  loadingOlder: boolean
  onLoadOlder: () => Promise<void>
}

export function MessageList({
  messages,
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
  starredIds,
  onStar,
  onUnstar,

  playingAudio,
  togglePlayAudio,
  formatTime,
  highlightMentions,
  messagesEndRef,
  typingArray,
  highlightedMessageId,
  hasMoreMessages,
  loadingOlder,
  onLoadOlder
}: MessageListProps) {
  const listRef = useRef<HTMLDivElement>(null)

  // Keep the viewport anchored on the same message after older history is prepended.
  const handleLoadOlder = async () => {
    const el = listRef.current
    const prevHeight = el?.scrollHeight ?? 0
    const prevTop = el?.scrollTop ?? 0
    await onLoadOlder()
    requestAnimationFrame(() => {
      if (el) el.scrollTop = el.scrollHeight - prevHeight + prevTop
    })
  }

  return (
    <div className={styles['messages-list']} ref={listRef}>
      {hasMoreMessages && messages.length > 0 && (
        <button className={styles['load-older-btn']} onClick={handleLoadOlder} disabled={loadingOlder}>
          {loadingOlder ? 'Cargando...' : '↑ Cargar mensajes anteriores'}
        </button>
      )}
      {messages.length === 0 ? (
        <div className={styles['no-messages'] || 'no-messages'}>
          <div className={styles['no-messages-icon'] || 'no-messages-icon'}>💬</div>
          <h3>No hay mensajes aún</h3>
          <p>¡Sé el primero en escribir!</p>
        </div>
      ) : (
        messages.map((msg) => (
          <MessageItem
            key={msg.id || msg.tempId}
            message={msg}
            currentUser={currentUser}
            editingMessageId={editingMessageId}
            editContent={editContent}
            setEditContent={setEditContent}
            onSaveEdit={onSaveEdit}
            onCancelEdit={onCancelEdit}
            onStartEdit={onStartEdit}
            onDelete={onDelete}
            onPin={onPin}
            onUnpin={onUnpin}
            onReply={onReply}
            onToggleReaction={onToggleReaction}
            isHighlighted={msg.id === highlightedMessageId}
            isStarred={starredIds.has(msg.id)}
            onStar={onStar}
            onUnstar={onUnstar}

            playingAudio={playingAudio}
            togglePlayAudio={togglePlayAudio}
            formatTime={formatTime}
            highlightMentions={highlightMentions}
          />
        ))
      )}
      
      {typingArray.length > 0 && (
        <div className={styles['typing-indicator']}>
          {typingArray.join(', ')} {typingArray.length === 1 ? 'está escribiendo...' : 'están escribiendo...'}
        </div>
      )}
      
      <div ref={messagesEndRef} />
    </div>
  )
}
