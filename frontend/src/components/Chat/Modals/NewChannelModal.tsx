import { useState } from 'react'
import { Modal, Button } from '../../ui'
import { Select } from '../../ui/Select'
import { User } from '../../../types'
import { getUserColor } from '../ChatUtils'
import styles from '../../../pages/SlackChat.module.css'

interface NewChannelModalProps {
  newChannel: { name: string; description: string; type: 'public' | 'private'; member_ids: number[] }
  setNewChannel: (channel: any) => void
  allUsers: User[]
  currentUser: User | null
  onClose: () => void
  onCreate: () => void
}

export function NewChannelModal({
  newChannel,
  setNewChannel,
  allUsers,
  currentUser,
  onClose,
  onCreate
}: NewChannelModalProps) {
  const [memberSearch, setMemberSearch] = useState('')

  // Same eligibility rules as AddMembersModal: exclude self/superadmins and other companies.
  const selectableUsers = allUsers.filter(u => {
    if (u.id === currentUser?.id) return false
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

    if (memberSearch.trim()) {
      const term = memberSearch.toLowerCase()
      if (!u.name?.toLowerCase().includes(term) && !u.email?.toLowerCase().includes(term)) return false
    }
    return true
  })

  const toggleMember = (id: number) => {
    const selected = newChannel.member_ids.includes(id)
    setNewChannel({
      ...newChannel,
      member_ids: selected ? newChannel.member_ids.filter(m => m !== id) : [...newChannel.member_ids, id],
    })
  }

  return (
    <Modal
      isOpen
      onClose={onClose}
      title="Crear canal"
      size="sm"
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>Cancelar</Button>
          <Button onClick={onCreate} disabled={!newChannel.name.trim()}>Crear canal</Button>
        </>
      }
    >
      <div className={styles['form-group']}>
        <label>Nombre del canal</label>
        <input
          type="text"
          value={newChannel.name}
          onChange={(e) => setNewChannel({ ...newChannel, name: e.target.value })}
          placeholder="ej: general"
          autoFocus
        />
      </div>
      <div className={styles['form-group']}>
        <label>Descripción (opcional)</label>
        <input
          type="text"
          value={newChannel.description}
          onChange={(e) => setNewChannel({ ...newChannel, description: e.target.value })}
          placeholder="¿De qué trata este canal?"
        />
      </div>
      <div className={styles['form-group']}>
        <label>Tipo</label>
        <Select
          fullWidth
          value={newChannel.type}
          onChange={(v) => setNewChannel({ ...newChannel, type: v as 'public' | 'private' })}
          options={[
            { value: 'public', label: 'Público' },
            { value: 'private', label: 'Privado' },
          ]}
        />
      </div>
      {newChannel.type === 'public' ? (
        <p className={styles['hint']}>
          📢 Todos los miembros de la empresa se unirán automáticamente a este canal y podrán ver todo el historial.
        </p>
      ) : (
      <div className={styles['form-group']}>
        <label>
          Miembros iniciales (opcional)
          {newChannel.member_ids.length > 0 && ` — ${newChannel.member_ids.length} seleccionados`}
        </label>
        <input
          type="text"
          value={memberSearch}
          onChange={(e) => setMemberSearch(e.target.value)}
          placeholder="Buscar por nombre o correo..."
        />
        <div className={styles['users-list']} style={{ maxHeight: '160px', overflowY: 'auto', display: 'flex', flexDirection: 'column', gap: '4px', marginTop: '8px' }}>
          {selectableUsers.map(u => {
            const selected = newChannel.member_ids.includes(u.id)
            return (
              <div
                key={u.id}
                className={styles['user-item']}
                style={selected ? { background: '#ede9fe' } : undefined}
                onClick={() => toggleMember(u.id)}
              >
                <div className={styles['user-avatar']} style={{ background: getUserColor(u.name || '') }}>
                  {u.name?.charAt(0).toUpperCase()}
                </div>
                <div className={styles['user-info']}>
                  <span className={styles['user-name']}>{u.name}</span>
                  <span className={styles['user-email']}>{u.email}</span>
                </div>
                {selected && <span style={{ marginLeft: 'auto', color: 'var(--primary)', fontWeight: 700 }}>✓</span>}
              </div>
            )
          })}
          {selectableUsers.length === 0 && (
            <div style={{ textAlign: 'center', padding: '12px 8px', color: '#64748b', fontSize: '13px' }}>
              No se encontraron usuarios
            </div>
          )}
        </div>
        <p className={styles['hint']} style={{ marginTop: '8px' }}>
          🔒 Canal por invitación: los miembros solo verán los mensajes enviados después de su ingreso.
        </p>
      </div>
      )}
    </Modal>
  )
}
