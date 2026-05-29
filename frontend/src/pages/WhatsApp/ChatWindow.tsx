import React from 'react'
import { Send, CheckCheck } from 'lucide-react'
import { ticketService, Ticket, TicketMessage } from '../../services/ticket.service'
import styles from '../WhatsApp.module.css'

interface ChatWindowProps {
  activeTicket: Ticket
  setActiveTicket: React.Dispatch<React.SetStateAction<Ticket | null>>
  tickets: Ticket[]
  setTickets: React.Dispatch<React.SetStateAction<Ticket[]>>
  loadingMessages: boolean
  inputText: string
  setInputText: (val: string) => void
  sending: boolean
  handleSend: () => void
  handleBack: () => void
  showMobileChat: boolean
  messagesEndRef: React.RefObject<HTMLDivElement | null>
  getInitials: (name: string) => string
  formatTime: (iso: string) => string
}

export default function ChatWindow({
  activeTicket,
  setActiveTicket,
  tickets,
  setTickets,
  loadingMessages,
  inputText,
  setInputText,
  sending,
  handleSend,
  handleBack,
  showMobileChat,
  messagesEndRef,
  getInitials,
  formatTime
}: ChatWindowProps) {
  return (
    <main className={`${styles.chatArea} ${!showMobileChat ? styles.chatAreaHidden : ''}`}>
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
            {activeTicket.contact?.company_name && (
              <span style={{ fontSize: '12px', color: '#128C7E', marginLeft: '8px', background: 'rgba(18,140,126,0.1)', padding: '2px 8px', borderRadius: '4px' }}>
                {activeTicket.contact.company_name}
              </span>
            )}
            {activeTicket.contact?.parent_contact && (
              <span style={{ fontSize: '12px', color: '#41525d', marginLeft: '8px', background: 'rgba(0,0,0,0.05)', padding: '2px 8px', borderRadius: '4px' }}>
                Asociado a: {activeTicket.contact.parent_contact.name}
              </span>
            )}
          </span>
          <span className={styles.chatHeaderStatus} style={{ color: '#667781' }}>
            {activeTicket.contact?.phone
              ? `+${activeTicket.contact.phone.replace(/^\+/, '')}`
              : activeTicket.title}
          </span>
        </div>
        <div className={styles.chatHeaderActions} style={{ display: 'flex', gap: '8px', alignItems: 'center' }}>
          <select
            value={activeTicket.contact?.parent_contact_id || ''}
            onChange={async (e) => {
              const val = e.target.value
              const parentId = val ? parseInt(val) : null
              if (activeTicket.contact) {
                const updated = await ticketService.updateContact(activeTicket.contact.id, {
                  parent_contact_id: parentId
                })
                setActiveTicket(prev => prev ? {
                  ...prev,
                  contact: { ...prev.contact, ...updated }
                } : null)
                // Update in tickets list too
                setTickets(prev => prev.map(t => t.id === activeTicket.id ? { ...t, contact: { ...t.contact, ...updated } } : t))
              }
            }}
            style={{
              fontSize: '11px',
              padding: '4px 8px',
              borderRadius: '4px',
              border: '1px solid #ccc',
              background: '#f8f9fa',
              color: '#41525d',
              cursor: 'pointer'
            }}
          >
            <option value="">-- Vincular Empresa / Contacto Principal --</option>
            {tickets
              .filter(t => t.contact && t.contact.id !== activeTicket.contact?.id && !t.contact.parent_contact_id)
              .map(t => (
                <option key={t.contact!.id} value={t.contact!.id}>
                  {t.contact!.company_name ? `${t.contact!.company_name} (${t.contact!.name})` : t.contact!.name}
                </option>
              ))}
          </select>
          <button
            onClick={async () => {
              const newName = prompt("Editar Nombre del Contacto:", activeTicket.contact?.name || '')
              const newCompany = prompt("Editar Nombre de Empresa:", activeTicket.contact?.company_name || '')
              if (activeTicket.contact && (newName !== null || newCompany !== null)) {
                try {
                  const updated = await ticketService.updateContact(activeTicket.contact.id, {
                    name: newName || activeTicket.contact.name,
                    company_name: newCompany !== null ? newCompany : activeTicket.contact.company_name
                  })
                  setActiveTicket(prev => prev ? {
                    ...prev,
                    contact: { ...prev.contact, ...updated }
                  } : null)
                  setTickets(prev => prev.map(t => t.contact?.id === activeTicket.contact?.id ? { ...t, contact: { ...t.contact, ...updated } } : t))
                } catch (err) {
                  console.error('Error updating contact:', err)
                  alert('No se pudo guardar la edición del contacto.')
                }
              }
            }}
            style={{
              fontSize: '11px',
              padding: '4px 8px',
              borderRadius: '4px',
              border: '1px solid #128C7E',
              background: '#128C7E',
              color: 'white',
              cursor: 'pointer',
              fontWeight: 'bold'
            }}
          >
            Editar
          </button>
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
    </main>
  )
}
