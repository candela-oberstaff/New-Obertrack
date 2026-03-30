import { useState, useEffect, useRef } from 'react'
import { useAuth } from '../context/AuthContext'
import { userService, uploadService } from '../services/api'
import type { User } from '../types'
import { 
  X, 
  MessageSquare, 
  Menu, 
  Paperclip, 
  Hourglass, 
  Send 
} from 'lucide-react'
import styles from './Chat.module.css'

interface Message {
  id: number
  user_id: number
  content: string
  created_at: string
  attachment?: {
    url: string
    filename: string
  }
  user?: {
    name: string
  }
}

export default function Chat() {
  const { user, token } = useAuth()
  const [messages, setMessages] = useState<Message[]>([])
  const [newMessage, setNewMessage] = useState('')
  const [isConnected, setIsConnected] = useState(false)
  const [users, setUsers] = useState<User[]>([])
  const [selectedUser, setSelectedUser] = useState<number | null>(null)
  const [showUserList, setShowUserList] = useState(true)
  const [isUploading, setIsUploading] = useState(false)
  const wsRef = useRef<WebSocket | null>(null)
  const messagesEndRef = useRef<HTMLDivElement>(null)
  const fileInputRef = useRef<HTMLInputElement>(null)

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
          'Authorization': `Bearer ${token}`
        }
      })
      if (response.ok) {
        const data = await response.json()
        setMessages(data ? data.reverse() : [])
      }
    } catch (error) {
      console.error('Error fetching messages:', error)
    }
  }

  const fetchUsers = async () => {
    try {
      const data = await userService.getAll()
      if (data && data.data) {
        setUsers(data.data.filter(u => u.id !== user?.id))
      } else {
        setUsers([])
      }
    } catch (error) {
      console.error('Error fetching users:', error)
    }
  }

  const connectWebSocket = () => {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const ws = new WebSocket(`${protocol}//${window.location.host}/ws/chat?token=${token}`)

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
            attachment: message.attachment,
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

  const sendMessage = (e?: React.FormEvent, attachment?: { url: string; filename: string }) => {
    if (e) e.preventDefault()
    const content = attachment ? `${newMessage} [Archivo: ${attachment.filename}]` : newMessage
    if ((!content.trim() && !attachment) || !wsRef.current || !isConnected) return

    const message = {
      type: 'chat_message',
      content: content,
      attachment: attachment,
    }

    wsRef.current.send(JSON.stringify(message))
    setMessages(prev => [...prev, {
      id: Date.now(),
      user_id: user?.id || 0,
      content: content,
      attachment: attachment,
      created_at: new Date().toISOString(),
      user: { name: user?.name || 'Tú' }
    }])
    setNewMessage('')
  }

  const handleFileUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return

    setIsUploading(true)
    try {
      const result = await uploadService.upload(file)
      sendMessage(undefined, { url: result.url, filename: file.name })
    } catch (error) {
      console.error('Error uploading file:', error)
    } finally {
      setIsUploading(false)
      if (fileInputRef.current) {
        fileInputRef.current.value = ''
      }
    }
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
    <div className={styles['chat-page']}>
      <div className={styles['chat-layout']}>
        <div className={`${styles['chat-sidebar']} ${showUserList ? styles['open'] : ''}`}>
          <div className={styles['sidebar-header']}>
            <h3>Conversaciones</h3>
            <button className={styles['close-sidebar']} onClick={() => setShowUserList(false)}><X size={20} /></button>
          </div>
          <div className={styles['user-list']}>
            <div 
              className={`${styles['user-item']} ${!selectedUser ? styles['active'] : ''}`}
              onClick={() => setSelectedUser(null)}
            >
              <div className={`${styles['user-avatar']} ${styles['all']}`}><MessageSquare size={18} /></div>
              <div className={styles['user-info']}>
                <span className={styles['user-name']}>Todos</span>
                <span className={styles['user-preview']}>Mensajes del equipo</span>
              </div>
            </div>
            {users.map(u => (
              <div 
                key={u.id} 
                className={`${styles['user-item']} ${selectedUser === u.id ? styles['active'] : ''}`}
                onClick={() => setSelectedUser(u.id)}
              >
                <div className={styles['user-avatar']}>
                  {u.name?.charAt(0).toUpperCase()}
                </div>
                <div className={styles['user-info']}>
                  <span className={styles['user-name']}>{u.name}</span>
                  <span className={styles['user-role']}>{u.job_title || u.user_type}</span>
                </div>
              </div>
            ))}
          </div>
        </div>

        <div className={styles['chat-main']}>
          <div className={styles['chat-header']}>
            <button className={styles['menu-btn']} onClick={() => setShowUserList(true)}><Menu size={20} /></button>
            <div className={styles['chat-title']}>
              <h2>{selectedUser ? users.find(u => u.id === selectedUser)?.name : 'Equipo'}</h2>
              <span className={styles['connection-dot']}></span>
            </div>
            <span className={`${styles['connection-badge']} ${isConnected ? styles['connected'] : styles['disconnected']}`}>
              {isConnected ? 'En línea' : 'Conectando...'}
            </span>
          </div>

            <div className={styles['chat-messages']}>
            {filteredMessages.length === 0 ? (
              <div className={styles['empty-chat']}>
                <span className={styles['empty-icon']}><MessageSquare size={40} /></span>
                <p>No hay mensajes todavía</p>
                <span className={styles['empty-hint']}>¡Inicia la conversación!</span>
              </div>
            ) : (
              filteredMessages.map((msg) => (
                <div
                  key={msg.id}
                  className={`${styles['message']} ${msg.user_id === user?.id ? styles['own'] : ''}`}
                >
                  {msg.user_id !== user?.id && (
                    <div className={styles['message-avatar']}>
                      {msg.user?.name?.charAt(0).toUpperCase() || '?'}
                    </div>
                  )}
                  <div className={styles['message-bubble']}>
                    {msg.user_id !== user?.id && (
                      <span className={styles['message-sender']}>{msg.user?.name || 'Usuario'}</span>
                    )}
                    {msg.content && <p className={styles['message-text']}>{msg.content}</p>}
                    {msg.attachment && (
                      <a 
                        href={msg.attachment.url} 
                        target="_blank" 
                        rel="noopener noreferrer"
                        className={styles['message-attachment']}
                      >
                        <Paperclip size={14} /> {msg.attachment.filename}
                      </a>
                    )}
                    <span className={styles['message-time']}>{formatTime(msg.created_at)}</span>
                  </div>
                </div>
              ))
            )}
            <div ref={messagesEndRef} />
          </div>

          <form className={styles['chat-input']} onSubmit={(e) => sendMessage(e)}>
            <input
              type="text"
              value={newMessage}
              onChange={(e) => setNewMessage(e.target.value)}
              placeholder="Escribe un mensaje..."
              disabled={!isConnected}
            />
            <input
              type="file"
              ref={fileInputRef}
              onChange={handleFileUpload}
              accept=".pdf,.doc,.docx,.xls,.xlsx,.jpg,.jpeg,.png,.gif,.webp,.mp3,.wav,.ogg,.webm"
              style={{ display: 'none' }}
            />
            <button
              type="button"
              onClick={() => fileInputRef.current?.click()}
              disabled={!isConnected || isUploading}
              className={styles['attach-btn']}
              title="Adjuntar archivo"
            >
              {isUploading ? <Hourglass size={18} className={styles['spin'] || 'spin'} /> : <Paperclip size={18} />}
            </button>
            <button
              type="submit"
              disabled={!isConnected || (!newMessage.trim() && !isUploading)}
              className={styles['send-btn']}
            >
              <Send size={18} />
            </button>
          </form>
        </div>
      </div>
    </div>
  )
}
