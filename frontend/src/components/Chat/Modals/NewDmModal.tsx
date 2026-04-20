import { User } from '../../../types'
import styles from '../../../pages/SlackChat.module.css'
import { getUserColor } from '../ChatUtils'

interface NewDmModalProps {
  allUsers: User[]
  onSelectUser: (userId: number) => void
  onClose: () => void
  currentUser: User | null
}

export function NewDmModal({
  allUsers,
  onSelectUser,
  onClose,
  currentUser
}: NewDmModalProps) {
  // Filter out current user
  const usersToDisplay = allUsers.filter(u => u.id !== currentUser?.id)

  return (
    <div className={styles['modal-overlay']} onClick={onClose}>
      <div className={styles['modal-content']} onClick={e => e.stopPropagation()}>
        <div className={styles['modal-header']}>
          <h2>Nuevo mensaje directo</h2>
          <button className={styles['modal-close-btn']} onClick={onClose}>&times;</button>
        </div>
        
        <p className={styles['hint']}>Selecciona a alguien para empezar a chatear</p>
        <div className={styles['users-list']}>
          {usersToDisplay.length > 0 ? (
            usersToDisplay.map(u => (
              <div key={u.id} className={styles['user-item']} onClick={() => onSelectUser(u.id)}>
                <div 
                  className={styles['user-avatar']} 
                  style={{ background: getUserColor(u.name || '') }}
                >
                  {u.name?.charAt(0).toUpperCase()}
                </div>
                <div className={styles['user-info']}>
                  <span className={styles['user-name']}>{u.name}</span>
                  <span className={styles['user-email']}>{u.email}</span>
                </div>
                <div className={styles['user-status-indicator']}>
                   {/* Optional: Add status dot here if available */}
                </div>
              </div>
            ))
          ) : (
            <div className={styles['no-users']}>No se encontraron otros usuarios</div>
          )}
        </div>
        <div className={styles['modal-actions']}>
          <button onClick={onClose} className={styles['cancel-btn']}>Cancelar</button>
        </div>
      </div>
    </div>
  )
}
