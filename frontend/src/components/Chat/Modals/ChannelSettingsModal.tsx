import { Channel, ChannelMember } from '../../../types/chat'
import { User } from '../../../types'

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
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal-content" onClick={e => e.stopPropagation()}>
        <div className="modal-header">
          <h2>Configuración de #{selectedChannel.name}</h2>
          <button className="close-btn" onClick={onClose}>×</button>
        </div>

        {selectedChannel.description && (
          <p className="channel-desc">{selectedChannel.description}</p>
        )}
        
        <div className="members-section">
          <h3>Miembros ({channelMembers.length})</h3>
          <div className="members-list">
            {channelMembers.map(member => (
              <div key={member.id} className="member-item">
                <div className="member-avatar">{member.name?.charAt(0).toUpperCase()}</div>
                <span className="member-name">{member.name}</span>
                {member.id === selectedChannel.created_by && (
                  <span className="owner-badge">Owner</span>
                )}
                {member.id !== selectedChannel.created_by && member.id !== currentUser?.id && (
                  <button className="remove-btn" onClick={() => onRemoveMember(member.id)} title="Eliminar del canal">×</button>
                )}
                {member.id === currentUser?.id && member.id !== selectedChannel.created_by && (
                  <button className="leave-btn-small" onClick={() => onLeaveChannel(selectedChannel.id)}>Salir</button>
                )}
              </div>
            ))}
          </div>
          <button className="btn-add-member" onClick={onShowAddMembers}>+ Añadir personas</button>
        </div>
        
        <div className="modal-actions">
          <button onClick={onClose}>Cerrar</button>
        </div>
      </div>
    </div>
  )
}
