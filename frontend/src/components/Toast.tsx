import { useNotification } from '../context/NotificationContext'
import './Toast.css'

export default function Toast() {
  const { notifications, removeNotification } = useNotification()

  const getIcon = (type: string) => {
    switch (type) {
      case 'success': return '✓'
      case 'error': return '✕'
      case 'warning': return '⚠'
      case 'info': return 'ℹ'
      default: return '•'
    }
  }

  if (notifications.length === 0) return null

  return (
    <div className="toast-container">
      {notifications.map((notification) => (
        <div 
          key={notification.id} 
          className={`toast toast-${notification.type}`}
          onClick={() => removeNotification(notification.id)}
        >
          <span className="toast-icon">{getIcon(notification.type)}</span>
          <span className="toast-message">{notification.message}</span>
          <button className="toast-close" onClick={(e) => {
            e.stopPropagation()
            removeNotification(notification.id)
          }}>✕</button>
        </div>
      ))}
    </div>
  )
}
