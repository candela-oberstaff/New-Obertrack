import { useState, useEffect } from 'react'
import { adminService } from '../services/api'
import { useAuth } from '../context/AuthContext'
import type { User } from '../types'
import './Admin.css'

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

interface CompanyMetric {
  id: number
  name: string
  professionals: number
  hours_this_month: number
  tasks_completed: number
  active_users: number
}

interface InactiveUser {
  id: number
  name: string
  email: string
  company: string
  last_active: string
  days_inactive: number
}

interface Activity {
  type: string
  user: string
  company: string
  details: string
  timestamp: string
}

export default function Admin() {
  useAuth() // initialize auth
  const [metrics, setMetrics] = useState<DashboardMetrics | null>(null)
  const [companies, setCompanies] = useState<CompanyMetric[]>([])
  const [inactiveUsers, setInactiveUsers] = useState<InactiveUser[]>([])
  const [activities, setActivities] = useState<Activity[]>([])
  const [users, setUsers] = useState<User[]>([])
  const [activeTab, setActiveTab] = useState<'dashboard' | 'companies' | 'users' | 'inactive'>('dashboard')
  const [isLoading, setIsLoading] = useState(true)
  const [showUserModal, setShowUserModal] = useState(false)

  const [userForm, setUserForm] = useState({
    name: '',
    email: '',
    password: '',
    user_type: 'profesional',
    company_name: '',
    job_title: '',
    empleador_id: undefined as number | undefined,
    is_manager: false,
  })

  useEffect(() => {
    fetchDashboard()
  }, [])

  const fetchDashboard = async () => {
    setIsLoading(true)
    try {
      const [metricsRes, companiesRes, inactiveRes, activityRes, usersRes] = await Promise.all([
        adminService.getDashboard(),
        adminService.getCompanies(),
        adminService.getInactiveUsers(7),
        adminService.getRecentActivity(),
        adminService.getUsers({}),
      ])
      setMetrics(metricsRes)
      setCompanies(companiesRes)
      setInactiveUsers(inactiveRes)
      setActivities(activityRes)
      setUsers(usersRes.data)
    } catch (error) {
      console.error('Error fetching admin data:', error)
    } finally {
      setIsLoading(false)
    }
  }

  const handleCreateUser = async (e: React.FormEvent) => {
    e.preventDefault()
    try {
      await adminService.createUser(userForm)
      setShowUserModal(false)
      resetForm()
      fetchDashboard()
    } catch (error) {
      console.error('Error creating user:', error)
    }
  }

  const handleDeleteUser = async (id: number) => {
    if (!confirm('¿Estás seguro de eliminar este usuario?')) return
    try {
      await adminService.deleteUser(id)
      fetchDashboard()
    } catch (error) {
      console.error('Error deleting user:', error)
    }
  }

  const handleToggleUserStatus = async (userData: User) => {
    try {
      await adminService.updateUser(userData.id, { is_active: !userData.is_active })
      fetchDashboard()
    } catch (error) {
      console.error('Error toggling user status:', error)
    }
  }

  const resetForm = () => {
    setUserForm({
      name: '',
      email: '',
      password: '',
      user_type: 'profesional',
      company_name: '',
      job_title: '',
      empleador_id: undefined,
      is_manager: false,
    })
  }

  const getRoleColor = (userType: string, isManager: boolean, isSuperadmin: boolean) => {
    if (isSuperadmin) return '#8b5cf6'
    if (isManager) return '#f59e0b'
    if (userType === 'empleador') return '#3b82f6'
    return '#10b981'
  }

  const getRoleLabel = (userType: string, isManager: boolean, isSuperadmin: boolean) => {
    if (isSuperadmin) return 'Super Admin'
    if (isManager) return 'Manager'
    if (userType === 'empleador') return 'Empresa'
    return 'Profesional'
  }

  if (isLoading) {
    return (
      <div className="admin-loading">
        <div className="spinner" />
        <p>Cargando panel de administración...</p>
      </div>
    )
  }

  return (
    <div className="admin-page">
      <div className="admin-header">
        <div>
          <h1>Panel de Administración</h1>
          <p className="admin-subtitle">Monitor de Obertrack - Customer Success</p>
        </div>
        <button className="btn-primary" onClick={() => setShowUserModal(true)}>
          + Nuevo Usuario
        </button>
      </div>

      <div className="admin-tabs">
        <button 
          className={`tab-btn ${activeTab === 'dashboard' ? 'active' : ''}`}
          onClick={() => setActiveTab('dashboard')}
        >
          📊 Dashboard
        </button>
        <button 
          className={`tab-btn ${activeTab === 'companies' ? 'active' : ''}`}
          onClick={() => setActiveTab('companies')}
        >
          🏢 Empresas
        </button>
        <button 
          className={`tab-btn ${activeTab === 'users' ? 'active' : ''}`}
          onClick={() => setActiveTab('users')}
        >
          👥 Usuarios
        </button>
        <button 
          className={`tab-btn ${activeTab === 'inactive' ? 'active' : ''}`}
          onClick={() => setActiveTab('inactive')}
        >
          ⚠️ Inactivos {inactiveUsers.length > 0 && <span className="badge">{inactiveUsers.length}</span>}
        </button>
      </div>

      {activeTab === 'dashboard' && metrics && (
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
                <button className="alert-action" onClick={() => setActiveTab('inactive')}>
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
      )}

      {activeTab === 'companies' && (
        <div className="admin-content">
          <h3>Empresas Activas</h3>
          <div className="companies-table">
            <table>
              <thead>
                <tr>
                  <th>Empresa</th>
                  <th>Profesionales</th>
                  <th>Usuarios Activos</th>
                  <th>Horas Mes</th>
                  <th>Tareas Completadas</th>
                </tr>
              </thead>
              <tbody>
                {companies.map(company => (
                  <tr key={company.id}>
                    <td className="company-name">{company.name || 'Sin nombre'}</td>
                    <td>{company.professionals}</td>
                    <td>{company.active_users}</td>
                    <td>{company.hours_this_month.toFixed(1)}h</td>
                    <td>{company.tasks_completed}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {activeTab === 'users' && (
        <div className="admin-content">
          <h3>Todos los Usuarios</h3>
          <div className="users-table">
            <table>
              <thead>
                <tr>
                  <th>Nombre</th>
                  <th>Email</th>
                  <th>Tipo</th>
                  <th>Empresa</th>
                  <th>Estado</th>
                  <th>Acciones</th>
                </tr>
              </thead>
              <tbody>
                {users.map(u => (
                  <tr key={u.id}>
                    <td>{u.name}</td>
                    <td>{u.email}</td>
                    <td>
                      <span className="role-badge" style={{ background: getRoleColor(u.user_type, u.is_manager || false, u.is_superadmin || false) }}>
                        {getRoleLabel(u.user_type, u.is_manager || false, u.is_superadmin || false)}
                      </span>
                    </td>
                    <td>{u.company_name || '-'}</td>
                    <td>
                      <span className={`status-pill ${u.is_active !== false ? 'active' : 'inactive'}`}>
                        {u.is_active !== false ? 'Activo' : 'Inactivo'}
                      </span>
                    </td>
                    <td>
                      <div className="action-buttons">
                        <button 
                          className="btn-icon"
                          onClick={() => handleToggleUserStatus(u)}
                          title={u.is_active !== false ? 'Desactivar' : 'Activar'}
                        >
                          {u.is_active !== false ? '🚫' : '✅'}
                        </button>
                        <button 
                          className="btn-icon danger"
                          onClick={() => handleDeleteUser(u.id)}
                          title="Eliminar"
                        >
                          🗑️
                        </button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {activeTab === 'inactive' && (
        <div className="admin-content">
          <h3>Profesionales Inactivos</h3>
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
      )}

      {showUserModal && (
        <div className="modal-overlay" onClick={() => { setShowUserModal(false); resetForm() }}>
          <div className="modal" onClick={e => e.stopPropagation()}>
            <div className="modal-header">
              <h2>Nuevo Usuario</h2>
              <button className="close-btn" onClick={() => { setShowUserModal(false); resetForm() }}>✕</button>
            </div>
            <form onSubmit={handleCreateUser}>
              <div className="form-group">
                <label>Nombre</label>
                <input
                  type="text"
                  value={userForm.name}
                  onChange={e => setUserForm({ ...userForm, name: e.target.value })}
                  required
                />
              </div>
              <div className="form-group">
                <label>Email</label>
                <input
                  type="email"
                  value={userForm.email}
                  onChange={e => setUserForm({ ...userForm, email: e.target.value })}
                  required
                />
              </div>
              <div className="form-group">
                <label>Contraseña</label>
                <input
                  type="password"
                  value={userForm.password}
                  onChange={e => setUserForm({ ...userForm, password: e.target.value })}
                  required
                />
              </div>
              <div className="form-group">
                <label>Tipo de Usuario</label>
                <select
                  value={userForm.user_type}
                  onChange={e => setUserForm({ ...userForm, user_type: e.target.value })}
                >
                  <option value="profesional">Profesional</option>
                  <option value="empleador">Empresa</option>
                </select>
              </div>
              {userForm.user_type === 'empleador' && (
                <div className="form-group">
                  <label>Nombre de Empresa</label>
                  <input
                    type="text"
                    value={userForm.company_name}
                    onChange={e => setUserForm({ ...userForm, company_name: e.target.value })}
                  />
                </div>
              )}
              {userForm.user_type === 'profesional' && (
                <div className="form-group">
                  <label>Empresa (Empleador)</label>
                  <select
                    value={userForm.empleador_id || ''}
                    onChange={e => setUserForm({ ...userForm, empleador_id: e.target.value ? Number(e.target.value) : undefined })}
                  >
                    <option value="">Seleccionar empresa...</option>
                    {companies.filter(c => c.name).map(c => (
                      <option key={c.id} value={c.id}>{c.name}</option>
                    ))}
                  </select>
                </div>
              )}
              <div className="form-group">
                <label>Puesto</label>
                <input
                  type="text"
                  value={userForm.job_title}
                  onChange={e => setUserForm({ ...userForm, job_title: e.target.value })}
                />
              </div>
              <div className="modal-actions">
                <button type="button" className="btn-cancel" onClick={() => { setShowUserModal(false); resetForm() }}>
                  Cancelar
                </button>
                <button type="submit" className="btn-primary">
                  Crear Usuario
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  )
}
