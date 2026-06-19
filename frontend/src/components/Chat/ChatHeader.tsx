import { LifeBuoy } from 'lucide-react'
import { Channel } from '../../types/chat'
import type { User } from '../../types'
import { PinIcon, UserPlusIcon, InfoIcon, LogOutIcon, SearchIcon, StarIcon } from './Icons'
import { isSupportChannel, supportLabel, dmContactName } from './ChatUtils'
import { SupportTicketControls } from './SupportTicketControls'
import styles from '../../pages/SlackChat.module.css'

// Nombre a mostrar de un canal en el encabezado (limpia los de soporte).
const channelDisplayName = (c: Channel) =>
  c.type === 'direct' ? dmContactName(c) : isSupportChannel(c) ? supportLabel(c.name) : c.name

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
  /** Muestra el botón "Soporte" (usuarios cliente) para contactar a Customer Success. */
  showSupportButton?: boolean
  onContactSupport?: () => void
  contactingSupport?: boolean
  // Gestión del ticket de soporte (cuando el canal seleccionado es de soporte).
  currentUserId?: number
  isSupportAgent?: boolean
  supportAgents?: User[]
  onClaimSupport?: () => void
  onAssignSupport?: (assigneeId: number) => void
  onResolveSupport?: () => void
  supportBusy?: boolean
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
  showSupportButton,
  onContactSupport,
  contactingSupport,
  currentUserId,
  isSupportAgent,
  supportAgents,
  onClaimSupport,
  onAssignSupport,
  onResolveSupport,
  supportBusy,
}: ChatHeaderProps) {
  return (
    <div className={styles['chat-header-bar']}>
      <button className={styles['mobile-channels-toggle']} onClick={() => setShowMobileChannels(!showMobileChannels)}>
        {selectedChannel ? (
          `${selectedChannel.type === 'direct' ? '○' : isSupportChannel(selectedChannel) ? '🛟' : selectedChannel.type === 'private' ? '🔒' : '#'} ${channelDisplayName(selectedChannel)}`
        ) : 'Seleccionar canal'}
      </button>

      <div className={styles['channel-tabs']}>
        {selectedChannel && (
          <div className={`${styles['channel-tab']} ${styles['active']}`}>
            <span>{selectedChannel.type === 'direct' ? '○' : isSupportChannel(selectedChannel) ? '🛟' : selectedChannel.type === 'private' ? '🔒' : '#'}</span>
            {channelDisplayName(selectedChannel)}
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
        {selectedChannel && isSupportChannel(selectedChannel) && (
          <SupportTicketControls
            channel={selectedChannel}
            currentUserId={currentUserId}
            isSupportAgent={!!isSupportAgent}
            supportAgents={supportAgents || []}
            onClaim={() => onClaimSupport?.()}
            onAssign={(id) => onAssignSupport?.(id)}
            onResolve={() => onResolveSupport?.()}
            busy={supportBusy}
          />
        )}
        {showSupportButton && (
          <button
            onClick={onContactSupport}
            disabled={contactingSupport}
            title="Contactar a Customer Success"
            style={{
              display: 'inline-flex', alignItems: 'center', gap: 6,
              background: '#7c3aed', color: '#fff', border: 'none',
              borderRadius: 8, padding: '7px 14px', fontWeight: 700, fontSize: 13,
              cursor: contactingSupport ? 'wait' : 'pointer', opacity: contactingSupport ? 0.7 : 1,
              width: 'auto',
            }}
          >
            <LifeBuoy size={15} /> {contactingSupport ? 'Abriendo…' : 'Soporte'}
          </button>
        )}
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
