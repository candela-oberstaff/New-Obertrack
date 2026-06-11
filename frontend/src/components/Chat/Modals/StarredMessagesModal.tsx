import { Message } from '../../../types/chat'
import styles from '../../../pages/SlackChat.module.css'
import { Modal, Button } from '../../ui'

interface StarredMessagesModalProps {
  starredMessages: Message[]
  onUnstar: (id: number) => void
  onClose: () => void
  formatTime: (date: string) => string
}

export function StarredMessagesModal({
  starredMessages,
  onUnstar,
  onClose,
  formatTime
}: StarredMessagesModalProps) {
  return (
    <Modal
      isOpen
      onClose={onClose}
      title="⭐ Mensajes destacados"
      size="md"
      footer={<Button variant="secondary" onClick={onClose}>Cerrar</Button>}
    >
      <div className={styles['starred-list']}>
        {starredMessages.length === 0 ? (
          <p className={styles['no-starred']}>No hay mensajes destacados</p>
        ) : (
          starredMessages.map(msg => (
            <div key={msg.id} className={styles['starred-item']}>
              <div className={styles['starred-header']}>
                <span className={styles['starred-author']}>{msg.user?.name}</span>
                <span className={styles['starred-time']}>{formatTime(msg.created_at)}</span>
              </div>
              <p className={styles['starred-text']}>{msg.content}</p>
              <button className={styles['unstar-btn']} onClick={() => onUnstar(msg.id)}>
                Quitar de destacados
              </button>
            </div>
          ))
        )}
      </div>
    </Modal>
  )
}
