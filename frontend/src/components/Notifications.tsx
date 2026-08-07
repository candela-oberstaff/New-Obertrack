import { useState, useEffect, useRef } from 'react'
import { useNavigate } from 'react-router-dom'
import { notificationService, type Notification } from '../services/api'
import { useAuth } from '../context/AuthContext'
import {
  desktopPermission,
  requestDesktopPermission,
  showDesktopNotification,
  type DesktopPermission,
} from '../lib/desktopNotifications'
import {
  isNotificationSoundEnabled,
  setNotificationSoundEnabled,
  playNotificationSound,
} from '../lib/notificationSound'
import {
  Bell,
  BellRing,
  ClipboardList,
  CheckCircle2, 
  XCircle, 
  MessageSquare,
  AtSign,
  Mail,
  UserPlus,
  LogOut,
  Volume2,
  VolumeX
} from 'lucide-react'
import styles from './Notifications.module.css'

const LIVE_NOTIFICATION_TYPES = new Set([
  'task_assigned',
  'mention',
  'support',
  'task_created',
  'task_updated',
  'task_completed',
  'board_invitation',
  'board_join_request',
  'board_invitation_accepted',
  'board_invitation_rejected',
  'board_request_approved',
  'board_request_rejected',
  'board_member_left',
])

const ALERT_NOTIFICATION_TYPES = new Set([
  'task_assigned',
  'mention',
  'support',
  'board_invitation',
])

const RECONNECT_BASE_DELAY = 1000
const RECONNECT_MAX_DELAY = 30000

function parseNotificationLink(data: Notification['data']): string | null {
  if (!data) return null
  try {
    const parsed = typeof data === 'string' ? JSON.parse(data) : data
    const link = (parsed as { link?: unknown })?.link
    return typeof link === 'string' ? link : null
  } catch {
    return null
  }
}

export default function Notifications() {
  const [notifications, setNotifications] = useState<Notification[]>([])
  const [unreadCount, setUnreadCount] = useState(0)
  const [isOpen, setIsOpen] = useState(false)
  const [_isLoading, _setIsLoading] = useState(false)
  const [permission, setPermission] = useState<DesktopPermission>(() => desktopPermission())
  const [soundEnabled, setSoundEnabled] = useState(() => isNotificationSoundEnabled())
  const navigate = useNavigate()
  const wsRef = useRef<WebSocket | null>(null)
  const reconnectTimerRef = useRef<number | null>(null)
  const reconnectAttemptsRef = useRef(0)
  const closingRef = useRef(false)
  const { user } = useAuth()
  const navigateRef = useRef(navigate)
  navigateRef.current = navigate

  useEffect(() => {
    fetchNotifications()
    const interval = setInterval(fetchUnreadCount, 30000)
    return () => clearInterval(interval)
  }, [])

  useEffect(() => {
    if (!user) return

    closingRef.current = false
    reconnectAttemptsRef.current = 0

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const wsUrl = `${protocol}//${window.location.host}/ws/notifications`

    const scheduleReconnect = () => {
      if (closingRef.current || reconnectTimerRef.current !== null) return
      const attempt = reconnectAttemptsRef.current
      const delay = Math.min(RECONNECT_BASE_DELAY * 2 ** attempt, RECONNECT_MAX_DELAY)
      reconnectAttemptsRef.current = attempt + 1
      reconnectTimerRef.current = window.setTimeout(() => {
        reconnectTimerRef.current = null
        connect()
      }, delay + Math.random() * 1000)
    }

    const connect = () => {
      if (closingRef.current) return

      const ws = new WebSocket(wsUrl)
      wsRef.current = ws

      ws.onopen = () => {
        const wasReconnect = reconnectAttemptsRef.current > 0
        reconnectAttemptsRef.current = 0
        if (wasReconnect) fetchNotifications()
      }

      ws.onmessage = (event) => {
        try {
          const message = JSON.parse(event.data)
          if (LIVE_NOTIFICATION_TYPES.has(message.type)) {
            const newNotification: Notification = {
              id: message.data?.id || Date.now(),
              user_id: 0,
              type: message.type,
              title: message.data?.title || 'Nueva notificación',
              message: message.data?.message || '',
              data: message.data?.data,
              read_at: undefined,
              created_at: new Date().toISOString(),
            }
            setNotifications(prev => [newNotification, ...prev])
            setUnreadCount(prev => prev + 1)

            if (ALERT_NOTIFICATION_TYPES.has(message.type)) {
              playNotificationSound()

              const link = parseNotificationLink(newNotification.data)
              showDesktopNotification({
                title: newNotification.title,
                body: newNotification.message,
                tag: `obertrack-${message.type}-${newNotification.id}`,
                onClick: link ? () => navigateRef.current(link) : undefined,
              })
            }

            if (
              message.type === 'task_assigned' ||
              message.type === 'task_created' ||
              message.type === 'task_updated' ||
              message.type === 'task_completed'
            ) {
              window.dispatchEvent(new CustomEvent('task-assigned'))
            }
          }
        } catch (error) {
          console.error('Error parsing notification:', error)
        }
      }

      ws.onclose = () => {
        scheduleReconnect()
      }

      ws.onerror = () => {
        ws.close()
      }
    }

    connect()

    const handleVisibility = () => {
      if (document.hidden) return
      fetchUnreadCount()
      const ws = wsRef.current
      if (!ws || ws.readyState === WebSocket.CLOSED || ws.readyState === WebSocket.CLOSING) {
        reconnectAttemptsRef.current = 0
        if (reconnectTimerRef.current !== null) {
          clearTimeout(reconnectTimerRef.current)
          reconnectTimerRef.current = null
        }
        connect()
      }
    }
    document.addEventListener('visibilitychange', handleVisibility)

    return () => {
      closingRef.current = true
      document.removeEventListener('visibilitychange', handleVisibility)
      if (reconnectTimerRef.current !== null) {
        clearTimeout(reconnectTimerRef.current)
        reconnectTimerRef.current = null
      }
      wsRef.current?.close()
      wsRef.current = null
    }
  }, [user])

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

  const handleNotificationClick = async (notification: Notification) => {
    // Mark as read if needed
    if (!notification.read_at) {
      handleMarkAsRead(notification.id)
    }

    // Close dropdown
    setIsOpen(false)

    const link = parseNotificationLink(notification.data)
    if (link) navigate(link)
  }

  const handleToggleOpen = () => {
    const next = !isOpen
    setIsOpen(next)
    if (next) fetchNotifications()
  }

  const handleEnableDesktop = async () => {
    const result = await requestDesktopPermission()
    setPermission(result)
    // Con el permiso concedido, registra también la suscripción Web Push: así
    // los avisos llegan aunque la pestaña esté CERRADA, no solo con la app
    // abierta. Best-effort: si falla, el escritorio local sigue funcionando.
    if (result === 'granted') {
      try {
        const { pushService } = await import('../services/push.service')
        const registration = await navigator.serviceWorker.register('/sw.js')
        const publicKey = await pushService.getPublicKey()
        if (publicKey) {
          const padding = '='.repeat((4 - (publicKey.length % 4)) % 4)
          const base64 = (publicKey + padding).replace(/-/g, '+').replace(/_/g, '/')
          const raw = window.atob(base64)
          const appKey = new Uint8Array(raw.length)
          for (let i = 0; i < raw.length; i++) appKey[i] = raw.charCodeAt(i)
          const sub = await registration.pushManager.getSubscription()
            ?? await registration.pushManager.subscribe({ userVisibleOnly: true, applicationServerKey: appKey as BufferSource })
          await pushService.subscribe(sub.toJSON())
        }
      } catch (e) {
        console.error('No se pudo registrar Web Push:', e)
      }
    }
  }

  const handleToggleSound = () => {
    const next = !soundEnabled
    setSoundEnabled(next)
    setNotificationSoundEnabled(next)
    if (next) playNotificationSound()
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
      case 'task_created': return <ClipboardList size={18} className="text-emerald-500" />
      case 'task_updated': return <ClipboardList size={18} className="text-amber-500" />
      case 'task_completed': return <CheckCircle2 size={18} className="text-green-500" />
      case 'work_hour_approved': return <CheckCircle2 size={18} className="text-green-500" />
      case 'work_hour_rejected': return <XCircle size={18} className="text-red-500" />
      case 'new_comment': return <MessageSquare size={18} className="text-indigo-500" />
      case 'mention': return <AtSign size={18} className="text-orange-500" />
      case 'board_invitation': return <Mail size={18} className="text-violet-500" />
      case 'board_join_request': return <UserPlus size={18} className="text-amber-500" />
      case 'board_invitation_accepted':
      case 'board_request_approved': return <CheckCircle2 size={18} className="text-green-500" />
      case 'board_invitation_rejected':
      case 'board_request_rejected': return <XCircle size={18} className="text-red-500" />
      case 'board_member_left': return <LogOut size={18} className="text-gray-500" />
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
    <div className={styles['notifications-container']}>
      <button className={styles['notification-bell']} onClick={handleToggleOpen} title="Notificaciones">
        <Bell size={20} />
        {unreadCount > 0 && (
          <span className={styles['notification-badge']}>{unreadCount > 9 ? '9+' : unreadCount}</span>
        )}
      </button>

      {isOpen && (
        <div className={styles['notifications-dropdown']}>
          <div className={styles['notifications-header']}>
            <h3>Notificaciones</h3>
            <div className={styles['notifications-header-actions']}>
              {unreadCount > 0 && (
                <button className={styles['mark-all-read']} onClick={handleMarkAllAsRead}>
                  Marcar todas como leídas
                </button>
              )}
              <button
                className={styles['sound-toggle']}
                onClick={handleToggleSound}
                title={soundEnabled ? 'Silenciar notificaciones' : 'Activar sonido'}
                aria-label={soundEnabled ? 'Silenciar notificaciones' : 'Activar sonido'}
                aria-pressed={soundEnabled}
              >
                {soundEnabled ? <Volume2 size={15} /> : <VolumeX size={15} />}
              </button>
            </div>
          </div>

          {permission === 'default' && (
            <button className={styles['enable-desktop']} onClick={handleEnableDesktop}>
              <BellRing size={14} />
              Activar notificaciones del navegador (aunque cierres la pestaña)
            </button>
          )}
          {permission === 'granted' && (
            <div className={styles['enable-desktop']} style={{ cursor: 'default', opacity: 0.75 }}>
              <BellRing size={14} />
              Notificaciones del navegador activadas ✓
            </div>
          )}
          {permission === 'denied' && (
            <div className={styles['enable-desktop']} style={{ cursor: 'default', opacity: 0.75 }} title="El permiso está bloqueado: haz clic en el candado de la barra de direcciones → Notificaciones → Permitir">
              <BellRing size={14} />
              Notificaciones bloqueadas — actívalas desde el candado del navegador
            </div>
          )}

          <div className={styles['notifications-list']}>
            {notifications.length === 0 ? (
              <div className={styles['no-notifications']}>No hay notificaciones</div>
            ) : (
              notifications.map(notification => (
                <div
                  key={notification.id}
                  className={`${styles['notification-item']} ${!notification.read_at ? styles['unread'] : ''}`}
                  onClick={() => handleNotificationClick(notification)}
                >
                  <span className={styles['notification-icon']}>{getNotificationIcon(notification.type)}</span>
                  <div className={styles['notification-content']}>
                    <div className={styles['notification-title']}>{notification.title}</div>
                    <div className={styles['notification-message']}>{notification.message}</div>
                    <div className={styles['notification-time']}>{formatDate(notification.created_at)}</div>
                  </div>
                  {!notification.read_at && <span className={styles['unread-dot']}></span>}
                </div>
              ))
            )}
          </div>
        </div>
      )}
    </div>
  )
}
