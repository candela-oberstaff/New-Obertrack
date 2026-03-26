import { Channel } from '../../types/chat'
import { PinIcon, UserPlusIcon, InfoIcon, LogOutIcon } from './Icons'

interface ChatHeaderProps {
  selectedChannel: Channel | null
  showMobileChannels: boolean
  setShowMobileChannels: (show: boolean) => void
  showPinnedMessages: boolean
  setShowPinnedMessages: (show: boolean) => void
  pinnedMessagesCount: number
  setShowAddMembers: (show: boolean) => void
  setShowChannelSettings: (show: boolean) => void
  leaveChannel: (channelId: number) => void
}

export function ChatHeader({
  selectedChannel,
  showMobileChannels,
  setShowMobileChannels,
  setShowPinnedMessages,
  pinnedMessagesCount,
  setShowAddMembers,
  setShowChannelSettings,
  leaveChannel,
}: ChatHeaderProps) {
  return (
    <div className="chat-header-bar">
      <button className="mobile-channels-toggle" onClick={() => setShowMobileChannels(!showMobileChannels)}>
        {selectedChannel ? `# ${selectedChannel.name}` : 'Seleccionar canal'}
      </button>
      
      <div className="channel-tabs">
        {selectedChannel && (
          <div className="channel-tab active">
            <span>{selectedChannel.type === 'private' ? '🔒' : '#'}</span>
            {selectedChannel.name}
          </div>
        )}
      </div>

      <div className="channel-actions">
        {selectedChannel && (
          <>
            <button onClick={() => setShowPinnedMessages(true)} title="Mensajes fijados" className="pin-btn">
              <PinIcon />
              {pinnedMessagesCount > 0 && <span className="pin-count">{pinnedMessagesCount}</span>}
            </button>
            <button onClick={() => setShowAddMembers(true)} title="Añadir personas">
              <UserPlusIcon />
            </button>
            <button onClick={() => setShowChannelSettings(true)} title="Info del canal">
              <InfoIcon />
            </button>
            <button onClick={() => leaveChannel(selectedChannel.id)} title="Salir del canal" className="leave-btn">
              <LogOutIcon />
            </button>
          </>
        )}
      </div>
    </div>
  )
}
