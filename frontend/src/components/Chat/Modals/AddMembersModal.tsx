import { User } from '../../../types'
import styles from '../../../pages/SlackChat.module.css'

interface AddMembersModalProps {
  allUsers: User[]
  isMember: (userId: number) => boolean
  onAddMember: (userId: number) => void
  onClose: () => void
}

export function AddMembersModal({
  allUsers,
  isMember,
  onAddMember,
  onClose
}: AddMembersModalProps) {
  return (
    <div className={styles['modal-overlay']} onClick={onClose}>
      <div className={styles['modal-content']} onClick={e => e.stopPropagation()}>
        <div className={styles['modal-header']}>
          <h2>Añadir personas al canal</h2>
          <button className={styles['close-btn']} onClick={onClose}>×</button>
        </div>
        
        <p className={styles['hint']}>Haz clic en un usuario para añadirlo</p>
        <div className={styles['users-list']}>
          {allUsers
            .filter(u => !isMember(u.id))
            .map(u => (
              <div key={u.id} className={styles['user-item']} onClick={() => { onAddMember(u.id); }}>
                <div className={styles['user-avatar']}>{u.name?.charAt(0).toUpperCase()}</div>
                <div className={styles['user-info']}>
                  <span className={styles['user-name']}>{u.name}</span>
                  <span className={styles['user-email']}>{u.email}</span>
                </div>
              </div>
            ))}
        </div>
        <div className={styles['modal-actions']}>
          <button onClick={onClose}>Cerrar</button>
        </div>
      </div>
    </div>
  )
}
