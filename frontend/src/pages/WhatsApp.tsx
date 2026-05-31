import { useState, useEffect, useRef } from 'react'
import { useAuth } from '../context/AuthContext'
import styles from './WhatsApp.module.css'
import Avatar from '../components/Common/Avatar'
import { Search, MoreVertical, Send, CheckCheck } from 'lucide-react'
import { ticketService, Ticket, TicketMessage } from '../services/ticket.service'

// ── Helpers ─────────────────────────────────────────────────────────────────
const isConnected = (status?: string) =>
  status === 'CONNECTED' || status === 'WORKING'

const formatTime = (iso: string) => {
  const d = new Date(iso)
  const now = new Date()
  const isToday = d.toDateString() === now.toDateString()
  const yesterday = new Date(now)
  yesterday.setDate(now.getDate() - 1)
  const isYesterday = d.toDateString() === yesterday.toDateString()
  if (isToday) return d.toLocaleTimeString('es-AR', { hour: '2-digit', minute: '2-digit' })
  if (isYesterday) return 'Ayer'
  return d.toLocaleDateString('es-AR', { day: '2-digit', month: '2-digit' })
}

const getInitials = (name: string) =>
  name.split(' ').map(w => w[0]).join('').toUpperCase().slice(0, 2)

// ── Component ────────────────────────────────────────────────────────────────
export default function WhatsApp() {
  const { user } = useAuth()
  const [tickets, setTickets] = useState<Ticket[]>([])
  const [activeTicket, setActiveTicket] = useState<Ticket | null>(null)
  const [loadingTickets, setLoadingTickets] = useState(true)
  const [loadingMessages, setLoadingMessages] = useState(false)
  const [inputText, setInputText] = useState('')
  const [sending, setSending] = useState(false)
  const [search, setSearch] = useState('')
  const [showMobileChat, setShowMobileChat] = useState(false)
  const [wahaStatus, setWahaStatus] = useState<{ status: string; qr?: { image: string } } | null>(null)
  const messagesEndRef = useRef<HTMLDivElement>(null)

  // Fetch WAHA status
  useEffect(() => {
    ticketService.getWahaStatus()
      .then(data => setWahaStatus(data))
      .catch(err => console.error('Error fetching WAHA status:', err))
  }, [])

  // Fetch tickets list
  useEffect(() => {
    setLoadingTickets(true)
    ticketService.getTickets()
      .then(data => {
        // Only show WhatsApp tickets (that have at least one WA message or any open ticket)
        setTickets(data)
      })
      .catch(err => console.error('Error fetching tickets:', err))
      .finally(() => setLoadingTickets(false))
  }, [])

  // Scroll to bottom on new messages
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [activeTicket?.messages])

  const handleSelectTicket = async (ticket: Ticket) => {
    setShowMobileChat(true)
    setLoadingMessages(true)
    try {
      const full = await ticketService.getTicket(ticket.id)
      setActiveTicket(full)
    } catch (err) {
      console.error('Error fetching ticket detail:', err)
    } finally {
      setLoadingMessages(false)
    }
  }

  const handleBack = () => {
    setShowMobileChat(false)
    setActiveTicket(null)
  }

  const handleSend = async () => {
    if (!inputText.trim() || !activeTicket || sending) return
    const text = inputText.trim()
    setInputText('')
    setSending(true)
    try {
      const newMsg = await ticketService.sendMessage(activeTicket.id, text, 'whatsapp')
      setActiveTicket(prev => prev ? {
        ...prev,
        messages: [...(prev.messages ?? []), newMsg]
      } : null)
    } catch (err) {
      console.error('Error sending message:', err)
      setInputText(text) // restore on error
    } finally {
      setSending(false)
    }
  }

  // Get last WA message for sidebar preview
  const lastMsg = (ticket: Ticket) => {
    const msgs = ticket.messages ?? []
    if (msgs.length === 0) return 'Sin mensajes'
    const last = msgs[msgs.length - 1]
    return last.content
  }

  const filteredTickets = tickets.filter(t =>
    (t.contact?.name ?? '').toLowerCase().includes(search.toLowerCase()) ||
    (t.contact?.phone ?? '').includes(search)
  )

  return (
    <div className={styles.page}>
      {/* ── Sidebar ─────────────────────────────────────────── */}
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
                    <span className={styles.contactName}>{ticket.contact?.name ?? ticket.title}</span>
                    <span className={styles.contactTime}>{formatTime(ticket.updated_at)}</span>
                  </div>
                  <div className={styles.contactRow}>
                    <span className={styles.contactLastMsg}>{lastMsg(ticket)}</span>
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

      {/* ── Chat area ───────────────────────────────────────── */}
      <main className={`${styles.chatArea} ${!showMobileChat ? styles.chatAreaHidden : ''}`}>
        {activeTicket ? (
          <>
            {/* Chat header */}
            <div className={styles.chatHeader}>
              <button className={styles.backBtn} onClick={handleBack} aria-label="Volver">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" width="20" height="20">
                  <path d="M15 18l-6-6 6-6"/>
                </svg>
              </button>
              <div className={styles.chatHeaderAvatar}>
                {getInitials(activeTicket.contact?.name ?? activeTicket.title)}
              </div>
              <div className={styles.chatHeaderInfo}>
                <span className={styles.chatHeaderName}>
                  {activeTicket.contact?.name ?? activeTicket.title}
                </span>
                <span className={styles.chatHeaderStatus} style={{ color: '#667781' }}>
                  {activeTicket.contact?.phone
                    ? `+${activeTicket.contact.phone.replace(/^\+/, '')}`
                    : activeTicket.title}
                </span>
              </div>
              <div className={styles.chatHeaderActions}>
                <button className={styles.chatIconBtn} title="Más opciones"><MoreVertical size={18} /></button>
              </div>
            </div>

            {/* Messages */}
            <div className={styles.messages}>
              {loadingMessages ? (
                <div style={{ margin: 'auto', color: '#667781', fontSize: '14px' }}>Cargando mensajes...</div>
              ) : (activeTicket.messages ?? []).length === 0 ? (
                <div style={{ margin: 'auto', color: '#667781', fontSize: '14px' }}>No hay mensajes aún</div>
              ) : (
                (activeTicket.messages ?? []).map((msg: TicketMessage) => {
                  const isOwn = msg.sender_type === 'agent'
                  return (
                    <div
                      key={msg.id}
                      className={`${styles.msgRow} ${isOwn ? styles.msgRowOwn : ''}`}
                    >
                      <div className={`${styles.msgBubble} ${isOwn ? styles.msgBubbleOwn : ''}`}>
                        <p className={styles.msgText}>{msg.content}</p>
                        <div className={styles.msgMeta}>
                          <span className={styles.msgTime}>{formatTime(msg.created_at)}</span>
                          {isOwn && (
                            <CheckCheck size={14} className={`${styles.msgCheck} ${styles.msgCheckRead}`} />
                          )}
                        </div>
                      </div>
                    </div>
                  )
                })
              )}
              <div ref={messagesEndRef} />
            </div>

            {/* Input */}
            <div className={styles.inputBar}>
              <input
                type="text"
                placeholder="Escribe un mensaje"
                className={styles.textInput}
                value={inputText}
                onChange={e => setInputText(e.target.value)}
                onKeyDown={e => e.key === 'Enter' && handleSend()}
                disabled={sending}
              />
              <button
                className={`${styles.sendBtn} ${inputText.trim() ? styles.sendBtnActive : ''}`}
                onClick={handleSend}
                title="Enviar"
                disabled={!inputText.trim() || sending}
              >
                <Send size={20} />
              </button>
            </div>
          </>
        ) : (
          /* Empty state */
          <div className={styles.emptyState}>
            <div className={styles.emptyIcon}>
              <svg viewBox="0 0 64 64" fill="none" xmlns="http://www.w3.org/2000/svg" width="80" height="80">
                <circle cx="32" cy="32" r="32" fill="url(#wa-bg)" opacity="0.15"/>
                <path d="M32 10C19.85 10 10 19.85 10 32c0 3.9 1.05 7.56 2.89 10.71L10 54l11.56-2.83A21.85 21.85 0 0 0 32 54c12.15 0 22-9.85 22-22S44.15 10 32 10Z" fill="url(#wa-bg)"/>
                <defs>
                  <linearGradient id="wa-bg" x1="10" y1="10" x2="54" y2="54" gradientUnits="userSpaceOnUse">
                    <stop stopColor="#25D366"/>
                    <stop offset="1" stopColor="#128C7E"/>
                  </linearGradient>
                </defs>
              </svg>
            </div>
            <h2 className={styles.emptyTitle}>WhatsApp Web</h2>
            <p className={styles.emptyDesc}>Seleccioná una conversación para ver los mensajes</p>
          </div>
        )}
      </main>
    </div>
  )
}
