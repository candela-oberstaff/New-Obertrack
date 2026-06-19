import { useState } from 'react'
import { Channel, ChannelMember } from '../../../types/chat'
import { User } from '../../../types'
import styles from '../../../pages/SlackChat.module.css'
import { getUserColor, isSupportChannel } from '../ChatUtils'
import { Modal, Button } from '../../ui'
import { channelService } from '../../../services/channel.service'
import { useNotification } from '../../../context/NotificationContext'

interface ChannelSettingsModalProps {
  selectedChannel: Channel
  channelMembers: ChannelMember[]
  currentUser: User | null
  isSuperadmin?: boolean
  onClose: () => void
  onRemoveMember: (id: number) => void
  onLeaveChannel: (id: number) => void
  onShowAddMembers: () => void
  onUpdateChannel: (id: number, updates: { name: string; description: string; type?: 'public' | 'private' }) => Promise<void>
  onDeleteChannel?: (id: number) => void
  onRefreshMembers?: (channelId: number) => void
}

export function ChannelSettingsModal({
  selectedChannel,
  channelMembers,
  currentUser,
  isSuperadmin = false,
  onClose,
  onRemoveMember,
  onLeaveChannel,
  onShowAddMembers,
  onUpdateChannel,
  onDeleteChannel,
  onRefreshMembers
}: ChannelSettingsModalProps) {
  const { error: notifyError } = useNotification()
  const [isEditing, setIsEditing] = useState(false)
  const [editName, setEditName] = useState(selectedChannel.name)
  const [editDescription, setEditDescription] = useState(selectedChannel.description || '')
  // Privacidad editable solo para canales normales (no DM, no soporte). El
  // backend rechaza (400) cambiar el type de direct/soporte de todos modos.
  const [editType, setEditType] = useState<'public' | 'private'>(
    selectedChannel.type === 'private' ? 'private' : 'public'
  )
  const canEditPrivacy = selectedChannel.type !== 'direct' && !isSupportChannel(selectedChannel)
  const [isSaving, setIsSaving] = useState(false)
  const [updatingRoleId, setUpdatingRoleId] = useState<number | null>(null)

  const isOwner = currentUser?.id === selectedChannel.created_by
  // Rol del usuario actual dentro de este canal (si role no llega, se trata como 'member').
  const myRole = channelMembers.find(m => m.id === currentUser?.id)?.role ?? 'member'
  // Gestionar (editar info, eliminar canal, añadir/quitar miembros):
  // creador, admin del canal o superadmin.
  const canManage = isOwner || myRole === 'admin' || isSuperadmin
  // Promover/degradar admins: solo creador o superadmin (no los admins normales).
  const canAssignAdmins = isOwner || isSuperadmin
  // Eliminar canal: solo el creador o un superadmin. El backend igual valida y
  // rechaza (400) los no eliminables (DM/soporte).
  const canDelete = !!onDeleteChannel && (isOwner || isSuperadmin)

  const handleSetRole = async (userId: number, role: 'admin' | 'member') => {
    setUpdatingRoleId(userId)
    try {
      await channelService.setMemberRole(selectedChannel.id, userId, role)
      onRefreshMembers?.(selectedChannel.id)
    } catch (e) {
      console.error(e)
      notifyError('No se pudo actualizar el rol del miembro')
    } finally {
      setUpdatingRoleId(null)
    }
  }

  const handleSave = async () => {
    if (!editName.trim()) return
    setIsSaving(true)
    try {
      await onUpdateChannel(selectedChannel.id, {
        name: editName,
        description: editDescription,
        ...(canEditPrivacy ? { type: editType } : {}),
      })
      setIsEditing(false)
    } catch (e) {
      console.error(e)
    } finally {
      setIsSaving(false)
    }
  }

  return (
    <Modal
      isOpen
      onClose={onClose}
      size="md"
      title={isEditing ? 'Editar canal' : `Configuración de ${selectedChannel.type === 'private' ? '🔒' : '#'}${selectedChannel.name}`}
      footer={
        isEditing ? (
          <>
            <Button variant="secondary" onClick={() => setIsEditing(false)} disabled={isSaving}>Cancelar</Button>
            <Button onClick={handleSave} loading={isSaving} disabled={!editName.trim()}>Guardar</Button>
          </>
        ) : (
          <Button variant="secondary" onClick={onClose}>Cerrar</Button>
        )
      }
    >
      {isEditing ? (
        <div className={styles['edit-form']} style={{ display: 'flex', flexDirection: 'column', gap: '12px' }}>
          <div className={styles['form-group']}>
            <label style={{ fontWeight: '500', fontSize: '14px', display: 'block', marginBottom: '6px' }}>Nombre del canal</label>
            <input
              type="text"
              value={editName}
              onChange={(e) => setEditName(e.target.value)}
              style={{ width: '100%', padding: '8px 12px', borderRadius: '6px', border: '1px solid #e2e8f0' }}
              autoFocus
            />
          </div>
          <div className={styles['form-group']}>
            <label style={{ fontWeight: '500', fontSize: '14px', display: 'block', marginBottom: '6px' }}>Descripción</label>
            <textarea
              value={editDescription}
              onChange={(e) => setEditDescription(e.target.value)}
              rows={3}
              style={{ width: '100%', padding: '8px 12px', borderRadius: '6px', border: '1px solid #e2e8f0', fontFamily: 'inherit' }}
            />
          </div>
          {canEditPrivacy && (
            <div className={styles['form-group']}>
              <label style={{ fontWeight: '500', fontSize: '14px', display: 'block', marginBottom: '6px' }}>Privacidad</label>
              <div style={{ display: 'flex', gap: '16px' }}>
                <label style={{ display: 'flex', alignItems: 'center', gap: '6px', cursor: 'pointer', fontSize: '14px' }}>
                  <input
                    type="radio"
                    name="channel-privacy"
                    value="public"
                    checked={editType === 'public'}
                    onChange={() => setEditType('public')}
                  />
                  # Público
                </label>
                <label style={{ display: 'flex', alignItems: 'center', gap: '6px', cursor: 'pointer', fontSize: '14px' }}>
                  <input
                    type="radio"
                    name="channel-privacy"
                    value="private"
                    checked={editType === 'private'}
                    onChange={() => setEditType('private')}
                  />
                  🔒 Privado
                </label>
              </div>
            </div>
          )}
        </div>
      ) : (
        <>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: '16px' }}>
            <div style={{ flex: 1, paddingRight: '12px' }}>
              {selectedChannel.description ? (
                <p className={styles['channel-desc']} style={{ margin: 0, color: '#64748b' }}>{selectedChannel.description}</p>
              ) : (
                <p className={styles['channel-desc']} style={{ margin: 0, fontStyle: 'italic', color: '#94a3b8' }}>Sin descripción</p>
              )}
            </div>
            {canManage && (
              <button
                onClick={() => setIsEditing(true)}
                style={{ background: '#f1f5f9', border: '1px solid #cbd5e1', padding: '6px 12px', borderRadius: '6px', cursor: 'pointer', fontSize: '13px', fontWeight: '500', flexShrink: 0 }}
              >
                Editar info
              </button>
            )}
          </div>

          <div className={styles['members-section']}>
            <h3>Miembros ({channelMembers.length})</h3>
            <div className={styles['members-list']}>
              {channelMembers.map(member => {
                const isCreator = member.id === selectedChannel.created_by
                // El creador siempre cuenta como admin y no se puede degradar.
                const isAdmin = isCreator || member.role === 'admin'
                return (
                <div key={member.id} className={styles['member-item']}>
                  <div
                    className={styles['member-avatar']}
                    style={{ background: getUserColor(member.name || '') }}
                  >
                    {member.name?.charAt(0).toUpperCase()}
                  </div>
                  <span className={styles['member-name']}>{member.name}</span>
                  {isCreator && (
                    <span className={styles['owner-badge']}>Creador</span>
                  )}
                  {!isCreator && member.role === 'admin' && (
                    <span className={styles['owner-badge']}>Admin</span>
                  )}
                  {canAssignAdmins && !isCreator && (
                    <button
                      onClick={() => handleSetRole(member.id, isAdmin ? 'member' : 'admin')}
                      disabled={updatingRoleId === member.id}
                      title={isAdmin ? 'Quitar admin' : 'Hacer admin'}
                      style={{ fontSize: '11px', whiteSpace: 'nowrap' }}
                    >
                      {isAdmin ? 'Quitar admin' : 'Hacer admin'}
                    </button>
                  )}
                  {canManage && !isCreator && member.id !== currentUser?.id && (
                    <button className={styles['remove-btn']} onClick={() => onRemoveMember(member.id)} title="Eliminar del canal">×</button>
                  )}
                  {member.id === currentUser?.id && !isCreator && (
                    <button className={styles['leave-btn-small']} onClick={() => onLeaveChannel(selectedChannel.id)}>Salir</button>
                  )}
                </div>
                )
              })}
            </div>
            {canManage && (
              <button className={styles['btn-add-member']} onClick={onShowAddMembers}>+ Añadir personas</button>
            )}
          </div>

          {canDelete && (
            <div style={{ marginTop: '20px', paddingTop: '16px', borderTop: '1px solid #e2e8f0' }}>
              <Button variant="danger" onClick={() => onDeleteChannel?.(selectedChannel.id)}>
                Eliminar canal
              </Button>
            </div>
          )}
        </>
      )}
    </Modal>
  )
}
