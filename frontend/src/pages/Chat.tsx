import { useState, useEffect, useRef } from 'react'
import { useAuth } from '../context/AuthContext'
import { userService } from '../services/api'
import type { User } from '../types'
import './Chat.css'

interface Message {
  id: number
  user_id: number
  content: string
  created_at: string
  user?: {
    name: string
  }
}

export default function Chat() {
  const { user } = useAuth()
  const [messages, setMessages] = useState<Message[]>([])
  const [newMessage, setNewMessage] = useState('')
  const [isConnected, setIsConnected] = useState(false)
  const [users, setUsers] = useState<User[]>([])
  const [selectedUser, setSelectedUser] = useState<number | null>(null)
  const [showUserList, setShowUserList] = useState(true)
  const wsRef = useRef<WebSocket | null>(null)
  const messagesEndRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    fetchMessages()
    fetchUsers()
    connectWebSocket()

    return () => {
      if (wsRef.current) {
        wsRef.current.close()
      }
    }
  }, [])

  useEffect(() => {
    scrollToBottom()
  }, [messages])

  const fetchMessages = async () => {
    try {
      const response = await fetch('/api/chat/messages', {
        headers: {
          'Authorization': `Bearer ${localStorage.getItem('token')}`
        }
      })
      if (response.ok) {
        const data = await response.json()
        setMessages(data.reverse())
      }
    } catch (error) {
      console.error('Error fetching messages:', error)
    }
  }

  const fetchUsers = async () => {
    try {
      const data = await userService.getAll()
      setUsers(data.data.filter(u => u.id !== user?.id))
    } catch (error) {
      console.error('Error fetching users:', error)
    }
  }

  const connectWebSocket = () => {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const ws = new WebSocket(`${protocol}//${window.location.host}/ws/chat`)

    ws.onopen = () => {
      setIsConnected(true)
    }

    ws.onmessage = (event) => {
      try {
        const message = JSON.parse(event.data)
        if (message.type === 'chat_message') {
          setMessages(prev => [...prev, {
            id: Date.now(),
            user_id: message.user_id,
            content: message.content,
            created_at: message.timestamp,
            user: message.user_name ? { name: message.user_name } : undefined
          }])
        }
      } catch (e) {
        console.error('Error parsing message:', e)
      }
    }

    ws.onclose = () => {
      setIsConnected(false)
      setTimeout(connectWebSocket, 3000)
    }

    ws.onerror = (error) => {
      console.error('WebSocket error:', error)
    }

    wsRef.current = ws
  }

  const sendMessage = (e: React.FormEvent) => {
    e.preventDefault()
    if (!newMessage.trim() || !wsRef.current || !isConnected) return

    const message = {
      type: 'chat_message',
      content: newMessage,
    }

    wsRef.current.send(JSON.stringify(message))
    setMessages(prev => [...prev, {
      id: Date.now(),
      user_id: user?.id || 0,
      content: newMessage,
      created_at: new Date().toISOString(),
      user: { name: user?.name || 'Tú' }
    }])
    setNewMessage('')
  }

  const scrollToBottom = () => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }

  const formatTime = (dateString: string) => {
    const date = new Date(dateString)
    const now = new Date()
    const diff = now.getTime() - date.getTime()
    const days = Math.floor(diff / (1000 * 60 * 60 * 24))
    
    if (days === 0) {
      return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
    } else if (days === 1) {
      return 'Ayer'
    } else {
      return date.toLocaleDateString('es-ES', { day: 'numeric', month: 'short' })
    }
  }

  const filteredMessages = selectedUser
    ? messages.filter(m => m.user_id === selectedUser)
    : messages

  return (
    <div className="chat-page">
      <div className="chat-layout">
        <div className={`chat-sidebar ${showUserList ? 'open' : ''}`}>
          <div className="sidebar-header">
            <h3>Conversaciones</h3>
            <button className="close-sidebar" onClick={() => setShowUserList(false)}>✕</button>
          </div>
          <div className="user-list">
            <div 
              className={`user-item ${!selectedUser ? 'active' : ''}`}
              onClick={() => setSelectedUser(null)}
            >
              <div className="user-avatar all">💬</div>
              <div className="user-info">
                <span className="user-name">Todos</span>
                <span className="user-preview">Mensajes del equipo</span>
              </div>
            </div>
            {users.map(u => (
              <div 
                key={u.id} 
                className={`user-item ${selectedUser === u.id ? 'active' : ''}`}
                onClick={() => setSelectedUser(u.id)}
              >
                <div className="user-avatar">
                  {u.name?.charAt(0).toUpperCase()}
                </div>
                <div className="user-info">
                  <span className="user-name">{u.name}</span>
                  <span className="user-role">{u.job_title || u.user_type}</span>
                </div>
              </div>
            ))}
          </div>
        </div>

        <div className="chat-main">
          <div className="chat-header">
            <button className="menu-btn" onClick={() => setShowUserList(true)}>☰</button>
            <div className="chat-title">
              <h2>{selectedUser ? users.find(u => u.id === selectedUser)?.name : 'Equipo'}</h2>
              <span className="connection-dot"></span>
            </div>
            <span className={`connection-badge ${isConnected ? 'connected' : 'disconnected'}`}>
              {isConnected ? 'En línea' : 'Conectando...'}
            </span>
          </div>

          <div className="chat-messages">
            {filteredMessages.length === 0 ? (
              <div className="empty-chat">
                <span className="empty-icon">💬</span>
                <p>No hay mensajes todavía</p>
                <span className="empty-hint">¡Inicia la conversación!</span>
              </div>
            ) : (
              filteredMessages.map((msg) => (
                <div
                  key={msg.id}
                  className={`message ${msg.user_id === user?.id ? 'own' : ''}`}
                >
                  {msg.user_id !== user?.id && (
                    <div className="message-avatar">
                      {msg.user?.name?.charAt(0).toUpperCase() || '?'}
                    </div>
                  )}
                  <div className="message-bubble">
                    {msg.user_id !== user?.id && (
                      <span className="message-sender">{msg.user?.name || 'Usuario'}</span>
                    )}
                    <p className="message-text">{msg.content}</p>
                    <span className="message-time">{formatTime(msg.created_at)}</span>
                  </div>
                </div>
              ))
            )}
            <div ref={messagesEndRef} />
          </div>

          <form className="chat-input" onSubmit={sendMessage}>
            <input
              type="text"
              value={newMessage}
              onChange={(e) => setNewMessage(e.target.value)}
              placeholder="Escribe un mensaje..."
              disabled={!isConnected}
            />
            <button 
              type="submit" 
              disabled={!isConnected || !newMessage.trim()}
              className="send-btn"
            >
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                <line x1="22" y1="2" x2="11" y2="13" />
                <polygon points="22 2 15 22 11 13 2 9 22 2" />
              </svg>
            </button>
          </form>
        </div>
      </div>
    </div>
  )
}
