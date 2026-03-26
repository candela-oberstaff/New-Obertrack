import { Message } from '../../../types/chat'
import { User } from '../../../types'

interface PinnedMessagesModalProps {
  pinnedMessages: Message[]
  currentUser: User | null
  onUnpin: (id: number) => void
  onClose: () => void
  formatTime: (date: string) => string
}

export function PinnedMessagesModal({
  pinnedMessages,
  currentUser,
  onUnpin,
  onClose,
  formatTime
}: PinnedMessagesModalProps) {
  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal-content pinned" onClick={e => e.stopPropagation()}>
        <div className="modal-header">
          <h2>📌 Mensajes fijados</h2>
          <button className="close-btn" onClick={onClose}>×</button>
        </div>

        <div className="pinned-list">
          {pinnedMessages.length === 0 ? (
            <p className="no-pinned">No hay mensajes fijados</p>
          ) : (
            pinnedMessages.map(msg => (
              <div key={msg.id} className="pinned-item">
                <div className="pinned-header">
                  <span className="pinned-author">{msg.user?.name}</span>
                  <span className="pinned-time">{formatTime(msg.created_at)}</span>
                </div>
                <p className="pinned-text">{msg.content}</p>
                {msg.user_id === currentUser?.id && (
                  <button className="unpin-btn" onClick={() => onUnpin(msg.id)}>
                    Desfijar
                  </button>
                )}
              </div>
            ))
          )}
        </div>
        <div className="modal-actions">
          <button onClick={onClose}>Cerrar</button>
        </div>
      </div>
    </div>
  )
}
