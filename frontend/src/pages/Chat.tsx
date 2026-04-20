import { useState, useRef, useEffect } from 'react'
import { useAuth } from '../context/AuthContext'
import { useChat } from '../hooks'
import { Send } from 'lucide-react'
import styles from './Chat.module.css'

interface User {
  id: number
  name: string
  avatar?: string
  user_type: string
  is_active: boolean
  is_online?: boolean
}

export default function Chat() {
  const { user } = useAuth()
  const {
    messages,
    users,
    isLoading,
    sendMessage,
    markAsRead,
  } = useChat()

  const [inputValue, setInputValue] = useState('')
  const [searchQuery, setSearchQuery] = useState('')
  const messagesEndRef = useRef<HTMLDivElement>(null)
  const inputRef = useRef<HTMLInputElement>(null)

  const filteredUsers = users.filter((u: User) => {
    if (!searchQuery) return true
    return u.name.toLowerCase().includes(searchQuery.toLowerCase())
  })

  const handleSend = async () => {
    if (!inputValue.trim()) return
    await sendMessage(inputValue)
    setInputValue('')
    inputRef.current?.focus()
  }

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      handleSend()
    }
  }

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages])

  useEffect(() => {
    if (messages.length > 0) {
      markAsRead()
    }
  }, [messages, markAsRead])

  const formatTime = (dateStr: string) => {
    const date = new Date(dateStr)
    return date.toLocaleTimeString('es-ES', { hour: '2-digit', minute: '2-digit' })
  }

  const formatDate = (dateStr: string) => {
    const date = new Date(dateStr)
    const today = new Date()
    if (date.toDateString() === today.toDateString()) {
      return 'Hoy'
    }
    const yesterday = new Date(today)
    yesterday.setDate(today.getDate() - 1)
    if (date.toDateString() === yesterday.toDateString()) {
      return 'Ayer'
    }
    return date.toLocaleDateString('es-ES', { day: 'numeric', month: 'short' })
  }

  if (isLoading) {
    return (
      <div className={styles['chat-page']}>
        <div className={styles['chat-loading']}>
          <div className={styles['spinner']} />
          <p>Cargando chat...</p>
        </div>
      </div>
    )
  }

  return (
    <div className={styles['chat-page']}>
      <div className={styles['chat-sidebar']}>
        <div className={styles['sidebar-header']}>
          <h2>Chat</h2>
        </div>
        <div className={styles['search-box']}>
          <input
            type="text"
            placeholder="Buscar usuarios..."
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
          />
        </div>
        <div className={styles['users-list']}>
          {filteredUsers.length === 0 ? (
            <div className={styles['no-users']}>
              {searchQuery ? 'No se encontraron usuarios' : 'No hay usuarios disponibles'}
            </div>
          ) : (
            filteredUsers.map((u: User) => (
              <div key={u.id} className={styles['user-item']}>
                <div className={styles['user-avatar']}>
                  {u.name?.charAt(0).toUpperCase()}
                  {u.is_online && <span className={styles['online-indicator']} />}
                </div>
                <div className={styles['user-info']}>
                  <span className={styles['user-name']}>{u.name}</span>
                  <span className={styles['user-status']}>
                    {u.is_online ? 'En línea' : 'Desconectado'}
                  </span>
                </div>
              </div>
            ))
          )}
        </div>
      </div>

      <div className={styles['chat-main']}>
        <div className={styles['messages-container']}>
          <div className={styles['messages-list']}>
            {messages.map((msg: any, index: number) => {
              const isOwn = msg.user_id === user?.id || msg.UserID === user?.id
              const showDate = index === 0 ||
                new Date(msg.created_at).toDateString() !==
                new Date(messages[index - 1].created_at).toDateString()

              return (
                <div key={msg.id || index}>
                  {showDate && (
                    <div className={styles['date-separator']}>
                      <span>{formatDate(msg.created_at)}</span>
                    </div>
                  )}
                  <div className={`${styles['message']} ${isOwn ? styles['own'] : styles['other']}`}>
                    {!isOwn && (
                      <div className={styles['message-avatar']}>
                        {msg.user?.name?.charAt(0).toUpperCase() || '?'}
                      </div>
                    )}
                    <div className={styles['message-bubble']}>
                      {!isOwn && (
                        <span className={styles['message-sender']}>{msg.user?.name || 'Usuario'}</span>
                      )}
                      <p className={styles['message-text']}>{msg.content || msg.Content}</p>
                      <span className={styles['message-time']}>
                        {formatTime(msg.created_at)}
                      </span>
                    </div>
                  </div>
                </div>
              )
            })}
            <div ref={messagesEndRef} />
          </div>
        </div>

        <div className={styles['chat-input']}>
          <input
            ref={inputRef}
            type="text"
            placeholder="Escribe un mensaje..."
            value={inputValue}
            onChange={(e) => setInputValue(e.target.value)}
            onKeyDown={handleKeyDown}
          />
          <button onClick={handleSend} disabled={!inputValue.trim()}>
            <Send size={20} />
          </button>
        </div>
      </div>
    </div>
  )
}
