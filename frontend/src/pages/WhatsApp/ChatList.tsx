import Avatar from '../../components/Common/Avatar'
import { Search } from 'lucide-react'
import { Ticket } from '../../services/ticket.service'
import styles from '../WhatsApp.module.css'

interface ChatListProps {
  user: any
  tickets: Ticket[]
  activeTicket: Ticket | null
  loadingTickets: boolean
  wahaStatus: { status: string; qr?: { image: string } } | null
  search: string
  setSearch: (val: string) => void
  handleSelectTicket: (ticket: Ticket) => void
  showMobileChat: boolean
  filteredTickets: Ticket[]
  lastMsg: (ticket: Ticket) => string
  formatTime: (iso: string) => string
  getInitials: (name: string) => string
  isConnected: (status?: string) => boolean
}

export default function ChatList({
  user,
  activeTicket,
  loadingTickets,
  wahaStatus,
  search,
  setSearch,
  handleSelectTicket,
  showMobileChat,
  filteredTickets,
  lastMsg,
  formatTime,
  getInitials,
  isConnected
}: ChatListProps) {
  return (
    <aside className={`${styles.sidebar} ${showMobileChat ? styles.sidebarHidden : ''}`}>
      {/* Header */}
      <div className={styles.sidebarHeader}>
        <div className={styles.sidebarHeaderLeft}>
          <Avatar src={user?.avatar} name={user?.name} size="sm" />
          <span className={styles.sidebarTitle}>WhatsApp</span>
        </div>
        <div className={styles.sidebarHeaderActions}>
          <span
            style={{
              backgroundColor: isConnected(wahaStatus?.status ?? '') ? '#25D366' : '#EF4444',
              display: 'inline-block',
              width: '10px',
              height: '10px',
              borderRadius: '50%',
              alignSelf: 'center',
              marginRight: '8px',
              flexShrink: 0
            }}
            title={`Estado: ${wahaStatus?.status ?? 'Desconocido'}`}
          />
        </div>
      </div>

      {/* Connection banner */}
      <div
        className={styles.mockBanner}
        style={{
          background: isConnected(wahaStatus?.status ?? '') ? 'rgba(37,211,102,0.08)' : 'rgba(239,68,68,0.08)',
          borderBottom: isConnected(wahaStatus?.status ?? '') ? '1px solid rgba(37,211,102,0.2)' : '1px solid rgba(239,68,68,0.2)',
          color: isConnected(wahaStatus?.status ?? '') ? '#128C7E' : '#EF4444'
        }}
      >
        <span
          className={styles.mockBadge}
          style={{
            background: isConnected(wahaStatus?.status ?? '')
              ? 'linear-gradient(135deg,#25D366,#128C7E)'
              : 'linear-gradient(135deg,#EF4444,#DC2626)'
          }}
        >
          {wahaStatus?.status ?? 'CARGANDO'}
        </span>
        <span>
          {isConnected(wahaStatus?.status ?? '')
            ? 'WhatsApp Conectado'
            : 'Desconectado · Escanea el QR para conectar'}
        </span>
      </div>

      {/* QR code when disconnected */}
      {!isConnected(wahaStatus?.status ?? '') && wahaStatus?.qr?.image && (
        <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', padding: '20px', borderBottom: '1px solid #e9edef', gap: '10px' }}>
          <h4 style={{ margin: 0, fontSize: '14px', color: '#41525d' }}>Vincular dispositivo</h4>
          <img
            src={wahaStatus.qr.image}
            alt="WAHA QR Code"
            style={{ width: '180px', height: '180px', border: '1px solid #e9edef', padding: '5px', borderRadius: '4px' }}
          />
          <p style={{ margin: 0, fontSize: '11px', color: '#667781', textAlign: 'center' }}>
            WhatsApp {'>'} Dispositivos vinculados {'>'} Vincular un dispositivo
          </p>
        </div>
      )}

      {/* Search */}
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

      {/* Ticket / contact list */}
      <ul className={styles.contactList}>
        {loadingTickets ? (
          <li style={{ padding: '20px', textAlign: 'center', color: '#667781', fontSize: '14px' }}>
            Cargando chats...
          </li>
        ) : filteredTickets.length === 0 ? (
          <li style={{ padding: '20px', textAlign: 'center', color: '#667781', fontSize: '14px' }}>
            No hay conversaciones aún
          </li>
        ) : (
          filteredTickets.map(ticket => (
            <li
              key={ticket.id}
              className={`${styles.contactItem} ${activeTicket?.id === ticket.id ? styles.contactItemActive : ''}`}
              onClick={() => handleSelectTicket(ticket)}
            >
              <div className={styles.contactAvatarWrap}>
                <div className={styles.contactAvatar}>
                  {getInitials(ticket.contact?.name ?? ticket.title)}
                </div>
              </div>
              <div className={styles.contactInfo}>
                <div className={styles.contactRow}>
                  <span className={styles.contactName}>
                    {ticket.contact?.name ?? ticket.title}
                    {ticket.contact?.company_name && (
                      <span style={{ fontSize: '10px', color: '#128C7E', marginLeft: '6px', background: 'rgba(18,140,126,0.1)', padding: '2px 6px', borderRadius: '4px', fontWeight: 'normal' }}>
                        {ticket.contact.company_name}
                      </span>
                    )}
                  </span>
                  <span className={styles.contactTime}>{formatTime(ticket.updated_at)}</span>
                </div>
                <div className={styles.contactRow}>
                  <span className={styles.contactLastMsg}>
                    {ticket.contact?.parent_contact && `[Empresa: ${ticket.contact.parent_contact.name}] `}
                    {lastMsg(ticket)}
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
