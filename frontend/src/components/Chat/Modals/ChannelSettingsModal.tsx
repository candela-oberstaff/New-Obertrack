import { Channel, ChannelMember } from '../../../types/chat'
import { User } from '../../../types'
import styles from '../../../pages/SlackChat.module.css'
import { getUserColor } from '../ChatUtils'

interface ChannelSettingsModalProps {
  selectedChannel: Channel
  channelMembers: ChannelMember[]
  currentUser: User | null
  onClose: () => void
  onRemoveMember: (id: number) => void
  onLeaveChannel: (id: number) => void
  onShowAddMembers: () => void
}

export function ChannelSettingsModal({
  selectedChannel,
  channelMembers,
  currentUser,
  onClose,
  onRemoveMember,
  onLeaveChannel,
  onShowAddMembers
}: ChannelSettingsModalProps) {
  return (
    <div className={styles['modal-overlay']} onClick={onClose}>
      <div className={styles['modal-content']} onClick={e => e.stopPropagation()}>
        <div className={styles['modal-header']}>
          <h2>Configuración de #{selectedChannel.name}</h2>
          <button className={styles['close-btn']} onClick={onClose}>×</button>
        </div>

        {selectedChannel.description && (
          <p className={styles['channel-desc']}>{selectedChannel.description}</p>
        )}
        
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
                  <span className={styles['owner-badge']}>Owner</span>
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
        
        <div className={styles['modal-actions']}>
          <button onClick={onClose}>Cerrar</button>
        </div>
      </div>
    </div>
  )
}
