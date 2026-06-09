import { useState } from 'react'
import { Channel, ChannelMember } from '../../../types/chat'
import { User } from '../../../types'
import styles from '../../../pages/SlackChat.module.css'
import { getUserColor } from '../ChatUtils'
import { Modal, Button } from '../../ui'

interface ChannelSettingsModalProps {
  selectedChannel: Channel
  channelMembers: ChannelMember[]
  currentUser: User | null
  onClose: () => void
  onRemoveMember: (id: number) => void
  onLeaveChannel: (id: number) => void
  onShowAddMembers: () => void
  onUpdateChannel: (id: number, updates: { name: string; description: string }) => Promise<void>
}

export function ChannelSettingsModal({
  selectedChannel,
  channelMembers,
  currentUser,
  onClose,
  onRemoveMember,
  onLeaveChannel,
  onShowAddMembers,
  onUpdateChannel
}: ChannelSettingsModalProps) {
  const [isEditing, setIsEditing] = useState(false)
  const [editName, setEditName] = useState(selectedChannel.name)
  const [editDescription, setEditDescription] = useState(selectedChannel.description || '')
  const [isSaving, setIsSaving] = useState(false)

  const isOwner = currentUser?.id === selectedChannel.created_by

  const handleSave = async () => {
    if (!editName.trim()) return
    setIsSaving(true)
    try {
      await onUpdateChannel(selectedChannel.id, { name: editName, description: editDescription })
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
            {isOwner && (
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
              {channelMembers.map(member => (
                <div key={member.id} className={styles['member-item']}>
                  <div
                    className={styles['member-avatar']}
                    style={{ background: getUserColor(member.name || '') }}
                  >
                    {member.name?.charAt(0).toUpperCase()}
                  </div>
                  <span className={styles['member-name']}>{member.name}</span>
                  {member.id === selectedChannel.created_by && (
                    <span className={styles['owner-badge']}>Creador</span>
                  )}
                  {member.id !== selectedChannel.created_by && member.id !== currentUser?.id && (
                    <button className={styles['remove-btn']} onClick={() => onRemoveMember(member.id)} title="Eliminar del canal">×</button>
                  )}
                  {member.id === currentUser?.id && member.id !== selectedChannel.created_by && (
                    <button className={styles['leave-btn-small']} onClick={() => onLeaveChannel(selectedChannel.id)}>Salir</button>
                  )}
                </div>
              ))}
            </div>
            <button className={styles['btn-add-member']} onClick={onShowAddMembers}>+ Añadir personas</button>
          </div>
        </>
      )}
    </Modal>
  )
}
