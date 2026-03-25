

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
    <div className="admin-content">
      <div className="metrics-grid">
        <div className="metric-card">
          <div className="metric-icon companies">🏢</div>
          <div className="metric-info">
            <span className="metric-value">{metrics.total_companies}</span>
            <span className="metric-label">Empresas</span>
          </div>
        </div>
        <div className="metric-card">
          <div className="metric-icon professionals">👷</div>
          <div className="metric-info">
            <span className="metric-value">{metrics.total_professionals}</span>
            <span className="metric-label">Profesionales</span>
          </div>
        </div>
        <div className="metric-card">
          <div className="metric-icon hours">⏱️</div>
          <div className="metric-info">
            <span className="metric-value">{metrics.total_hours_worked.toFixed(0)}h</span>
            <span className="metric-label">Horas Totales</span>
          </div>
        </div>
        <div className="metric-card">
          <div className="metric-icon approved">✅</div>
          <div className="metric-info">
            <span className="metric-value">{metrics.approved_hours.toFixed(0)}h</span>
            <span className="metric-label">Horas Aprobadas</span>
          </div>
        </div>
        <div className="metric-card">
          <div className="metric-icon pending">⏳</div>
          <div className="metric-info">
            <span className="metric-value">{metrics.pending_hours.toFixed(0)}h</span>
            <span className="metric-label">Horas Pendientes</span>
          </div>
        </div>
        <div className="metric-card">
          <div className="metric-icon tasks">📋</div>
          <div className="metric-info">
            <span className="metric-value">{metrics.completed_tasks}/{metrics.total_tasks}</span>
            <span className="metric-label">Tareas Completadas</span>
          </div>
        </div>
      </div>

      <div className="alerts-section">
        {metrics.inactive_warning > 0 && (
          <div className="alert-card warning">
            <span className="alert-icon">⚠️</span>
            <div className="alert-content">
              <strong>{metrics.inactive_warning} profesionales sin actividad reciente</strong>
              <p>No han registrado horas en los últimos 3 días</p>
            </div>
            <button className="alert-action" onClick={onViewInactive}>
              Ver
            </button>
          </div>
        )}
        <div className="alert-card info">
          <span className="alert-icon">📈</span>
          <div className="alert-content">
            <strong>{metrics.active_today} profesionales activos hoy</strong>
            <p>Han registrado su jornada</p>
          </div>
        </div>
      </div>

      <div className="activity-section">
        <h3>Actividad Reciente</h3>
        <div className="activity-list">
          {activities.length === 0 ? (
            <p className="no-activity">Sin actividad reciente</p>
          ) : (
            activities.map((activity, idx) => (
              <div key={idx} className="activity-item">
                <div className="activity-icon">{activity.type === 'work_hour' ? '⏱️' : '📋'}</div>
                <div className="activity-details">
                  <span className="activity-user">{activity.user}</span>
                  <span className="activity-company">{activity.company}</span>
                </div>
                <span className="activity-desc">{activity.details}</span>
                <span className="activity-time">
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
