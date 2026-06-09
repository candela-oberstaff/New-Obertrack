import { useState } from 'react'
import { User } from '../../../types'
import styles from '../../../pages/SlackChat.module.css'
import { getUserColor } from '../ChatUtils'
import { Modal, Button } from '../../ui'

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
  const [searchTerm, setSearchTerm] = useState('')

  const filteredUsers = allUsers.filter(u => {
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

    // Search filter
    if (searchTerm.trim()) {
      const term = searchTerm.toLowerCase()
      const nameMatch = u.name?.toLowerCase().includes(term)
      const emailMatch = u.email?.toLowerCase().includes(term)
      if (!nameMatch && !emailMatch) return false
    }

    return true
  })

  return (
    <Modal
      isOpen
      onClose={onClose}
      title="Añadir personas al canal"
      size="sm"
      footer={<Button variant="secondary" onClick={onClose}>Cerrar</Button>}
    >
      <p className={styles['hint']}>Haz clic en un usuario para añadirlo</p>

      <div style={{ marginBottom: '16px' }}>
        <input
          type="text"
          placeholder="Buscar por nombre o correo..."
          value={searchTerm}
          onChange={(e) => setSearchTerm(e.target.value)}
          style={{
            width: '100%',
            padding: '10px 12px',
            border: '1px solid #cbd5e1',
            borderRadius: '8px',
            fontSize: '14px',
            outline: 'none',
            boxSizing: 'border-box',
            transition: 'border-color 0.2s',
          }}
          onFocus={(e) => e.target.style.borderColor = '#1d1c1d'}
          onBlur={(e) => e.target.style.borderColor = '#cbd5e1'}
          autoFocus
        />
      </div>

      <div className={styles['users-list']} style={{ maxHeight: '280px', overflowY: 'auto', display: 'flex', flexDirection: 'column', gap: '4px' }}>
        {filteredUsers.length > 0 ? (
          filteredUsers.map(u => (
            <div key={u.id} className={styles['user-item']} onClick={() => { onAddMember(u.id) }}>
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
          ))
        ) : (
          <div style={{ textAlign: 'center', padding: '24px 8px', color: '#64748b', fontSize: '14px' }}>
            No se encontraron usuarios
          </div>
        )}
      </div>
    </Modal>
  )
}
