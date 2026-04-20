import { Building2, User, Clock, CheckCircle2, Hourglass, ClipboardList, AlertTriangle, TrendingUp } from 'lucide-react'
import styles from './Admin.module.css'

interface DashboardMetrics {
  total_companies: number
  total_professionals: number
  total_managers: number
  total_hours_worked: number
  approved_hours: number
  pending_hours: number
  total_tasks: number
  completed_tasks: number
  pending_tasks: number
  active_today: number
  inactive_warning: number
}

interface Activity {
  type: string
  user: string
  company: string
  details: string
  timestamp: string
}

interface AdminDashboardProps {
  metrics: DashboardMetrics
  activities: Activity[]
  onViewInactive: () => void
}

export function AdminDashboard({ metrics, activities, onViewInactive }: AdminDashboardProps) {
  return (
    <div className={styles['admin-content']}>
      <div className={styles['metrics-grid']}>
        <div className={styles['metric-card']}>
          <div className={`${styles['metric-icon']} ${styles['companies']}`}><Building2 size={24} /></div>
          <div className={styles['metric-info']}>
            <span className={styles['metric-value']}>{metrics.total_companies}</span>
            <span className={styles['metric-label']}>Empresas</span>
          </div>
        </div>
        <div className={styles['metric-card']}>
          <div className={`${styles['metric-icon']} ${styles['professionals']}`}><User size={24} /></div>
          <div className={styles['metric-info']}>
            <span className={styles['metric-value']}>{metrics.total_professionals}</span>
            <span className={styles['metric-label']}>Profesionales</span>
          </div>
        </div>
        <div className={styles['metric-card']}>
          <div className={`${styles['metric-icon']} ${styles['hours']}`}><Clock size={24} /></div>
          <div className={styles['metric-info']}>
            <span className={styles['metric-value']}>{metrics.total_hours_worked.toFixed(0)}h</span>
            <span className={styles['metric-label']}>Horas Totales</span>
          </div>
        </div>
        <div className={styles['metric-card']}>
          <div className={`${styles['metric-icon']} ${styles['approved']}`}><CheckCircle2 size={24} /></div>
          <div className={styles['metric-info']}>
            <span className={styles['metric-value']}>{metrics.approved_hours.toFixed(0)}h</span>
            <span className={styles['metric-label']}>Horas Aprobadas</span>
          </div>
        </div>
        <div className={styles['metric-card']}>
          <div className={`${styles['metric-icon']} ${styles['pending']}`}><Hourglass size={24} /></div>
          <div className={styles['metric-info']}>
            <span className={styles['metric-value']}>{metrics.pending_hours.toFixed(0)}h</span>
            <span className={styles['metric-label']}>Horas Pendientes</span>
          </div>
        </div>
        <div className={styles['metric-card']}>
          <div className={`${styles['metric-icon']} ${styles['tasks']}`}><ClipboardList size={24} /></div>
          <div className={styles['metric-info']}>
            <span className={styles['metric-value']}>{metrics.completed_tasks}/{metrics.total_tasks}</span>
            <span className={styles['metric-label']}>Tareas Completadas</span>
          </div>
        </div>
      </div>

      <div className={styles['alerts-section']}>
        {metrics.inactive_warning > 0 && (
          <div className={`${styles['alert-card']} ${styles['warning']}`}>
            <span className={styles['alert-icon']}><AlertTriangle size={20} /></span>
            <div className={styles['alert-content']}>
              <strong>{metrics.inactive_warning} profesionales sin actividad reciente</strong>
              <p>No han registrado horas en los últimos 3 días</p>
            </div>
            <button className={styles['alert-action']} onClick={onViewInactive}>
              Ver
            </button>
          </div>
        )}
        <div className={`${styles['alert-card']} ${styles['info']}`}>
          <span className={styles['alert-icon']}><TrendingUp size={20} /></span>
          <div className={styles['alert-content']}>
            <strong>{metrics.active_today} profesionales activos hoy</strong>
            <p>Han registrado su jornada</p>
          </div>
        </div>
      </div>

      <div className={styles['activity-section']}>
        <h3>Actividad Reciente</h3>
        <div className={styles['activity-list']}>
          {activities.length === 0 ? (
            <p className={styles['no-activity']}>Sin actividad reciente</p>
          ) : (
            activities.map((activity, idx) => (
              <div key={idx} className={styles['activity-item']}>
                <div className={styles['activity-icon']}>{activity.type === 'work_hour' ? <Clock size={16} /> : <ClipboardList size={16} />}</div>
                <div className={styles['activity-details']}>
                  <span className={styles['activity-user']}>{activity.user}</span>
                  <span className={styles['activity-company']}>{activity.company}</span>
                </div>
                <span className={styles['activity-desc']}>{activity.details}</span>
                <span className={styles['activity-time']}>
                  {new Date(activity.timestamp).toLocaleTimeString('es-ES', { hour: '2-digit', minute: '2-digit' })}
                </span>
              </div>
            ))
          )}
        </div>
      </div>
    </div>
  )
}
