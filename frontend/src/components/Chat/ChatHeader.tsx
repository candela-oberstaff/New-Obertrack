import { Channel } from '../../types/chat'
import { PinIcon, UserPlusIcon, InfoIcon, LogOutIcon, SearchIcon, StarIcon } from './Icons'
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
  onShowSearch: () => void
  onShowStarred: () => void
  recipientStatus?: 'online' | 'away' | 'offline'
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
  onShowSearch,
  onShowStarred,
  recipientStatus,
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
            {selectedChannel.type === 'direct' && recipientStatus && (
              <span
                className={`${styles['status-dot']} ${styles[recipientStatus]}`}
                title={recipientStatus === 'online' ? 'En línea' : recipientStatus === 'away' ? 'Ausente' : 'Desconectado'}
              />
            )}
          </div>
        )}
      </div>

      <div className={styles['channel-actions']}>
        <button onClick={onShowStarred} title="Mensajes destacados">
          <StarIcon />
        </button>
        {selectedChannel && (
          <>
            <button onClick={onShowSearch} title="Buscar en el canal">
              <SearchIcon />
            </button>
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
