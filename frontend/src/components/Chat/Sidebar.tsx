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
        style={{ width: chatSidebarCollapsed ? 52 : chatSidebarWidth }}
      >
        {!chatSidebarCollapsed && (
          <>
            <div className="channels-panel-header">
              <h3>Obertrack</h3>
              <button className="new-msg-btn" onClick={() => setShowNewChannelModal(true)} title="Nuevo canal">
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                  <path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/><path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"/>
                </svg>
              </button>
            </div>

            <div className="channel-list-mini">
              <div className="channel-group-label">
                <span>Canales</span>
                <button className="add-btn-mini" onClick={() => setShowNewChannelModal(true)} title="Crear canal">
                  +
                </button>
              </div>
              
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

              <div className="channel-group-label">
                <span>Mensajes directos</span>
                <button className="add-btn-mini">+</button>
              </div>
              
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
