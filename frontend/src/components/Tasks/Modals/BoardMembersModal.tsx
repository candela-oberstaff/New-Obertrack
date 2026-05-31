import type { User, Board } from '../../../types'
import styles from '../../../pages/Tasks.module.css'

interface BoardMembersModalProps {
  isOpen: boolean
  onClose: () => void
  selectedBoard: Board | null
  optimisticMembers: number[]
  setOptimisticMembers: (ids: number[]) => void
  users: User[]
  onUpdateMembers: (newMembers: number[]) => Promise<void>
}

export function BoardMembersModal({
  isOpen,
  onClose,
  selectedBoard,
  optimisticMembers,
  setOptimisticMembers,
  users,
  onUpdateMembers
}: BoardMembersModalProps) {
  if (!isOpen || !selectedBoard) return null

  return (
    <div className={styles['modal-overlay']} onClick={onClose}>
      <div className={`${styles['modal']} ${styles['board-modal']} ${styles['wide'] || 'wide'}`} onClick={(e) => e.stopPropagation()}>
        <h2>Miembros del Tablero</h2>
        <p style={{ color: '#64748b', marginBottom: '20px', marginTop: '-16px' }}>{selectedBoard.name}</p>
        <div className={styles['form-group']}>
          <label>Selecciona los miembros que quieres agregar al tablero</label>
          <div className={styles['members-select']}>
            {users.map(user => {
              const isMember = optimisticMembers.includes(user.id)
              return (
                <label key={user.id} className={styles['member-checkbox']}>
                  <div className={styles['left-section']}>
                    <input
                      type="checkbox"
                      checked={isMember}
                      onChange={(e) => {
                        const newOptimisticMembers = e.target.checked
                          ? [...optimisticMembers, user.id]
                          : optimisticMembers.filter(id => id !== user.id)
                        setOptimisticMembers(newOptimisticMembers)
                        onUpdateMembers(newOptimisticMembers)
                      }}
                    />
                    <span className={styles['checkbox-custom']}></span>
                    <div className={styles['member-avatar']}>
                      {user.name.split(' ').map((n: string) => n[0]).join('').substring(0, 2).toUpperCase()}
                    </div>
                    <div className={styles['member-info']}>
                      <span className={styles['member-name']}>{user.name}</span>
                      <span className={styles['member-role']}>
                        {user.user_type === 'empleado' || user.user_type === 'profesional'
                          ? 'Profesional'
                          : user.user_type === 'empleador'
                          ? 'Empresa'
                          : user.user_type === 'superadmin'
                          ? 'Super Admin'
                          : user.user_type}
                      </span>
                    </div>
                  </div>
                  <span className={`${styles['member-status'] || 'member-status'} ${isMember ? (styles['active'] || 'active') : (styles['inactive'] || 'inactive')}`}>
                    {isMember ? 'En tablero' : 'No asignado'}
                  </span>
                </label>
              )
            })}
          </div>
        </div>
        <div className={styles['modal-actions']}>
          <button type="button" className={styles['btn-primary']} onClick={onClose}>Listo</button>
        </div>
      </div>
    </div>
  )
}
