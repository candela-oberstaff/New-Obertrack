import { NavLink, Outlet, useNavigate, useLocation } from 'react-router-dom'
import { useAuth } from '../../context/AuthContext'
import Notifications from '../Notifications'
import { useState, useEffect } from 'react'
import { channelService } from '../../services/api'
import {
  LayoutDashboard,
  CheckSquare,
  Clock,
  FileText,
  Activity,
  MessageCircle,
  User,
  Settings,
  ChevronRight,
  ChevronLeft,
  LogOut,
  Plug,
  Wrench,
  Menu,
  X,
  Inbox,
  MessageSquare,
  GraduationCap,
  Building2,
  Compass,
  Map
} from 'lucide-react'
import Avatar from '../Common/Avatar'
import { startCurrentPageTour, startSystemTour } from '../../lib/tour'
import styles from './Layout.module.css'

export default function Layout() {
  const { user, logout } = useAuth()
  const navigate = useNavigate()
  const location = useLocation()
  const [sidebarCollapsed, setSidebarCollapsed] = useState(true)
  const [totalChatUnread, setTotalChatUnread] = useState(0)
  const [isMobileSidebarOpen, setIsMobileSidebarOpen] = useState(false)

  const isChatPage = location.pathname.startsWith('/chat') || location.pathname.startsWith('/whatsapp')

  useEffect(() => {
    setIsMobileSidebarOpen(false)
  }, [location.pathname])


  useEffect(() => {
    if (!user) return

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
  }, [user])

  useEffect(() => {
    if (!user?.id || window.innerWidth < 768) return
    const key = `obertrack_tour_seen_${user.id}`
    if (localStorage.getItem(key)) return
    localStorage.setItem(key, '1')
    const timer = setTimeout(() => startSystemTour(), 800)
    return () => clearTimeout(timer)
  }, [user?.id])

  const handleLogout = () => {
    logout()
    navigate('/login')
  }

  const navItems = [
    { path: '/dashboard', label: 'Dashboard', icon: <LayoutDashboard size={20} /> },
    { path: '/tasks', label: 'Tareas', icon: <CheckSquare size={20} /> },
    { path: '/tickets', label: 'Tickets', icon: <Inbox size={20} />, customerSuccessOnly: true },
    { path: '/work-hours', label: 'Horas', icon: <Clock size={20} /> },
    { path: '/reports', label: 'Reportes', icon: <FileText size={20} />, adminOnly: true },
    { path: '/chat', label: 'Chat', icon: <MessageCircle size={20} /> },
    { path: '/tutoriales', label: 'Tutoriales', icon: <GraduationCap size={20} /> },
    { path: '/profile', label: 'Perfil', icon: <User size={20} /> },
  ].filter(item => {
    if (item.customerSuccessOnly && user?.user_type !== 'customer_success') return false
    if (item.adminOnly && !user?.is_superadmin && user?.user_type !== 'empleador') return false
    return true
  })

  if (user?.is_superadmin) {
    navItems.splice(1, 0, { path: '/admin', label: 'Admin', icon: <Settings size={20} /> })
    navItems.splice(2, 0, { path: '/admin/tenants', label: 'Empresas', icon: <Building2 size={20} /> })
    // Add Tools after Chat (which is currently at index 5 or 6 depending on Admin)
    const toolsIndex = navItems.findIndex(item => item.path === '/admin/tools')
    if (toolsIndex !== -1) {
      navItems.splice(toolsIndex + 1, 0, { path: '/admin/metrics', label: 'Métricas', icon: <Activity size={20} /> })
    } else {
      const chatIndex = navItems.findIndex(item => item.path === '/chat')
      if (chatIndex !== -1) {
        navItems.splice(chatIndex + 1, 0, { path: '/admin/tools', label: 'Tools', icon: <Wrench size={20} /> })
        navItems.splice(chatIndex + 2, 0, { path: '/admin/metrics', label: 'Métricas', icon: <Activity size={20} /> })
      }
    }
  }

  const getRoleLabel = () => {
    if (user?.is_superadmin) return 'Super Admin'
    if (user?.is_manager) return 'Manager'
    if (user?.user_type === 'empleador') return 'Empresa'
    return 'Profesional'
  }

  return (
    <div className={`${styles['app-layout']} ${sidebarCollapsed && !isMobileSidebarOpen ? styles['sidebar-collapsed'] : ''} ${isMobileSidebarOpen ? styles['mobile-open'] : ''}`}>
      {isMobileSidebarOpen && (
        <div 
          className={styles['sidebar-overlay']} 
          onClick={() => setIsMobileSidebarOpen(false)}
        />
      )}
      <aside className={`${styles['sidebar']} ${isMobileSidebarOpen ? styles['mobile-open'] : ''}`}>
        <div className={styles['sidebar-header']}>
          {sidebarCollapsed
            ? <img src="/logos/Isotipo_Color.png" alt="Obertrack" className={styles['sidebar-isotipo']} />
            : <img src="/logos/Horizontal_Blanco.png" alt="Obertrack" className={styles['sidebar-logo']} />
          }
          <button 
            className={styles['close-mobile-btn']} 
            onClick={() => setIsMobileSidebarOpen(false)}
            aria-label="Cerrar menú"
          >
            <X size={20} />
          </button>
        </div>

        <div className={styles['sidebar-user']}>
          <Avatar 
            src={user?.avatar} 
            name={user?.name} 
            size="md" 
          />
          {(!sidebarCollapsed || isMobileSidebarOpen) && (
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
              end={navItems.some(other => other.path !== item.path && other.path.startsWith(item.path + '/'))}
              data-tour={item.path}
              className={({ isActive }) => `${styles['nav-item']} ${isActive ? styles['active'] : ''}`}
              title={item.label}
            >
              <span className={styles['nav-icon']}>
                {item.icon}
                {item.path === '/chat' && totalChatUnread > 0 && !isChatPage && (
                  <span className={styles['nav-badge']}>{totalChatUnread > 9 ? '9+' : totalChatUnread}</span>
                )}
              </span>
              {(!sidebarCollapsed || isMobileSidebarOpen) && <span className={styles['nav-label']}>{item.label}</span>}
            </NavLink>
          ))}
        </nav>

        <div className={styles['sidebar-footer']}>
          <button className={styles['collapse-btn']} onClick={() => setSidebarCollapsed(!sidebarCollapsed)} title={sidebarCollapsed ? "Expandir" : "Colapsar"}>
            {sidebarCollapsed ? <ChevronRight size={20} /> : <ChevronLeft size={20} />}
          </button>
          <button className={styles['logout-btn']} onClick={handleLogout} title="Cerrar Sesión">
            <LogOut size={20} />
            {(!sidebarCollapsed || isMobileSidebarOpen) && <span className={styles['btn-text']}>Cerrar Sesión</span>}
          </button>
        </div>
      </aside>

      <main className={styles['main-content']}>
        <div className={styles['top-bar']}>
          <button 
            className={styles['menu-toggle-btn']} 
            onClick={() => setIsMobileSidebarOpen(true)}
            aria-label="Abrir menú"
          >
            <Menu size={24} />
          </button>
          <div className={styles['top-bar-actions']}>
            <button
              type="button"
              className={`${styles['tour-btn']} ${styles['tour-btn-secondary']}`}
              onClick={startSystemTour}
              title="Recorrido del menú lateral"
              aria-label="Recorrido del menú lateral"
            >
              <Map size={18} />
              <span>Menú</span>
            </button>
            <button
              type="button"
              className={styles['tour-btn']}
              onClick={() => startCurrentPageTour(location.pathname)}
              title="Recorrido guiado"
              aria-label="Recorrido guiado"
              data-tour="topbar-current-tour"
            >
              <Compass size={18} />
              <span>Recorrido guiado</span>
            </button>
            {user?.user_type === 'customer_success' && (
              <NavLink
                to="/whatsapp"
                className={({ isActive }) => `${styles['plugin-btn']} ${styles['plugin-btn-wa']} ${isActive ? styles['active'] : ''}`}
                title="WhatsApp"
              >
                <MessageSquare size={20} />
              </NavLink>
            )}
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
