import { useState, useEffect, useRef } from 'react'
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

const displayName = (ticket: WhatsAppChatTicket) =>
  ticket.contact_name.trim() || ticket.subject || ticket.contact_phone || 'Sin nombre'

export default function WhatsApp() {
  const { user } = useAuth()
  const [activeTab, setActiveTab] = useState<ChatTab>('me')
  const [myChats, setMyChats] = useState<WhatsAppChatTicket[]>([])
  const [unassignedChats, setUnassignedChats] = useState<WhatsAppChatTicket[]>([])
  const [activeTicket, setActiveTicket] = useState<WhatsAppChatTicket | null>(null)
  const [activeMessages, setActiveMessages] = useState<WhatsAppMessageDTO[]>([])
  const [loadingTickets, setLoadingTickets] = useState(true)
  const [loadingMessages, setLoadingMessages] = useState(false)
  const [inputText, setInputText] = useState('')
  const [sending, setSending] = useState(false)
  const [search, setSearch] = useState('')
  const [showMobileChat, setShowMobileChat] = useState(false)
  const messagesEndRef = useRef<HTMLDivElement>(null)

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

  // Handle active messages polling when a chat is open
  useEffect(() => {
    if (!activeTicket) return

    const pollMessages = async () => {
      try {
        const msgs = await ticketService.getWaChatMessages(activeTicket.zoho_id)
        // Only update if length changed or something simple for now
        // A better approach would be checking last message ID
        setActiveMessages(prev => {
          if (msgs.length !== prev.length) return msgs
          return prev
        })
      } catch (err) {
        console.error('Error polling messages:', err)
      }
    }

    const interval = setInterval(pollMessages, 10000)
    return () => clearInterval(interval)
  }, [activeTicket?.zoho_id])

  const handleSelectTicket = async (ticket: WhatsAppChatTicket) => {
    setShowMobileChat(true)
    setLoadingMessages(true)
    setActiveTicket(ticket)
    setActiveMessages([])
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

  useEffect(() => {
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
      />

      {activeTicket ? (
        <ChatWindow
          activeTicket={activeTicket}
          activeMessages={activeMessages}
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
        />
      ) : (
        <EmptyState />
      )}
    </div>
  )
}
