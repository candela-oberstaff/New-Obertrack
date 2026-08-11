import { useState, useEffect, useRef } from 'react'
import { useSearchParams } from 'react-router-dom'
import { useAuth } from '../context/AuthContext'
import styles from './WhatsApp.module.css'
import { ticketService, WhatsAppChatTicket, WhatsAppMessageDTO } from '../services/ticket.service'
import ChatList from './WhatsApp/ChatList'
import ChatWindow from './WhatsApp/ChatWindow'
import EmptyState from './WhatsApp/EmptyState'

type ChatTab = 'me' | 'unassigned'

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

// Huella del chat para decidir si el polling debe re-renderizar. Incluye el
// estado de entrega, no solo los ids, porque un mensaje del outbox cambia de
// pendiente a enviado sin que varíe la lista.
const chatSignature = (msgs: WhatsAppMessageDTO[]) =>
  msgs.map(m => `${m.id}:${m.delivery_status ?? ''}`).join('|')

const displayName = (ticket: WhatsAppChatTicket) =>
  ticket.contact_name.trim() || ticket.subject || ticket.contact_phone || 'Sin nombre'

export default function WhatsApp() {
  const { user } = useAuth()
  const [searchParams, setSearchParams] = useSearchParams()
  const [activeTab, setActiveTab] = useState<ChatTab>('me')
  const [myChats, setMyChats] = useState<WhatsAppChatTicket[]>([])
  const [unassignedChats, setUnassignedChats] = useState<WhatsAppChatTicket[]>([])
  const [activeTicket, setActiveTicket] = useState<WhatsAppChatTicket | null>(null)
  const [activeMessages, setActiveMessages] = useState<WhatsAppMessageDTO[]>([])
  const [loadingTickets, setLoadingTickets] = useState(true)
  const [loadingMessages, setLoadingMessages] = useState(false)
  const [loadingOlder, setLoadingOlder] = useState(false)
  const [displayLimit, setDisplayLimit] = useState(20)
  const [hasMoreMessages, setHasMoreMessages] = useState(true)
  const [inputText, setInputText] = useState('')
  const [sending, setSending] = useState(false)
  const [search, setSearch] = useState('')
  const [showMobileChat, setShowMobileChat] = useState(false)
  const [readTimestamps, setReadTimestamps] = useState<Record<string, string>>({})
  const messagesEndRef = useRef<HTMLDivElement>(null)
  const skipScrollRef = useRef(false)

  const currentChats = activeTab === 'me' ? myChats : unassignedChats

  const fetchTickets = async (isPoll = false) => {
    if (!isPoll) setLoadingTickets(true)
    try {
      const all = await ticketService.getWaChats()
      const byRecent = (a: WhatsAppChatTicket, b: WhatsAppChatTicket) =>
        new Date(b.modified_time).getTime() - new Date(a.modified_time).getTime()
      const meId = user?.id != null ? String(user.id) : ''
      setMyChats(all.filter(c => c.assignee_id && c.assignee_id === meId).sort(byRecent))
      setUnassignedChats(all.filter(c => !c.assignee_id).sort(byRecent))
    } catch (err) {
      console.error('Error fetching WhatsApp chats:', err)
    } finally {
      if (!isPoll) setLoadingTickets(false)
    }
  }

  useEffect(() => {
    fetchTickets()
    
    // Setup long polling loop (every 30 seconds)
    const interval = setInterval(() => {
      fetchTickets(true)
    }, 30000)

    return () => clearInterval(interval)
  }, []) // Empty deps for mount/unmount

  // Cargar las marcas de tiempo leídas desde localStorage al iniciar
  useEffect(() => {
    try {
      const saved = localStorage.getItem('wa_read_timestamps')
      if (saved) {
        setReadTimestamps(JSON.parse(saved))
      }
    } catch (e) {
      console.error('Error loading read timestamps', e)
    }
  }, [])

  // Marcar chat como leído mientras esté abierto y reciba nuevos mensajes
  useEffect(() => {
    if (activeTicket) {
      const latestTicket = myChats.find(t => t.zoho_id === activeTicket.zoho_id) || 
                           unassignedChats.find(t => t.zoho_id === activeTicket.zoho_id)
      
      if (latestTicket) {
        setReadTimestamps(prev => {
          const prevTime = prev[latestTicket.zoho_id]
          if (!prevTime || new Date(latestTicket.modified_time).getTime() > new Date(prevTime).getTime()) {
            const next = { ...prev, [latestTicket.zoho_id]: latestTicket.modified_time }
            localStorage.setItem('wa_read_timestamps', JSON.stringify(next))
            return next
          }
          return prev
        })
      }
    }
  }, [activeTicket, myChats, unassignedChats])

  // Handle active messages polling when a chat is open
  useEffect(() => {
    if (!activeTicket) return

    const pollMessages = async () => {
      try {
        const msgs = await ticketService.getWaChatMessages(activeTicket.zoho_id)
        setActiveMessages(prev => {
          // Comparar solo la cantidad dejaba la vista congelada ante cualquier
          // cambio sobre un mensaje ya presente: en particular la transición
          // pendiente → enviado/fallido del outbox, que no altera el total.
          if (chatSignature(msgs) === chatSignature(prev)) return prev
          return msgs
        })
      } catch (err) {
        console.error('Error polling messages:', err)
      }
    }

    const interval = setInterval(pollMessages, 10000)
    return () => clearInterval(interval)
  }, [activeTicket?.zoho_id])

  // Deep link desde la ficha del contacto: /whatsapp?ticketId=<id>. El enlace ya
  // existía y la página no leía el parámetro, así que caía en la bandeja sin
  // seleccionar nada. Se espera a que carguen los chats, se abre la pestaña donde
  // vive el ticket y se selecciona.
  useEffect(() => {
    const wanted = searchParams.get('ticketId')
    if (!wanted || loadingTickets) return
    const mine = myChats.find(c => c.zoho_id === wanted)
    const target = mine ?? unassignedChats.find(c => c.zoho_id === wanted)
    if (target) {
      setActiveTab(mine ? 'me' : 'unassigned')
      handleSelectTicket(target)
    }
    // Se limpia siempre: si el ticket no está en la bandeja (otra sesión de WAHA,
    // o ya resuelto) reintentar en cada render no lo va a hacer aparecer.
    setSearchParams({}, { replace: true })
  }, [searchParams, loadingTickets, myChats, unassignedChats])

  const handleSelectTicket = async (ticket: WhatsAppChatTicket) => {
    setShowMobileChat(true)
    setLoadingMessages(true)
    setActiveTicket(ticket)
    setActiveMessages([])
    setDisplayLimit(20)
    setHasMoreMessages(true)
    setInputText('')
    try {
      const msgs = await ticketService.getWaChatMessages(ticket.zoho_id)
      setActiveMessages(msgs)
    } catch (err) {
      console.error('Error fetching messages:', err)
    } finally {
      setLoadingMessages(false)
    }
  }

  const handleAssign = async () => {
    if (!activeTicket) return
    try {
      await ticketService.assignWaChat(activeTicket.zoho_id)
      await fetchTickets()
      setActiveTicket(prev => prev ? { ...prev, assignee_id: user?.id != null ? String(user.id) : '' } : null)
    } catch (err) {
      console.error('Error assigning chat:', err)
    }
  }

  const handleSend = async (_templateId?: string) => {
    if (!inputText.trim() || !activeTicket || sending) return
    const text = inputText.trim()
    setInputText('')
    setSending(true)
    try {
      const newMsg = await ticketService.sendWaChatMessage(activeTicket.zoho_id, text)
      setActiveMessages(prev => [...prev, newMsg])
    } catch (err) {
      console.error('Error sending message:', err)
      setInputText(text)
    } finally {
      setSending(false)
    }
  }

  const handleLoadOlder = async () => {
    if (!activeTicket) return
    setLoadingOlder(true)
    skipScrollRef.current = true
    try {
      if (displayLimit >= activeMessages.length) {
        const imported = await ticketService.loadOlderWaChatMessages(activeTicket.zoho_id)
        if (imported === 0) {
          setHasMoreMessages(false)
        } else {
          const msgs = await ticketService.getWaChatMessages(activeTicket.zoho_id)
          setActiveMessages(msgs)
        }
      }
      setDisplayLimit(prev => prev + 10)
    } catch (err) {
      console.error('Error loading older messages:', err)
    } finally {
      setLoadingOlder(false)
    }
  }

  useEffect(() => {
    if (skipScrollRef.current) {
      skipScrollRef.current = false
      return
    }
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [activeMessages])

  const handleBack = () => {
    setShowMobileChat(false)
    setActiveTicket(null)
    setActiveMessages([])
    setInputText('')
  }

  const isUnassignedChat = activeTab === 'unassigned'
  const isAssignedToMe = !!activeTicket?.assignee_id && activeTicket.assignee_id === (user?.id != null ? String(user.id) : '')

  const filteredTickets = currentChats.filter(t =>
    (t.contact_name ?? '').toLowerCase().includes(search.toLowerCase()) ||
    (t.contact_phone ?? '').includes(search) ||
    (t.subject ?? '').toLowerCase().includes(search.toLowerCase())
  )

  return (
    <div className={styles.page}>
      <ChatList
        user={user}
        tickets={currentChats}
        activeTicket={activeTicket}
        loadingTickets={loadingTickets}
        search={search}
        setSearch={setSearch}
        handleSelectTicket={handleSelectTicket}
        showMobileChat={showMobileChat}
        filteredTickets={filteredTickets}
        formatTime={formatTime}
        getInitials={getInitials}
        activeTab={activeTab}
        setActiveTab={setActiveTab}
        myChatsCount={myChats.length}
        unassignedChatsCount={unassignedChats.length}
        displayName={displayName}
        onSynced={() => fetchTickets(true)}
        readTimestamps={readTimestamps}
      />

      {activeTicket ? (
        <ChatWindow
          activeTicket={activeTicket}
          activeMessages={activeMessages.slice(-displayLimit)}
          loadingMessages={loadingMessages}
          inputText={inputText}
          setInputText={setInputText}
          sending={sending}
          handleSend={handleSend}
          handleAssign={handleAssign}
          handleBack={handleBack}
          showMobileChat={showMobileChat}
          messagesEndRef={messagesEndRef}
          getInitials={getInitials}
          formatTime={formatTime}
          isUnassignedChat={isUnassignedChat}
          isAssignedToMe={isAssignedToMe}
          handleLoadOlder={handleLoadOlder}
          loadingOlder={loadingOlder}
          hasMoreMessages={hasMoreMessages}
        />
      ) : (
        <EmptyState />
      )}
    </div>
  )
}
