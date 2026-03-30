import { NavLink, Outlet, useNavigate, useLocation } from 'react-router-dom'
import { useAuth } from '../../context/AuthContext'
import Notifications from '../Notifications'
import { useState } from 'react'
import { 
  LayoutDashboard, 
  CheckSquare, 
  Clock, 
  BarChart3, 
  MessageCircle, 
  User, 
  Settings,
  ChevronRight,
  ChevronLeft,
  LogOut
} from 'lucide-react'
import styles from './Layout.module.css'

export default function Layout() {
  const { user, logout } = useAuth()
  const navigate = useNavigate()
  const location = useLocation()
  const [sidebarCollapsed, setSidebarCollapsed] = useState(false)
  
  const isChatPage = location.pathname === '/chat'

  const handleLogout = () => {
    logout()
    navigate('/login')
  }

  const navItems = [
    { path: '/dashboard', label: 'Dashboard', icon: <LayoutDashboard size={20} /> },
    { path: '/tasks', label: 'Tareas', icon: <CheckSquare size={20} /> },
    { path: '/work-hours', label: 'Horas', icon: <Clock size={20} /> },
    { path: '/reports', label: 'Reportes', icon: <BarChart3 size={20} /> },
    { path: '/chat', label: 'Chat', icon: <MessageCircle size={20} /> },
    { path: '/profile', label: 'Perfil', icon: <User size={20} /> },
  ]

  if (user?.is_superadmin) {
    navItems.splice(1, 0, { path: '/admin', label: 'Admin', icon: <Settings size={20} /> })
  }

  const getRoleLabel = () => {
    if (user?.is_superadmin) return 'Super Admin'
    if (user?.is_manager) return 'Manager'
    if (user?.user_type === 'empleador') return 'Empresa'
    return 'Profesional'
  }

  return (
    <div className={`${styles['app-layout']} ${sidebarCollapsed ? styles['sidebar-collapsed'] : ''}`}>
      <aside className={styles['sidebar']}>
        <div className={styles['sidebar-header']}>
          <h1>{sidebarCollapsed ? 'O' : 'Obertrack'}</h1>
        </div>
        
        <div className={styles['sidebar-user']}>
          <div className={styles['user-avatar']}>
            {user?.name?.charAt(0).toUpperCase()}
          </div>
          {!sidebarCollapsed && (
            <div className={styles['user-info']}>
              <span className={styles['user-name']}>{user?.name}</span>
              <span className={styles['user-role']}>{getRoleLabel()}</span>
            </div>
          )}
        </div>

        <nav className={styles['sidebar-nav']}>
          {navItems.map((item) => (
            <NavLink
              key={item.path}
              to={item.path}
              className={({ isActive }) => `${styles['nav-item']} ${isActive ? styles['active'] : ''}`}
              title={item.label}
            >
              <span className={styles['nav-icon']}>{item.icon}</span>
              {!sidebarCollapsed && <span className={styles['nav-label']}>{item.label}</span>}
            </NavLink>
          ))}
        </nav>

        <div className={styles['sidebar-footer']}>
          <button className={styles['collapse-btn']} onClick={() => setSidebarCollapsed(!sidebarCollapsed)} title={sidebarCollapsed ? "Expandir" : "Colapsar"}>
            {sidebarCollapsed ? <ChevronRight size={20} /> : <ChevronLeft size={20} />}
          </button>
          <button className={styles['logout-btn']} onClick={handleLogout} title="Cerrar Sesión">
            <LogOut size={20} />
            {!sidebarCollapsed && <span className={styles['btn-text']}>Cerrar Sesión</span>}
          </button>
        </div>
      </aside>

      <main className={styles['main-content']}>
        <div className={styles['top-bar']}>
          <Notifications />
        </div>
        <div className={`${styles['outlet-container']} ${isChatPage ? styles['chat-layout'] : ''}`}>
          <Outlet />
        </div>
      </main>
    </div>
  )
}
