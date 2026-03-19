import { NavLink, Outlet, useNavigate } from 'react-router-dom'
import { useAuth } from '../../context/AuthContext'
import Notifications from '../Notifications'
import { useState } from 'react'
import './Layout.css'

export default function Layout() {
  const { user, logout } = useAuth()
  const navigate = useNavigate()
  const [sidebarCollapsed, setSidebarCollapsed] = useState(false)

  const handleLogout = () => {
    logout()
    navigate('/login')
  }

  const navItems = [
    { path: '/dashboard', label: 'Dashboard', icon: '▣' },
    { path: '/tasks', label: 'Tareas', icon: '☐' },
    { path: '/work-hours', label: 'Horas', icon: '◷' },
    { path: '/reports', label: 'Reportes', icon: '◑' },
    { path: '/chat', label: 'Chat', icon: '◉' },
    { path: '/profile', label: 'Perfil', icon: '○' },
  ]

  if (user?.is_superadmin) {
    navItems.splice(1, 0, { path: '/admin', label: 'Admin', icon: '⚙️' })
  }

  const getRoleLabel = () => {
    if (user?.is_superadmin) return 'Super Admin'
    if (user?.is_manager) return 'Manager'
    if (user?.user_type === 'empleador') return 'Empresa'
    return 'Profesional'
  }

  return (
    <div className={`app-layout ${sidebarCollapsed ? 'sidebar-collapsed' : ''}`}>
      <aside className="sidebar">
        <div className="sidebar-header">
          <h1>{sidebarCollapsed ? 'O' : 'Obertrack'}</h1>
        </div>
        
        <div className="sidebar-user">
          <div className="user-avatar">
            {user?.name?.charAt(0).toUpperCase()}
          </div>
          {!sidebarCollapsed && (
            <div className="user-info">
              <span className="user-name">{user?.name}</span>
              <span className="user-role">{getRoleLabel()}</span>
            </div>
          )}
        </div>

        <nav className="sidebar-nav">
          {navItems.map((item) => (
            <NavLink
              key={item.path}
              to={item.path}
              className={({ isActive }) => `nav-item ${isActive ? 'active' : ''}`}
              title={item.label}
            >
              <span className="nav-icon">{item.icon}</span>
              {!sidebarCollapsed && <span className="nav-label">{item.label}</span>}
            </NavLink>
          ))}
        </nav>

        <div className="sidebar-footer">
          <button className="collapse-btn" onClick={() => setSidebarCollapsed(!sidebarCollapsed)} style={{ marginBottom: '8px' }}>
            {sidebarCollapsed ? '▶' : '◀'}
          </button>
          <button className="logout-btn" onClick={handleLogout}>
            {sidebarCollapsed ? '⏻' : 'Cerrar Sesión'}
          </button>
        </div>
      </aside>

      <main className="main-content">
        <div className="top-bar">
          <Notifications />
        </div>
        <Outlet />
      </main>
    </div>
  )
}
