import { useState, useEffect, useRef, useCallback } from 'react'
import { useAuth } from '../context/AuthContext'
import { channelService } from '../services/api'
import { uploadService } from '../services/api'
import type { User } from '../types'
import './SlackChat.css'

interface Channel {
  id: number
  name: string
  description: string
  type: 'public' | 'private'
  created_by: number
  unread_count: number
  created_at: string
}

interface Message {
  id: number
  channel_id: number
  user_id: number
  content: string
  attachment?: string
  file_name?: string
  file_size?: number
  file_type?: string
  is_pinned?: boolean
  is_edited?: boolean
  is_deleted?: boolean
  parent_id?: number
  reactions?: MessageReaction[]
  created_at: string
  user?: User
  tempId?: string
}

interface MessageReaction {
  id: number
  message_id: number
  user_id: number
  emoji: string
  user?: User
}

interface ChannelMember {
  id: number
  name: string
  email: string
}

export default function SlackChat() {
  const { user } = useAuth()
  const [channels, setChannels] = useState<Channel[]>([])
  const [selectedChannel, setSelectedChannel] = useState<Channel | null>(null)
  const [messages, setMessages] = useState<Message[]>([])
  const [pinnedMessages, setPinnedMessages] = useState<Message[]>([])
  const [newMessage, setNewMessage] = useState('')
  const [showNewChannelModal, setShowNewChannelModal] = useState(false)
  const [showChannelSettings, setShowChannelSettings] = useState(false)
  const [showAddMembers, setShowAddMembers] = useState(false)
  const [showPinnedMessages, setShowPinnedMessages] = useState(false)
  const [channelMembers, setChannelMembers] = useState<ChannelMember[]>([])
  const [allUsers, setAllUsers] = useState<User[]>([])
  const [isConnected, setIsConnected] = useState(false)
  const [showMobileChannels, setShowMobileChannels] = useState(false)
  const [typingUsers, setTypingUsers] = useState<Map<number, string>>(new Map())
  const [chatSidebarCollapsed, setChatSidebarCollapsed] = useState(false)
  const [chatSidebarWidth, setChatSidebarWidth] = useState(220)
  const [isResizing, setIsResizing] = useState(false)
  const [isRecording, setIsRecording] = useState(false)
  const [playingAudio, setPlayingAudio] = useState<string | null>(null)
  const wsRef = useRef<WebSocket | null>(null)
  const messagesEndRef = useRef<HTMLDivElement>(null)
  const fileInputRef = useRef<HTMLInputElement>(null)
  const [isUploading, setIsUploading] = useState(false)
  const [newChannel, setNewChannel] = useState({ name: '', description: '', type: 'public' as 'public' | 'private' })
  const typingTimeoutRef = useRef<Map<number, ReturnType<typeof setTimeout>>>(new Map())
  const mediaRecorderRef = useRef<MediaRecorder | null>(null)
  const audioChunksRef = useRef<Blob[]>([])
  const chatSidebarRef = useRef<HTMLDivElement>(null)
  const audioRef = useRef<HTMLAudioElement | null>(null)
  const [unreadCount, setUnreadCount] = useState(0)
  const [showEmojiPicker, setShowEmojiPicker] = useState<number | null>(null)
  const [editingMessageId, setEditingMessageId] = useState<number | null>(null)
  const [editContent, setEditContent] = useState('')
  const [showThread, setShowThread] = useState<Message | null>(null)
  const [threadReplies, setThreadReplies] = useState<Message[]>([])
  const [showSearch, setShowSearch] = useState(false)
  const [searchQuery, setSearchQuery] = useState('')
  const [searchResults, setSearchResults] = useState<Message[]>([])
  const [showStarred, setShowStarred] = useState(false)
  const [starredMessages, setStarredMessages] = useState<Message[]>([])
  const [_userStatuses, _setUserStatuses] = useState<Map<number, string>>(new Map())

  useEffect(() => {
    fetchChannels()
    fetchAllUsers()
    return () => {
      if (wsRef.current) wsRef.current.close()
    }
  }, [])

  useEffect(() => {
    if (selectedChannel) {
      fetchMessages(selectedChannel.id)
      fetchChannelMembers(selectedChannel.id)
      fetchPinnedMessages(selectedChannel.id)
      connectWebSocket()
    }
  }, [selectedChannel])

  useEffect(() => {
    scrollToBottom()
  }, [messages])

  useEffect(() => {
    if (unreadCount > 0) {
      const link = document.querySelector("link[rel='icon']") as HTMLLinkElement || document.createElement('link')
      link.rel = 'icon'
      link.href = `data:image/svg+xml,<svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 100 100'><text y='.9em' font-size='90'>🔴</text></svg>`
      document.head.appendChild(link)
    }
  }, [unreadCount])

  const playNotificationSound = () => {
    const audioContext = new (window.AudioContext || (window as any).webkitAudioContext)()
    const oscillator = audioContext.createOscillator()
    const gainNode = audioContext.createGain()
    oscillator.connect(gainNode)
    gainNode.connect(audioContext.destination)
    oscillator.frequency.value = 800
    oscillator.type = 'sine'
    gainNode.gain.setValueAtTime(0.1, audioContext.currentTime)
    gainNode.gain.exponentialRampToValueAtTime(0.01, audioContext.currentTime + 0.3)
    oscillator.start(audioContext.currentTime)
    oscillator.stop(audioContext.currentTime + 0.3)
  }

  const fetchChannels = async () => {
    try {
      const data = await channelService.getChannels()
      setChannels(data || [])
    } catch (error) {
      console.error('Error fetching channels:', error)
    }
  }

  const fetchAllUsers = async () => {
    try {
      const data = await channelService.getAllUsers()
      setAllUsers(data || [])
    } catch (error) {
      console.error('Error fetching users:', error)
    }
  }

  const fetchMessages = async (channelId: number) => {
    try {
      const data = await channelService.getMessages(channelId)
      setMessages(data || [])
    } catch (error) {
      console.error('Error fetching messages:', error)
    }
  }

  const fetchPinnedMessages = async (channelId: number) => {
    try {
      const data = await channelService.getPinnedMessages(channelId)
      setPinnedMessages(data || [])
    } catch (error) {
      console.error('Error fetching pinned messages:', error)
    }
  }

  const fetchChannelMembers = async (channelId: number) => {
    try {
      const data = await channelService.getMembers(channelId)
      setChannelMembers(data || [])
    } catch (error) {
      console.error('Error fetching members:', error)
    }
  }

  const connectWebSocket = () => {
    if (wsRef.current) {
      wsRef.current.close()
    }

    const wsUrl = `ws://${window.location.hostname}:8080/ws/channels`
    const ws = new WebSocket(wsUrl)

    ws.onopen = () => {
      setIsConnected(true)
    }

    ws.onmessage = (event) => {
      try {
        const msg = JSON.parse(event.data)
        if (msg.channel_id === selectedChannel?.id) {
          if (msg.type === 'message') {
            setMessages(prev => [...prev.filter(m => m.tempId !== msg.data.tempId), msg.data])
            if (msg.data.user_id !== user?.id) {
              playNotificationSound()
            }
          } else if (msg.type === 'typing') {
            handleTyping(msg.user_id, msg.user_name, msg.channel_id)
          } else if (msg.type === 'message_pinned') {
            setMessages(prev => prev.map(m => m.id === msg.data.id ? { ...m, is_pinned: true } : m))
            fetchPinnedMessages(selectedChannel!.id)
          } else if (msg.type === 'message_unpinned') {
            setMessages(prev => prev.map(m => m.id === msg.data.id ? { ...m, is_pinned: false } : m))
            fetchPinnedMessages(selectedChannel!.id)
          } else if (msg.type === 'message_edited') {
            setMessages(prev => prev.map(m => m.id === msg.data.id ? { ...m, ...msg.data, is_edited: true } : m))
          } else if (msg.type === 'message_deleted') {
            setMessages(prev => prev.map(m => m.id === msg.data.id ? { ...m, is_deleted: true, content: '[Mensaje eliminado]' } : m))
          } else if (msg.type === 'reaction_added') {
            setMessages(prev => prev.map(m => m.id === msg.data.message_id ? { ...m, reactions: [...(m.reactions || []), msg.data.reaction] } : m))
          } else if (msg.type === 'reaction_removed') {
            setMessages(prev => prev.map(m => m.id === msg.data.message_id ? { ...m, reactions: (m.reactions || []).filter(r => !(r.user_id === msg.data.user_id && r.emoji === msg.data.emoji)) } : m))
          } else if (msg.type === 'thread_reply') {
            setThreadReplies(prev => [...prev, msg.data])
          }
        } else if (msg.type === 'message' && msg.user_id !== user?.id) {
          setUnreadCount(prev => prev + 1)
          playNotificationSound()
          setChannels(prev => prev.map(c => c.id === msg.channel_id ? { ...c, unread_count: (c.unread_count || 0) + 1 } : c))
        }
      } catch (e) {
        console.error('Error parsing message:', e)
      }
    }

    ws.onclose = () => {
      setIsConnected(false)
    }

    ws.onerror = () => {
      setIsConnected(false)
    }

    wsRef.current = ws
  }

  const handleTyping = (userId: number, userName: string, channelId: number) => {
    if (channelId !== selectedChannel?.id) return
    if (userId === user?.id) return

    setTypingUsers(prev => new Map(prev).set(userId, userName))

    if (typingTimeoutRef.current.has(userId)) {
      clearTimeout(typingTimeoutRef.current.get(userId)!)
    }

    const timeout = setTimeout(() => {
      setTypingUsers(prev => {
        const newMap = new Map(prev)
        newMap.delete(userId)
        return newMap
      })
    }, 3000)

    typingTimeoutRef.current.set(userId, timeout)
  }

  const sendTypingIndicator = () => {
    if (!wsRef.current || !isConnected || !selectedChannel) return

    wsRef.current.send(JSON.stringify({
      type: 'typing',
      channel_id: selectedChannel.id,
      user_name: user?.name
    }))
  }

  const scrollToBottom = () => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }

  const formatTime = (dateString: string) => {
    const date = new Date(dateString)
    const now = new Date()
    const diff = now.getTime() - date.getTime()
    const hours = Math.floor(diff / 3600000)
    const days = Math.floor(diff / 86400000)

    if (hours < 1) return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
    if (hours < 24) return `Hoy ${date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}`
    if (days < 7) return `${days}d`
    return date.toLocaleDateString('es-ES', { day: 'numeric', month: 'short' })
  }

  const sendMessage = useCallback((_e?: React.FormEvent, attachment?: { url: string; filename: string }, tempId?: string) => {
    if ((!newMessage.trim() && !attachment) || !selectedChannel) return

    const content = attachment ? `${newMessage} [Archivo: ${attachment.filename}]` : newMessage

    channelService.sendMessage(selectedChannel.id, { content, attachment: attachment?.url, file_name: attachment?.filename })
      .then(msg => {
        setMessages(prev => prev.filter(m => m.tempId !== tempId).concat([{ ...msg }]))
        setNewMessage('')
      })
      .catch(err => {
        console.error('Error sending message:', err)
        setMessages(prev => prev.filter(m => m.tempId !== tempId))
      })
  }, [selectedChannel, newMessage])

  const handleInputKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      if (newMessage.trim()) {
        const tempId = `temp-${Date.now()}`
        const optimisticMsg: Message = {
          id: 0,
          channel_id: selectedChannel!.id,
          user_id: user!.id,
          content: newMessage,
          tempId,
          created_at: new Date().toISOString(),
          user: user!
        }
        setMessages(prev => [...prev, optimisticMsg])
        sendMessage(undefined, undefined, tempId)
      }
    }
  }

  const handleFileUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return

    setIsUploading(true)
    try {
      const result = await uploadService.upload(file)
      const tempId = `temp-${Date.now()}`
      const optimisticMsg: Message = {
        id: 0,
        channel_id: selectedChannel!.id,
        user_id: user!.id,
        content: `[Archivo: ${file.name}]`,
        tempId,
        created_at: new Date().toISOString(),
        user: user!
      }
      setMessages(prev => [...prev, optimisticMsg])
      sendMessage(undefined, { url: result.url, filename: file.name }, tempId)
    } catch (error) {
      console.error('Error uploading file:', error)
    } finally {
      setIsUploading(false)
      if (fileInputRef.current) fileInputRef.current.value = ''
    }
  }

  const startRecording = async () => {
    try {
      const stream = await navigator.mediaDevices.getUserMedia({ audio: true })
      const mediaRecorder = new MediaRecorder(stream)
      audioChunksRef.current = []

      mediaRecorder.ondataavailable = (e) => {
        if (e.data.size > 0) {
          audioChunksRef.current.push(e.data)
        }
      }

      mediaRecorder.onstop = async () => {
        const audioBlob = new Blob(audioChunksRef.current, { type: 'audio/webm' })
        stream.getTracks().forEach(track => track.stop())
        
        try {
          const formData = new FormData()
          formData.append('file', audioBlob, 'voice_note.webm')
          const result = await uploadService.upload(audioBlob as unknown as File)
          const tempId = `temp-${Date.now()}`
          const optimisticMsg: Message = {
            id: 0,
            channel_id: selectedChannel!.id,
            user_id: user!.id,
            content: '[Nota de voz]',
            tempId,
            attachment: result.url,
            file_name: 'Nota de voz',
            file_type: 'audio/webm',
            created_at: new Date().toISOString(),
            user: user!
          }
          setMessages(prev => [...prev, optimisticMsg])
          sendMessage(undefined, { url: result.url, filename: 'Nota de voz' }, tempId)
        } catch (error) {
          console.error('Error uploading voice note:', error)
        }
      }

      mediaRecorderRef.current = mediaRecorder
      mediaRecorder.start()
      setIsRecording(true)
    } catch (error) {
      console.error('Error starting recording:', error)
    }
  }

  const stopRecording = () => {
    if (mediaRecorderRef.current && isRecording) {
      mediaRecorderRef.current.stop()
      setIsRecording(false)
    }
  }

  const togglePlayAudio = (url: string) => {
    if (playingAudio === url) {
      audioRef.current?.pause()
      setPlayingAudio(null)
    } else {
      if (audioRef.current) {
        audioRef.current.pause()
      }
      audioRef.current = new Audio(url)
      audioRef.current.onended = () => setPlayingAudio(null)
      audioRef.current.play()
      setPlayingAudio(url)
    }
  }

  const addReaction = async (messageId: number, emoji: string) => {
    if (!selectedChannel) return
    try {
      await channelService.addReaction(selectedChannel.id, messageId, emoji)
      setShowEmojiPicker(null)
    } catch (error) {
      console.error('Error adding reaction:', error)
    }
  }

  const removeReaction = async (messageId: number, emoji: string) => {
    if (!selectedChannel) return
    try {
      await channelService.removeReaction(selectedChannel.id, messageId, emoji)
    } catch (error) {
      console.error('Error removing reaction:', error)
    }
  }

  const startEditMessage = (message: Message) => {
    setEditingMessageId(message.id)
    setEditContent(message.content)
  }

  const cancelEdit = () => {
    setEditingMessageId(null)
    setEditContent('')
  }

  const saveEdit = async () => {
    if (!selectedChannel || !editingMessageId || !editContent.trim()) return
    const editedContent = editContent
    setMessages(prev => prev.map(m => m.id === editingMessageId ? { ...m, content: editedContent, is_edited: true } : m))
    setEditingMessageId(null)
    setEditContent('')
    try {
      await channelService.editMessage(selectedChannel.id, editingMessageId, editedContent)
    } catch (error) {
      console.error('Error editing message:', error)
    }
  }

  const deleteMessage = async (messageId: number) => {
    if (!selectedChannel) return
    if (!confirm('¿Eliminar este mensaje?')) return
    setMessages(prev => prev.map(m => m.id === messageId ? { ...m, is_deleted: true, content: '[Mensaje eliminado]' } : m))
    try {
      await channelService.deleteMessage(selectedChannel.id, messageId)
    } catch (error) {
      console.error('Error deleting message:', error)
    }
  }

  const openThread = async (message: Message) => {
    if (!selectedChannel) return
    setShowThread(message)
    try {
      const replies = await channelService.getThreadReplies(selectedChannel.id, message.id)
      setThreadReplies(replies || [])
    } catch (error) {
      console.error('Error fetching thread replies:', error)
    }
  }

  const closeThread = () => {
    setShowThread(null)
    setThreadReplies([])
  }

  const sendThreadReply = async (content: string) => {
    if (!selectedChannel || !showThread || !content.trim()) return
    try {
      const reply = await channelService.sendThreadReply(selectedChannel.id, showThread.id, content)
      setThreadReplies(prev => [...prev, reply])
    } catch (error) {
      console.error('Error sending thread reply:', error)
    }
  }

  const unstarMessage = async (messageId: number) => {
    try {
      await channelService.unstarMessage(messageId)
    } catch (error) {
      console.error('Error unstarring message:', error)
    }
  }

  const handleSearch = async () => {
    if (!selectedChannel || !searchQuery.trim()) return
    try {
      const results = await channelService.searchMessages(selectedChannel.id, searchQuery)
      setSearchResults(results || [])
    } catch (error) {
      console.error('Error searching messages:', error)
    }
  }

  const loadStarredMessages = async () => {
    try {
      const messages = await channelService.getStarredMessages()
      setStarredMessages(messages || [])
    } catch (error) {
      console.error('Error loading starred messages:', error)
    }
  }

  const createChannel = async () => {
    if (!newChannel.name.trim()) return
    try {
      await channelService.createChannel(newChannel)
      setShowNewChannelModal(false)
      setNewChannel({ name: '', description: '', type: 'public' })
      fetchChannels()
    } catch (error) {
      console.error('Error creating channel:', error)
    }
  }

  const leaveChannel = async (channelId: number) => {
    try {
      await channelService.leaveChannel(channelId)
      if (selectedChannel?.id === channelId) {
        setSelectedChannel(null)
      }
      fetchChannels()
    } catch (error) {
      console.error('Error leaving channel:', error)
    }
  }

  const addMember = async (userId: number) => {
    if (!selectedChannel) return
    try {
      await channelService.addMember(selectedChannel.id, userId)
      fetchChannelMembers(selectedChannel.id)
    } catch (error) {
      console.error('Error adding member:', error)
    }
  }

  const removeMember = async (userId: number) => {
    if (!selectedChannel) return
    try {
      await channelService.removeMember(selectedChannel.id, userId)
      fetchChannelMembers(selectedChannel.id)
    } catch (error) {
      console.error('Error removing member:', error)
    }
  }

  const pinMessage = async (messageId: number) => {
    if (!selectedChannel) return
    try {
      await channelService.pinMessage(selectedChannel.id, messageId)
      setMessages(prev => prev.map(m => m.id === messageId ? { ...m, is_pinned: true } : m))
      fetchPinnedMessages(selectedChannel.id)
    } catch (error) {
      console.error('Error pinning message:', error)
    }
  }

  const unpinMessage = async (messageId: number) => {
    if (!selectedChannel) return
    try {
      await channelService.unpinMessage(selectedChannel.id, messageId)
      setMessages(prev => prev.map(m => m.id === messageId ? { ...m, is_pinned: false } : m))
      fetchPinnedMessages(selectedChannel.id)
    } catch (error) {
      console.error('Error unpinning message:', error)
    }
  }

  const handleMouseDown = (e: React.MouseEvent) => {
    e.preventDefault()
    setIsResizing(true)
    document.addEventListener('mousemove', handleMouseMove)
    document.addEventListener('mouseup', handleMouseUp)
  }

  const handleMouseMove = useCallback((e: MouseEvent) => {
    if (!isResizing) return
    const newWidth = Math.max(150, Math.min(400, e.clientX))
    setChatSidebarWidth(newWidth)
  }, [isResizing])

  const handleMouseUp = useCallback(() => {
    setIsResizing(false)
    document.removeEventListener('mousemove', handleMouseMove)
    document.removeEventListener('mouseup', handleMouseUp)
  }, [isResizing])

  const publicChannels = (channels || []).filter(c => c.type === 'public')
  const privateChannels = (channels || []).filter(c => c.type === 'private')
  const typingArray = Array.from(typingUsers.values())
  const isMember = (userId: number) => channelMembers.some(m => m.id === userId)
  const isVoiceNote = (msg: Message) => msg.file_type?.startsWith('audio/') || msg.file_name?.includes('voice') || msg.attachment?.includes('.webm') || msg.attachment?.includes('.mp3')

  return (
    <div className="chat-page">
      <div className="chat-header-bar">
        <button className="mobile-channels-toggle" onClick={() => setShowMobileChannels(!showMobileChannels)}>
          {selectedChannel ? `# ${selectedChannel.name}` : 'Seleccionar canal'}
        </button>
        
        <div className="channel-tabs">
          {selectedChannel && (
            <div className="channel-tab active">
              <span>{selectedChannel.type === 'private' ? '🔒' : '#'}</span>
              {selectedChannel.name}
            </div>
          )}
        </div>

        <div className="channel-actions">
          {selectedChannel && (
            <>
              <button onClick={() => { setShowSearch(!showSearch); setSearchQuery(''); setSearchResults([]); }} title="Buscar">
                ⌕
              </button>
              <button onClick={() => { setShowStarred(true); loadStarredMessages(); }} title="Mensajes starred">
                ☆
              </button>
              <button onClick={() => setShowPinnedMessages(true)} title="Mensajes fijados">
                • {pinnedMessages.length > 0 && <span className="pin-count">{pinnedMessages.length}</span>}
              </button>
              <button onClick={() => setShowAddMembers(true)} title="Añadir personas">
                ⊕
              </button>
              <button onClick={() => setShowChannelSettings(true)} title="Info del canal">
                ◈
              </button>
              <button onClick={() => leaveChannel(selectedChannel.id)} title="Salir">
                ⊗
              </button>
            </>
          )}
          <button onClick={() => setShowNewChannelModal(true)} title="Crear canal">
            +
          </button>
        </div>
      </div>

      <div className="chat-body">
        <div 
          ref={chatSidebarRef}
          className={`channels-panel ${showMobileChannels ? 'open' : ''} ${chatSidebarCollapsed ? 'collapsed' : ''}`}
          style={{ width: chatSidebarCollapsed ? 40 : chatSidebarWidth }}
        >
          {!chatSidebarCollapsed && (
            <>
              <div className="channels-panel-header">
                <h3>Canales</h3>
                <button onClick={() => setShowNewChannelModal(true)}>+</button>
              </div>

              <div className="channel-list-mini">
                <div className="channel-group-label">Públicos</div>
                {publicChannels.map(channel => (
                  <div
                    key={channel.id}
                    className={`channel-mini-item ${selectedChannel?.id === channel.id ? 'active' : ''}`}
                    onClick={() => {
                      setSelectedChannel(channel)
                      setShowMobileChannels(false)
                    }}
                  >
                    <span className="channel-mini-icon">#</span>
                    <span className="channel-mini-name">{channel.name}</span>
                    {channel.unread_count > 0 && (
                      <span className="unread-badge">{channel.unread_count}</span>
                    )}
                  </div>
                ))}

                <div className="channel-group-label">Privados</div>
                {privateChannels.map(channel => (
                  <div
                    key={channel.id}
                    className={`channel-mini-item ${selectedChannel?.id === channel.id ? 'active' : ''}`}
                    onClick={() => {
                      setSelectedChannel(channel)
                      setShowMobileChannels(false)
                    }}
                  >
                    <span className="channel-mini-icon">○</span>
                    <span className="channel-mini-name">{channel.name}</span>
                  </div>
                ))}
              </div>
            </>
          )}
          
          <button 
            className="sidebar-toggle"
            onClick={() => setChatSidebarCollapsed(!chatSidebarCollapsed)}
          >
            {chatSidebarCollapsed ? '→' : '←'}
          </button>
        </div>

        <div 
          className="resize-handle"
          onMouseDown={handleMouseDown}
          style={{ cursor: isResizing ? 'col-resize' : undefined }}
        />

        <div className="chat-area">
          {selectedChannel ? (
            <>
              <div className="messages-container">
                {messages.length === 0 ? (
                  <div className="no-messages">
                    <span className="no-messages-icon">💬</span>
                    <p>No hay mensajes en #{selectedChannel.name}</p>
                    <small>Sé el primero en enviar un mensaje</small>
                  </div>
                ) : (
                  messages.map(msg => (
                    <div key={msg.id || msg.tempId} className={`message-item ${msg.user_id === user?.id ? 'own' : ''} ${msg.is_pinned ? 'pinned' : ''}`}>
                      <div className="message-avatar">
                        {msg.user?.name?.charAt(0).toUpperCase() || '?'}
                      </div>
                      <div className="message-content">
                        <div className="message-header">
                          <span className="message-author">{msg.user?.name || 'Usuario'}</span>
                          <span className="message-time">{formatTime(msg.created_at)}</span>
                          {msg.is_pinned && <span className="pinned-badge">📌</span>}
                        </div>
                        {editingMessageId === msg.id ? (
                          <div className="edit-container">
                            <input
                              type="text"
                              value={editContent}
                              onChange={(e) => setEditContent(e.target.value)}
                              className="edit-input"
                            />
                            <button onClick={saveEdit}>Guardar</button>
                            <button onClick={cancelEdit}>Cancelar</button>
                          </div>
                        ) : (
                          <>
                            {msg.content && !msg.attachment && <p className="message-text">{msg.content}</p>}
                            {msg.is_edited && <span className="edited-indicator">(editado)</span>}
                            {msg.attachment && isVoiceNote(msg) ? (
                              <div className="voice-note">
                                <button 
                                  className={`play-btn ${playingAudio === msg.attachment ? 'playing' : ''}`}
                                  onClick={() => togglePlayAudio(msg.attachment!)}
                                >
                                  {playingAudio === msg.attachment ? '⏸️' : '▶️'}
                                </button>
                                <div className="voice-waveform">
                                  <span>🎤 {msg.file_name || 'Nota de voz'}</span>
                                </div>
                              </div>
                            ) : msg.attachment ? (
                              <a href={msg.attachment} target="_blank" rel="noopener noreferrer" className="message-attachment">
                                {msg.file_type?.startsWith('image/') ? (
                                  <img src={msg.attachment} alt={msg.file_name} className="attachment-image" />
                                ) : (
                                  <>
                                    {msg.file_type?.startsWith('audio/') ? '🎤' : 
                                     msg.file_type?.startsWith('image/') ? '🖼️' : '📎'} 
                                    {msg.file_name}
                                  </>
                                )}
                              </a>
                            ) : null}
                          </>
                        )}
                        {msg.tempId && <span className="sending-indicator">Enviando...</span>}
                        {msg.reactions && msg.reactions.length > 0 && (
                          <div className="reactions-display">
                            {msg.reactions.map((reaction, idx) => (
                              <span
                                key={idx}
                                className="reaction-badge"
                                onClick={() => reaction.user_id === user?.id ? removeReaction(msg.id, reaction.emoji) : null}
                              >
                                {reaction.emoji}
                              </span>
                            ))}
                          </div>
                        )}
                        <div className="message-actions">
                          {!msg.tempId && !msg.is_deleted && (
                            <>
                              <button className="action-btn" onClick={() => setShowEmojiPicker(showEmojiPicker === msg.id ? null : msg.id)} title="Reaccionar">
                                +
                              </button>
                              {!msg.parent_id && (
                                <button className="action-btn" onClick={() => openThread(msg)} title="Hilo">
                                  ↩
                                </button>
                              )}
                              {msg.user_id === user?.id && (
                                <>
                                  <button className="action-btn" onClick={() => startEditMessage(msg)} title="Editar">
                                    ✎
                                  </button>
                                  <button className="action-btn delete" onClick={() => deleteMessage(msg.id)} title="Eliminar">
                                    ×
                                  </button>
                                </>
                              )}
                              <button className="action-btn" onClick={() => msg.is_pinned ? unpinMessage(msg.id) : pinMessage(msg.id)} title={msg.is_pinned ? 'Desfijar' : 'Fijar'}>
                                {msg.is_pinned ? '✓' : '↵'}
                              </button>
                            </>
                          )}
                        </div>
                        {showEmojiPicker === msg.id && (
                          <div className="emoji-picker">
                            {['👍', '❤️', '😂', '😮', '😢', '🙏', '🎉', '🔥', '👀', '💯'].map(emoji => (
                              <button key={emoji} onClick={() => addReaction(msg.id, emoji)}>{emoji}</button>
                            ))}
                          </div>
                        )}
                      </div>
                    </div>
                  ))
                )}
                <div ref={messagesEndRef} />
              </div>

              {typingArray.length > 0 && (
                <div className="typing-indicator">
                  {typingArray.join(', ')} {typingArray.length === 1 ? 'está' : 'están'} escribiendo...
                </div>
              )}

              <form className="message-input-container" onSubmit={(e) => { e.preventDefault(); if(newMessage.trim()) { const tempId = `temp-${Date.now()}`; const optimisticMsg: Message = { id: 0, channel_id: selectedChannel.id, user_id: user!.id, content: newMessage, tempId, created_at: new Date().toISOString(), user: user! }; setMessages(prev => [...prev, optimisticMsg]); sendMessage(undefined, undefined, tempId); } }}>
                <input
                  type="file"
                  ref={fileInputRef}
                  onChange={handleFileUpload}
                  accept=".pdf,.doc,.docx,.xls,.xlsx,.jpg,.jpeg,.png,.gif,.webp,.mp3,.wav,.ogg"
                  style={{ display: 'none' }}
                />
                <button
                  type="button"
                  onClick={() => fileInputRef.current?.click()}
                  disabled={isUploading}
                  className="attach-btn"
                  title="Adjuntar archivo"
                >
                  {isUploading ? '⏳' : '📎'}
                </button>
                <button
                  type="button"
                  onClick={isRecording ? stopRecording : startRecording}
                  className={`record-btn ${isRecording ? 'recording' : ''}`}
                  title={isRecording ? 'Detener grabación' : 'Grabar nota de voz'}
                >
                  {isRecording ? '⏹️' : '🎤'}
                </button>
                <input
                  type="text"
                  value={newMessage}
                  onChange={(e) => { setNewMessage(e.target.value); sendTypingIndicator(); }}
                  onKeyDown={handleInputKeyDown}
                  placeholder={`Enviar mensaje a #${selectedChannel.name}`}
                />
                <button type="submit" disabled={!newMessage.trim()}>
                  ➤
                </button>
              </form>
            </>
          ) : (
            <div className="no-channel-selected">
              <div className="no-channel-icon">#</div>
              <h2>Selecciona un canal</h2>
              <p>Elige un canal de la lista para empezar a chatear</p>
              <button onClick={() => setShowNewChannelModal(true)} className="create-first-channel">
                ➕ Crear tu primer canal
              </button>
            </div>
          )}
        </div>
      </div>

      {showNewChannelModal && (
        <div className="modal-overlay" onClick={() => setShowNewChannelModal(false)}>
          <div className="modal-content" onClick={e => e.stopPropagation()}>
            <h2>Crear canal</h2>
            <div className="form-group">
              <label>Nombre del canal</label>
              <input
                type="text"
                value={newChannel.name}
                onChange={(e) => setNewChannel({...newChannel, name: e.target.value})}
                placeholder="ej: general"
              />
            </div>
            <div className="form-group">
              <label>Descripción (opcional)</label>
              <input
                type="text"
                value={newChannel.description}
                onChange={(e) => setNewChannel({...newChannel, description: e.target.value})}
                placeholder="¿De qué trata este canal?"
              />
            </div>
            <div className="form-group">
              <label>Tipo</label>
              <select
                value={newChannel.type}
                onChange={(e) => setNewChannel({...newChannel, type: e.target.value as 'public' | 'private'})}
              >
                <option value="public">Público</option>
                <option value="private">Privado</option>
              </select>
            </div>
            <div className="modal-actions">
              <button onClick={() => setShowNewChannelModal(false)}>Cancelar</button>
              <button onClick={createChannel}>Crear canal</button>
            </div>
          </div>
        </div>
      )}

      {showChannelSettings && selectedChannel && (
        <div className="modal-overlay" onClick={() => setShowChannelSettings(false)}>
          <div className="modal-content" onClick={e => e.stopPropagation()}>
            <h2>Configuración de #{selectedChannel.name}</h2>
            {selectedChannel.description && (
              <p className="channel-desc">{selectedChannel.description}</p>
            )}
            <div className="members-section">
              <h3>Miembros ({channelMembers.length})</h3>
              <div className="members-list">
                {channelMembers.map(member => (
                  <div key={member.id} className="member-item">
                    <div className="member-avatar">{member.name?.charAt(0).toUpperCase()}</div>
                    <span className="member-name">{member.name}</span>
                    {member.id === selectedChannel.created_by && (
                      <span className="owner-badge">Owner</span>
                    )}
                    {member.id !== selectedChannel.created_by && member.id !== user?.id && (
                      <button onClick={() => removeMember(member.id)}>×</button>
                    )}
                    {member.id === user?.id && member.id !== selectedChannel.created_by && (
                      <button onClick={() => leaveChannel(selectedChannel.id)}>Salir</button>
                    )}
                  </div>
                ))}
              </div>
              <button onClick={() => setShowAddMembers(true)}>+ Añadir personas</button>
            </div>
            <div className="modal-actions">
              <button onClick={() => setShowChannelSettings(false)}>Cerrar</button>
            </div>
          </div>
        </div>
      )}

      {showAddMembers && (
        <div className="modal-overlay" onClick={() => setShowAddMembers(false)}>
          <div className="modal-content" onClick={e => e.stopPropagation()}>
            <h2>Añadir personas al canal</h2>
            <p className="hint">Haz clic en un usuario para añadirlo</p>
            <div className="users-list">
              {allUsers
                .filter(u => !isMember(u.id))
                .map(u => (
                  <div key={u.id} className="user-item" onClick={() => { addMember(u.id); }}>
                    <div className="user-avatar">{u.name?.charAt(0).toUpperCase()}</div>
                    <div className="user-info">
                      <span className="user-name">{u.name}</span>
                      <span className="user-email">{u.email}</span>
                    </div>
                  </div>
                ))}
            </div>
            <div className="modal-actions">
              <button onClick={() => setShowAddMembers(false)}>Cerrar</button>
            </div>
          </div>
        </div>
      )}

      {showPinnedMessages && (
        <div className="modal-overlay" onClick={() => setShowPinnedMessages(false)}>
          <div className="modal-content pinned" onClick={e => e.stopPropagation()}>
            <h2>📌 Mensajes fijados</h2>
            <div className="pinned-list">
              {pinnedMessages.length === 0 ? (
                <p className="no-pinned">No hay mensajes fijados</p>
              ) : (
                pinnedMessages.map(msg => (
                  <div key={msg.id} className="pinned-item">
                    <div className="pinned-header">
                      <span className="pinned-author">{msg.user?.name}</span>
                      <span className="pinned-time">{formatTime(msg.created_at)}</span>
                    </div>
                    <p className="pinned-text">{msg.content}</p>
                    {msg.user_id === user?.id && (
                      <button className="unpin-btn" onClick={() => { unpinMessage(msg.id); setShowPinnedMessages(false); }}>
                        Desfijar
                      </button>
                    )}
                  </div>
                ))
              )}
            </div>
            <div className="modal-actions">
              <button onClick={() => setShowPinnedMessages(false)}>Cerrar</button>
            </div>
          </div>
        </div>
      )}

      {showSearch && (
        <div className="modal-overlay" onClick={() => setShowSearch(false)}>
          <div className="modal-content search" onClick={e => e.stopPropagation()}>
            <h2>🔍 Buscar mensajes</h2>
            <div className="search-input-container">
              <input
                type="text"
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                placeholder="Escribe para buscar..."
                onKeyDown={(e) => e.key === 'Enter' && handleSearch()}
              />
              <button onClick={handleSearch}>Buscar</button>
            </div>
            <div className="search-results">
              {searchResults.map(msg => (
                <div key={msg.id} className="search-result-item" onClick={() => { setShowSearch(false); }}>
                  <div className="search-result-header">
                    <span className="search-result-author">{msg.user?.name}</span>
                    <span className="search-result-time">{formatTime(msg.created_at)}</span>
                  </div>
                  <p className="search-result-content">{msg.content}</p>
                </div>
              ))}
              {searchQuery && searchResults.length === 0 && (
                <p className="no-results">No se encontraron mensajes</p>
              )}
            </div>
            <div className="modal-actions">
              <button onClick={() => setShowSearch(false)}>Cerrar</button>
            </div>
          </div>
        </div>
      )}

      {showStarred && (
        <div className="modal-overlay" onClick={() => setShowStarred(false)}>
          <div className="modal-content starred" onClick={e => e.stopPropagation()}>
            <h2>⭐ Mensajes starred</h2>
            <div className="starred-list">
              {starredMessages.length === 0 ? (
                <p className="no-starred">No hay mensajes starred</p>
              ) : (
                starredMessages.map(msg => (
                  <div key={msg.id} className="starred-item">
                    <div className="starred-header">
                      <span className="starred-author">{msg.user?.name}</span>
                      <span className="starred-time">{formatTime(msg.created_at)}</span>
                    </div>
                    <p className="starred-text">{msg.content}</p>
                    {msg.user_id === user?.id && (
                      <button className="unstar-btn" onClick={() => { unstarMessage(msg.id); loadStarredMessages(); }}>
                        Quitar star
                      </button>
                    )}
                  </div>
                ))
              )}
            </div>
            <div className="modal-actions">
              <button onClick={() => setShowStarred(false)}>Cerrar</button>
            </div>
          </div>
        </div>
      )}

      {showThread && (
        <div className="modal-overlay" onClick={closeThread}>
          <div className="modal-content thread" onClick={e => e.stopPropagation()}>
            <h2>💬 Hilo</h2>
            <div className="thread-parent">
              <div className="message-header">
                <span className="message-author">{showThread.user?.name}</span>
                <span className="message-time">{formatTime(showThread.created_at)}</span>
              </div>
              <p className="message-text">{showThread.content}</p>
            </div>
            <div className="thread-replies">
              {threadReplies.map(reply => (
                <div key={reply.id} className="thread-reply">
                  <div className="message-header">
                    <span className="message-author">{reply.user?.name}</span>
                    <span className="message-time">{formatTime(reply.created_at)}</span>
                  </div>
                  <p className="message-text">{reply.content}</p>
                </div>
              ))}
            </div>
            <form className="thread-input" onSubmit={(e) => { e.preventDefault(); const input = e.target.querySelector('input') as HTMLInputElement; if(input.value.trim()) { sendThreadReply(input.value); input.value = ''; } }}>
              <input type="text" placeholder="Responder al hilo..." />
              <button type="submit">Enviar</button>
            </form>
            <div className="modal-actions">
              <button onClick={closeThread}>Cerrar</button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
