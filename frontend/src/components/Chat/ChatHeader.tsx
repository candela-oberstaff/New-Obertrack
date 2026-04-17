import { Channel } from '../../types/chat'
import { PinIcon, UserPlusIcon, InfoIcon, LogOutIcon } from './Icons'
import styles from '../../pages/SlackChat.module.css'

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
    <div className={styles['chat-header-bar']}>
      <button className={styles['mobile-channels-toggle']} onClick={() => setShowMobileChannels(!showMobileChannels)}>
        {selectedChannel ? (
          `${selectedChannel.type === 'direct' ? '○' : selectedChannel.type === 'private' ? '🔒' : '#'} ${selectedChannel.type === 'direct' ? (selectedChannel.recipient?.name || selectedChannel.name) : selectedChannel.name}`
        ) : 'Seleccionar canal'}
      </button>
      
      <div className={styles['channel-tabs']}>
        {selectedChannel && (
          <div className={`${styles['channel-tab']} ${styles['active']}`}>
            <span>{selectedChannel.type === 'direct' ? '○' : selectedChannel.type === 'private' ? '🔒' : '#'}</span>
            {selectedChannel.type === 'direct' ? (selectedChannel.recipient?.name || selectedChannel.name) : selectedChannel.name}
          </div>
        )}
      </div>

      <div className={styles['channel-actions']}>
        {selectedChannel && (
          <>
            <button onClick={() => setShowPinnedMessages(true)} title="Mensajes fijados" className={styles['pin-btn'] || 'pin-btn'}>
              <PinIcon />
              {pinnedMessagesCount > 0 && <span className={styles['pin-count']}>{pinnedMessagesCount}</span>}
            </button>
            <button onClick={() => setShowAddMembers(true)} title="Añadir personas">
              <UserPlusIcon />
            </button>
            <button onClick={() => setShowChannelSettings(true)} title="Info del canal">
              <InfoIcon />
            </button>
            <button onClick={() => leaveChannel(selectedChannel.id)} title="Salir del canal" className={styles['leave-btn']}>
              <LogOutIcon />
            </button>
          </>
        )}
      </div>
    </div>
  )
}
