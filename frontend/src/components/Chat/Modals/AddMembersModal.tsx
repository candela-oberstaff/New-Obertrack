import { useMemo, useState } from 'react'
import { Check, Loader2, UserPlus } from 'lucide-react'
import { User } from '../../../types'
import styles from '../../../pages/SlackChat.module.css'
import { getUserColor } from '../ChatUtils'
import { Modal, Button } from '../../ui'

interface AddMembersModalProps {
  allUsers: User[]
  isMember: (userId: number) => boolean
  onAddMember: (userId: number) => Promise<void>
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
  const [pending, setPending] = useState<Set<number>>(new Set())
  const [added, setAdded] = useState<Set<number>>(new Set())

  const handleAdd = async (userId: number) => {
    if (pending.has(userId) || added.has(userId)) return

    setPending(prev => new Set(prev).add(userId))
    try {
      await onAddMember(userId)
      setAdded(prev => new Set(prev).add(userId))
    } catch {
      // El padre ya muestra el error; aquí solo se libera la fila para reintentar.
    } finally {
      setPending(prev => {
        const next = new Set(prev)
        next.delete(userId)
        return next
      })
    }
  }

  const filteredUsers = useMemo(() => allUsers.filter(u => {
    // Los recién añadidos siguen en la lista, marcados, en vez de desaparecer.
    if (isMember(u.id) && !added.has(u.id)) return false

    if (currentUser?.user_type === 'profesional' && (u.user_type === 'superadmin' || u.is_superadmin)) {
      return false
    }

    if (u.user_type === 'superadmin' || u.is_superadmin) return false

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

    if (searchTerm.trim()) {
      const term = searchTerm.toLowerCase()
      const nameMatch = u.name?.toLowerCase().includes(term)
      const emailMatch = u.email?.toLowerCase().includes(term)
      if (!nameMatch && !emailMatch) return false
    }

    return true
  }).sort((a, b) => (a.name || '').localeCompare(b.name || '', 'es', { sensitivity: 'base' })),
    [allUsers, isMember, added, currentUser, searchTerm])

  return (
    <Modal
      isOpen
      onClose={onClose}
      title="Añadir personas al canal"
      size="sm"
      footer={<Button variant="secondary" onClick={onClose}>Listo</Button>}
    >
      <p className={styles['hint']}>
        {added.size > 0
          ? `${added.size} ${added.size === 1 ? 'persona añadida' : 'personas añadidas'}. Puedes seguir añadiendo o cerrar.`
          : 'Haz clic en una persona para añadirla al canal'}
      </p>

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
          filteredUsers.map(u => {
            const isPending = pending.has(u.id)
            const isAdded = added.has(u.id)
            const disabled = isPending || isAdded
            return (
              <button
                key={u.id}
                type="button"
                className={styles['user-item']}
                onClick={() => handleAdd(u.id)}
                disabled={disabled}
                style={{
                  width: '100%',
                  textAlign: 'left',
                  font: 'inherit',
                  background: isAdded ? '#f0fdf4' : undefined,
                  cursor: disabled ? 'default' : 'pointer',
                  opacity: isPending ? 0.6 : 1,
                }}
              >
                <div
                  className={styles['user-avatar']}
                  style={{ background: getUserColor(u.name || '') }}
                >
                  {u.name?.charAt(0).toUpperCase()}
                </div>
                <div className={styles['user-info']} style={{ flex: 1, minWidth: 0 }}>
                  <span className={styles['user-name']}>{u.name}</span>
                  <span className={styles['user-email']}>{u.email}</span>
                </div>
                <span style={{ display: 'flex', alignItems: 'center', gap: '6px', flexShrink: 0, fontSize: '13px', color: isAdded ? '#16a34a' : '#64748b' }}>
                  {isPending ? (
                    <Loader2 size={16} className={styles['spinning']} />
                  ) : isAdded ? (
                    <><Check size={16} /> Añadido</>
                  ) : (
                    <UserPlus size={16} />
                  )}
                </span>
              </button>
            )
          })
        ) : (
          <div style={{ textAlign: 'center', padding: '24px 8px', color: '#64748b', fontSize: '14px' }}>
            {searchTerm.trim() ? 'No se encontraron personas' : 'Todos ya son miembros del canal'}
          </div>
        )}
      </div>
    </Modal>
  )
}
