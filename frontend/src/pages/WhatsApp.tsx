import { useState, useEffect, useRef } from 'react'
import { useSearchParams } from 'react-router-dom'
import { useAuth } from '../context/AuthContext'
import styles from './WhatsApp.module.css'
import { ticketService, Ticket } from '../services/ticket.service'
import ChatList from './WhatsApp/ChatList'
import ChatWindow from './WhatsApp/ChatWindow'
import EmptyState from './WhatsApp/EmptyState'

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

export default function WhatsApp() {
  const { user } = useAuth()
  const [searchParams] = useSearchParams()
  const ticketIdParam = searchParams.get('ticketId')
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

  // Fetch WAHA status — poll every 5 s while disconnected, stop once connected
  useEffect(() => {
    const fetchStatus = () => {
      ticketService.getWahaStatus()
        .then(data => setWahaStatus(data))
        .catch(err => console.error('Error fetching WAHA status:', err))
    }

    fetchStatus() // initial call

    const interval = setInterval(() => {
      // Only keep polling while not connected
      if (!isConnected(wahaStatus?.status)) {
        fetchStatus()
      }
    }, 5000)

    return () => clearInterval(interval)
  }, [wahaStatus?.status])

  // Fetch tickets list
  useEffect(() => {
    setLoadingTickets(true)
    ticketService.getTickets()
      .then(data => {
        setTickets(data)
      })
      .catch(err => console.error('Error fetching tickets:', err))
      .finally(() => setLoadingTickets(false))
  }, [])

  // Auto-select ticket from query param when tickets are loaded
  useEffect(() => {
    if (ticketIdParam && tickets.length > 0) {
      const found = tickets.find(t => t.zoho_id === ticketIdParam)
      if (found) {
        handleSelectTicket(found)
      }
    }
  }, [ticketIdParam, tickets])

  // Scroll to bottom on new messages
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [activeTicket?.messages])

  const handleSelectTicket = async (ticket: Ticket) => {
    setShowMobileChat(true)
    setLoadingMessages(true)
    try {
      const full = await ticketService.getTicket(ticket.zoho_id)
      setActiveTicket(full.ticket)
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
      const newMsg = await ticketService.sendMessage(activeTicket.zoho_id, text, 'whatsapp')
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
      <ChatList
        user={user}
        tickets={tickets}
        activeTicket={activeTicket}
        loadingTickets={loadingTickets}
        wahaStatus={wahaStatus}
        search={search}
        setSearch={setSearch}
        handleSelectTicket={handleSelectTicket}
        showMobileChat={showMobileChat}
        filteredTickets={filteredTickets}
        lastMsg={lastMsg}
        formatTime={formatTime}
        getInitials={getInitials}
        isConnected={isConnected}
      />

      {activeTicket ? (
        <ChatWindow
          activeTicket={activeTicket}
          setActiveTicket={setActiveTicket}
          tickets={tickets}
          setTickets={setTickets}
          loadingMessages={loadingMessages}
          inputText={inputText}
          setInputText={setInputText}
          sending={sending}
          handleSend={handleSend}
          handleBack={handleBack}
          showMobileChat={showMobileChat}
          messagesEndRef={messagesEndRef}
          getInitials={getInitials}
          formatTime={formatTime}
        />
      ) : (
        <EmptyState />
      )}
    </div>
  )
}
