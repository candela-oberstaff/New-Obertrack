import type { ReactNode } from 'react'
import { LifeBuoy, Inbox } from 'lucide-react'
import { Channel, SupportTicket } from '../../types/chat'
import { isSupportChannel, supportLabel, supportStatusMeta, dmContactName } from './ChatUtils'
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
  fetchAllUsers: () => Promise<void>
  onMouseDownResize: (e: React.MouseEvent) => void
  isResizing: boolean
  // Optional content rendered under the workspace header (e.g. superadmin company selector)
  headerExtra?: ReactNode
  userStatuses: Map<number, 'online' | 'away' | 'offline'>
  // Cola de soporte (solo agentes): solicitudes sin asignar para aceptar.
  isSupportAgent?: boolean
  pendingSupport?: SupportTicket[]
  onAcceptSupport?: (channelId: number) => void
  supportBusy?: boolean
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
  fetchAllUsers,
  onMouseDownResize,
  isResizing,
  headerExtra,
  userStatuses,
  isSupportAgent,
  pendingSupport,
  onAcceptSupport,
  supportBusy,
}: SidebarProps) {
  // Los canales pendientes (sin asignar) se muestran en "Pendientes", no en la lista.
  const pendingIds = new Set((pendingSupport || []).map(t => t.channel_id))
  const supportChannels = channels.filter(c => isSupportChannel(c) && !pendingIds.has(c.id))
  const activeChannels = channels.filter(c => (c.type === 'public' || c.type === 'private') && !isSupportChannel(c))
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

            {headerExtra && (
              <div style={{ padding: '8px 12px' }}>{headerExtra}</div>
            )}

            <div className={styles['channel-list-mini']}>
              {isSupportAgent && pendingSupport && pendingSupport.length > 0 && (
                <>
                  <div className={styles['channel-group-label']}>
                    <span style={{ display: 'inline-flex', alignItems: 'center', gap: 6, color: '#b45309' }}>
                      <Inbox size={13} /> Pendientes
                    </span>
                    <span style={{ fontSize: 11, fontWeight: 700, color: '#b45309', background: 'rgba(245,158,11,0.14)', borderRadius: 999, padding: '1px 7px' }}>
                      {pendingSupport.length}
                    </span>
                  </div>

                  {pendingSupport.map(ticket => (
                    <div
                      key={ticket.channel_id}
                      className={styles['channel-mini-item']}
                      style={{ borderLeft: '3px solid #f59e0b', background: 'rgba(245,158,11,0.06)', alignItems: 'center' }}
                    >
                      <span style={{ display: 'flex', flexDirection: 'column', gap: 2, minWidth: 0, flex: 1 }}>
                        <span className={styles['channel-mini-name']}>
                          {ticket.requester?.name || `Solicitud #${ticket.channel_id}`}
                        </span>
                        <span style={{ fontSize: 11, color: '#b45309' }}>Solicita soporte</span>
                      </span>
                      <button
                        disabled={supportBusy}
                        onClick={() => onAcceptSupport?.(ticket.channel_id)}
                        title="Aceptar y atender"
                        style={{
                          border: 'none', background: '#7c3aed', color: '#fff', borderRadius: 7,
                          padding: '5px 12px', fontSize: 12, fontWeight: 700,
                          cursor: supportBusy ? 'wait' : 'pointer', whiteSpace: 'nowrap', width: 'auto',
                        }}
                      >
                        Aceptar
                      </button>
                    </div>
                  ))}
                </>
              )}

              {supportChannels.length > 0 && (
                <>
                  <div className={styles['channel-group-label']}>
                    <span style={{ display: 'inline-flex', alignItems: 'center', gap: 6, color: '#7c3aed' }}>
                      <LifeBuoy size={13} /> Soporte
                    </span>
                  </div>

                  {supportChannels.map(channel => (
                    <div
                      key={channel.id}
                      className={`${styles['channel-mini-item']} ${selectedChannel?.id === channel.id ? styles['active'] : ''}`}
                      onClick={() => {
                        setSelectedChannel(channel)
                        setShowMobileChannels(false)
                      }}
                      style={{
                        borderLeft: '3px solid #7c3aed',
                        background: selectedChannel?.id === channel.id ? undefined : 'rgba(124,58,237,0.06)',
                      }}
                    >
                      <span className={styles['channel-mini-icon']} style={{ color: '#7c3aed', alignSelf: 'flex-start', marginTop: 2 }}>
                        <LifeBuoy size={14} />
                      </span>
                      <span style={{ display: 'flex', flexDirection: 'column', gap: 2, minWidth: 0, flex: 1 }}>
                        <span className={styles['channel-mini-name']}>{supportLabel(channel.name)}</span>
                        {channel.support && (
                          <span style={{ fontSize: 11, color: supportStatusMeta(channel.support.status).color, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>
                            {channel.support.status === 'resolved'
                              ? '✓ Resuelto'
                              : channel.support.assignee_name
                                ? `Atendido por ${channel.support.assignee_name}`
                                : (isSupportAgent ? 'Sin asignar' : 'En cola')}
                          </span>
                        )}
                      </span>
                      {channel.unread_count > 0 && (
                        <span className={styles['unread-badge']}>{channel.unread_count}</span>
                      )}
                    </div>
                  ))}
                </>
              )}

              <div className={styles['channel-group-label']}>
                <span>Canales</span>
                <button className={styles['add-btn-mini']} onClick={() => setShowNewChannelModal(true)} title="Crear canal">
                  +
                </button>
              </div>
              
              {activeChannels.map(channel => (
                <div
                  key={channel.id}
                  className={`${styles['channel-mini-item']} ${selectedChannel?.id === channel.id ? styles['active'] : ''}`}
                  onClick={() => {
                    setSelectedChannel(channel)
                    setShowMobileChannels(false)
                  }}
                >
                  <span className={styles['channel-mini-icon']}>
                    {channel.type === 'private' ? '🔒' : '#'}
                  </span>
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
                  onClick={() => { fetchAllUsers(); setShowNewDmModal(true) }}
                  title="Nuevo mensaje directo"
                >
                  +
                </button>
              </div>
              
              {directMessages.map(channel => {
                const status = channel.recipient ? (userStatuses.get(channel.recipient.id) || 'offline') : 'offline'
                return (
                <div
                  key={channel.id}
                  className={`${styles['channel-mini-item']} ${selectedChannel?.id === channel.id ? styles['active'] : ''}`}
                  onClick={() => {
                    setSelectedChannel(channel)
                    setShowMobileChannels(false)
                  }}
                >
                  <span
                    className={`${styles['status-dot']} ${styles[status]}`}
                    title={status === 'online' ? 'En línea' : status === 'away' ? 'Ausente' : 'Desconectado'}
                  />
                  <span className={styles['channel-mini-name']}>
                    {channel.type === 'direct' ? dmContactName(channel) : channel.name}
                  </span>
                  {channel.unread_count > 0 && (
                    <span className={styles['unread-badge']}>{channel.unread_count}</span>
                  )}
                </div>
                )
              })}
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
