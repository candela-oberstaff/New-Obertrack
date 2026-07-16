import Avatar from '../../components/Common/Avatar'
import { Search, Users, User } from 'lucide-react'
import { WhatsAppChatTicket } from '../../services/ticket.service'
import WahaStatus from './WahaStatus'
import styles from '../WhatsApp.module.css'

interface ChatListProps {
  user: any
  tickets: WhatsAppChatTicket[]
  activeTicket: WhatsAppChatTicket | null
  loadingTickets: boolean
  search: string
  setSearch: (val: string) => void
  handleSelectTicket: (ticket: WhatsAppChatTicket) => void
  showMobileChat: boolean
  filteredTickets: WhatsAppChatTicket[]
  formatTime: (iso: string) => string
  getInitials: (name: string) => string
  activeTab: 'me' | 'unassigned'
  setActiveTab: (tab: 'me' | 'unassigned') => void
  myChatsCount: number
  unassignedChatsCount: number
  displayName: (ticket: WhatsAppChatTicket) => string
}

export default function ChatList({
  user,
  activeTicket,
  loadingTickets,
  search,
  setSearch,
  handleSelectTicket,
  showMobileChat,
  filteredTickets,
  formatTime,
  getInitials,
  activeTab,
  setActiveTab,
  myChatsCount,
  unassignedChatsCount,
  displayName
}: ChatListProps) {

  const handleTabClick = (tab: 'me' | 'unassigned') => {
    setActiveTab(tab)
    setSearch('')
  }

  return (
    <aside className={`${styles.sidebar} ${showMobileChat ? styles.sidebarHidden : ''}`}>
      <div className={styles.sidebarHeader}>
        <div className={styles.sidebarHeaderLeft}>
          <Avatar src={user?.avatar} name={user?.name} size="sm" />
          <span className={styles.sidebarTitle}>WhatsApp</span>
        </div>
      </div>

      <WahaStatus />

      <div style={{ display: 'flex', borderBottom: '1px solid #e9edef', flexShrink: 0 }}>
        <button
          onClick={() => handleTabClick('me')}
          style={{
            flex: 1,
            padding: '12px 8px',
            border: 'none',
            background: 'transparent',
            cursor: 'pointer',
            fontSize: '14px',
            fontWeight: activeTab === 'me' ? 600 : 400,
            color: activeTab === 'me' ? '#128C7E' : '#54656f',
            borderBottom: activeTab === 'me' ? '2px solid #128C7E' : '2px solid transparent',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            gap: '6px',
            transition: 'all 0.15s',
            fontFamily: 'inherit'
          }}
        >
          <User size={14} />
          Mis Chats
          {myChatsCount > 0 && (
            <span style={{
              background: '#25D366',
              color: 'white',
              fontSize: '10px',
              fontWeight: 700,
              minWidth: '18px',
              height: '18px',
              borderRadius: '9px',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              padding: '0 4px'
            }}>
              {myChatsCount}
            </span>
          )}
        </button>
        <button
          onClick={() => handleTabClick('unassigned')}
          style={{
            flex: 1,
            padding: '12px 8px',
            border: 'none',
            background: 'transparent',
            cursor: 'pointer',
            fontSize: '14px',
            fontWeight: activeTab === 'unassigned' ? 600 : 400,
            color: activeTab === 'unassigned' ? '#128C7E' : '#54656f',
            borderBottom: activeTab === 'unassigned' ? '2px solid #128C7E' : '2px solid transparent',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            gap: '6px',
            transition: 'all 0.15s',
            fontFamily: 'inherit'
          }}
        >
          <Users size={14} />
          Sin Asignar
          {unassignedChatsCount > 0 && (
            <span style={{
              background: '#EF4444',
              color: 'white',
              fontSize: '10px',
              fontWeight: 700,
              minWidth: '18px',
              height: '18px',
              borderRadius: '9px',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              padding: '0 4px'
            }}>
              {unassignedChatsCount}
            </span>
          )}
        </button>
      </div>

      <div className={styles.searchBar}>
        <Search size={16} className={styles.searchIcon} />
        <input
          type="text"
          placeholder="Buscar contacto..."
          className={styles.searchInput}
          value={search}
          onChange={e => setSearch(e.target.value)}
        />
      </div>

      <ul className={styles.contactList}>
        {loadingTickets ? (
          <li style={{ padding: '20px', textAlign: 'center', color: '#667781', fontSize: '14px' }}>
            Cargando chats...
          </li>
        ) : filteredTickets.length === 0 ? (
          <li style={{ padding: '20px', textAlign: 'center', color: '#667781', fontSize: '14px' }}>
            {search ? 'Sin resultados' : (activeTab === 'unassigned' ? 'No hay chats sin asignar' : 'No hay conversaciones aún')}
          </li>
        ) : (
          filteredTickets.map(ticket => (
            <li
              key={ticket.zoho_id}
              className={`${styles.contactItem} ${activeTicket?.zoho_id === ticket.zoho_id ? styles.contactItemActive : ''}`}
              onClick={() => handleSelectTicket(ticket)}
            >
              <div className={styles.contactAvatarWrap}>
                <div className={styles.contactAvatar}>
                  {getInitials(displayName(ticket))}
                </div>
              </div>
              <div className={styles.contactInfo}>
                <div className={styles.contactRow}>
                  <span className={styles.contactName}>
                    {displayName(ticket)}
                    {ticket.contact_phone && (
                      <span style={{ fontSize: '10px', color: '#128C7E', marginLeft: '6px', background: 'rgba(18,140,126,0.1)', padding: '2px 6px', borderRadius: '4px', fontWeight: 'normal' }}>
                        {ticket.contact_phone}
                      </span>
                    )}
                  </span>
                  <span className={styles.contactTime}>{formatTime(ticket.modified_time)}</span>
                </div>
                <div className={styles.contactRow}>
                  <span className={styles.contactLastMsg}>
                    {ticket.subject || 'Chat de WhatsApp'}
                  </span>
                  {ticket.status === 'open' && (
                    <span className={styles.unreadBadge} style={{ background: '#25D366' }}>•</span>
                  )}
                </div>
              </div>
            </li>
          ))
        )}
      </ul>
    </aside>
  )
}
