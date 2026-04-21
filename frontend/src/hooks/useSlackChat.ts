import { useState, useEffect, useRef, useCallback } from 'react'
import { channelService, uploadService } from '../services/api'
import type { User } from '../types'
import type { Channel, Message, ChannelMember } from '../types/chat'
import { playNotificationSound } from '../components/Chat/ChatUtils'

export function useSlackChat(user: User | null, token: string | null) {
  const [channels, setChannels] = useState<Channel[]>([])
  const [selectedChannel, setSelectedChannel] = useState<Channel | null>(null)
  const [messages, setMessages] = useState<Message[]>([])
  const [pinnedMessages, setPinnedMessages] = useState<Message[]>([])
  const [newMessage, setNewMessage] = useState('')
  const [channelMembers, setChannelMembers] = useState<ChannelMember[]>([])
  const [allUsers, setAllUsers] = useState<User[]>([])
  const [isConnected, setIsConnected] = useState(false)
  const [typingUsers, setTypingUsers] = useState<Map<number, string>>(new Map())
  const [unreadCount, setUnreadCount] = useState(0)
  const [isRecording, setIsRecording] = useState(false)
  const [isUploading, setIsUploading] = useState(false)
  const [showThread, setShowThread] = useState<Message | null>(null)
  const [threadReplies, setThreadReplies] = useState<Message[]>([])
  
  const wsRef = useRef<WebSocket | null>(null)
  const typingTimeoutRef = useRef<Map<number, ReturnType<typeof setTimeout>>>(new Map())
  const mediaRecorderRef = useRef<MediaRecorder | null>(null)
  const audioChunksRef = useRef<Blob[]>([])

  const fetchChannels = useCallback(async () => {
    try {
      const data = await channelService.getChannels()
      setChannels(data || [])
    } catch (error) {
      console.error('Error fetching channels:', error)
    }
  }, [])

  const fetchAllUsers = useCallback(async () => {
    try {
      const data = await channelService.getAllUsers()
      setAllUsers(data || [])
    } catch (error) {
      console.error('Error fetching users:', error)
    }
  }, [])

  const fetchMessages = useCallback(async (channelId: number) => {
    try {
      const data = await channelService.getMessages(channelId)
      setMessages(data || [])
    } catch (error) {
      console.error('Error fetching messages:', error)
    }
  }, [])

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

  const handleTyping = useCallback((userId: number, userName: string, channelId: number) => {
    if (channelId !== selectedChannel?.id || userId === user?.id) return
    setTypingUsers(prev => new Map(prev).set(userId, userName))
    if (typingTimeoutRef.current.has(userId)) clearTimeout(typingTimeoutRef.current.get(userId)!)
    const timeout = setTimeout(() => {
      setTypingUsers(prev => {
        const newMap = new Map(prev); newMap.delete(userId); return newMap
      })
    }, 3000)
    typingTimeoutRef.current.set(userId, timeout)
  }, [selectedChannel?.id, user?.id])

  const connectWebSocket = useCallback(() => {
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
            channelService.markAsRead(selectedChannel!.id).then(() => {
              window.dispatchEvent(new CustomEvent('chat-unread-updated'))
              fetchChannels()
            })
            setChannels(prev => prev.map(c => c.id === selectedChannel!.id ? { ...c, unread_count: 0 } : c))
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
          window.dispatchEvent(new CustomEvent('chat-unread-updated'))
        }
      } catch (e) { console.error('Error parsing message:', e) }
    }
    ws.onclose = () => setIsConnected(false)
    ws.onerror = () => setIsConnected(false)
    wsRef.current = ws
  }, [token, selectedChannel, user?.id, handleTyping, fetchPinnedMessages])

  useEffect(() => {
    fetchChannels()
    fetchAllUsers()
    return () => { if (wsRef.current) wsRef.current.close() }
  }, [fetchChannels, fetchAllUsers])

  useEffect(() => {
    if (selectedChannel) {
      fetchMessages(selectedChannel.id)
      fetchChannelMembers(selectedChannel.id)
      fetchPinnedMessages(selectedChannel.id)
      connectWebSocket()
      channelService.markAsRead(selectedChannel.id).then(() => {
        window.dispatchEvent(new CustomEvent('chat-unread-updated'))
        fetchChannels()
      })
      setChannels(prev => prev.map(c => c.id === selectedChannel.id ? { ...c, unread_count: 0 } : c))
    }
  }, [selectedChannel, fetchMessages, fetchChannelMembers, fetchPinnedMessages, connectWebSocket])

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
      mediaRecorder.ondataavailable = (e) => { if (e.data.size > 0) audioChunksRef.current.push(e.data) }
      mediaRecorder.onstop = async () => {
        const audioBlob = new Blob(audioChunksRef.current, { type: 'audio/webm' })
        stream.getTracks().forEach(track => track.stop())
        try {
          const result = await uploadService.upload(audioBlob as any)
          const tempId = `temp-${Date.now()}`
          const optimisticMsg: Message = { id: 0, channel_id: selectedChannel!.id, user_id: user!.id, content: '[Nota de voz]', tempId, attachment: result.url, file_name: 'Nota de voz', file_type: 'audio/webm', created_at: new Date().toISOString(), user: user! }
          setMessages(prev => [...prev, optimisticMsg])
          sendMessage({ url: result.url, filename: 'Nota de voz' }, tempId)
        } catch (error) { console.error('Error uploading voice note:', error) }
      }
      mediaRecorderRef.current = mediaRecorder; mediaRecorder.start(); setIsRecording(true)
    } catch (e) { console.error('Error starting recording:', e) }
  }

  const stopRecording = () => { if (mediaRecorderRef.current && isRecording) { mediaRecorderRef.current.stop(); setIsRecording(false) } }

  const editMessage = async (id: number, content: string) => {
    if (!selectedChannel) return
    setMessages(prev => prev.map(m => m.id === id ? { ...m, content, is_edited: true } : m))
    try { await channelService.editMessage(selectedChannel.id, id, content) } catch (e) { console.error(e) }
  }

  const deleteMessage = async (id: number) => {
    if (selectedChannel && confirm('¿Eliminar mensaje?')) {
      setMessages(prev => prev.map(m => m.id === id ? { ...m, is_deleted: true, content: '[Mensaje eliminado]' } : m))
      try { await channelService.deleteMessage(selectedChannel.id, id) } catch (e) { console.error(e) }
    }
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
    newMessage, setNewMessage,
    channelMembers, fetchChannelMembers,
    allUsers, isConnected, unreadCount, setUnreadCount,
    typingUsers, isRecording, isUploading, setIsUploading,
    showThread, setShowThread, threadReplies, setThreadReplies,
    sendMessage, sendTypingIndicator, startRecording, stopRecording,
    editMessage, deleteMessage, pinMessage, unpinMessage, fetchChannels
  }
}
