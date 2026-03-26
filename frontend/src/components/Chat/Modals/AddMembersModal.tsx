import { User } from '../../../types'

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
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal-content" onClick={e => e.stopPropagation()}>
        <div className="modal-header">
          <h2>Añadir personas al canal</h2>
          <button className="close-btn" onClick={onClose}>×</button>
        </div>
        
        <p className="hint">Haz clic en un usuario para añadirlo</p>
        <div className="users-list">
          {allUsers
            .filter(u => !isMember(u.id))
            .map(u => (
              <div key={u.id} className="user-item" onClick={() => { onAddMember(u.id); }}>
                <div className="user-avatar">{u.name?.charAt(0).toUpperCase()}</div>
                <div className="user-info">
                  <span className="user-name">{u.name}</span>
                  <span className="user-email">{u.email}</span>
                </div>
              </div>
            ))}
        </div>
        <div className="modal-actions">
          <button onClick={onClose}>Cerrar</button>
        </div>
      </div>
    </div>
  )
}
