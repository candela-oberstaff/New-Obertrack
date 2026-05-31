import { CheckCircle2 } from 'lucide-react'
import styles from './Admin.module.css'

interface InactiveUser {
  id: number
  name: string
  email: string
  company: string
  last_active: string
  days_inactive: number
}

interface InactiveUserListProps {
  inactiveUsers: InactiveUser[]
}

export function InactiveUserList({ inactiveUsers }: InactiveUserListProps) {
  return (
    <div className={styles['admin-content']}>
      <div className={styles['admin-section-header'] || 'admin-section-header'}>
        <h3>Profesionales Inactivos</h3>
      </div>
      {inactiveUsers.length === 0 ? (
        <div className={styles['empty-state']}>
          <span className={styles['empty-icon']}><CheckCircle2 size={40} style={{ color: '#10b981' }} /></span>
          <p>¡Todos los profesionales están activos!</p>
        </div>
      ) : (
        <div className={styles['inactive-list']}>
          {inactiveUsers.map(u => (
            <div key={u.id} className={styles['inactive-card']}>
              <div className={styles['inactive-info']}>
                <span className={styles['inactive-name']}>{u.name}</span>
                <span className={styles['inactive-email']}>{u.email}</span>
                <span className={styles['inactive-company']}>{u.company}</span>
              </div>
              <div className={styles['inactive-status']}>
                <span className={`${styles['days-badge']} ${u.days_inactive > 7 ? styles['critical'] : u.days_inactive > 3 ? styles['warning'] : styles['mild']}`}>
                  {u.days_inactive} días sin actividad
                </span>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
