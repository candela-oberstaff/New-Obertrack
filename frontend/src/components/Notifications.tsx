import { useState, useEffect, useRef } from 'react'
import { notificationService, type Notification } from '../services/api'
import { useAuth } from '../context/AuthContext'
import { 
  Bell, 
  ClipboardList, 
  CheckCircle2, 
  XCircle, 
  MessageSquare, 
  AtSign 
} from 'lucide-react'
import './Notifications.css'

export default function Notifications() {
  const [notifications, setNotifications] = useState<Notification[]>([])
  const [unreadCount, setUnreadCount] = useState(0)
  const [isOpen, setIsOpen] = useState(false)
  const [_isLoading, _setIsLoading] = useState(false)
  const wsRef = useRef<WebSocket | null>(null)
  const { token } = useAuth()

  useEffect(() => {
    fetchNotifications()
    const interval = setInterval(fetchUnreadCount, 30000)
    return () => {
      clearInterval(interval)
      if (wsRef.current) {
        wsRef.current.close()
      }
    }
  }, [])

  useEffect(() => {
    if (!token) return

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const wsUrl = `${protocol}//${window.location.host}/ws/notifications?token=${token}`

    const connect = () => {
      wsRef.current = new WebSocket(wsUrl)

      wsRef.current.onopen = () => {
        console.log('Notifications WebSocket connected')
      }

      wsRef.current.onmessage = (event) => {
        try {
          const message = JSON.parse(event.data)
          if (message.type === 'task_assigned' || message.type === 'mention') {
            const newNotification: Notification = {
              id: message.data?.id || Date.now(),
              user_id: 0,
              type: message.type,
              title: message.data?.title || 'Nueva notificación',
              message: message.data?.message || '',
              read_at: undefined,
              created_at: new Date().toISOString(),
            }
            setNotifications(prev => [newNotification, ...prev])
            setUnreadCount(prev => prev + 1)
            
            if (message.type === 'task_assigned') {
              window.dispatchEvent(new CustomEvent('task-assigned'))
            }
          }
        } catch (error) {
          console.error('Error parsing notification:', error)
        }
      }

      wsRef.current.onclose = () => {
        console.log('Notifications WebSocket disconnected')
      }

      wsRef.current.onerror = (error) => {
        console.error('Notifications WebSocket error:', error)
      }
    }

    connect()

    return () => {
      if (wsRef.current) {
        wsRef.current.close()
      }
    }
  }, [token])

  const fetchNotifications = async () => {
    try {
      const data = await notificationService.getAll()
      if (!data) {
        setNotifications([])
        setUnreadCount(0)
        return
      }
      setNotifications(data)
      setUnreadCount(data.filter(n => !n.read_at).length)
    } catch (error) {
      console.error('Error fetching notifications:', error)
    }
  }

  const fetchUnreadCount = async () => {
    try {
      const count = await notificationService.getUnreadCount()
      setUnreadCount(count)
    } catch (error) {
      console.error('Error fetching unread count:', error)
    }
  }

  const handleMarkAsRead = async (id: number) => {
    try {
      await notificationService.markAsRead(id)
      setNotifications(prev =>
        prev.map(n => n.id === id ? { ...n, read_at: new Date().toISOString() } : n)
      )
      setUnreadCount(prev => Math.max(0, prev - 1))
    } catch (error) {
      console.error('Error marking as read:', error)
    }
  }

  const handleMarkAllAsRead = async () => {
    try {
      await notificationService.markAllAsRead()
      setNotifications(prev => prev.map(n => ({ ...n, read_at: new Date().toISOString() })))
      setUnreadCount(0)
    } catch (error) {
      console.error('Error marking all as read:', error)
    }
  }

  const getNotificationIcon = (type: string) => {
    switch (type) {
      case 'task_assigned': return <ClipboardList size={18} className="text-blue-500" />
      case 'work_hour_approved': return <CheckCircle2 size={18} className="text-green-500" />
      case 'work_hour_rejected': return <XCircle size={18} className="text-red-500" />
      case 'new_comment': return <MessageSquare size={18} className="text-indigo-500" />
      case 'mention': return <AtSign size={18} className="text-orange-500" />
      default: return <Bell size={18} className="text-gray-500" />
    }
  }

  const formatDate = (dateStr: string) => {
    const date = new Date(dateStr)
    const now = new Date()
    const diff = now.getTime() - date.getTime()
    const minutes = Math.floor(diff / 60000)
    const hours = Math.floor(diff / 3600000)
    const days = Math.floor(diff / 86400000)

    if (minutes < 1) return 'Ahora'
    if (minutes < 60) return `Hace ${minutes}m`
    if (hours < 24) return `Hace ${hours}h`
    if (days < 7) return `Hace ${days}d`
    return date.toLocaleDateString('es-ES')
  }

  return (
    <div className="notifications-container">
      <button className="notification-bell" onClick={() => setIsOpen(!isOpen)} title="Notificaciones">
        <Bell size={20} />
        {unreadCount > 0 && (
          <span className="notification-badge">{unreadCount > 9 ? '9+' : unreadCount}</span>
        )}
      </button>

      {isOpen && (
        <div className="notifications-dropdown">
          <div className="notifications-header">
            <h3>Notificaciones</h3>
            {unreadCount > 0 && (
              <button className="mark-all-read" onClick={handleMarkAllAsRead}>
                Marcar todas como leídas
              </button>
            )}
          </div>

          <div className="notifications-list">
            {notifications.length === 0 ? (
              <div className="no-notifications">No hay notificaciones</div>
            ) : (
              notifications.map(notification => (
                <div
                  key={notification.id}
                  className={`notification-item ${!notification.read_at ? 'unread' : ''}`}
                  onClick={() => !notification.read_at && handleMarkAsRead(notification.id)}
                >
                  <span className="notification-icon">{getNotificationIcon(notification.type)}</span>
                  <div className="notification-content">
                    <div className="notification-title">{notification.title}</div>
                    <div className="notification-message">{notification.message}</div>
                    <div className="notification-time">{formatDate(notification.created_at)}</div>
                  </div>
                  {!notification.read_at && <span className="unread-dot"></span>}
                </div>
              ))
            )}
          </div>
        </div>
      )}
    </div>
  )
}
