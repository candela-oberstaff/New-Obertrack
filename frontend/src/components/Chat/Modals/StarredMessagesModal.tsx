import { Message } from '../../../types/chat'
import { User } from '../../../types'

interface StarredMessagesModalProps {
  starredMessages: Message[]
  currentUser: User | null
  onUnstar: (id: number) => void
  onClose: () => void
  formatTime: (date: string) => string
}

export function StarredMessagesModal({
  starredMessages,
  currentUser,
  onUnstar,
  onClose,
  formatTime
}: StarredMessagesModalProps) {
  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal-content starred" onClick={e => e.stopPropagation()}>
        <div className="modal-header">
          <h2>⭐ Mensajes starred</h2>
          <button className="close-btn" onClick={onClose}>×</button>
        </div>

        <div className="starred-list">
          {starredMessages.length === 0 ? (
            <p className="no-starred">No hay mensajes starred</p>
          ) : (
            starredMessages.map(msg => (
              <div key={msg.id} className="starred-item">
                <div className="starred-header">
                  <span className="starred-author">{msg.user?.name}</span>
                  <span className="starred-time">{formatTime(msg.created_at)}</span>
                </div>
                <p className="starred-text">{msg.content}</p>
                {msg.user_id === currentUser?.id && (
                  <button className="unstar-btn" onClick={() => onUnstar(msg.id)}>
                    Quitar star
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
