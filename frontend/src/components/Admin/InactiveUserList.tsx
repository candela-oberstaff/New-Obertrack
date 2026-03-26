

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
    <div className="admin-content">
      <div className="admin-section-header">
        <h3>Profesionales Inactivos</h3>
      </div>
      {inactiveUsers.length === 0 ? (
        <div className="empty-state">
          <span className="empty-icon">🎉</span>
          <p>¡Todos los profesionales están activos!</p>
        </div>
      ) : (
        <div className="inactive-list">
          {inactiveUsers.map(u => (
            <div key={u.id} className="inactive-card">
              <div className="inactive-info">
                <span className="inactive-name">{u.name}</span>
                <span className="inactive-email">{u.email}</span>
                <span className="inactive-company">{u.company}</span>
              </div>
              <div className="inactive-status">
                <span className={`days-badge ${u.days_inactive > 7 ? 'critical' : u.days_inactive > 3 ? 'warning' : 'mild'}`}>
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
