import { useState, useEffect, useRef, useCallback, useMemo } from 'react'
import { useSearchParams } from 'react-router-dom'
import { useAuth } from '../context/AuthContext'
import { channelService, uploadService, adminService } from '../services/api'
import { Select } from '../components/ui/Select'
import type { User } from '../types'
import type { Message, SupportTicket } from '../types/chat'
import { useConfirm } from '../components/ui/ConfirmProvider'
import { useNotification } from '../context/NotificationContext'
import { Sidebar } from '../components/Chat/Sidebar'
import { ChatHeader } from '../components/Chat/ChatHeader'
import { MessageList } from '../components/Chat/MessageList'
import { MessageInput } from '../components/Chat/MessageInput'
import { canEditModule } from '../lib/permissions'
import { ThreadPanel } from '../components/Chat/ThreadPanel'
import { SupportContextPanel } from '../components/Chat/SupportContextPanel'
import { NewChannelModal } from '../components/Chat/Modals/NewChannelModal'
import { ChannelSettingsModal } from '../components/Chat/Modals/ChannelSettingsModal'
import { AddMembersModal } from '../components/Chat/Modals/AddMembersModal'
import { NewDmModal } from '../components/Chat/Modals/NewDmModal'
import { PinnedMessagesModal } from '../components/Chat/Modals/PinnedMessagesModal'
import { SearchModal } from '../components/Chat/Modals/SearchModal'
import { StarredMessagesModal } from '../components/Chat/Modals/StarredMessagesModal'
import { useSlackChat } from '../hooks/useSlackChat'
import { formatTime, buildMentionRegex, highlightMentionsWithRegex, getUserColor, isSupportChannel, newTempId } from '../components/Chat/ChatUtils'
import styles from './SlackChat.module.css'

interface CompanyOption { id: number; company_name: string }

export default function SlackChat() {
  const { user } = useAuth()
  const confirm = useConfirm()
  const { error: showError, success: showSuccess } = useNotification()
  const [searchParams, setSearchParams] = useSearchParams()
  const userIdParam = searchParams.get('userId')

  const isSuperadmin = !!user?.is_superadmin
  // Permiso del rol sobre el módulo Chat: con "Ver" el chat queda de solo
  // lectura (el backend igual bloquea los envíos con 403).
  const canEditChat = canEditModule(user, 'chat')

  // Botón de Soporte: lo ven los usuarios cliente (profesional/empleador) para
  // abrir un chat con Customer Success. CS/superadmin/IT no lo necesitan.
  const canRequestSupport = !!user && !isSuperadmin && (user.user_type === 'profesional' || user.user_type === 'empleador')
  const [contactingSupport, setContactingSupport] = useState(false)

  // Gestión de tickets de soporte: la pueden hacer CS y superadmins.
  const isSupportAgent = !!user && (isSuperadmin || user.user_type === 'customer_success' || user.user_type === 'analista_it')
  const [supportAgents, setSupportAgents] = useState<User[]>([])
  const [pendingSupport, setPendingSupport] = useState<SupportTicket[]>([])
  const [supportBusy, setSupportBusy] = useState(false)
  const [showSupportPanel, setShowSupportPanel] = useState(true)

  // Superadmin scope: pick a company so channels/DMs from different tenants never mix.
  const [companies, setCompanies] = useState<CompanyOption[]>([])
  const [selectedCompanyId, setSelectedCompanyIdState] = useState<number | null>(() => {
    const stored = localStorage.getItem('preferred_company_id')
    return stored ? Number(stored) : null
  })
  const setSelectedCompanyId = (id: number | null) => {
    setSelectedCompanyIdState(id)
    if (id) localStorage.setItem('preferred_company_id', String(id))
    else localStorage.removeItem('preferred_company_id')
  }

  const refreshPendingSupport = useCallback(() => {
    if (!isSupportAgent) return
    channelService.getPendingSupport(isSuperadmin ? selectedCompanyId : null).then(setPendingSupport).catch(() => {})
  }, [isSupportAgent, isSuperadmin, selectedCompanyId])

  useEffect(() => {
    if (!isSupportAgent) return
    channelService.getSupportAgents().then(setSupportAgents).catch(() => {})
    refreshPendingSupport()
  }, [isSupportAgent, refreshPendingSupport])

  // Mantén la cola de soporte viva sin recargar: refresca periódicamente y al
  // volver el foco a la ventana, para que un agente con la pantalla abierta vea
  // las solicitudes nuevas. Solo para agentes; limpia timer y listener al salir.
  useEffect(() => {
    if (!isSupportAgent) return
    const interval = setInterval(refreshPendingSupport, 25000)
    const onFocus = () => refreshPendingSupport()
    window.addEventListener('focus', onFocus)
    return () => {
      clearInterval(interval)
      window.removeEventListener('focus', onFocus)
    }
  }, [isSupportAgent, refreshPendingSupport])

  useEffect(() => {
    if (!isSuperadmin) return
    let active = true
    adminService.getTenants()
      .then((res: any) => {
        if (!active) return
        setCompanies((res || []).map((t: any) => ({
          id: t.id,
          company_name: t.company_name || t.owner_name || `Empresa ${t.id}`,
        })))
      })
      .catch((err) => console.error('Error fetching companies:', err))
    return () => { active = false }
  }, [isSuperadmin])

  const {
    channels, selectedChannel, setSelectedChannel,
    messages, setMessages, pinnedMessages,
    hasMoreMessages, loadingOlder, loadOlderMessages,
    newMessage, setNewMessage,
    channelMembers, fetchChannelMembers,
    allUsers, connectionLost, unreadCount,
    typingUsers, isRecording, isPaused, recordingStream, recordedBlob,
    pauseRecording, resumeRecording, cancelRecording, discardRecording, sendRecording,
    isUploading, setIsUploading,
    showThread, setShowThread, threadReplies, setThreadReplies,
    sendMessage, sendTypingIndicator, startRecording, stopRecording,
    editMessage, deleteMessage, pinMessage, unpinMessage, fetchChannels, fetchAllUsers,
    toggleReaction, starMessage, unstarMessage, starredMessages, starredIds, fetchStarredMessages,
    userStatuses,
    archivedChannels, fetchArchivedChannels,
  } = useSlackChat(user as any, isSuperadmin ? selectedCompanyId : null)

  const [showNewChannelModal, setShowNewChannelModal] = useState(false)
  const [showChannelSettings, setShowChannelSettings] = useState(false)
  const [showAddMembers, setShowAddMembers] = useState(false)
  const [showNewDmModal, setShowNewDmModal] = useState(false)
  const [showPinnedMessages, setShowPinnedMessages] = useState(false)
  const [showSearchModal, setShowSearchModal] = useState(false)
  const [searchQuery, setSearchQuery] = useState('')
  const [searchResults, setSearchResults] = useState<Message[]>([])
  const [showStarredModal, setShowStarredModal] = useState(false)
  const [showMobileChannels, setShowMobileChannels] = useState(false)
  const [chatSidebarCollapsed, setChatSidebarCollapsed] = useState(false)
  const [chatSidebarWidth, setChatSidebarWidth] = useState(220)
  const [isResizing, setIsResizing] = useState(false)
  const [playingAudio, setPlayingAudio] = useState<string | null>(null)
  const messagesEndRef = useRef<HTMLDivElement>(null)
  const audioRef = useRef<HTMLAudioElement | null>(null)
  const [newChannel, setNewChannel] = useState({ name: '', description: '', type: 'public' as 'public' | 'private', member_ids: [] as number[] })
  const [editingMessageId, setEditingMessageId] = useState<number | null>(null)
  const [editContent, setEditContent] = useState('')
  const [showMentionDropdown, setShowMentionDropdown] = useState(false)
  const [mentionFilterUsers, setMentionFilterUsers] = useState<User[]>([])
  const [mentionDismissedAt, setMentionDismissedAt] = useState<number | null>(null)
  const [originalFavicon, setOriginalFavicon] = useState<string | null>(null)
  const [highlightMessageId, setHighlightMessageId] = useState<number | null>(null)

  useEffect(() => {
    const icon = document.querySelector("link[rel*='icon']") as HTMLLinkElement
    if (icon) setOriginalFavicon(icon.href)
  }, [])

  // Deep link from notifications: /chat?channel=<id>&message=<id> opens the
  // channel and scrolls to the mentioned message.
  useEffect(() => {
    const channelParam = searchParams.get('channel')
    if (!channelParam || channels.length === 0) return
    const target = channels.find(c => c.id === parseInt(channelParam))
    if (!target) return
    const messageParam = searchParams.get('message')
    if (messageParam) setHighlightMessageId(parseInt(messageParam))
    setSelectedChannel(target)
    setSearchParams({}, { replace: true })
  }, [searchParams, channels, setSelectedChannel, setSearchParams])

  useEffect(() => {
    if (!highlightMessageId || messages.length === 0) return
    const el = document.getElementById(`message-${highlightMessageId}`)
    if (!el) return
    el.scrollIntoView({ behavior: 'smooth', block: 'center' })
    const timeout = setTimeout(() => setHighlightMessageId(null), 3000)
    return () => clearTimeout(timeout)
  }, [highlightMessageId, messages])

  useEffect(() => {
    if (userIdParam && allUsers.length > 0) {
      const recipientId = parseInt(userIdParam)
      if (recipientId && recipientId !== user?.id) {
        // Try to find if a DM with this user already exists in channels
        const existingDm = channels.find(c => 
          c.type === 'direct' && c.recipient?.id === recipientId
        )
        if (existingDm) {
          setSelectedChannel(existingDm)
          setSearchParams({}, { replace: true })
        } else {
          // Check if the recipient exists in allUsers
          const recipientExists = allUsers.some(u => u.id === recipientId)
          if (recipientExists) {
            channelService.createDM(recipientId, isSuperadmin ? selectedCompanyId : null).then(async (dm) => {
              await fetchChannels()
              setSelectedChannel(dm as any)
              setSearchParams({}, { replace: true })
            }).catch(e => console.error(e))
          }
        }
      }
    }
  }, [userIdParam, allUsers, channels, user?.id, setSearchParams, setSelectedChannel, fetchChannels])

  // Scroll to bottom only when the newest message changes (sending/receiving),
  // not when older history is prepended by the pagination.
  const lastMessageKeyRef = useRef<string | null>(null)
  useEffect(() => {
    const last = messages[messages.length - 1]
    const key = last ? `${last.id}-${last.tempId ?? ''}` : null
    if (key !== lastMessageKeyRef.current) {
      lastMessageKeyRef.current = key
      scrollToBottom()
    }
  }, [messages])

  useEffect(() => {
    let icon = document.querySelector("link[rel*='icon']") as HTMLLinkElement | null
    if (!icon) {
      icon = document.createElement('link')
      icon.rel = 'icon'
      document.head.appendChild(icon)
    }
    if (unreadCount > 0) {
      icon.href = `data:image/svg+xml,<svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 100 100'><text y='.9em' font-size='90'>🔴</text></svg>`
    } else {
      icon.href = originalFavicon ?? '/logos/Isotipo_Color.png'
    }
  }, [unreadCount, originalFavicon])

  // Stop and release any playing audio when the channel changes or the page
  // unmounts. Keyed only on the channel id (not playingAudio) so it never
  // pauses an audio element created later in the same render cycle.
  useEffect(() => {
    return () => {
      audioRef.current?.pause()
      audioRef.current = null
      setPlayingAudio(null)
    }
  }, [selectedChannel?.id])

  const scrollToBottom = () => messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' })

  // Mention highlighting is O(messages x users) per render: precompute the regex
  // once per allUsers change and pass a stable highlight callback so memoized
  // MessageItem rows don't re-render when unrelated state updates.
  const mentionRegex = useMemo(() => buildMentionRegex(allUsers), [allUsers])
  const currentUserName = user?.name
  const highlightMentions = useCallback(
    (content: string) => highlightMentionsWithRegex(content, mentionRegex, currentUserName),
    [mentionRegex, currentUserName],
  )

  // Accent/case-insensitive comparison for mention matching ("mend" matches "Méndez").
  const normalizeText = (s: string) => s.normalize('NFD').replace(/[̀-ͯ]/g, '').toLowerCase()

  // Live mention suggestions: filter as the user types after the last "@".
  useEffect(() => {
    const lastAt = newMessage.lastIndexOf('@')
    if (lastAt === -1 || mentionDismissedAt === lastAt) {
      setShowMentionDropdown(false)
      return
    }
    const query = normalizeText(newMessage.slice(lastAt + 1))
    if (query.length > 30) {
      setShowMentionDropdown(false)
      return
    }
    const startsWith: User[] = []
    const contains: User[] = []
    for (const u of allUsers) {
      if (u.id === user?.id) continue
      const name = normalizeText(u.name || '')
      if (name.startsWith(query)) startsWith.push(u)
      else if (query && name.includes(query)) contains.push(u)
    }
    const matches = [...startsWith, ...contains]
    setMentionFilterUsers(matches)
    setShowMentionDropdown(matches.length > 0)
  }, [newMessage, allUsers, mentionDismissedAt, user?.id])

  const insertMention = (u: User) => {
    const lastAt = newMessage.lastIndexOf('@')
    setNewMessage(newMessage.slice(0, lastAt + 1) + u.name + ' ')
    setShowMentionDropdown(false)
  }

  // Single send path shared by Enter and the send button. Capturing content and
  // clearing the input synchronously (before the async send resolves) prevents a
  // second Enter from queueing a duplicate optimistic message / re-send.
  const submitMessage = () => {
    if (!selectedChannel || !user) return
    const content = newMessage.trim()
    if (!content) return
    const tempId = newTempId()
    const optimisticMsg: Message = { id: 0, channel_id: selectedChannel.id, user_id: user.id, content, tempId, created_at: new Date().toISOString(), user: user as any }
    setMessages(prev => [...prev, optimisticMsg])
    setNewMessage('')
    sendMessage(undefined, tempId, content)
  }

  const handleInputKeyDown = (e: React.KeyboardEvent) => {
    if (showMentionDropdown && mentionFilterUsers.length > 0 && (e.key === 'Enter' || e.key === 'Tab') && !e.shiftKey) {
      e.preventDefault()
      insertMention(mentionFilterUsers[0])
      return
    }
    if (e.key === 'Escape' && showMentionDropdown) {
      setMentionDismissedAt(newMessage.lastIndexOf('@'))
      setShowMentionDropdown(false)
      return
    }
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      submitMessage()
    }
  }

  const handleFileUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return
    setIsUploading(true)
    try {
      const result = await uploadService.upload(file)
      const tempId = newTempId()
      const optimisticMsg: Message = { id: 0, channel_id: selectedChannel!.id, user_id: user!.id, content: `[Archivo: ${file.name}]`, tempId, attachment: result.url, file_name: file.name, file_type: file.type || undefined, created_at: new Date().toISOString(), user: user! as any }
      setMessages(prev => [...prev, optimisticMsg])
      sendMessage({ url: result.url, filename: file.name }, tempId, undefined, file.type || undefined)
    } catch (error) {
      console.error('Error uploading file:', error)
      showError('No se pudo subir el archivo. Intenta de nuevo.')
    }
    finally { setIsUploading(false); e.target.value = '' }
  }

  const togglePlayAudio = (url: string) => {
    if (playingAudio === url) { audioRef.current?.pause(); setPlayingAudio(null) }
    else { if (audioRef.current) audioRef.current.pause(); audioRef.current = new Audio(url); audioRef.current.onended = () => setPlayingAudio(null); audioRef.current.play(); setPlayingAudio(url) }
  }

  const handleSaveEdit = async () => {
    if (!selectedChannel || !editingMessageId || !editContent.trim()) return
    await editMessage(editingMessageId, editContent)
    setEditingMessageId(null); setEditContent('')
  }

  const leaveChannel = async (id: number) => {
    const isCreator = selectedChannel?.id === id && selectedChannel?.created_by === user?.id
    const ok = await confirm({
      title: 'Salir del canal',
      message: isCreator
        ? 'Eres el creador. Al salir, la administración se transferirá a otro miembro (a un admin del canal si existe). ¿Continuar?'
        : '¿Seguro que quieres salir de este canal? Dejarás de ver sus mensajes.',
      confirmLabel: 'Salir',
      variant: 'danger',
    })
    if (!ok) return
    try {
      await channelService.leaveChannel(id)
      if (selectedChannel?.id === id) setSelectedChannel(null)
      fetchChannels()
      fetchArchivedChannels()
    } catch (e: any) {
      console.error(e)
      showError(e?.response?.data?.error || 'No se pudo salir del canal. Intenta de nuevo.')
    }
  }
  const hideChannel = async (id: number) => {
    try {
      await channelService.hideChannel(id)
      if (selectedChannel?.id === id) setSelectedChannel(null)
      fetchChannels()
      fetchArchivedChannels()
    } catch (e: any) {
      console.error(e)
      showError(e?.response?.data?.error || 'No se pudo archivar el chat. Intenta de nuevo.')
    }
  }
  const restoreChannel = async (id: number) => {
    try {
      await channelService.unhideChannel(id)
      fetchChannels()
      fetchArchivedChannels()
    } catch (e: any) {
      console.error(e)
      showError(e?.response?.data?.error || 'No se pudo restaurar el chat. Intenta de nuevo.')
    }
  }
  const isSelectedArchived = !!selectedChannel && archivedChannels.some(c => c.id === selectedChannel.id)
  const addMember = async (id: number) => { if (selectedChannel) try { await channelService.addMember(selectedChannel.id, id); fetchChannelMembers(selectedChannel.id) } catch (e) { console.error(e) } }
  const removeMember = async (id: number) => { if (selectedChannel) try { await channelService.removeMember(selectedChannel.id, id); fetchChannelMembers(selectedChannel.id) } catch (e) { console.error(e) } }
  const handleUpdateChannel = async (id: number, updates: { name: string; description: string; type?: 'public' | 'private' }) => {
    try {
      const updated = await channelService.updateChannel(id, updates)
      setSelectedChannel(updated as any)
      fetchChannels()
    } catch (e: any) {
      console.error('Error updating channel:', e)
      // Surface the backend reason (p.ej. nombre duplicado 400, o 403) en vez de
      // fallar en silencio; re-lanzar para que el modal mantenga el form abierto.
      showError(e?.response?.data?.error || 'No se pudo actualizar el canal.')
      throw e
    }
  }

  const handleDeleteChannel = async (id: number) => {
    const ok = await confirm({
      title: 'Eliminar canal',
      message: '¿Seguro que quieres eliminar este canal? Esta acción no se puede deshacer y se perderán todos sus mensajes.',
      confirmLabel: 'Eliminar',
      variant: 'danger',
    })
    if (!ok) return
    try {
      await channelService.deleteChannel(id)
      setShowChannelSettings(false)
      if (selectedChannel?.id === id) setSelectedChannel(null)
      fetchChannels()
    } catch (e: any) {
      console.error('Error deleting channel:', e)
      // El backend rechaza con 400 los canales no eliminables (DM/soporte).
      showError(e?.response?.data?.error || 'No se pudo eliminar el canal. Intenta de nuevo.')
    }
  }

  const handleMouseDown = (e: React.MouseEvent) => {
    e.preventDefault(); setIsResizing(true)
    const onMouseMove = (e: MouseEvent) => { setChatSidebarWidth(Math.max(150, Math.min(400, e.clientX))) }
    const onMouseUp = () => { setIsResizing(false); document.removeEventListener('mousemove', onMouseMove); document.removeEventListener('mouseup', onMouseUp) }
    document.addEventListener('mousemove', onMouseMove); document.addEventListener('mouseup', onMouseUp)
  }

  const handleSearch = async () => {
    if (!selectedChannel || !searchQuery.trim()) return
    try {
      const results = await channelService.searchMessages(selectedChannel.id, searchQuery.trim())
      setSearchResults(results || [])
    } catch (e) {
      console.error('Error searching messages:', e)
      showError('No se pudo realizar la búsqueda.')
    }
  }

  // Public channels are visible without explicit membership, but unread tracking
  // and the member list need it — offer joining before participating.
  const isMemberOfSelected = !!user && channelMembers.some(m => m.id === user.id)
  const needsJoin = selectedChannel?.type === 'public' && channelMembers.length > 0 && !isMemberOfSelected
  // Modo supervisión (auditoría): un superadmin que ve un DM o un canal PRIVADO
  // del que NO es participante. No debe poder escribir (se vería como si fuera
  // parte de la conversación ajena); solo observa. Usamos el flag `supervised`
  // que calcula el backend (= no-miembro y DM/privado) como ÚNICA fuente de
  // verdad — la misma que agrupa la sección "Supervisión" del sidebar, así no
  // pueden divergir. Los canales públicos NO entran (uso normal). Los canales de
  // SOPORTE tampoco: tienen su propio flujo (tomar/atender) y el agente que
  // atiende debe poder escribir, aunque sea private y no fuera miembro al abrir.
  const isSupervising = !!selectedChannel?.supervised && !!selectedChannel && !isSupportChannel(selectedChannel)

  // Ticket de soporte activo atendido por OTRO agente: la conversación es suya.
  // El agente que mira queda en modo observador (sin composer, sin gestionar) y
  // su único camino es "Tomar" el ticket. No aplica al solicitante del ticket,
  // que siempre puede escribir en su propia solicitud.
  const supportInfo = selectedChannel && isSupportChannel(selectedChannel) ? (selectedChannel as any).support : null
  const supportAttendedByOther = !!(
    user && isSupportAgent && supportInfo && supportInfo.status !== 'resolved' &&
    supportInfo.assigned_to && supportInfo.assigned_to !== user.id &&
    supportInfo.requester_id !== user.id
  )

  const handleJoinChannel = async () => {
    if (!selectedChannel) return
    try {
      await channelService.joinChannel(selectedChannel.id)
      fetchChannelMembers(selectedChannel.id)
      fetchChannels()
    } catch (e) {
      console.error('Error joining channel:', e)
      showError('No se pudo unir al canal.')
    }
  }

  // Canal público: un no-miembro que lo abre se une automáticamente (sin fricción),
  // así puede escribir de inmediato y recibir mensajes/notificaciones en tiempo real.
  // El backend también auto-une al acceder; esto sincroniza el estado del frontend.
  useEffect(() => {
    if (needsJoin) handleJoinChannel()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [needsJoin, selectedChannel?.id])

  const handleSendGif = (url: string, title?: string) => {
    if (!selectedChannel || !user) return
    const tempId = newTempId()
    const optimisticMsg: Message = { id: 0, channel_id: selectedChannel.id, user_id: user.id, content: `[Archivo: ${title || 'GIF'}]`, tempId, attachment: url, file_name: title || 'GIF', file_type: 'image/gif', created_at: new Date().toISOString(), user: user as any }
    setMessages(prev => [...prev, optimisticMsg])
    sendMessage({ url, filename: title || 'GIF' }, tempId, '', 'image/gif')
  }

  const openThread = async (msg: Message) => {
    if (!selectedChannel) return
    setShowThread(msg)
    try {
      const replies = await channelService.getThreadReplies(selectedChannel.id, msg.id)
      setThreadReplies(replies || [])
    } catch (e) { console.error(e) }
  }

  const sendThreadReply = async (content: string) => {
    if (!selectedChannel || !showThread || !content.trim()) return
    // Optimistic reply (mirror of sendMessage): show it immediately so the sender
    // sees their reply even if the WS is down. id:0 + tempId let us reconcile later.
    const tempId = newTempId()
    const parentId = showThread.id
    const optimisticReply: Message = {
      id: 0,
      channel_id: selectedChannel.id,
      user_id: user!.id,
      content,
      tempId,
      parent_id: parentId,
      created_at: new Date().toISOString(),
      user: user! as any,
    }
    setThreadReplies(prev => [...prev, optimisticReply])
    try {
      const reply = await channelService.sendThreadReply(selectedChannel.id, parentId, content, tempId)
      // Reconcile deduping by tempId (our optimistic placeholder) AND by id (the
      // WS 'thread_reply' handler may have already appended the real reply).
      // M-3: carry the optimistic tempId onto the reconciled real reply (mirror of
      // sendMessage) so the React key (tempId ?? id) stays stable and the reply
      // row isn't remounted in the optimistic->real transition.
      setThreadReplies(prev => prev.filter(r => r.tempId !== tempId && r.id !== reply.id).concat([{ ...reply, tempId }]))
    } catch (e) {
      console.error(e)
      setThreadReplies(prev => prev.filter(r => r.tempId !== tempId))
      showError('No se pudo enviar la respuesta. Intenta de nuevo.')
    }
  }

  const handleContactSupport = async () => {
    if (contactingSupport) return
    setContactingSupport(true)
    try {
      const ch = await channelService.contactSupport()
      await fetchChannels()
      setSelectedChannel(ch as any)
      showSuccess('Chat de soporte abierto. Customer Success fue notificado.')
    } catch (e) {
      console.error('Error contacting support:', e)
      showError('No se pudo abrir el chat de soporte. Inténtalo de nuevo.')
    } finally {
      setContactingSupport(false)
    }
  }

  const applySupportTicket = (ticket: any) => {
    if (!selectedChannel) return
    setSelectedChannel({
      ...selectedChannel,
      support: {
        ...(selectedChannel as any).support,
        ticket_id: ticket.id,
        subject: ticket.subject,
        status: ticket.status,
        assigned_to: ticket.assigned_to,
        assignee_name: ticket.assignee?.name,
        requester_id: ticket.requester_id,
      },
    } as any)
    fetchChannels()
    // Recarga el transcript para mostrar el mensaje de sistema (tomó/asignó/resolvió).
    channelService.getMessages(selectedChannel.id).then(setMessages).catch(() => {})
  }

  const runSupportAction = async (fn: () => Promise<any>, okMsg: string) => {
    if (!selectedChannel || supportBusy) return
    setSupportBusy(true)
    try {
      const ticket = await fn()
      applySupportTicket(ticket)
      refreshPendingSupport()
      showSuccess(okMsg)
    } catch (e: any) {
      console.error('support action error:', e)
      showError(e?.response?.data?.error || 'No se pudo completar la acción.')
    } finally {
      setSupportBusy(false)
    }
  }

  // Reabre el panel de contexto al cambiar de canal (tras haberlo ocultado).
  useEffect(() => {
    setShowSupportPanel(true)
  }, [selectedChannel?.id])

  // Lee el ticket_id del MISMO ticket que muestra el panel/sidebar (el que elige
  // el dedup en la lista de canales), no de un selectedChannel posiblemente stale.
  // Así reasignar/tomar/resolver actúa sobre el ticket visible, y tras la acción
  // ese ticket queda como el más reciente → el panel refleja el nuevo responsable.
  const supportTicketId = () => {
    const fresh = channels.find(c => c.id === selectedChannel?.id) as any
    return (fresh?.support?.ticket_id ?? (selectedChannel as any)?.support?.ticket_id) as number | undefined
  }
  const handleClaimSupport = () => {
    const tid = supportTicketId(); if (!tid) return
    runSupportAction(() => channelService.claimSupport(tid), 'Tomaste el ticket.')
  }
  const handleAssignSupport = (assigneeId: number) => {
    const tid = supportTicketId(); if (!tid) return
    runSupportAction(() => channelService.assignSupport(tid, assigneeId), 'Ticket reasignado.')
  }
  const handleResolveSupport = () => {
    const tid = supportTicketId(); if (!tid) return
    runSupportAction(() => channelService.resolveSupport(tid), 'Ticket marcado como resuelto.')
  }

  const handleAcceptSupport = async (ticket: SupportTicket) => {
    if (supportBusy) return
    setSupportBusy(true)
    try {
      await channelService.claimSupport(ticket.id)
      await fetchChannels()
      refreshPendingSupport()
      try {
        const fresh = await channelService.getChannels(isSuperadmin ? selectedCompanyId : null)
        const ch = fresh.find(c => c.id === ticket.channel_id)
        if (ch) setSelectedChannel(ch as any)
      } catch { /* la lista ya se refrescó */ }
      showSuccess('Aceptaste la solicitud de soporte.')
    } catch (e: any) {
      console.error('accept support error:', e)
      showError(e?.response?.data?.error || 'No se pudo aceptar la solicitud.')
    } finally {
      setSupportBusy(false)
    }
  }

  return (
    <div className={styles['chat-page']}>
      {connectionLost && (
        <div className={styles['connection-banner']}>
          ⚠️ Conexión perdida — reconectando...
        </div>
      )}
      <ChatHeader
        selectedChannel={selectedChannel}
        showMobileChannels={showMobileChannels}
        setShowMobileChannels={setShowMobileChannels}
        showPinnedMessages={showPinnedMessages}
        setShowPinnedMessages={setShowPinnedMessages}
        pinnedMessagesCount={pinnedMessages.length}
        setShowAddMembers={setShowAddMembers}
        setShowChannelSettings={setShowChannelSettings}
        leaveChannel={leaveChannel}
        hideChannel={hideChannel}
        onShowSearch={() => { setSearchQuery(''); setSearchResults([]); setShowSearchModal(true) }}
        onShowStarred={() => { fetchStarredMessages(); setShowStarredModal(true) }}
        recipientStatus={selectedChannel?.type === 'direct' && selectedChannel.recipient
          ? (userStatuses.get(selectedChannel.recipient.id) || 'offline')
          : undefined}
        showSupportButton={canRequestSupport}
        onContactSupport={handleContactSupport}
        contactingSupport={contactingSupport}
        currentUserId={user?.id}
        isSupportAgent={isSupportAgent}
        supportAgents={supportAgents}
        onClaimSupport={handleClaimSupport}
        onAssignSupport={handleAssignSupport}
        onResolveSupport={handleResolveSupport}
        supportBusy={supportBusy}
      />

      <div className={styles['chat-body']}>
        <Sidebar
          channels={channels}
          archivedChannels={archivedChannels}
          onRestoreChannel={restoreChannel}
          selectedChannel={selectedChannel}
          setSelectedChannel={setSelectedChannel}
          showMobileChannels={showMobileChannels}
          setShowMobileChannels={setShowMobileChannels}
          chatSidebarCollapsed={chatSidebarCollapsed}
          setChatSidebarCollapsed={setChatSidebarCollapsed}
          chatSidebarWidth={chatSidebarWidth}
          setShowNewChannelModal={setShowNewChannelModal}
          setShowNewDmModal={setShowNewDmModal}
          fetchAllUsers={fetchAllUsers}
          onMouseDownResize={handleMouseDown}
          isResizing={isResizing}
          userStatuses={userStatuses}
          isSupportAgent={isSupportAgent}
          pendingSupport={pendingSupport}
          onAcceptSupport={handleAcceptSupport}
          supportBusy={supportBusy}
          headerExtra={isSuperadmin ? (
            <Select
              value={selectedCompanyId ?? ''}
              onChange={(v) => setSelectedCompanyId(v ? Number(v) : null)}
              clearable
              placeholder="Seleccione una empresa..."
              options={companies.map(c => ({ value: c.id, label: c.company_name }))}
            />
          ) : undefined}
        />

        <div className={styles['chat-main']}>
          {isSuperadmin && !selectedCompanyId ? (
            <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', height: '100%', textAlign: 'center', color: '#64748b', padding: '24px' }}>
              <h2 style={{ fontSize: '20px', fontWeight: 700, color: 'var(--black)', marginBottom: '8px' }}>Selecciona una empresa</h2>
              <p style={{ maxWidth: '380px' }}>
                Elige una empresa para ver sus canales y mensajes directos. La información de cada empresa se mantiene aislada.
              </p>
            </div>
          ) : selectedChannel ? (
            <>
              <MessageList
                messages={messages}
                currentUser={user as any}
                editingMessageId={editingMessageId}
                editContent={editContent}
                setEditContent={setEditContent}
                onSaveEdit={handleSaveEdit}
                onCancelEdit={() => { setEditingMessageId(null); setEditContent('') }}
                onStartEdit={(msg) => { setEditingMessageId(msg.id); setEditContent(msg.content) }}
                onDelete={deleteMessage}
                onPin={pinMessage}
                onUnpin={unpinMessage}
                onReply={openThread}
                onToggleReaction={toggleReaction}
                starredIds={starredIds}
                onStar={starMessage}
                onUnstar={unstarMessage}
                playingAudio={playingAudio}
                togglePlayAudio={togglePlayAudio}
                formatTime={formatTime}
                highlightMentions={highlightMentions}
                messagesEndRef={messagesEndRef}
                typingArray={Array.from(typingUsers.values())}
                highlightedMessageId={highlightMessageId}
                hasMoreMessages={hasMoreMessages}
                loadingOlder={loadingOlder}
                onLoadOlder={loadOlderMessages}
              />

              {!canEditChat ? (
                <div className={styles['join-banner']}>
                  <p>Tu rol tiene acceso de solo lectura en el Chat.</p>
                </div>
              ) : isSupervising ? (
                <div className={styles['join-banner']} style={{ background: 'rgba(124,58,237,0.08)', borderTop: '1px solid rgba(124,58,237,0.25)' }}>
                  <p style={{ margin: 0, color: '#6d28d9', fontWeight: 600 }}>
                    🔍 Modo supervisión — estás auditando esta conversación.
                  </p>
                  <p style={{ margin: '4px 0 0', color: '#7c3aed', fontSize: 13 }}>
                    No eres participante, por eso no puedes escribir aquí.
                  </p>
                </div>
              ) : isSelectedArchived ? (
                <div className={styles['join-banner']} style={{ background: 'rgba(100,116,139,0.08)', borderTop: '1px solid rgba(100,116,139,0.25)', display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 12, flexWrap: 'wrap' }}>
                  <p style={{ margin: 0, color: '#475569', fontWeight: 600 }}>
                    Este chat está archivado (solo lectura). Restáuralo para escribir.
                  </p>
                  <button
                    onClick={() => selectedChannel && restoreChannel(selectedChannel.id)}
                    style={{ border: 'none', background: '#7c3aed', color: '#fff', borderRadius: 8, padding: '7px 14px', fontWeight: 700, fontSize: 13, cursor: 'pointer', width: 'auto' }}
                  >
                    Restaurar
                  </button>
                </div>
              ) : supportAttendedByOther ? (
                <div className={styles['join-banner']} style={{ background: 'rgba(124,58,237,0.08)', borderTop: '1px solid rgba(124,58,237,0.25)', display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 12, flexWrap: 'wrap' }}>
                  <p style={{ margin: 0, color: '#6d28d9', fontWeight: 600 }}>
                    🎧 Este ticket lo atiende {supportInfo?.assignee_name || 'otro agente'}. Tómalo para poder responder.
                  </p>
                  <button
                    onClick={handleClaimSupport}
                    disabled={supportBusy}
                    style={{ border: 'none', background: '#7c3aed', color: '#fff', borderRadius: 8, padding: '7px 14px', fontWeight: 700, fontSize: 13, cursor: supportBusy ? 'wait' : 'pointer', width: 'auto' }}
                  >
                    Tomar ticket
                  </button>
                </div>
              ) : (
                <MessageInput
                  newMessage={newMessage}
                  setNewMessage={setNewMessage}
                  onKeyDown={handleInputKeyDown}
                  onFileUpload={handleFileUpload}
                  isUploading={isUploading}
                  isRecording={isRecording}
                  isPaused={isPaused}
                  recordingStream={recordingStream}
                  recordedBlob={recordedBlob}
                  startRecording={startRecording}
                  stopRecording={stopRecording}
                  pauseRecording={pauseRecording}
                  resumeRecording={resumeRecording}
                  cancelRecording={cancelRecording}
                  discardRecording={discardRecording}
                  sendRecording={sendRecording}
                  sendTypingIndicator={sendTypingIndicator}
                  onSendGif={handleSendGif}
                  onSend={submitMessage}
                />
              )}

              {showMentionDropdown && (
                <div className={styles['mention-dropdown']}>
                  {mentionFilterUsers.map((u, i) => (
                    <div key={u.id} className={`${styles['mention-option']} ${i === 0 ? styles['active'] : ''}`} onClick={() => insertMention(u)}>
                      <span 
                        className={styles['mention-user-avatar']}
                        style={{ background: getUserColor(u.name) }}
                      >
                        {u.name.charAt(0).toUpperCase()}
                      </span>
                      <span className={styles['mention-user-name']}>{u.name}</span>
                    </div>
                  ))}
                </div>
              )}
            </>
          ) : (
            <div className={styles['no-channel-selected']}>
              <div className={styles['no-channel-icon']}>#</div>
              {channels.length > 0 ? (
                <>
                  <h2>Selecciona un canal</h2>
                  <p>Elige un canal de la lista para empezar a chatear</p>
                </>
              ) : (
                <>
                  <h2>Aún no hay canales</h2>
                  <p>Crea el primer canal de tu empresa para empezar a chatear</p>
                  <button onClick={() => setShowNewChannelModal(true)} className={styles['create-first-channel']}>+ Crear tu primer canal</button>
                </>
              )}
            </div>
          )}
        </div>

        {isSupportAgent && selectedChannel && isSupportChannel(selectedChannel) && selectedChannel.support && showSupportPanel && (
          <SupportContextPanel channel={selectedChannel} onClose={() => setShowSupportPanel(false)} />
        )}
      </div>

      {showNewChannelModal && (
        <NewChannelModal
          newChannel={newChannel}
          setNewChannel={setNewChannel}
          allUsers={allUsers}
          currentUser={user as any}
          onClose={() => setShowNewChannelModal(false)}
          onCreate={async () => {
            if (!newChannel.name.trim()) return
            try { await channelService.createChannel(newChannel, isSuperadmin ? selectedCompanyId : null); setShowNewChannelModal(false); setNewChannel({ name: '', description: '', type: 'public', member_ids: [] }); fetchChannels() } catch (e) { console.error(e); showError('No se pudo crear el canal.') }
          }}
        />
      )}

      {showNewDmModal && (
        <NewDmModal
          allUsers={allUsers}
          onSelectUser={async (recipientId) => {
            try {
              const dm = await channelService.createDM(recipientId, isSuperadmin ? selectedCompanyId : null)
              await fetchChannels()
              setSelectedChannel(dm as any)
              setShowNewDmModal(false)
            } catch (e) {
              console.error('Error creating DM:', e)
              showError('No se pudo iniciar la conversación.')
            }
          }}
          onClose={() => setShowNewDmModal(false)}
          currentUser={user as any}
        />
      )}

      {showChannelSettings && selectedChannel && (
        <ChannelSettingsModal
          selectedChannel={selectedChannel}
          channelMembers={channelMembers}
          currentUser={user as any}
          isSuperadmin={isSuperadmin}
          onClose={() => setShowChannelSettings(false)}
          onRemoveMember={removeMember}
          onLeaveChannel={leaveChannel}
          onShowAddMembers={() => setShowAddMembers(true)}
          onUpdateChannel={handleUpdateChannel}
          onDeleteChannel={handleDeleteChannel}
          onRefreshMembers={fetchChannelMembers}
        />
      )}

      {showAddMembers && (
        <AddMembersModal
          allUsers={allUsers}
          isMember={(id) => channelMembers.some(m => m.id === id)}
          onAddMember={addMember}
          onClose={() => setShowAddMembers(false)}
          currentUser={user as any}
        />
      )}

      {showSearchModal && selectedChannel && (
        <SearchModal
          searchQuery={searchQuery}
          setSearchQuery={setSearchQuery}
          searchResults={searchResults}
          onSearch={handleSearch}
          onClose={() => { setShowSearchModal(false); setSearchQuery(''); setSearchResults([]) }}
          formatTime={formatTime}
        />
      )}

      {showStarredModal && (
        <StarredMessagesModal
          starredMessages={starredMessages}
          onUnstar={unstarMessage}
          onClose={() => setShowStarredModal(false)}
          formatTime={formatTime}
        />
      )}

      {showPinnedMessages && (
        <PinnedMessagesModal
          pinnedMessages={pinnedMessages}
          currentUser={user as any}
          onUnpin={unpinMessage}
          onClose={() => setShowPinnedMessages(false)}
          formatTime={formatTime}
        />
      )}

      <ThreadPanel
        showThread={showThread}
        threadReplies={threadReplies}
        onClose={() => { setShowThread(null); setThreadReplies([]) }}
        onSendReply={sendThreadReply}
        formatTime={formatTime}
      />
    </div>
  )
}
