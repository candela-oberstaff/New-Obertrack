import { Channel } from '../../types/chat'
import styles from '../../pages/SlackChat.module.css'

interface SidebarProps {
  channels: Channel[]
  selectedChannel: Channel | null
  setSelectedChannel: (channel: Channel) => void
  showMobileChannels: boolean
  setShowMobileChannels: (show: boolean) => void
  chatSidebarCollapsed: boolean
  setChatSidebarCollapsed: (collapsed: boolean) => void
  chatSidebarWidth: number
  setShowNewChannelModal: (show: boolean) => void
  setShowNewDmModal: (show: boolean) => void
  onMouseDownResize: (e: React.MouseEvent) => void
  isResizing: boolean
}

export function Sidebar({
  channels,
  selectedChannel,
  setSelectedChannel,
  showMobileChannels,
  setShowMobileChannels,
  chatSidebarCollapsed,
  setChatSidebarCollapsed,
  chatSidebarWidth,
  setShowNewChannelModal,
  setShowNewDmModal,
  onMouseDownResize,
  isResizing
}: SidebarProps) {
  const publicChannels = channels.filter(c => c.type === 'public')
  const privateChannels = channels.filter(c => c.type === 'private')
  const directMessages = channels.filter(c => c.type === 'direct')

  return (
    <>
      <div 
        className={`${styles['channels-panel']} ${showMobileChannels ? styles['open'] : ''} ${chatSidebarCollapsed ? styles['collapsed'] : ''}`}
        style={{ width: chatSidebarCollapsed ? 52 : chatSidebarWidth }}
      >
        {!chatSidebarCollapsed && (
          <>
            <div className={styles['channels-panel-header']}>
              <h3>Obertrack</h3>
              <button className={styles['new-msg-btn']} onClick={() => setShowNewChannelModal(true)} title="Nuevo canal">
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                  <path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/><path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"/>
                </svg>
              </button>
            </div>

            <div className={styles['channel-list-mini']}>
              <div className={styles['channel-group-label']}>
                <span>Canales</span>
                <button className={styles['add-btn-mini']} onClick={() => setShowNewChannelModal(true)} title="Crear canal">
                  +
                </button>
              </div>
              
              {publicChannels.map(channel => (
                <div
                  key={channel.id}
                  className={`${styles['channel-mini-item']} ${selectedChannel?.id === channel.id ? styles['active'] : ''}`}
                  onClick={() => {
                    setSelectedChannel(channel)
                    setShowMobileChannels(false)
                  }}
                >
                  <span className={styles['channel-mini-icon']}>#</span>
                  <span className={styles['channel-mini-name']}>{channel.name}</span>
                  {channel.unread_count > 0 && (
                    <span className={styles['unread-badge']}>{channel.unread_count}</span>
                  )}
                </div>
              ))}

              <div className={styles['channel-group-label']}>
                <span>Mensajes directos</span>
                <button 
                  className={styles['add-btn-mini']}
                  onClick={() => setShowNewDmModal(true)}
                  title="Nuevo mensaje directo"
                >
                  +
                </button>
              </div>
              
              {directMessages.map(channel => (
                <div
                  key={channel.id}
                  className={`${styles['channel-mini-item']} ${selectedChannel?.id === channel.id ? styles['active'] : ''}`}
                  onClick={() => {
                    setSelectedChannel(channel)
                    setShowMobileChannels(false)
                  }}
                >
                  <span className={styles['channel-mini-icon']}>○</span>
                  <span className={styles['channel-mini-name']}>
                    {channel.type === 'direct' ? (channel.recipient?.name || channel.name) : channel.name}
                  </span>
                  {channel.unread_count > 0 && (
                    <span className={styles['unread-badge']}>{channel.unread_count}</span>
                  )}
                </div>
              ))}
            </div>
          </>
        )}
        
        <button 
          className={styles['sidebar-toggle']}
          onClick={() => setChatSidebarCollapsed(!chatSidebarCollapsed)}
        >
          {chatSidebarCollapsed ? '→' : '←'}
        </button>
      </div>

      <div 
        className={styles['resize-handle']}
        onMouseDown={onMouseDownResize}
        style={{ cursor: isResizing ? 'col-resize' : undefined }}
      />
    </>
  )
}
