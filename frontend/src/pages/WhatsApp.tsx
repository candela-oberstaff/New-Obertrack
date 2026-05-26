import { useState } from 'react'
import { useAuth } from '../context/AuthContext'
import styles from './WhatsApp.module.css'
import Avatar from '../components/Common/Avatar'
import { Search, Phone, Video, MoreVertical, Paperclip, Smile, Mic, Send, CheckCheck } from 'lucide-react'

// ── Mock data ──────────────────────────────────────────────────────────────
interface Message {
  id: number
  text: string
  time: string
  own: boolean
  status: 'sent' | 'delivered' | 'read'
}

interface Contact {
  id: number
  name: string
  avatar?: string
  lastMessage: string
  time: string
  unread: number
  online: boolean
  messages: Message[]
}

const MOCK_CONTACTS: Contact[] = [
  {
    id: 1,
    name: 'Equipo Oberstaff',
    lastMessage: 'Reunión a las 15:00 hs ✅',
    time: '14:05',
    unread: 2,
    online: true,
    messages: [
      { id: 1, text: 'Buen día a todos 👋', time: '09:00', own: false, status: 'read' },
      { id: 2, text: '¿Confirman la reunión de las 15?', time: '09:15', own: false, status: 'read' },
      { id: 3, text: 'Sí, confirmado de mi parte', time: '09:20', own: true, status: 'read' },
      { id: 4, text: 'Igual aquí!', time: '09:22', own: true, status: 'read' },
      { id: 5, text: 'Reunión a las 15:00 hs ✅', time: '14:05', own: false, status: 'read' },
    ],
  },
  {
    id: 2,
    name: 'Lucía Fernández',
    lastMessage: 'Te mando el reporte en un momento',
    time: '13:42',
    unread: 0,
    online: true,
    messages: [
      { id: 1, text: 'Hola! ¿Terminaste el informe?', time: '13:30', own: true, status: 'read' },
      { id: 2, text: 'Casi listo, me faltan los gráficos', time: '13:35', own: false, status: 'read' },
      { id: 3, text: 'Te mando el reporte en un momento', time: '13:42', own: false, status: 'read' },
    ],
  },
  {
    id: 3,
    name: 'Martín Gómez',
    lastMessage: 'Ok perfecto, gracias!',
    time: '12:10',
    unread: 0,
    online: false,
    messages: [
      { id: 1, text: '¿Podés cubrir el turno del jueves?', time: '11:50', own: false, status: 'read' },
      { id: 2, text: 'Sí, sin problema', time: '12:05', own: true, status: 'read' },
      { id: 3, text: 'Ok perfecto, gracias!', time: '12:10', own: false, status: 'read' },
    ],
  },
  {
    id: 4,
    name: 'Soporte IT',
    lastMessage: 'El ticket fue resuelto',
    time: 'Ayer',
    unread: 1,
    online: false,
    messages: [
      { id: 1, text: 'Buen día, tengo un problema con mi acceso', time: 'Ayer 10:00', own: true, status: 'read' },
      { id: 2, text: 'Estamos revisando el caso', time: 'Ayer 10:30', own: false, status: 'read' },
      { id: 3, text: 'El ticket fue resuelto', time: 'Ayer 15:00', own: false, status: 'read' },
    ],
  },
  {
    id: 5,
    name: 'Ana Rodríguez',
    lastMessage: 'Che, ¿vieron el memo nuevo?',
    time: 'Ayer',
    unread: 0,
    online: false,
    messages: [
      { id: 1, text: 'Che, ¿vieron el memo nuevo?', time: 'Ayer 18:00', own: false, status: 'read' },
      { id: 2, text: 'Sí, ya lo leí', time: 'Ayer 18:05', own: true, status: 'read' },
    ],
  },
]

// ── Component ────────────────────────────────────────────────────────────────
export default function WhatsApp() {
  const { user } = useAuth()
  const [contacts] = useState<Contact[]>(MOCK_CONTACTS)
  const [activeContactId, setActiveContactId] = useState<number | null>(1)
  const [inputText, setInputText] = useState('')
  const [search, setSearch] = useState('')
  const [showMobileChat, setShowMobileChat] = useState(false)
  const [wahaStatus, setWahaStatus] = useState<{ status: string; qr?: { image: string } } | null>(null)

  useState(() => {
    // Fetch WAHA connection status
    fetch('/api/tickets/waha/status')
      .then(res => res.json())
      .then(data => setWahaStatus(data))
      .catch(err => console.error('Error fetching WAHA status:', err))
  })

  const activeContact = contacts.find(c => c.id === activeContactId) ?? null
  const filteredContacts = contacts.filter(c =>
    c.name.toLowerCase().includes(search.toLowerCase())
  )

  const handleSelectContact = (id: number) => {
    setActiveContactId(id)
    setShowMobileChat(true)
  }

  const handleBack = () => {
    setShowMobileChat(false)
  }

  const getInitials = (name: string) =>
    name.split(' ').map(w => w[0]).join('').toUpperCase().slice(0, 2)

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
              className={styles.statusDot} 
              style={{ 
                backgroundColor: wahaStatus?.status === 'CONNECTED' ? '#25D366' : '#EF4444',
                display: 'inline-block',
                width: '10px',
                height: '10px',
                borderRadius: '50%',
                alignSelf: 'center',
                marginRight: '8px'
              }}
              title={`Estado: ${wahaStatus?.status || 'Desconocido'}`}
            />
            <button className={styles.headerIconBtn} title="Nuevo chat">
              <svg viewBox="0 0 24 24" fill="currentColor" width="20" height="20">
                <path d="M1 9l11-7 11 7v11a2 2 0 0 1-2 2H3a2 2 0 0 1-2-2z" fill="none" stroke="currentColor" strokeWidth="1.5"/>
                <path d="M19 2H5a2 2 0 0 0-2 2v16l4-4h12a2 2 0 0 0 2-2V4a2 2 0 0 0-2-2zm-6 9h-2V9h2v2zm0-4h-2V5h2v2z" stroke="none" fill="currentColor" opacity="0"/>
                <path d="M12 3C8.13401 3 5 6.13401 5 10C5 13.866 8.13401 17 12 17C15.866 17 19 13.866 19 10C19 6.13401 15.866 3 12 3ZM12 15.5C8.96243 15.5 6.5 13.0376 6.5 10C6.5 6.96243 8.96243 4.5 12 4.5C15.0376 4.5 17.5 6.96243 17.5 10C17.5 13.0376 15.0376 15.5 12 15.5Z" opacity="0"/>
                {/* Pencil edit icon */}
                <path d="M20 2H4C2.9 2 2 2.9 2 4V22L6 18H20C21.1 18 22 17.1 22 16V4C22 2.9 21.1 2 20 2ZM20 16H5.17L4 17.17V4H20V16Z"/>
                <path d="M11 11H13V13H11V11ZM11 7H13V10H11V7Z" opacity="0.6"/>
                <rect x="10" y="5" width="4" height="1.5" rx="0.5"/>
                <rect x="10" y="8" width="4" height="1.5" rx="0.5"/>
                <rect x="10" y="11" width="2.5" height="1.5" rx="0.5"/>
              </svg>
            </button>
          </div>
        </div>

        {/* Dynamic connection banner */}
        <div 
          className={styles.mockBanner} 
          style={{ 
            background: wahaStatus?.status === 'CONNECTED' 
              ? 'rgba(37, 211, 102, 0.08)' 
              : 'rgba(239, 68, 68, 0.08)',
            borderBottom: wahaStatus?.status === 'CONNECTED'
              ? '1px solid rgba(37, 211, 102, 0.2)'
              : '1px solid rgba(239, 68, 68, 0.2)',
            color: wahaStatus?.status === 'CONNECTED' ? '#128C7E' : '#EF4444'
          }}
        >
          <span 
            className={styles.mockBadge}
            style={{
              background: wahaStatus?.status === 'CONNECTED'
                ? 'linear-gradient(135deg, #25D366, #128C7E)'
                : 'linear-gradient(135deg, #EF4444, #DC2626)'
            }}
          >
            {wahaStatus?.status || 'CARGANDO'}
          </span>
          <span>
            {wahaStatus?.status === 'CONNECTED' 
              ? 'WhatsApp Conectado Correctamente' 
              : 'Dispositivo Desconectado · Escanea el QR para conectar'}
          </span>
        </div>

        {/* QR scanner visual section */}
        {wahaStatus?.status !== 'CONNECTED' && wahaStatus?.qr?.image && (
          <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', padding: '20px', borderBottom: '1px solid #e9edef', gap: '10px' }}>
            <h4 style={{ margin: 0, fontSize: '14px', color: '#41525d' }}>Vincular dispositivo con QR</h4>
            <img 
              src={wahaStatus.qr.image} 
              alt="WAHA QR Code" 
              style={{ width: '180px', height: '180px', border: '1px solid #e9edef', padding: '5px', borderRadius: '4px' }} 
            />
            <p style={{ margin: 0, fontSize: '11px', color: '#667781', textAlign: 'center' }}>
              Abrí WhatsApp en tu teléfono &gt; Dispositivos vinculados &gt; Vincular un dispositivo
            </p>
          </div>
        )}

        {/* Search */}
        <div className={styles.searchBar}>
          <Search size={16} className={styles.searchIcon} />
          <input
            type="text"
            placeholder="Buscar o empezar un chat"
            className={styles.searchInput}
            value={search}
            onChange={e => setSearch(e.target.value)}
          />
        </div>

        {/* Contact list */}
        <ul className={styles.contactList}>
          {filteredContacts.map(contact => (
            <li
              key={contact.id}
              className={`${styles.contactItem} ${activeContactId === contact.id ? styles.contactItemActive : ''}`}
              onClick={() => handleSelectContact(contact.id)}
            >
              <div className={styles.contactAvatarWrap}>
                <div className={styles.contactAvatar}>
                  {getInitials(contact.name)}
                </div>
                {contact.online && <span className={styles.onlineDot} />}
              </div>
              <div className={styles.contactInfo}>
                <div className={styles.contactRow}>
                  <span className={styles.contactName}>{contact.name}</span>
                  <span className={styles.contactTime}>{contact.time}</span>
                </div>
                <div className={styles.contactRow}>
                  <span className={styles.contactLastMsg}>{contact.lastMessage}</span>
                  {contact.unread > 0 && (
                    <span className={styles.unreadBadge}>{contact.unread}</span>
                  )}
                </div>
              </div>
            </li>
          ))}
        </ul>
      </aside>

      {/* ── Chat area ───────────────────────────────────────── */}
      <main className={`${styles.chatArea} ${!showMobileChat ? styles.chatAreaHidden : ''}`}>
        {activeContact ? (
          <>
            {/* Chat header */}
            <div className={styles.chatHeader}>
              <button className={styles.backBtn} onClick={handleBack} aria-label="Volver">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" width="20" height="20">
                  <path d="M15 18l-6-6 6-6"/>
                </svg>
              </button>
              <div className={styles.chatHeaderAvatar}>
                {getInitials(activeContact.name)}
                {activeContact.online && <span className={styles.chatOnlineDot} />}
              </div>
              <div className={styles.chatHeaderInfo}>
                <span className={styles.chatHeaderName}>{activeContact.name}</span>
                <span className={styles.chatHeaderStatus}>
                  {activeContact.online ? 'en línea' : 'última vez hoy'}
                </span>
              </div>
              <div className={styles.chatHeaderActions}>
                <button className={styles.chatIconBtn} title="Llamada de voz"><Phone size={18} /></button>
                <button className={styles.chatIconBtn} title="Videollamada"><Video size={18} /></button>
                <button className={styles.chatIconBtn} title="Más opciones"><MoreVertical size={18} /></button>
              </div>
            </div>

            {/* Messages */}
            <div className={styles.messages}>
              {activeContact.messages.map(msg => (
                <div
                  key={msg.id}
                  className={`${styles.msgRow} ${msg.own ? styles.msgRowOwn : ''}`}
                >
                  <div className={`${styles.msgBubble} ${msg.own ? styles.msgBubbleOwn : ''}`}>
                    <p className={styles.msgText}>{msg.text}</p>
                    <div className={styles.msgMeta}>
                      <span className={styles.msgTime}>{msg.time}</span>
                      {msg.own && (
                        <CheckCheck
                          size={14}
                          className={`${styles.msgCheck} ${msg.status === 'read' ? styles.msgCheckRead : ''}`}
                        />
                      )}
                    </div>
                  </div>
                </div>
              ))}
            </div>

            {/* Input */}
            <div className={styles.inputBar}>
              <button className={styles.inputIconBtn} title="Emoji"><Smile size={22} /></button>
              <button className={styles.inputIconBtn} title="Adjuntar"><Paperclip size={22} /></button>
              <input
                type="text"
                placeholder="Escribe un mensaje"
                className={styles.textInput}
                value={inputText}
                onChange={e => setInputText(e.target.value)}
                onKeyDown={e => e.key === 'Enter' && setInputText('')}
              />
              {inputText.trim() ? (
                <button
                  className={`${styles.sendBtn} ${styles.sendBtnActive}`}
                  onClick={() => setInputText('')}
                  title="Enviar"
                >
                  <Send size={20} />
                </button>
              ) : (
                <button className={styles.sendBtn} title="Nota de voz">
                  <Mic size={20} />
                </button>
              )}
            </div>
          </>
        ) : (
          /* Empty state */
          <div className={styles.emptyState}>
            <div className={styles.emptyIcon}>
              <svg viewBox="0 0 64 64" fill="none" xmlns="http://www.w3.org/2000/svg" width="80" height="80">
                <circle cx="32" cy="32" r="32" fill="url(#wa-bg)" opacity="0.15"/>
                <path d="M32 10C19.85 10 10 19.85 10 32c0 3.9 1.05 7.56 2.89 10.71L10 54l11.56-2.83A21.85 21.85 0 0 0 32 54c12.15 0 22-9.85 22-22S44.15 10 32 10Z" fill="url(#wa-bg)"/>
                <path d="M26.5 23c-.5-1-1-1-1.5-1h-1.3c-.5 0-1.2.2-1.8.9-.6.7-2.4 2.3-2.4 5.6s2.5 6.5 2.8 7 4.9 7.5 11.8 10.5c1.6.7 2.9 1.1 3.8 1.4 1.6.5 3.1.4 4.2.3 1.3-.2 4-1.6 4.5-3.2.6-1.6.6-3 .4-3.2-.1-.2-.5-.4-1-.6z" fill="white" fillOpacity=".9"/>
                <defs>
                  <linearGradient id="wa-bg" x1="10" y1="10" x2="54" y2="54" gradientUnits="userSpaceOnUse">
                    <stop stopColor="#25D366"/>
                    <stop offset="1" stopColor="#128C7E"/>
                  </linearGradient>
                </defs>
              </svg>
            </div>
            <h2 className={styles.emptyTitle}>WhatsApp Web</h2>
            <p className={styles.emptyDesc}>Seleccioná un chat para comenzar a chatear</p>
          </div>
        )}
      </main>
    </div>
  )
}
