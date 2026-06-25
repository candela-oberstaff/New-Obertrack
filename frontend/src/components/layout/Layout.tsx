import { NavLink, Outlet, useNavigate, useLocation } from 'react-router-dom'
import { useAuth } from '../../context/AuthContext'
import { useQueryClient } from '@tanstack/react-query'
import { authService } from '../../services/api'
import Notifications from '../Notifications'
import { useState, useEffect } from 'react'
import { channelService } from '../../services/api'
import {
  LayoutDashboard,
  CheckSquare,
  Clock,
  FileText,
  MessageCircle,
  User,
  Settings,
  ChevronRight,
  ChevronLeft,
  LogOut,
  Wrench,
  Menu,
  X,
  Inbox,
  MessageSquare,
  GraduationCap,
  Building2,
  Users,
  Compass,
  Map,
  MapPin,
  Shield,
  UserCog,
  AlertTriangle,
} from 'lucide-react'

// Módulo de permisos (roles) que gobierna cada entrada del sidebar.
const MODULE_BY_PATH: Record<string, string> = {
  '/tasks': 'tasks',
  '/work-hours': 'hours',
  '/reports': 'reports',
  '/chat': 'chat',
  '/tickets': 'tickets',
  '/tutoriales': 'tutorials',
}
import Avatar from '../Common/Avatar'
import { startCurrentPageTour, startSystemTour } from '../../lib/tour'
import styles from './Layout.module.css'

// Module-level flag prevents the auto-tour from firing more than once per
// browser session, even if the Layout component unmounts/remounts.
let systemTourShownThisSession = false

export default function Layout() {
  const { user, setUser, logout } = useAuth()
  const navigate = useNavigate()
  const location = useLocation()
  const qc = useQueryClient()
  const [sidebarCollapsed, setSidebarCollapsed] = useState(true)
  const [totalChatUnread, setTotalChatUnread] = useState(0)
  const [isMobileSidebarOpen, setIsMobileSidebarOpen] = useState(false)
  const [switchingCompany, setSwitchingCompany] = useState(false)

  // Switcher multi-empresa: cambia la empresa activa, re-emite la sesión y
  // recarga todos los datos (que se scopean por el tenant del nuevo JWT).
  const handleSwitchCompany = async (companyId: number) => {
    if (companyId === user?.empleador_id || switchingCompany) return
    setSwitchingCompany(true)
    try {
      const updated = await authService.switchCompany(companyId)
      setUser(updated)
      await qc.invalidateQueries()
    } catch {
      /* el backend rechaza si no pertenece a esa empresa */
    } finally {
      setSwitchingCompany(false)
    }
  }

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
    if (systemTourShownThisSession) return
    const key = `obertrack_tour_seen_${user.id}`
    if (localStorage.getItem(key)) return
    localStorage.setItem(key, '1')
    systemTourShownThisSession = true
    const timer = setTimeout(() => startSystemTour(user.user_type, user.is_manager), 800)
    return () => clearTimeout(timer)
  }, [user?.id])

  const handleLogout = () => {
    logout()
    navigate('/login')
  }

  // Orden de menú y visibilidad por rol según la especificación del sistema:
  // el superadmin ve todo; CS (manager y analista) suman Admin/Empresas/
  // Tickets/Tools; el Analista de IT solo Métricas, Auditoría y Perfil;
  // Reportes y Roles y Grupos son de empresas (y superadmin).
  const isSuper = !!user?.is_superadmin
  const isCS = !isSuper && user?.user_type === 'customer_success'
  const isIT = !isSuper && user?.user_type === 'analista_it'
  const isEmployerType = user?.user_type === 'empleador'

  const navItems = [
    { path: '/dashboard', label: 'Dashboard', icon: <LayoutDashboard size={20} />, show: !isIT },
    { path: '/empresa', label: 'Profesionales', icon: <Users size={20} />, show: isEmployerType },
    { path: '/tasks', label: 'Tareas', icon: <CheckSquare size={20} />, show: !isIT },
    { path: '/work-hours', label: 'Horas', icon: <Clock size={20} />, show: !isIT },
    { path: '/reports', label: 'Reportes', icon: <FileText size={20} />, show: isSuper || isEmployerType },
    // Roles y Grupos: oculto para empresas en esta versión (solo superadmin).
    { path: '/roles-grupos', label: 'Roles y Grupos', icon: <UserCog size={20} />, show: isSuper },
    { path: '/chat', label: 'Chat', icon: <MessageCircle size={20} />, show: !isIT },
    { path: '/admin', label: 'Admin', icon: <Settings size={20} />, show: isSuper || isCS },
    { path: '/admin/tenants', label: 'Empresas', icon: <Building2 size={20} />, show: isSuper || isCS },
    { path: '/admin/mapa', label: 'Mapa', icon: <MapPin size={20} />, show: isSuper },
    { path: '/admin/incidentes', label: 'Incidentes', icon: <AlertTriangle size={20} />, show: isSuper },
    { path: '/tickets', label: 'Tickets', icon: <Inbox size={20} />, show: isCS || isSuper},
    { path: '/admin/tools', label: 'Tools', icon: <Wrench size={20} />, show: isSuper || isCS },
    { path: '/admin/audit', label: 'Auditoría', icon: <Shield size={20} />, show: isSuper || isIT },
    { path: '/tutoriales', label: 'Tutoriales', icon: <GraduationCap size={20} />, show: !isIT },
    { path: '/profile', label: 'Perfil', icon: <User size={20} />, show: true },
  ].filter(item => {
    if (!item.show) return false
    // Permisos por rol: si el usuario tiene roles asignados y el módulo quedó
    // en "sin acceso", se oculta del sidebar (el backend igual lo bloquea).
    // El superadmin y la cuenta empresa nunca se restringen (igual que el
    // backend en RequirePermission): ven todas sus páginas siempre.
    if (isSuper || isEmployerType) return true
    const moduleKey = MODULE_BY_PATH[item.path]
    if (moduleKey && user?.permissions && user.permissions[moduleKey] === 'none') return false
    return true
  })
  const getRoleLabel = () => {
    if (user?.is_superadmin) return 'Super Admin'
    if (user?.user_type === 'customer_success') return user?.is_manager ? 'CS Manager' : 'CS Analista'
    if (user?.user_type === 'analista_it') return 'Analista de IT'
    if (user?.is_manager) return 'Manager'
    if (user?.user_type === 'empleador') return 'Empresa'
    if (user?.user_type === 'superadmin') return 'Super Admin'
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

        {(!sidebarCollapsed || isMobileSidebarOpen) && (user?.companies?.length ?? 0) > 1 && (
          <div className={styles['company-switcher']}>
            <Building2 size={16} className={styles['company-switcher-icon']} />
            <select
              className={styles['company-switcher-select']}
              value={user?.empleador_id ?? ''}
              disabled={switchingCompany}
              onChange={(e) => handleSwitchCompany(Number(e.target.value))}
              title="Cambiar de empresa activa"
            >
              {user?.companies?.map((c) => (
                <option key={c.id} value={c.id}>{c.name}</option>
              ))}
            </select>
          </div>
        )}

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
              onClick={() => startSystemTour(user?.user_type, user?.is_manager)}
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
            {(user?.is_superadmin || user?.user_type === 'customer_success') && (
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
