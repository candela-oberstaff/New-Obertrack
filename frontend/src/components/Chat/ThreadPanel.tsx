import React from 'react'
import { Message } from '../../types/chat'

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
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal-content thread" onClick={e => e.stopPropagation()}>
        <div className="modal-header">
          <h2>💬 Hilo</h2>
          <button className="close-btn" onClick={onClose}>×</button>
        </div>
        
        <div className="thread-parent">
          <div className="message-header">
            <span className="message-author">{showThread.user?.name}</span>
            <span className="message-time">{formatTime(showThread.created_at)}</span>
          </div>
          <p className="message-text">{showThread.content}</p>
        </div>

        <div className="thread-replies">
          {threadReplies.length === 0 ? (
            <p className="no-replies">No hay respuestas aún</p>
          ) : (
            threadReplies.map(reply => (
              <div key={reply.id} className="thread-reply">
                <div className="message-header">
                  <span className="message-author">{reply.user?.name}</span>
                  <span className="message-time">{formatTime(reply.created_at)}</span>
                </div>
                <p className="message-text">{reply.content}</p>
              </div>
            ))
          )}
        </div>

        <form className="thread-input" onSubmit={handleSubmit}>
          <input type="text" placeholder="Responder al hilo..." autoComplete="off" />
          <button type="submit">Enviar</button>
        </form>
        
        <div className="modal-actions">
          <button onClick={onClose}>Cerrar</button>
        </div>
      </div>
    </div>
  )
}
