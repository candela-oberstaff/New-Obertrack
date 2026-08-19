import { NavLink, Outlet, useNavigate, useLocation } from 'react-router-dom'
import { useAuth } from '../../context/AuthContext'
import { useQueryClient } from '@tanstack/react-query'
import { authService } from '../../services/api'
import Notifications from '../Notifications'
import { NovedadAnnouncer } from '../Tutorials/NovedadAnnouncer'
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
  Network,
  Users,
  Compass,
  Map,
  MapPin,
  Shield,
  UserCog,
  AlertTriangle,
  Wallet,
  LifeBuoy,
  Trash2,
  SlidersHorizontal,
  Video,
} from 'lucide-react'

// Módulo de permisos (roles) que gobierna cada entrada del sidebar.
const MODULE_BY_PATH: Record<string, string> = {
  '/tasks': 'tasks',
  '/work-hours': 'hours',
  '/sesiones': 'meetings',
  '/reports': 'reports',
  '/chat': 'chat',
  '/tickets': 'tickets',
  '/novedades': 'tutorials',
}
import Avatar from '../Common/Avatar'
import { usePushNotifications } from '../../hooks/usePushNotifications'
import { startCurrentPageTour, startSystemTour } from '../../lib/tour'
import { WALLET_ENABLED, GOOGLE_INTEGRATIONS_ENABLED } from '../../config/features'
import { hierarchyLabel } from '../../lib/permissions'
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

  // Web Push: con permiso concedido renueva la suscripción en silencio; si el
  // permiso está sin decidir, ofrece activarlo con una tarjeta discreta.
  const push = usePushNotifications(!!user)
  const [enablingPush, setEnablingPush] = useState(false)

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
        // Misma empresa que el chat tiene seleccionada (la guarda ahí), para que
        // el badge y la barra hablen del mismo tenant.
        const stored = localStorage.getItem('preferred_company_id')
        const companyId = user.is_superadmin && stored ? Number(stored) : null
        const count = await channelService.getTotalUnreadCount(companyId)
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
  const isProfessional = user?.user_type === 'profesional'
  const isEndUser = isEmployerType || isProfessional
  // Customer Success trabaja con el alcance de un superadmin salvo en cuatro
  // pantallas: Papelera, Auditoría, Configuración y Novedades. Esas cuatro
  // siguen mirando `isSuper` a secas; el resto usa este.
  const isPlatformAdmin = isSuper || isCS

  const navItems = [
    { path: '/dashboard', label: 'Dashboard', icon: <LayoutDashboard size={20} />, show: !isIT },
    { path: '/empresa', label: 'Profesionales', icon: <Users size={20} />, show: isEmployerType },
    // La empresa mantiene su organigrama; el supervisor reordena su rama.
    { path: '/organigrama', label: 'Organigrama', icon: <Network size={20} />, show: isEmployerType || !!user?.is_supervisor },
    { path: '/tasks', label: 'Tareas', icon: <CheckSquare size={20} />, show: !isIT },
    { path: '/work-hours', label: 'Horas', icon: <Clock size={20} />, show: !isIT },
    { path: '/sesiones', label: 'Sesiones', icon: <Video size={20} />, show: GOOGLE_INTEGRATIONS_ENABLED && !isIT },
    { path: '/wallet', label: 'Wallet', icon: <Wallet size={20} />, show: WALLET_ENABLED && isProfessional },
    { path: '/reports', label: 'Reportes', icon: <FileText size={20} />, show: isPlatformAdmin || isEmployerType },
    // Roles y Grupos: oculto para empresas en esta versión (solo superadmin).
    { path: '/roles-grupos', label: 'Roles y Grupos', icon: <UserCog size={20} />, show: isPlatformAdmin },
    { path: '/chat', label: 'Chat', icon: <MessageCircle size={20} />, show: !isIT },
    { path: '/admin', label: 'Admin', icon: <Settings size={20} />, show: isSuper || isCS },
    { path: '/admin/tenants', label: 'Empresas', icon: <Building2 size={20} />, show: isSuper || isCS },
    { path: '/admin/mapa', label: 'Mapa', icon: <MapPin size={20} />, show: isPlatformAdmin },
    { path: '/admin/incidentes', label: 'Incidentes', icon: <AlertTriangle size={20} />, show: isPlatformAdmin },
    { path: '/tickets', label: 'Tickets', icon: <Inbox size={20} />, show: false },
    { path: '/tickets/soporte', label: 'Soporte', icon: <LifeBuoy size={20} />, show: isCS || isSuper || isIT },
    { path: '/admin/tools', label: 'Tools', icon: <Wrench size={20} />, show: isSuper || isCS },
    { path: '/admin/audit', label: 'Auditoría', icon: <Shield size={20} />, show: isSuper || isIT },
    { path: '/admin/settings', label: 'Configuración', icon: <SlidersHorizontal size={20} />, show: isSuper },
    { path: '/papelera', label: 'Papelera', icon: <Trash2 size={20} />, show: isSuper },
    { path: '/novedades', label: 'Novedades', icon: <GraduationCap size={20} />, show: !isIT && !isCS },
    { path: '/soporte', label: 'Soporte', icon: <LifeBuoy size={20} />, show: isEndUser },
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
    const level = hierarchyLabel(user)
    if (level) return level
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

        {/* Las novedades sin ver salen al frente nada más entrar, sin depender
            de que alguien se acuerde de visitar la sección. */}
        <NovedadAnnouncer />

        {push.canPrompt && (
          <div
            style={{
              position: 'fixed', right: 16, bottom: 16, zIndex: 60, maxWidth: 320,
              background: '#fff', border: '1px solid #e2e8f0', borderRadius: 12,
              boxShadow: '0 8px 24px rgba(15,23,42,0.12)', padding: '14px 16px',
            }}
          >
            <p style={{ margin: 0, fontSize: 13.5, fontWeight: 700, color: '#0f172a' }}>
              🔔 ¿Activar notificaciones del navegador?
            </p>
            <p style={{ margin: '6px 0 12px', fontSize: 12.5, color: '#64748b', lineHeight: 1.4 }}>
              Entérate de menciones, mensajes y avisos de soporte aunque tengas la pestaña cerrada.
            </p>
            <div style={{ display: 'flex', gap: 8, justifyContent: 'flex-end' }}>
              <button
                onClick={push.dismiss}
                style={{ border: '1px solid #e2e8f0', background: '#fff', color: '#475569', borderRadius: 8, padding: '6px 12px', fontSize: 12.5, fontWeight: 600, cursor: 'pointer', width: 'auto' }}
              >
                Ahora no
              </button>
              <button
                disabled={enablingPush}
                onClick={async () => {
                  setEnablingPush(true)
                  try { await push.enable() } finally { setEnablingPush(false); push.dismiss() }
                }}
                style={{ border: 'none', background: '#7c3aed', color: '#fff', borderRadius: 8, padding: '6px 14px', fontSize: 12.5, fontWeight: 700, cursor: enablingPush ? 'wait' : 'pointer', width: 'auto' }}
              >
                {enablingPush ? 'Activando…' : 'Activar'}
              </button>
            </div>
          </div>
        )}
      </main>
    </div>
  )
}
