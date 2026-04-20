import { useNotification } from '../context/NotificationContext'
import styles from './Toast.module.css'

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
    <div className={styles['toast-container']}>
      {notifications.map((notification) => (
        <div 
          key={notification.id} 
          className={`${styles.toast} ${styles[`toast-${notification.type}`]}`}
          onClick={() => removeNotification(notification.id)}
        >
          <span className={styles['toast-icon']}>{getIcon(notification.type)}</span>
          <span className={styles['toast-message']}>{notification.message}</span>
          <button className={styles['toast-close']} onClick={(e) => {
            e.stopPropagation()
            removeNotification(notification.id)
          }}>✕</button>
        </div>
      ))}
    </div>
  )
}
