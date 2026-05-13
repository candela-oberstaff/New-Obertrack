import { NavLink, Outlet, useNavigate, useLocation } from 'react-router-dom'
import { useAuth } from '../../context/AuthContext'
import Notifications from '../Notifications'
import { useState, useEffect } from 'react'
import { channelService } from '../../services/api'
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
  LogOut,
  Plug
} from 'lucide-react'
import styles from './Layout.module.css'

export default function Layout() {
  const { user, logout, token } = useAuth()
  const navigate = useNavigate()
  const location = useLocation()
  const [sidebarCollapsed, setSidebarCollapsed] = useState(false)
  const [totalChatUnread, setTotalChatUnread] = useState(0)
  const [avatarError, setAvatarError] = useState(false)

  const isChatPage = location.pathname.startsWith('/chat')

  useEffect(() => {
    if (!token) return

    const fetchTotalUnread = async () => {
      try {
        const count = await channelService.getTotalUnreadCount()
        setTotalChatUnread(count)
      } catch (error) {
        console.error('Error fetching chat unread count:', error)
      }
    }

    fetchTotalUnread()
    const interval = setInterval(fetchTotalUnread, 30000)

    const handleChatUpdate = () => fetchTotalUnread()
    window.addEventListener('chat-unread-updated', handleChatUpdate)

    return () => {
      clearInterval(interval)
      window.removeEventListener('chat-unread-updated', handleChatUpdate)
    }
  }, [token])

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
          {sidebarCollapsed
            ? <img src="/logos/Isotipo_Color.png" alt="Obertrack" className={styles['sidebar-isotipo']} />
            : <img src="/logos/Horizontal_Blanco.png" alt="Obertrack" className={styles['sidebar-logo']} />
          }
        </div>

        <div className={styles['sidebar-user']}>
          <div className={styles['user-avatar']}>
            {user?.avatar && !avatarError ? (
              <img 
                src={user.avatar} 
                alt={user.name} 
                className={styles['avatar-img']} 
                onError={() => setAvatarError(true)}
              />
            ) : (
              user?.name?.charAt(0).toUpperCase()
            )}
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
              <span className={styles['nav-icon']}>
                {item.icon}
                {item.path === '/chat' && totalChatUnread > 0 && !isChatPage && (
                  <span className={styles['nav-badge']}>{totalChatUnread > 9 ? '9+' : totalChatUnread}</span>
                )}
              </span>
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
          <div className={styles['top-bar-actions']}>
            <NavLink
              to="/google-chat"
              className={({ isActive }) => `${styles['plugin-btn']} ${isActive ? styles['active'] : ''}`}
              title="Google Chat"
            >
              <Plug size={20} />
            </NavLink>
            <Notifications />
          </div>
        </div>
        <div className={`${styles['outlet-container']} ${isChatPage ? styles['chat-layout'] : ''}`}>
          <Outlet />
        </div>
      </main>
    </div>
  )
}
