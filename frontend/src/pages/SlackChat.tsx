import { useState, useEffect, useRef, useCallback } from 'react'
import { useAuth } from '../context/AuthContext'
import { channelService, uploadService } from '../services/api'
import type { User } from '../types'
import type { Channel, Message, ChannelMember } from '../types/chat'
import { Sidebar } from '../components/Chat/Sidebar'
import { ChatHeader } from '../components/Chat/ChatHeader'
import { MessageList } from '../components/Chat/MessageList'
import { MessageInput } from '../components/Chat/MessageInput'
import { ThreadPanel } from '../components/Chat/ThreadPanel'
import { NewChannelModal } from '../components/Chat/Modals/NewChannelModal'
import { ChannelSettingsModal } from '../components/Chat/Modals/ChannelSettingsModal'
import { AddMembersModal } from '../components/Chat/Modals/AddMembersModal'
import { PinnedMessagesModal } from '../components/Chat/Modals/PinnedMessagesModal'
import './SlackChat.css'

// Singleton para AudioContext
let globalAudioContext: AudioContext | null = null

export default function SlackChat() {
  const { user, token } = useAuth()
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
  const [isUploading, setIsUploading] = useState(false)
  const [newChannel, setNewChannel] = useState({ name: '', description: '', type: 'public' as 'public' | 'private' })
  const typingTimeoutRef = useRef<Map<number, ReturnType<typeof setTimeout>>>(new Map())
  const mediaRecorderRef = useRef<MediaRecorder | null>(null)
  const audioChunksRef = useRef<Blob[]>([])
  const audioRef = useRef<HTMLAudioElement | null>(null)
  const [unreadCount, setUnreadCount] = useState(0)
  const [editingMessageId, setEditingMessageId] = useState<number | null>(null)
  const [editContent, setEditContent] = useState('')
  const [showThread, setShowThread] = useState<Message | null>(null)
  const [threadReplies, setThreadReplies] = useState<Message[]>([])
  const [showMentionDropdown, setShowMentionDropdown] = useState(false)
  const [mentionFilterUsers, setMentionFilterUsers] = useState<User[]>([])
  const [originalFavicon, setOriginalFavicon] = useState<string | null>(null)

  useEffect(() => {
    fetchChannels()
    fetchAllUsers()

    const icon = document.querySelector("link[rel*='icon']") as HTMLLinkElement
    if (icon) {
      setOriginalFavicon(icon.href)
    }

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
      
      // Mark as read when entering
      channelService.markAsRead(selectedChannel.id)
      setChannels(prev => prev.map(c => c.id === selectedChannel.id ? { ...c, unread_count: 0 } : c))
    }
  }, [selectedChannel])

  useEffect(() => {
    scrollToBottom()
  }, [messages])

  useEffect(() => {
    const icon = document.querySelector("link[rel*='icon']") as HTMLLinkElement
    if (!icon) return

    if (unreadCount > 0) {
      icon.href = `data:image/svg+xml,<svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 100 100'><text y='.9em' font-size='90'>🔴</text></svg>`
    } else if (originalFavicon) {
      icon.href = originalFavicon
    }
  }, [unreadCount, originalFavicon])

  const playNotificationSound = () => {
    try {
      if (!globalAudioContext) {
        globalAudioContext = new (window.AudioContext || (window as any).webkitAudioContext)()
      }

      const audioContext = globalAudioContext
      if (audioContext.state === 'suspended') {
        audioContext.resume()
      }

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
    } catch (e) {
      console.error('Error playing notification sound:', e)
    }
  }

  const highlightMentions = (content: string): React.ReactNode => {
    const mentionRegex = /@(\w+)/g
    const parts: React.ReactNode[] = []
    let lastIndex = 0
    let match

    while ((match = mentionRegex.exec(content)) !== null) {
      if (match.index > lastIndex) {
        parts.push(content.slice(lastIndex, match.index))
      }
      const mentionedName = match[1].toLowerCase()
      const mentionedUser = allUsers.find(u => u.name.toLowerCase().replace(/\s+/g, '') === mentionedName)
      if (mentionedUser) {
        parts.push(<span key={match.index} className="mention-highlight">@{match[1]}</span>)
      } else {
        parts.push(`@${match[1]}`)
      }
      lastIndex = mentionRegex.lastIndex
    }

    if (lastIndex < content.length) {
      parts.push(content.slice(lastIndex))
    }

    return parts.length > 0 ? parts : content
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
    if (wsRef.current) wsRef.current.close()

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const wsUrl = `${protocol}//${window.location.host}/ws/channels?token=${token}`
    const ws = new WebSocket(wsUrl)

    ws.onopen = () => setIsConnected(true)
    ws.onmessage = (event) => {
      try {
        const msg = JSON.parse(event.data)
        if (msg.channel_id === selectedChannel?.id) {
          if (msg.type === 'message') {
            if (msg.user_id === user?.id) return
            setMessages(prev => {
              const filtered = msg.data.tempId ? prev.filter(m => m.tempId !== msg.data.tempId) : prev
              return [...filtered, msg.data]
            })
            playNotificationSound()
            channelService.markAsRead(selectedChannel!.id)
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

          } else if (msg.type === 'thread_reply') {
            setThreadReplies(prev => [...prev, msg.data])
          }
        } else if (msg.type === 'message' && msg.user_id !== user?.id) {
          setUnreadCount(prev => prev + 1)
          playNotificationSound()
          setChannels(prev => prev.map(c => c.id === msg.channel_id ? { ...c, unread_count: (c.unread_count || 0) + 1 } : c))
        }
      } catch (e) { console.error('Error parsing message:', e) }
    }
    ws.onclose = () => setIsConnected(false)
    ws.onerror = () => setIsConnected(false)
    wsRef.current = ws
  }

  const handleTyping = (userId: number, userName: string, channelId: number) => {
    if (channelId !== selectedChannel?.id || userId === user?.id) return
    setTypingUsers(prev => new Map(prev).set(userId, userName))
    if (typingTimeoutRef.current.has(userId)) clearTimeout(typingTimeoutRef.current.get(userId)!)
    const timeout = setTimeout(() => {
      setTypingUsers(prev => {
        const newMap = new Map(prev); newMap.delete(userId); return newMap
      })
    }, 3000)
    typingTimeoutRef.current.set(userId, timeout)
  }

  const sendTypingIndicator = () => {
    if (!wsRef.current || !isConnected || !selectedChannel) return
    wsRef.current.send(JSON.stringify({ type: 'typing', channel_id: selectedChannel.id, user_name: user?.name }))
  }

  const scrollToBottom = () => messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' })

  const formatTime = (dateString: string) => {
    const date = new Date(dateString)
    const now = new Date()
    const diff = now.getTime() - date.getTime()
    if (diff < 3600000) return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
    if (diff < 86400000) return `Hoy ${date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}`
    return date.toLocaleDateString('es-ES', { day: 'numeric', month: 'short' })
  }

  const sendMessage = useCallback((_e?: React.FormEvent, attachment?: { url: string; filename: string }, tempId?: string) => {
    if ((!newMessage.trim() && !attachment) || !selectedChannel) return
    const content = attachment ? `${newMessage} [Archivo: ${attachment.filename}]` : newMessage
    channelService.sendMessage(selectedChannel.id, { content, attachment: attachment?.url, file_name: attachment?.filename })
      .then(msg => {
        setMessages(prev => prev.filter(m => m.tempId !== tempId).concat([{ ...msg }]))
        setNewMessage('')
      }).catch(err => {
        console.error('Error sending message:', err)
        setMessages(prev => prev.filter(m => m.tempId !== tempId))
      })
  }, [selectedChannel, newMessage])

  const handleInputKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      if (newMessage.trim()) {
        const tempId = `temp-${Date.now()}`
        const optimisticMsg: Message = { id: 0, channel_id: selectedChannel!.id, user_id: user!.id, content: newMessage, tempId, created_at: new Date().toISOString(), user: user! }
        setMessages(prev => [...prev, optimisticMsg])
        sendMessage(undefined, undefined, tempId)
      }
    }

    // Handle mentions
    if (e.key === '@') {
      setMentionFilterUsers(allUsers)
      setShowMentionDropdown(true)
    } else if (showMentionDropdown && (e.key === 'Escape' || e.key === ' ')) {
      setShowMentionDropdown(false)
    }
  }

  const handleFileUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return
    setIsUploading(true)
    try {
      const result = await uploadService.upload(file)
      const tempId = `temp-${Date.now()}`
      const optimisticMsg: Message = { id: 0, channel_id: selectedChannel!.id, user_id: user!.id, content: `[Archivo: ${file.name}]`, tempId, created_at: new Date().toISOString(), user: user! }
      setMessages(prev => [...prev, optimisticMsg])
      sendMessage(undefined, { url: result.url, filename: file.name }, tempId)
    } catch (error) { console.error('Error uploading file:', error) }
    finally { setIsUploading(false); e.target.value = '' }
  }

  const startRecording = async () => {
    try {
      const stream = await navigator.mediaDevices.getUserMedia({ audio: true })
      const mediaRecorder = new MediaRecorder(stream)
      audioChunksRef.current = []
      mediaRecorder.ondataavailable = (e) => { if (e.data.size > 0) audioChunksRef.current.push(e.data) }
      mediaRecorder.onstop = async () => {
        const audioBlob = new Blob(audioChunksRef.current, { type: 'audio/webm' })
        stream.getTracks().forEach(track => track.stop())
        try {
          const result = await uploadService.upload(audioBlob as any)
          const tempId = `temp-${Date.now()}`
          const optimisticMsg: Message = { id: 0, channel_id: selectedChannel!.id, user_id: user!.id, content: '[Nota de voz]', tempId, attachment: result.url, file_name: 'Nota de voz', file_type: 'audio/webm', created_at: new Date().toISOString(), user: user! }
          setMessages(prev => [...prev, optimisticMsg])
          sendMessage(undefined, { url: result.url, filename: 'Nota de voz' }, tempId)
        } catch (error) { console.error('Error uploading voice note:', error) }
      }
      mediaRecorderRef.current = mediaRecorder; mediaRecorder.start(); setIsRecording(true)
    } catch (e) { console.error('Error starting recording:', e) }
  }

  const stopRecording = () => { if (mediaRecorderRef.current && isRecording) { mediaRecorderRef.current.stop(); setIsRecording(false) } }

  const togglePlayAudio = (url: string) => {
    if (playingAudio === url) { audioRef.current?.pause(); setPlayingAudio(null) }
    else { if (audioRef.current) audioRef.current.pause(); audioRef.current = new Audio(url); audioRef.current.onended = () => setPlayingAudio(null); audioRef.current.play(); setPlayingAudio(url) }
  }



  const handleSaveEdit = async () => {
    if (!selectedChannel || !editingMessageId || !editContent.trim()) return
    const content = editContent; const id = editingMessageId
    setMessages(prev => prev.map(m => m.id === id ? { ...m, content, is_edited: true } : m))
    setEditingMessageId(null); setEditContent('')
    try { await channelService.editMessage(selectedChannel.id, id, content) } catch (e) { console.error(e) }
  }

  const deleteMessage = async (id: number) => { if (selectedChannel && confirm('¿Eliminar mensaje?')) { setMessages(prev => prev.map(m => m.id === id ? { ...m, is_deleted: true, content: '[Mensaje eliminado]' } : m)); try { await channelService.deleteMessage(selectedChannel.id, id) } catch (e) { console.error(e) } } }
  const openThread = async (msg: Message) => { if (!selectedChannel) return; setShowThread(msg); try { const replies = await channelService.getThreadReplies(selectedChannel.id, msg.id); setThreadReplies(replies || []) } catch (e) { console.error(e) } }
  const sendThreadReply = async (content: string) => { if (selectedChannel && showThread && content.trim()) try { const reply = await channelService.sendThreadReply(selectedChannel.id, showThread.id, content); setThreadReplies(prev => [...prev, reply]) } catch (e) { console.error(e) } }

  const leaveChannel = async (id: number) => { try { await channelService.leaveChannel(id); if (selectedChannel?.id === id) setSelectedChannel(null); fetchChannels() } catch (e) { console.error(e) } }
  const addMember = async (id: number) => { if (selectedChannel) try { await channelService.addMember(selectedChannel.id, id); fetchChannelMembers(selectedChannel.id) } catch (e) { console.error(e) } }
  const removeMember = async (id: number) => { if (selectedChannel) try { await channelService.removeMember(selectedChannel.id, id); fetchChannelMembers(selectedChannel.id) } catch (e) { console.error(e) } }
  const pinMessage = async (id: number) => { if (selectedChannel) try { await channelService.pinMessage(selectedChannel.id, id); setMessages(prev => prev.map(m => m.id === id ? { ...m, is_pinned: true } : m)); fetchPinnedMessages(selectedChannel.id) } catch (e) { console.error(e) } }
  const unpinMessage = async (id: number) => { if (selectedChannel) try { await channelService.unpinMessage(selectedChannel.id, id); setMessages(prev => prev.map(m => m.id === id ? { ...m, is_pinned: false } : m)); fetchPinnedMessages(selectedChannel.id) } catch (e) { console.error(e) } }

  const handleMouseDown = (e: React.MouseEvent) => {
    e.preventDefault(); setIsResizing(true)
    const onMouseMove = (e: MouseEvent) => { setChatSidebarWidth(Math.max(150, Math.min(400, e.clientX))) }
    const onMouseUp = () => { setIsResizing(false); document.removeEventListener('mousemove', onMouseMove); document.removeEventListener('mouseup', onMouseUp) }
    document.addEventListener('mousemove', onMouseMove); document.addEventListener('mouseup', onMouseUp)
  }

  return (
    <div className="chat-page">
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
      />

      <div className="chat-body">
        <Sidebar
          channels={channels}
          selectedChannel={selectedChannel}
          setSelectedChannel={setSelectedChannel}
          showMobileChannels={showMobileChannels}
          setShowMobileChannels={setShowMobileChannels}
          chatSidebarCollapsed={chatSidebarCollapsed}
          setChatSidebarCollapsed={setChatSidebarCollapsed}
          chatSidebarWidth={chatSidebarWidth}
          setShowNewChannelModal={setShowNewChannelModal}
          onMouseDownResize={handleMouseDown}
          isResizing={isResizing}
        />

        <div className="chat-main">
          {selectedChannel ? (
            <>
              <MessageList
                messages={messages}
                currentUser={user}
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

                playingAudio={playingAudio}
                togglePlayAudio={togglePlayAudio}
                formatTime={formatTime}
                highlightMentions={highlightMentions}
                messagesEndRef={messagesEndRef}
                typingArray={Array.from(typingUsers.values())}
              />

              <MessageInput
                newMessage={newMessage}
                setNewMessage={setNewMessage}
                onKeyDown={handleInputKeyDown}
                onFileUpload={handleFileUpload}
                isUploading={isUploading}
                isRecording={isRecording}
                startRecording={startRecording}
                stopRecording={stopRecording}
                sendTypingIndicator={sendTypingIndicator}
                onSend={() => {
                  if (newMessage.trim()) {
                    const tempId = `temp-${Date.now()}`
                    const optimisticMsg: Message = { id: 0, channel_id: selectedChannel!.id, user_id: user!.id, content: newMessage, tempId, created_at: new Date().toISOString(), user: user! }
                    setMessages(prev => [...prev, optimisticMsg])
                    sendMessage(undefined, undefined, tempId)
                  }
                }}
              />

              {showMentionDropdown && (
                <div className="mention-dropdown">
                  {mentionFilterUsers.map(u => (
                    <div key={u.id} className="mention-user-item" onClick={() => {
                      const lastAt = newMessage.lastIndexOf('@')
                      setNewMessage(newMessage.slice(0, lastAt + 1) + u.name + ' ')
                      setShowMentionDropdown(false)
                    }}>
                      <span className="mention-user-avatar">{u.name.charAt(0).toUpperCase()}</span>
                      <span className="mention-user-name">{u.name}</span>
                    </div>
                  ))}
                </div>
              )}
            </>
          ) : (
            <div className="no-channel-selected">
              <div className="no-channel-icon">#</div>
              <h2>Selecciona un canal</h2>
              <p>Elige un canal de la lista para empezar a chatear</p>
              <button onClick={() => setShowNewChannelModal(true)} className="create-first-channel">+ Crear tu primer canal</button>
            </div>
          )}
        </div>
      </div>

      {showNewChannelModal && (
        <NewChannelModal
          newChannel={newChannel}
          setNewChannel={setNewChannel}
          onClose={() => setShowNewChannelModal(false)}
          onCreate={async () => {
            if (!newChannel.name.trim()) return
            try { await channelService.createChannel(newChannel); setShowNewChannelModal(false); setNewChannel({ name: '', description: '', type: 'public' }); fetchChannels() } catch (e) { console.error(e) }
          }}
        />
      )}

      {showChannelSettings && selectedChannel && (
        <ChannelSettingsModal
          selectedChannel={selectedChannel}
          channelMembers={channelMembers}
          currentUser={user}
          onClose={() => setShowChannelSettings(false)}
          onRemoveMember={removeMember}
          onLeaveChannel={leaveChannel}
          onShowAddMembers={() => setShowAddMembers(true)}
        />
      )}

      {showAddMembers && (
        <AddMembersModal
          allUsers={allUsers}
          isMember={(id) => channelMembers.some(m => m.id === id)}
          onAddMember={addMember}
          onClose={() => setShowAddMembers(false)}
        />
      )}

      {showPinnedMessages && (
        <PinnedMessagesModal
          pinnedMessages={pinnedMessages}
          currentUser={user}
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
