import React from 'react'
import { Channel } from '../../types/chat'

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
  onMouseDownResize,
  isResizing
}: SidebarProps) {
  const publicChannels = channels.filter(c => c.type === 'public')
  const privateChannels = channels.filter(c => c.type === 'private')

  return (
    <>
      <div 
        className={`channels-panel ${showMobileChannels ? 'open' : ''} ${chatSidebarCollapsed ? 'collapsed' : ''}`}
        style={{ width: chatSidebarCollapsed ? 40 : chatSidebarWidth }}
      >
        {!chatSidebarCollapsed && (
          <>
            <div className="channels-panel-header">
              <h3>Canales</h3>
              <button onClick={() => setShowNewChannelModal(true)}>+</button>
            </div>

            <div className="channel-list-mini">
              <div className="channel-group-label">Públicos</div>
              {publicChannels.map(channel => (
                <div
                  key={channel.id}
                  className={`channel-mini-item ${selectedChannel?.id === channel.id ? 'active' : ''}`}
                  onClick={() => {
                    setSelectedChannel(channel)
                    setShowMobileChannels(false)
                  }}
                >
                  <span className="channel-mini-icon">#</span>
                  <span className="channel-mini-name">{channel.name}</span>
                  {channel.unread_count > 0 && (
                    <span className="unread-badge">{channel.unread_count}</span>
                  )}
                </div>
              ))}

              <div className="channel-group-label">Privados</div>
              {privateChannels.map(channel => (
                <div
                  key={channel.id}
                  className={`channel-mini-item ${selectedChannel?.id === channel.id ? 'active' : ''}`}
                  onClick={() => {
                    setSelectedChannel(channel)
                    setShowMobileChannels(false)
                  }}
                >
                  <span className="channel-mini-icon">○</span>
                  <span className="channel-mini-name">{channel.name}</span>
                  {channel.unread_count > 0 && (
                    <span className="unread-badge">{channel.unread_count}</span>
                  )}
                </div>
              ))}
            </div>
          </>
        )}
        
        <button 
          className="sidebar-toggle"
          onClick={() => setChatSidebarCollapsed(!chatSidebarCollapsed)}
        >
          {chatSidebarCollapsed ? '→' : '←'}
        </button>
      </div>

      <div 
        className="resize-handle"
        onMouseDown={onMouseDownResize}
        style={{ cursor: isResizing ? 'col-resize' : undefined }}
      />
    </>
  )
}
