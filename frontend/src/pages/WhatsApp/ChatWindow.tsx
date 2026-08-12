import React, { useState } from 'react'
import { Send, CheckCheck, Clock, AlertCircle, UserRoundPlus, Download, Loader2 } from 'lucide-react'
import { ticketService, WhatsAppMessageDTO } from '../../services/ticket.service'
import styles from '../WhatsApp.module.css'

// Estado de entrega de un mensaje propio. Los mensajes previos al outbox llegan
// sin delivery_status, así que la ausencia de valor se trata como entregado.
function DeliveryIcon({ status }: { status?: string }) {
  if (status === 'pending') {
    return <Clock size={13} className={styles.msgCheck} aria-label="Enviando…" />
  }
  if (status === 'failed') {
    return <AlertCircle size={13} className={styles.msgCheck} color="#B91C1C" aria-label="No entregado" />
  }
  return <CheckCheck size={14} className={`${styles.msgCheck} ${styles.msgCheckRead}`} aria-label="Entregado" />
}

interface ChatWindowProps {
  activeTicket: {
    zoho_id: string
    contact_name?: string
    contact_phone?: string
    subject?: string
    assignee_id?: string
    department_id?: string
  }
  activeMessages: WhatsAppMessageDTO[]
  loadingMessages: boolean
  inputText: string
  setInputText: (val: string) => void
  sending: boolean
  handleSend: () => void
  handleAssign: () => void
  handleBack: () => void
  showMobileChat: boolean
  messagesEndRef: React.RefObject<HTMLDivElement | null>
  getInitials: (name: string) => string
  formatTime: (iso: string) => string
  isUnassignedChat: boolean
  isAssignedToMe: boolean
  handleLoadOlder: () => void
  loadingOlder: boolean
  hasMoreMessages: boolean
}

export default function ChatWindow({
  activeTicket,
  activeMessages,
  loadingMessages,
  inputText,
  setInputText,
  sending,
  handleSend,
  handleAssign,
  handleBack,
  showMobileChat,
  messagesEndRef,
  getInitials,
  formatTime,
  isUnassignedChat,
  isAssignedToMe,
  handleLoadOlder,
  loadingOlder,
  hasMoreMessages
}: ChatWindowProps) {
  const [downloadingMsgId, setDownloadingMsgId] = useState<string | null>(null)
  
  const contactName = activeTicket.contact_name || activeTicket.subject || 'Sin nombre'
  const canWriteInput = isAssignedToMe || !isUnassignedChat

  const MEDIA_PLACEHOLDERS = [
    '📷 Imagen recibida',
    '🎥 Video recibido',
    '🎤 Nota de voz recibida',
    '📄 Documento recibido',
    '🏷️ Sticker recibido',
    '📍 Ubicación recibida',
    '👤 Contacto recibido',
    '📎 Archivo adjunto recibido'
  ]

  const isMediaMessage = (msg: WhatsAppMessageDTO) => {
    if (!msg.external_id) return false
    const content = msg.content.trim()
    if (MEDIA_PLACEHOLDERS.includes(content)) return true

    // Detectar extensiones comunes de archivos adjuntos (documentos, audios, imágenes, videos)
    const commonExtensions = [
      '.pdf', '.doc', '.docx', '.xls', '.xlsx', '.ppt', '.pptx', '.txt', '.csv', '.rtf',
      '.png', '.jpg', '.jpeg', '.gif', '.webp', '.ogg', '.mp3', '.wav', '.mp4', '.avi', '.zip', '.rar'
    ]
    const lowerContent = content.toLowerCase()
    return commonExtensions.some(ext => lowerContent.endsWith(ext))
  }

  const handleDownloadMedia = async (msg: WhatsAppMessageDTO) => {
    if (!activeTicket || downloadingMsgId) return
    setDownloadingMsgId(msg.id)
    try {
      const { blob, contentType } = await ticketService.downloadWaMedia(activeTicket.zoho_id, msg.external_id)
      const url = window.URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      
      let ext = ''
      if (contentType.includes('image/jpeg')) ext = '.jpg'
      else if (contentType.includes('image/png')) ext = '.png'
      else if (contentType.includes('video/mp4')) ext = '.mp4'
      else if (contentType.includes('audio/ogg')) ext = '.ogg'
      else if (contentType.includes('application/pdf')) ext = '.pdf'
      
      a.download = `adjunto_${msg.id}${ext}`
      document.body.appendChild(a)
      a.click()
      window.URL.revokeObjectURL(url)
      document.body.removeChild(a)
    } catch (err) {
      console.error('Error downloading media:', err)
      alert('No se pudo descargar el archivo. Puede que ya no esté disponible.')
    } finally {
      setDownloadingMsgId(null)
    }
  }

  const onSubmit = () => {
    handleSend()
  }

  return (
    <main className={`${styles.chatArea} ${!showMobileChat ? styles.chatAreaHidden : ''}`}>
      <div className={styles.chatHeader}>
        <button className={styles.backBtn} onClick={handleBack} aria-label="Volver">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" width="20" height="20">
            <path d="M15 18l-6-6 6-6"/>
          </svg>
        </button>
        <div className={styles.chatHeaderAvatar}>
          {getInitials(contactName)}
        </div>
        <div className={styles.chatHeaderInfo}>
          <span className={styles.chatHeaderName}>{contactName}</span>
          <span className={styles.chatHeaderStatus} style={{ color: '#667781' }}>
            {activeTicket.contact_phone
              ? `+${activeTicket.contact_phone.replace(/^\+/, '')}`
              : contactName}
          </span>
        </div>
        <div className={styles.chatHeaderActions} style={{ display: 'flex', gap: '8px', alignItems: 'center' }}>
          {isUnassignedChat && !isAssignedToMe && (
            <button
              onClick={handleAssign}
              style={{
                fontSize: '12px',
                padding: '6px 14px',
                borderRadius: '20px',
                border: 'none',
                background: 'linear-gradient(135deg, #25D366, #128C7E)',
                color: 'white',
                cursor: 'pointer',
                fontWeight: 600,
                display: 'flex',
                alignItems: 'center',
                gap: '6px',
                boxShadow: '0 2px 8px rgba(37,211,102,0.3)',
                transition: 'transform 0.15s',
                fontFamily: 'inherit'
              }}
            >
              <UserRoundPlus size={14} />
              Tomar este chat
            </button>
          )}
          {isAssignedToMe && (
            <span style={{
              fontSize: '11px',
              padding: '4px 10px',
              borderRadius: '20px',
              background: 'rgba(37,211,102,0.1)',
              color: '#128C7E',
              fontWeight: 600
            }}>
              Asignado a ti
            </span>
          )}
        </div>
      </div>

      <div className={styles.messages}>
        {loadingMessages ? (
          <div style={{ margin: 'auto', color: '#667781', fontSize: '14px' }}>Cargando mensajes...</div>
        ) : activeMessages.length === 0 ? (
          <div style={{ margin: 'auto', color: '#667781', fontSize: '14px' }}>No hay mensajes aún</div>
        ) : (
          <>
            <div style={{ textAlign: 'center', margin: '10px 0 20px' }}>
              <button 
                onClick={handleLoadOlder} 
                disabled={loadingOlder || !hasMoreMessages}
                className={styles.loadOlderBtn}
              >
                {loadingOlder ? 'Cargando...' : !hasMoreMessages ? 'Todos los mensajes cargados' : 'Cargar mensajes anteriores'}
              </button>
            </div>
            {activeMessages.map((msg) => {
              const isOwn = msg.direction === 'outgoing'
            return (
              <div key={msg.id} className={`${styles.msgRow} ${isOwn ? styles.msgRowOwn : ''}`}>
                <div className={`${styles.msgBubble} ${isOwn ? styles.msgBubbleOwn : ''}`}>
                  {!isOwn && (
                    <p style={{ fontSize: '11px', fontWeight: 600, color: '#128C7E', margin: '0 0 4px 0' }}>
                      {msg.author_name || 'Cliente'}
                    </p>
                  )}
                  <p className={styles.msgText}>{msg.content}</p>
                  
                  {isMediaMessage(msg) && (
                    <button 
                      className={styles.mediaDownloadBtn} 
                      onClick={() => handleDownloadMedia(msg)}
                      disabled={downloadingMsgId === msg.id}
                      title="Descargar archivo"
                    >
                      {downloadingMsgId === msg.id ? <Loader2 size={14} className={styles.spin} /> : <Download size={14} />}
                      <span>{downloadingMsgId === msg.id ? 'Descargando...' : 'Descargar'}</span>
                    </button>
                  )}

                  <div className={styles.msgMeta}>
                    <span className={styles.msgTime}>{formatTime(msg.created_time)}</span>
                    {isOwn && <DeliveryIcon status={msg.delivery_status} />}
                  </div>
                  {isOwn && msg.delivery_status === 'failed' && (
                    <p style={{ fontSize: '11px', color: '#B91C1C', margin: '4px 0 0 0' }}>
                      No se pudo entregar. Reenvialo cuando WhatsApp esté disponible.
                    </p>
                  )}
                </div>
              </div>
            )
          })}
          </>
        )}
        <div ref={messagesEndRef} />
      </div>

      {canWriteInput ? (
        <div style={{ display: 'flex', flexDirection: 'column', background: '#f0f2f5', borderTop: '1px solid #e9edef' }}>
          <div className={styles.inputBar}>
            <input
              type="text"
              placeholder="Escribe un mensaje"
              className={styles.textInput}
              value={inputText}
              onChange={e => setInputText(e.target.value)}
              onKeyDown={e => e.key === 'Enter' && onSubmit()}
              disabled={sending}
            />
            <button
              className={`${styles.sendBtn} ${inputText.trim() ? styles.sendBtnActive : ''}`}
              onClick={onSubmit}
              title="Enviar"
              disabled={!inputText.trim() || sending}
            >
              <Send size={20} />
            </button>
          </div>
        </div>
      ) : (
        <div className={styles.inputDisabledMessage}>
          {isUnassignedChat ? 'Toma el chat para poder responder' : 'No tienes permiso para escribir en este chat'}
        </div>
      )}
    </main>
  )
}
