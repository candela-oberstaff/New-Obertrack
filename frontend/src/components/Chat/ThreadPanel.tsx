import React from 'react'
import { Message } from '../../types/chat'
import styles from '../../pages/SlackChat.module.css'

interface ThreadPanelProps {
  showThread: Message | null
  threadReplies: Message[]
  onClose: () => void
  onSendReply: (content: string) => void
  formatTime: (date: string) => string
}

export function ThreadPanel({
  showThread,
  threadReplies,
  onClose,
  onSendReply,
  formatTime
}: ThreadPanelProps) {
  if (!showThread) return null

  const handleSubmit = (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault()
    const form = e.currentTarget
    const input = form.querySelector('input') as HTMLInputElement
    const content = input.value.trim()
    if (content) {
      onSendReply(content)
      input.value = ''
    }
  }

  return (
    <div className={styles['modal-overlay']} onClick={onClose}>
      <div className={`${styles['modal-content']} ${styles['thread']}`} onClick={e => e.stopPropagation()}>
        <div className={styles['modal-header'] || 'modal-header'}>
          <h2>💬 Hilo</h2>
          <button className={styles['close-btn'] || 'close-btn'} onClick={onClose}>×</button>
        </div>
        
        <div className={styles['thread-parent']}>
          <div className={styles['message-header']}>
            <span className={styles['message-author']}>{showThread.user?.name}</span>
            <span className={styles['message-time']}>{formatTime(showThread.created_at)}</span>
          </div>
          <p className={styles['message-text']}>{showThread.content}</p>
        </div>

        <div className={styles['thread-replies']}>
          {threadReplies.length === 0 ? (
            <p className={styles['no-replies'] || 'no-replies'}>No hay respuestas aún</p>
          ) : (
            threadReplies.map(reply => (
              <div key={reply.tempId ?? reply.id} className={styles['thread-reply']}>
                <div className={styles['message-header']}>
                  <span className={styles['message-author']}>{reply.user?.name}</span>
                  <span className={styles['message-time']}>{formatTime(reply.created_at)}</span>
                </div>
                <p className={styles['message-text']}>{reply.content}</p>
              </div>
            ))
          )}
        </div>

        <form className={styles['thread-input']} onSubmit={handleSubmit}>
          <input type="text" placeholder="Responder al hilo..." autoComplete="off" />
          <button type="submit">Enviar</button>
        </form>
        
        <div className={styles['modal-actions']}>
          <button onClick={onClose}>Cerrar</button>
        </div>
      </div>
    </div>
  )
}
