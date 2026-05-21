import { User } from '../../../types'
import styles from '../../../pages/SlackChat.module.css'
import { getUserColor } from '../ChatUtils'

interface AddMembersModalProps {
  allUsers: User[]
  isMember: (userId: number) => boolean
  onAddMember: (userId: number) => void
  onClose: () => void
  currentUser: User | null
}

export function AddMembersModal({
  allUsers,
  isMember,
  onAddMember,
  onClose,
  currentUser
}: AddMembersModalProps) {
  return (
    <div className={styles['modal-overlay']} onClick={onClose}>
      <div className={styles['modal-content']} onClick={e => e.stopPropagation()}>
        <div className={styles['modal-header']}>
          <h2>Añadir personas al canal</h2>
        </div>
        
        <p className={styles['hint']}>Haz clic en un usuario para añadirlo</p>
        <div className={styles['users-list']}>
          {allUsers
            .filter(u => {
              // Standard member filter
              if (isMember(u.id)) return false
              
              // Restriction: Professionals cannot add Superadmins
              if (currentUser?.user_type === 'profesional' && (u.user_type === 'superadmin' || u.is_superadmin)) {
                return false
              }

              // Exclude superadmins
              if (u.user_type === 'superadmin') return false

              // Company restriction
              let companyID = 0
              if (currentUser?.user_type !== 'superadmin') {
                companyID = currentUser?.user_type === 'empleador' ? currentUser.id : (currentUser?.empleador_id || 0)
              }

              if (companyID) {
                if (u.user_type === 'empleador') {
                  if (u.id !== companyID) return false
                } else if (u.user_type === 'profesional' || u.user_type === 'empleado') {
                  if (u.empleador_id !== companyID) return false
                } else {
                  return false
                }
              }
              
              return true
            })
            .map(u => (
              <div key={u.id} className={styles['user-item']} onClick={() => { onAddMember(u.id); }}>
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
