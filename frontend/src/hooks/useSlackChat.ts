import { useState, useEffect, useRef, useCallback } from 'react'
import { channelService, uploadService } from '../services/api'
import type { User } from '../types'
import type { Channel, Message, ChannelMember, MessageReaction, UserStatus } from '../types/chat'
import { playNotificationSound } from '../components/Chat/ChatUtils'
import { useConfirm } from '../components/ui/ConfirmProvider'
import { useNotification } from '../context/NotificationContext'

export function useSlackChat(user: User | null, companyId: number | null = null) {
  const [channels, setChannels] = useState<Channel[]>([])
  const [selectedChannel, setSelectedChannel] = useState<Channel | null>(null)
  const [messages, setMessages] = useState<Message[]>([])
  const [hasMoreMessages, setHasMoreMessages] = useState(false)
  const [loadingOlder, setLoadingOlder] = useState(false)
  const [pinnedMessages, setPinnedMessages] = useState<Message[]>([])
  const [newMessage, setNewMessage] = useState('')
  const [channelMembers, setChannelMembers] = useState<ChannelMember[]>([])
  const [allUsers, setAllUsers] = useState<User[]>([])
  const [isConnected, setIsConnected] = useState(false)
  const [typingUsers, setTypingUsers] = useState<Map<number, string>>(new Map())
  const [unreadCount, setUnreadCount] = useState(0)
  const [isRecording, setIsRecording] = useState(false)
  const [isPaused, setIsPaused] = useState(false)
  const [recordingStream, setRecordingStream] = useState<MediaStream | null>(null)
  const [recordedBlob, setRecordedBlob] = useState<Blob | null>(null)
  const [isUploading, setIsUploading] = useState(false)
  const [showThread, setShowThread] = useState<Message | null>(null)
  const [threadReplies, setThreadReplies] = useState<Message[]>([])
  const [starredMessages, setStarredMessages] = useState<Message[]>([])
  const [starredIds, setStarredIds] = useState<Set<number>>(new Set())
  const [userStatuses, setUserStatuses] = useState<Map<number, UserStatus['status']>>(new Map())
  
  const confirm = useConfirm()
  const { error: showError } = useNotification()

  const wsRef = useRef<WebSocket | null>(null)
  const reconnectAttemptsRef = useRef(0)
  const reconnectTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const closedByUsRef = useRef(false)
  const [connectionLost, setConnectionLost] = useState(false)
  const typingTimeoutRef = useRef<Map<number, ReturnType<typeof setTimeout>>>(new Map())
  const mediaRecorderRef = useRef<MediaRecorder | null>(null)
  const audioChunksRef = useRef<Blob[]>([])
  const recordingCancelledRef = useRef(false)
  const selectedChannelIdRef = useRef<number | null>(null)
  const userIdRef = useRef<number | null>(null)
  const messagesRef = useRef<Message[]>([])
  const threadRepliesRef = useRef<Message[]>([])

  useEffect(() => {
    messagesRef.current = messages
  }, [messages])

  useEffect(() => {
    threadRepliesRef.current = threadReplies
  }, [threadReplies])

  useEffect(() => {
    selectedChannelIdRef.current = selectedChannel?.id ?? null
  }, [selectedChannel?.id])

  useEffect(() => {
    userIdRef.current = user?.id ?? null
  }, [user?.id])

  const fetchChannels = useCallback(async () => {
    try {
      const data = await channelService.getChannels(companyId)
      setChannels(data || [])
    } catch (error) {
      console.error('Error fetching channels:', error)
    }
  }, [companyId])

  const fetchAllUsers = useCallback(async () => {
    try {
      const data = await channelService.getAllUsers(companyId)
      setAllUsers(data || [])
    } catch (error) {
      console.error('Error fetching users:', error)
    }
  }, [companyId])

  const PAGE_SIZE = 100

  const fetchMessages = useCallback(async (channelId: number) => {
    try {
      const data = await channelService.getMessages(channelId)
      setMessages(data || [])
      setHasMoreMessages((data || []).length >= PAGE_SIZE)
    } catch (error) {
      console.error('Error fetching messages:', error)
    }
  }, [])

  // Page into older history: fetch messages before the oldest one on screen.
  const loadOlderMessages = useCallback(async () => {
    const channelId = selectedChannelIdRef.current
    if (!channelId || loadingOlder) return
    setLoadingOlder(true)
    try {
      const oldestId = messagesRef.current.find(m => m.id > 0)?.id ?? 0
      if (!oldestId) return
      const older = await channelService.getMessages(channelId, oldestId)
      if (selectedChannelIdRef.current !== channelId) return
      setMessages(prev => [...(older || []), ...prev])
      setHasMoreMessages((older || []).length >= PAGE_SIZE)
    } catch (error) {
      console.error('Error loading older messages:', error)
    } finally {
      setLoadingOlder(false)
    }
  }, [loadingOlder])

  const fetchPinnedMessages = useCallback(async (channelId: number) => {
    try {
      const data = await channelService.getPinnedMessages(channelId)
      setPinnedMessages(data || [])
    } catch (error) {
      console.error('Error fetching pinned messages:', error)
    }
  }, [])

  const fetchChannelMembers = useCallback(async (channelId: number) => {
    try {
      const data = await channelService.getMembers(channelId)
      setChannelMembers(data || [])
    } catch (error) {
      console.error('Error fetching members:', error)
    }
  }, [])

  const fetchStarredMessages = useCallback(async () => {
    try {
      const data = await channelService.getStarredMessages()
      setStarredMessages(data || [])
      setStarredIds(new Set((data || []).map(m => m.id)))
    } catch (error) {
      console.error('Error fetching starred messages:', error)
    }
  }, [])

  // Apply a reaction change to whichever list holds the message (main list and thread panel).
  const applyReactionAdded = useCallback((messageId: number, reaction: MessageReaction) => {
    const update = (list: Message[]) => list.map(m => {
      if (m.id !== messageId) return m
      const exists = (m.reactions || []).some(r => r.user_id === reaction.user_id && r.emoji === reaction.emoji)
      return exists ? m : { ...m, reactions: [...(m.reactions || []), reaction] }
    })
    setMessages(update)
    setThreadReplies(update)
  }, [])

  const applyReactionRemoved = useCallback((messageId: number, userId: number, emoji: string) => {
    const update = (list: Message[]) => list.map(m =>
      m.id !== messageId
        ? m
        : { ...m, reactions: (m.reactions || []).filter(r => !(r.user_id === userId && r.emoji === emoji)) }
    )
    setMessages(update)
    setThreadReplies(update)
  }, [])

  const handleTyping = useCallback((userId: number, userName: string, channelId: number) => {
    if (channelId !== selectedChannelIdRef.current || userId === userIdRef.current) return
    setTypingUsers(prev => new Map(prev).set(userId, userName))
    if (typingTimeoutRef.current.has(userId)) clearTimeout(typingTimeoutRef.current.get(userId)!)
    const timeout = setTimeout(() => {
      setTypingUsers(prev => {
        const newMap = new Map(prev); newMap.delete(userId); return newMap
      })
    }, 3000)
    typingTimeoutRef.current.set(userId, timeout)
  }, [])

  const connectWebSocket = useCallback(() => {
    if (wsRef.current && wsRef.current.readyState !== WebSocket.CLOSED) return
    closedByUsRef.current = false

    // Auth travels via the httpOnly cookie on the same-origin WS handshake.
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const wsUrl = `${protocol}//${window.location.host}/ws/channels`
    const ws = new WebSocket(wsUrl)

    ws.onopen = () => {
      setIsConnected(true)
      setConnectionLost(false)
      if (reconnectAttemptsRef.current > 0) {
        // Recovered after a drop: re-sync whatever we missed while offline.
        fetchChannels()
        const channelId = selectedChannelIdRef.current
        if (channelId) {
          fetchMessages(channelId)
          fetchPinnedMessages(channelId)
        }
      }
      reconnectAttemptsRef.current = 0
    }
    ws.onmessage = (event) => {
      try {
        const msg = JSON.parse(event.data)
        const selectedChannelId = selectedChannelIdRef.current
        const currentUserId = userIdRef.current

        if (selectedChannelId !== null && msg.channel_id === selectedChannelId) {
          if (msg.type === 'message') {
            if (msg.user_id === currentUserId) return
            setMessages(prev => {
              const filtered = msg.data.tempId ? prev.filter(m => m.tempId !== msg.data.tempId) : prev
              return [...filtered, msg.data]
            })
            playNotificationSound()
            // Don't refetch the whole channel list per incoming message; the
            // local setChannels below already zeroes the active channel's badge.
            channelService.markAsRead(selectedChannelId).then(() => {
              window.dispatchEvent(new CustomEvent('chat-unread-updated'))
            })
            setChannels(prev => prev.map(c => c.id === selectedChannelId ? { ...c, unread_count: 0 } : c))
          } else if (msg.type === 'typing') {
            handleTyping(msg.user_id, msg.user_name, msg.channel_id)
          } else if (msg.type === 'message_pinned') {
            setMessages(prev => prev.map(m => m.id === msg.data.id ? { ...m, is_pinned: true } : m))
            fetchPinnedMessages(selectedChannelId)
          } else if (msg.type === 'message_unpinned') {
            setMessages(prev => prev.map(m => m.id === msg.data.id ? { ...m, is_pinned: false } : m))
            fetchPinnedMessages(selectedChannelId)
          } else if (msg.type === 'reaction_added') {
            applyReactionAdded(msg.data.message_id, msg.data.reaction)
          } else if (msg.type === 'reaction_removed') {
            applyReactionRemoved(msg.data.message_id, msg.data.user_id, msg.data.emoji)
          } else if (msg.type === 'message_edited') {
            setMessages(prev => prev.map(m => m.id === msg.data.id ? { ...m, ...msg.data, is_edited: true } : m))
          } else if (msg.type === 'message_deleted') {
            setMessages(prev => prev.map(m => m.id === msg.data.id ? { ...m, is_deleted: true, content: '[Mensaje eliminado]' } : m))
          } else if (msg.type === 'thread_reply') {
            setThreadReplies(prev => {
              if (prev.some(r => r.id === msg.data.id)) return prev
              return [...prev, msg.data]
            })
            if (msg.data.parent_id) {
              setMessages(prev => prev.map(m =>
                m.id === msg.data.parent_id
                  ? { ...m, reply_count: (m.reply_count || 0) + 1 }
                  : m
              ))
            }
          }
        } else if (msg.type === 'message' && msg.user_id !== currentUserId) {
          setUnreadCount(prev => prev + 1)
          playNotificationSound()
          setChannels(prev => prev.map(c => c.id === msg.channel_id ? { ...c, unread_count: (c.unread_count || 0) + 1 } : c))
          window.dispatchEvent(new CustomEvent('chat-unread-updated'))
        }
      } catch (e) { console.error('Error parsing message:', e) }
    }
    ws.onclose = () => {
      setIsConnected(false)
      if (closedByUsRef.current) return
      // Reconnect with exponential backoff: 1s, 2s, 4s, 8s, 16s, then 30s.
      setConnectionLost(true)
      const attempt = reconnectAttemptsRef.current++
      const delay = Math.min(30000, 1000 * 2 ** Math.min(attempt, 5))
      reconnectTimerRef.current = setTimeout(connectWebSocket, delay)
    }
    ws.onerror = () => setIsConnected(false)
    wsRef.current = ws
  }, [fetchChannels, fetchMessages, fetchPinnedMessages, handleTyping, applyReactionAdded, applyReactionRemoved])

  // Reset the open conversation when the company scope changes (superadmin).
  useEffect(() => {
    setSelectedChannel(null)
    setMessages([])
  }, [companyId])

  useEffect(() => {
    fetchChannels()
    fetchAllUsers()
    fetchStarredMessages()
    connectWebSocket()
    return () => {
      closedByUsRef.current = true
      if (reconnectTimerRef.current) clearTimeout(reconnectTimerRef.current)
      if (wsRef.current) wsRef.current.close()
    }
  }, [fetchChannels, fetchAllUsers, fetchStarredMessages, connectWebSocket])

  // Presence: report online/away/offline based on page visibility and chat lifetime.
  useEffect(() => {
    if (!user?.id) return
    channelService.updateStatus('online').catch(() => {})
    const onVisibility = () => {
      channelService.updateStatus(document.hidden ? 'away' : 'online').catch(() => {})
    }
    document.addEventListener('visibilitychange', onVisibility)
    return () => {
      document.removeEventListener('visibilitychange', onVisibility)
      channelService.updateStatus('offline').catch(() => {})
    }
  }, [user?.id])

  // Poll statuses of everyone visible in the chat (DM list, members, mentions).
  useEffect(() => {
    if (allUsers.length === 0) return
    const ids = allUsers.map(u => u.id)
    let active = true
    const load = async () => {
      try {
        const statuses = await channelService.getStatuses(ids)
        if (!active) return
        setUserStatuses(new Map((statuses || []).map(s => [s.user_id, s.status])))
      } catch (error) {
        console.error('Error fetching statuses:', error)
      }
    }
    load()
    const interval = setInterval(load, 30000)
    return () => { active = false; clearInterval(interval) }
  }, [allUsers])

  useEffect(() => {
    const selectedChannelId = selectedChannel?.id
    if (selectedChannelId) {
      fetchMessages(selectedChannelId)
      fetchChannelMembers(selectedChannelId)
      fetchPinnedMessages(selectedChannelId)
      channelService.markAsRead(selectedChannelId).then(() => {
        window.dispatchEvent(new CustomEvent('chat-unread-updated'))
        fetchChannels()
      })
      setChannels(prev => prev.map(c => c.id === selectedChannelId ? { ...c, unread_count: 0 } : c))
    }
  }, [selectedChannel?.id, fetchMessages, fetchChannelMembers, fetchPinnedMessages, fetchChannels])

  const sendMessage = useCallback((attachment?: { url: string; filename: string }, tempId?: string, contentOverride?: string) => {
    const textToSend = contentOverride !== undefined ? contentOverride : newMessage
    if ((!textToSend.trim() && !attachment) || !selectedChannel) return
    const content = attachment ? `${textToSend} [Archivo: ${attachment.filename}]` : textToSend
    channelService.sendMessage(selectedChannel.id, { content, attachment: attachment?.url, file_name: attachment?.filename })
      .then(msg => {
        setMessages(prev => prev.filter(m => m.tempId !== tempId).concat([{ ...msg }]))
        setNewMessage('')
      }).catch(err => {
        console.error('Error sending message:', err)
        setMessages(prev => prev.filter(m => m.tempId !== tempId))
      })
  }, [selectedChannel, newMessage])

  const sendTypingIndicator = useCallback(() => {
    if (!wsRef.current || !isConnected || !selectedChannel) return
    wsRef.current.send(JSON.stringify({ type: 'typing', channel_id: selectedChannel.id, user_name: user?.name }))
  }, [isConnected, selectedChannel, user?.name])

  const startRecording = async () => {
    try {
      const stream = await navigator.mediaDevices.getUserMedia({ audio: true })
      const mediaRecorder = new MediaRecorder(stream)
      audioChunksRef.current = []
      recordingCancelledRef.current = false
      setRecordedBlob(null)
      mediaRecorder.ondataavailable = (e) => { if (e.data.size > 0) audioChunksRef.current.push(e.data) }
      // Stopping no longer sends: it produces a blob the user can preview first.
      mediaRecorder.onstop = () => {
        const audioBlob = new Blob(audioChunksRef.current, { type: 'audio/webm' })
        stream.getTracks().forEach(track => track.stop())
        setRecordingStream(null)
        setIsPaused(false)
        if (recordingCancelledRef.current) { recordingCancelledRef.current = false; return }
        setRecordedBlob(audioBlob)
      }
      mediaRecorderRef.current = mediaRecorder
      mediaRecorder.start()
      setRecordingStream(stream)
      setIsRecording(true)
      setIsPaused(false)
    } catch (e) { console.error('Error starting recording:', e) }
  }

  const pauseRecording = () => {
    if (mediaRecorderRef.current && isRecording && !isPaused) {
      mediaRecorderRef.current.pause()
      setIsPaused(true)
    }
  }

  const resumeRecording = () => {
    if (mediaRecorderRef.current && isRecording && isPaused) {
      mediaRecorderRef.current.resume()
      setIsPaused(false)
    }
  }

  const stopRecording = () => { if (mediaRecorderRef.current && isRecording) { mediaRecorderRef.current.stop(); setIsRecording(false) } }

  const cancelRecording = () => {
    if (mediaRecorderRef.current && isRecording) {
      recordingCancelledRef.current = true
      mediaRecorderRef.current.stop()
      setIsRecording(false)
    }
    setRecordedBlob(null)
  }

  const discardRecording = () => setRecordedBlob(null)

  const sendRecording = async () => {
    if (!recordedBlob || !selectedChannel || !user) return
    const blob = recordedBlob
    setRecordedBlob(null)
    try {
      setIsUploading(true)
      const result = await uploadService.upload(blob as any)
      const tempId = `temp-${Date.now()}`
      const optimisticMsg: Message = { id: 0, channel_id: selectedChannel.id, user_id: user.id, content: '[Nota de voz]', tempId, attachment: result.url, file_name: 'Nota de voz', file_type: 'audio/webm', created_at: new Date().toISOString(), user }
      setMessages(prev => [...prev, optimisticMsg])
      sendMessage({ url: result.url, filename: 'Nota de voz' }, tempId)
    } catch (error) { console.error('Error uploading voice note:', error) }
    finally { setIsUploading(false) }
  }

  const editMessage = async (id: number, content: string) => {
    if (!selectedChannel) return
    // Snapshot for rollback if the request fails.
    const prevMessages = messagesRef.current
    const prevThreadReplies = threadRepliesRef.current
    const apply = (list: Message[]) => list.map(m => m.id === id ? { ...m, content, is_edited: true } : m)
    setMessages(apply)
    setThreadReplies(apply)
    try {
      await channelService.editMessage(selectedChannel.id, id, content)
    } catch (e) {
      console.error(e)
      setMessages(prevMessages)
      setThreadReplies(prevThreadReplies)
      showError('No se pudo editar el mensaje. Intenta de nuevo.')
    }
  }

  const deleteMessage = async (id: number) => {
    if (!selectedChannel) return
    const ok = await confirm({
      title: 'Eliminar mensaje',
      message: '¿Eliminar este mensaje?',
      confirmLabel: 'Eliminar',
      variant: 'danger',
    })
    if (!ok) return
    // Snapshot for rollback if the request fails.
    const prevMessages = messagesRef.current
    const prevThreadReplies = threadRepliesRef.current
    const apply = (list: Message[]) => list.map(m => m.id === id ? { ...m, is_deleted: true, content: '[Mensaje eliminado]' } : m)
    setMessages(apply)
    setThreadReplies(apply)
    try {
      await channelService.deleteMessage(selectedChannel.id, id)
    } catch (e) {
      console.error(e)
      setMessages(prevMessages)
      setThreadReplies(prevThreadReplies)
      showError('No se pudo eliminar el mensaje. Intenta de nuevo.')
    }
  }

  const toggleReaction = async (message: Message, emoji: string) => {
    if (!selectedChannel || !user) return
    const mine = (message.reactions || []).some(r => r.user_id === user.id && r.emoji === emoji)
    try {
      if (mine) {
        applyReactionRemoved(message.id, user.id, emoji)
        await channelService.removeReaction(selectedChannel.id, message.id, emoji)
      } else {
        const reaction = await channelService.addReaction(selectedChannel.id, message.id, emoji)
        applyReactionAdded(message.id, reaction)
      }
    } catch (e) { console.error('Error toggling reaction:', e) }
  }

  const starMessage = async (id: number) => {
    try {
      await channelService.starMessage(id)
      setStarredIds(prev => new Set(prev).add(id))
      fetchStarredMessages()
    } catch (e) { console.error('Error starring message:', e) }
  }

  const unstarMessage = async (id: number) => {
    try {
      await channelService.unstarMessage(id)
      setStarredIds(prev => { const next = new Set(prev); next.delete(id); return next })
      setStarredMessages(prev => prev.filter(m => m.id !== id))
    } catch (e) { console.error('Error unstarring message:', e) }
  }

  const pinMessage = async (id: number) => {
    if (selectedChannel) {
      try { await channelService.pinMessage(selectedChannel.id, id); setMessages(prev => prev.map(m => m.id === id ? { ...m, is_pinned: true } : m)); fetchPinnedMessages(selectedChannel.id) } catch (e) { console.error(e) }
    }
  }

  const unpinMessage = async (id: number) => {
    if (selectedChannel) {
      try { await channelService.unpinMessage(selectedChannel.id, id); setMessages(prev => prev.map(m => m.id === id ? { ...m, is_pinned: false } : m)); fetchPinnedMessages(selectedChannel.id) } catch (e) { console.error(e) }
    }
  }

  return {
    channels, selectedChannel, setSelectedChannel,
    messages, setMessages, pinnedMessages,
    hasMoreMessages, loadingOlder, loadOlderMessages,
    newMessage, setNewMessage,
    channelMembers, fetchChannelMembers,
    allUsers, isConnected, connectionLost, unreadCount, setUnreadCount,
    typingUsers, isRecording, isPaused, recordingStream, recordedBlob,
    pauseRecording, resumeRecording, cancelRecording, discardRecording, sendRecording,
    isUploading, setIsUploading,
    showThread, setShowThread, threadReplies, setThreadReplies,
    sendMessage, sendTypingIndicator, startRecording, stopRecording,
    editMessage, deleteMessage, pinMessage, unpinMessage, fetchChannels, fetchAllUsers,
    toggleReaction, starMessage, unstarMessage, starredMessages, starredIds, fetchStarredMessages,
    userStatuses
  }
}
