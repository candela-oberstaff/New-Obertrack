import { useEffect, useMemo, useState } from 'react'
import { Search } from 'lucide-react'
import type { User, Board, BoardInvitation } from '../../../types'
import styles from '../../../pages/Tasks.module.css'
import { Modal, Button } from '../../ui'

interface BoardMembersModalProps {
  isOpen: boolean
  onClose: () => void
  selectedBoard: Board | null
  users: User[]
  pendingInvitations: BoardInvitation[]
  onInvite: (userId: number) => Promise<void>
  onRemoveMember: (userId: number) => Promise<void>
  onCancelInvitation: (invId: number) => Promise<void>
}

export function BoardMembersModal({
  isOpen,
  onClose,
  selectedBoard,
  users,
  pendingInvitations,
  onInvite,
  onRemoveMember,
  onCancelInvitation,
}: BoardMembersModalProps) {
  const [query, setQuery] = useState('')
  const [busyId, setBusyId] = useState<number | null>(null)

  useEffect(() => {
    if (!isOpen) setQuery('')
  }, [isOpen])

  const memberIds = useMemo(
    () => new Set((selectedBoard?.members || []).map((m) => m.id)),
    [selectedBoard],
  )
  const pendingByUser = useMemo(() => {
    const m = new Map<number, number>()
    pendingInvitations.forEach((inv) => m.set(inv.user_id, inv.id))
    return m
  }, [pendingInvitations])

  const visibleUsers = useMemo(() => {
    const sorted = [...users].sort((a, b) =>
      (a.name || '').localeCompare(b.name || '', 'es', { sensitivity: 'base' })
    )
    const q = query.trim().toLowerCase()
    if (!q) return sorted
    return sorted.filter(
      (u) => (u.name || '').toLowerCase().includes(q) || (u.email || '').toLowerCase().includes(q)
    )
  }, [users, query])

  if (!selectedBoard) return null

  const toggle = async (user: User) => {
    setBusyId(user.id)
    try {
      const invId = pendingByUser.get(user.id)
      if (memberIds.has(user.id)) {
        await onRemoveMember(user.id)
      } else if (invId) {
        await onCancelInvitation(invId)
      } else {
        await onInvite(user.id)
      }
    } finally {
      setBusyId(null)
    }
  }

  const statusOf = (user: User) => {
    if (memberIds.has(user.id)) return { label: 'En tablero', bg: '#dcfce7', color: '#15803d' }
    if (pendingByUser.has(user.id)) return { label: 'Invitado (pendiente)', bg: '#fef3c7', color: '#b45309' }
    return { label: 'No asignado', bg: '#f1f5f9', color: '#64748b' }
  }

  const actionLabel = (user: User) => {
    if (memberIds.has(user.id)) return 'Quitar'
    if (pendingByUser.has(user.id)) return 'Cancelar'
    return 'Invitar'
  }

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title="Miembros del Tablero"
      size="md"
      footer={<Button onClick={onClose}>Listo</Button>}
    >
      <p style={{ color: '#64748b', marginBottom: '8px', marginTop: 0 }}>{selectedBoard.name}</p>
      <p style={{ color: '#94a3b8', fontSize: 13, marginTop: 0, marginBottom: 16 }}>
        Al invitar, la persona recibe una notificación y decide si acepta.
      </p>

      <div className={styles['form-group']}>
        <div style={{ position: 'relative', marginBottom: 12 }}>
          <Search
            size={16}
            style={{ position: 'absolute', left: 11, top: '50%', transform: 'translateY(-50%)', color: '#94a3b8', pointerEvents: 'none' }}
          />
          <input
            type="text"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="Buscar usuario..."
            style={{ width: '100%', padding: '9px 12px 9px 34px', border: '1px solid #e2e8f0', borderRadius: 8, fontSize: 14, outline: 'none', boxSizing: 'border-box' }}
          />
        </div>

        <div className={styles['members-select']}>
          {visibleUsers.length === 0 && (
            <p style={{ textAlign: 'center', color: '#94a3b8', fontSize: 14, padding: '20px 0', margin: 0 }}>
              No se encontraron usuarios
            </p>
          )}
          {visibleUsers.map((user) => {
            const st = statusOf(user)
            const isCreator = user.id === selectedBoard.created_by
            return (
              <div key={user.id} className={styles['member-checkbox']} style={{ cursor: 'default' }}>
                <div className={styles['left-section']}>
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
                <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
                  <span style={{ background: st.bg, color: st.color, fontSize: 12, fontWeight: 700, padding: '4px 10px', borderRadius: 999, whiteSpace: 'nowrap' }}>
                    {st.label}
                  </span>
                  {isCreator ? (
                    <span style={{ fontSize: 12, color: '#94a3b8', whiteSpace: 'nowrap' }}>Creador</span>
                  ) : (
                    <Button
                      size="sm"
                      variant={memberIds.has(user.id) ? 'danger' : 'secondary'}
                      loading={busyId === user.id}
                      onClick={() => toggle(user)}
                    >
                      {actionLabel(user)}
                    </Button>
                  )}
                </div>
              </div>
            )
          })}
        </div>
      </div>
    </Modal>
  )
}
