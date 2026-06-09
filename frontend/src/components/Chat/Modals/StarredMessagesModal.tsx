import { Message } from '../../../types/chat'
import { User } from '../../../types'
import styles from '../../../pages/SlackChat.module.css'
import { Modal, Button } from '../../ui'

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
    <Modal
      isOpen
      onClose={onClose}
      title="⭐ Mensajes starred"
      size="md"
      footer={<Button variant="secondary" onClick={onClose}>Cerrar</Button>}
    >
      <div className={styles['starred-list']}>
        {starredMessages.length === 0 ? (
          <p className={styles['no-starred']}>No hay mensajes starred</p>
        ) : (
          starredMessages.map(msg => (
            <div key={msg.id} className={styles['starred-item']}>
              <div className={styles['starred-header']}>
                <span className={styles['starred-author']}>{msg.user?.name}</span>
                <span className={styles['starred-time']}>{formatTime(msg.created_at)}</span>
              </div>
              <p className={styles['starred-text']}>{msg.content}</p>
              {msg.user_id === currentUser?.id && (
                <button className={styles['unstar-btn']} onClick={() => onUnstar(msg.id)}>
                  Quitar star
                </button>
              )}
            </div>
          ))
        )}
      </div>
    </Modal>
  )
}
