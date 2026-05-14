import { useState, useCallback, useEffect, useRef } from 'react'
import { channelService } from '../services/api'
import type { User, Message } from '../types'

interface UseChatReturn {
  // Data
  messages: Message[]
  users: User[]
  selectedUser: User | null
  isLoading: boolean

  // Actions
  setSelectedUser: (user: User | null) => void
  sendMessage: (content: string) => Promise<void>
  fetchMessages: () => Promise<void>
  fetchUsers: () => Promise<void>
  markAsRead: () => Promise<void>

  // WebSocket
  connectWebSocket: () => void
  disconnectWebSocket: () => void
}

export function useChat(): UseChatReturn {
  const [messages, setMessages] = useState<Message[]>([])
  const [users, setUsers] = useState<User[]>([])
  const [selectedUser, setSelectedUser] = useState<User | null>(null)
  const [isLoading, setIsLoading] = useState(true)
  const wsRef = useRef<WebSocket | null>(null)
  const reconnectTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  const fetchMessages = useCallback(async () => {
    try {
      setIsLoading(true)
      // Get general channel messages (channel ID 1 as default)
      const data = await channelService.getMessages(1)
      setMessages(data || [])
    } catch (error) {
      console.error('Error fetching messages:', error)
    } finally {
      setIsLoading(false)
    }
  }, [])

  const fetchUsers = useCallback(async () => {
    try {
      const data = await channelService.getAllUsers()
      setUsers(data || [])
    } catch (error) {
      console.error('Error fetching users:', error)
    }
  }, [])

  const markAsRead = useCallback(async () => {
    try {
      await channelService.markAsRead(1) // Channel ID 1 as default
      // Notify Layout to refresh unread count
      window.dispatchEvent(new CustomEvent('chat-unread-updated'))
    } catch (error) {
      console.error('Error marking as read:', error)
    }
  }, [])

  const sendMessage = useCallback(async (content: string) => {
    if (!content.trim()) return
    try {
      await channelService.sendMessage(1, { content })
      fetchMessages()
    } catch (error) {
      console.error('Error sending message:', error)
    }
  }, [fetchMessages])

  const connectWebSocket = useCallback(() => {
    if (wsRef.current?.readyState === WebSocket.OPEN) return

    const token = localStorage.getItem('token')
    if (!token) return

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const wsUrl = `${protocol}//${window.location.host}/ws/channels?token=${token}`
    const ws = new WebSocket(wsUrl)

    ws.onopen = () => {
      console.log('Chat WebSocket connected')
      if (reconnectTimeoutRef.current) {
        clearTimeout(reconnectTimeoutRef.current)
        reconnectTimeoutRef.current = null
      }
    }

    ws.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data)
        if (data.type === 'message' || data.type === 'chat_message') {
          setMessages(prev => [...prev, {
            ...data.data,
            created_at: new Date().toISOString(),
          }])
        }
        if (data.type === 'user_status') {
          setUsers(prev => prev.map(u =>
            u.id === data.user_id ? { ...u, is_online: data.status === 'online' } : u
          ))
        }
      } catch (err) {
        console.error('Error parsing WebSocket message:', err)
      }
    }

    ws.onclose = () => {
      console.log('Chat WebSocket disconnected')
      wsRef.current = null
      reconnectTimeoutRef.current = setTimeout(() => {
        connectWebSocket()
      }, 3000)
    }

    ws.onerror = (error) => {
      console.error('Chat WebSocket error:', error)
    }

    wsRef.current = ws
  }, [])

  const disconnectWebSocket = useCallback(() => {
    if (reconnectTimeoutRef.current) {
      clearTimeout(reconnectTimeoutRef.current)
      reconnectTimeoutRef.current = null
    }
    if (wsRef.current) {
      wsRef.current.close()
      wsRef.current = null
    }
  }, [])

  useEffect(() => {
    fetchMessages()
    fetchUsers()
    connectWebSocket()

    return () => {
      disconnectWebSocket()
    }
  }, [fetchMessages, fetchUsers, connectWebSocket, disconnectWebSocket])

  return {
    messages,
    users,
    selectedUser,
    isLoading,
    setSelectedUser,
    sendMessage,
    fetchMessages,
    fetchUsers,
    markAsRead,
    connectWebSocket,
    disconnectWebSocket,
  }
}
