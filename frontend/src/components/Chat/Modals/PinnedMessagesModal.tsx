import { Message } from '../../../types/chat'
import { User } from '../../../types'
import styles from '../../../pages/SlackChat.module.css'
import { Modal, Button } from '../../ui'

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
    <Modal
      isOpen
      onClose={onClose}
      title="📌 Mensajes fijados"
      size="md"
      footer={<Button variant="secondary" onClick={onClose}>Cerrar</Button>}
    >
      <div className={styles['pinned-list']}>
        {pinnedMessages.length === 0 ? (
          <p className={styles['no-pinned']}>No hay mensajes fijados</p>
        ) : (
          pinnedMessages.map(msg => (
            <div key={msg.id} className={styles['pinned-item']}>
              <div className={styles['pinned-header']}>
                <span className={styles['pinned-author']}>{msg.user?.name}</span>
                <span className={styles['pinned-time']}>{formatTime(msg.created_at)}</span>
              </div>
              <p className={styles['pinned-text']}>{msg.content}</p>
              {msg.user_id === currentUser?.id && (
                <button className={styles['unpin-btn']} onClick={() => onUnpin(msg.id)}>
                  Desfijar
                </button>
              )}
            </div>
          ))
        )}
      </div>
    </Modal>
  )
}
