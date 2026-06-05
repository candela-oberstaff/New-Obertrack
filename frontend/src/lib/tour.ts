import { driver, type DriveStep } from 'driver.js'
import 'driver.js/dist/driver.css'
import './tour.css'

interface TourSection {
  path: string
  title: string
  description: string
}

interface ModuleTourStep {
  selector?: string
  title: string
  description: string
  side?: 'top' | 'right' | 'bottom' | 'left'
  align?: 'start' | 'center' | 'end'
}

interface ModuleTour {
  match: (pathname: string) => boolean
  title: string
  description: string
  steps: ModuleTourStep[]
}

function getSections(role?: string): TourSection[] {
  const isEmployer = role === 'empleador' || role === 'empresa'
  const isManager = role === 'manager'

  const workHoursDesc = isEmployer || isManager
    ? 'Revisa y aprueba las jornadas registradas por tu profesional.'
    : 'Registra tus jornadas y lleva control de tus horas trabajadas.'

  return [
    { path: '/dashboard', title: 'Dashboard', description: 'Tu punto de partida: un resumen general de la actividad y los indicadores clave.' },
    { path: '/admin', title: 'Admin', description: 'Panel para gestionar usuarios de la plataforma y ver la actividad.' },
    { path: '/admin/tenants', title: 'Empresas', description: 'Administra las empresas, sus empleados y el seguimiento de cada una.' },
    { path: '/tasks', title: 'Tareas', description: 'Tableros y tareas para organizar el trabajo del equipo por fases.' },
    { path: '/tickets', title: 'Tickets', description: 'Bandeja de soporte para atender y dar seguimiento a las solicitudes.' },
    { path: '/work-hours', title: 'Horas', description: workHoursDesc },
    { path: '/reports', title: 'Reportes', description: 'Informes y exportaciones de horas y productividad.' },
    { path: '/chat', title: 'Chat', description: 'Mensajería interna por canales y mensajes directos con tu equipo.' },
    { path: '/admin/tools', title: 'Tools', description: 'Herramientas administrativas avanzadas de la plataforma.' },
    { path: '/admin/metrics', title: 'Métricas', description: 'Métricas globales del sistema.' },
    { path: '/tutoriales', title: 'Tutoriales', description: 'Guías en video para aprender a usar cada sección de Obertrack.' },
    { path: '/profile', title: 'Perfil', description: 'Tu cuenta: datos personales, preferencias y configuración.' },
  ]
}

const MODULE_TOURS: ModuleTour[] = [
  {
    match: (pathname) => pathname === '/dashboard',
    title: 'Dashboard',
    description: 'Este recorrido te muestra el resumen principal, los accesos rápidos y los bloques de seguimiento diario.',
    steps: [
      { selector: '[data-tour="dashboard-header"]', title: 'Inicio del día', description: 'Aquí ves el saludo, la fecha y el acceso rápido para crear o revisar tareas.', side: 'bottom', align: 'start' },
      { selector: '[data-tour="dashboard-stats"]', title: 'Indicadores clave', description: 'Resume horas de la semana, horas aprobadas, pendientes y tareas completadas.', side: 'bottom', align: 'center' },
      { selector: '[data-tour="dashboard-hours-chart"]', title: 'Horas de la semana', description: 'Compara las horas registradas por día contra la meta diaria.', side: 'right', align: 'start' },
      { selector: '[data-tour="dashboard-tasks-card"]', title: 'Próximas tareas', description: 'Muestra pendientes recientes y permite entrar al módulo de tareas.', side: 'left', align: 'start' },
      { selector: '[data-tour="dashboard-hours-card"]', title: 'Registro reciente', description: 'Lista las jornadas más recientes y su estado de aprobación.', side: 'left', align: 'start' },
      { selector: '[data-tour="dashboard-team-card"]', title: 'Equipo de trabajo', description: 'Disponible para roles de gestión; muestra miembros del equipo asociados.', side: 'top', align: 'start' },
      { selector: '[data-tour="dashboard-quick-actions"]', title: 'Accesos rápidos', description: 'Atajos para navegar a tareas, horas, chat y perfil sin volver al menú lateral.', side: 'top', align: 'center' },
    ],
  },
  {
    match: (pathname) => pathname === '/admin',
    title: 'Panel de Administración',
    description: 'Este módulo permite monitorear actividad y administrar usuarios.',
    steps: [
      { selector: '[data-tour="admin-header"]', title: 'Administración', description: 'Desde aquí gestionas usuarios y revisas actividad general del sistema.', side: 'bottom', align: 'start' },
      { selector: '[data-tour="admin-tabs"]', title: 'Pestañas del módulo', description: 'Cambia entre resumen, listado de usuarios y registro de actividad.', side: 'bottom', align: 'start' },
      { selector: '[data-tour="admin-stats"]', title: 'Resumen operativo', description: 'Estos indicadores muestran usuarios, tableros, tareas y actividad base.', side: 'bottom', align: 'center' },
      { selector: '[data-tour="admin-recent-activity"]', title: 'Actividad reciente', description: 'Últimos eventos registrados para detectar movimiento o cambios importantes.', side: 'top', align: 'start' },
      { selector: '[data-tour="admin-search"]', title: 'Buscar usuarios', description: 'Filtra usuarios por nombre o correo antes de ejecutar acciones.', side: 'bottom', align: 'start' },
      { selector: '[data-tour="admin-users-table"]', title: 'Tabla de usuarios', description: 'Aquí se revisan datos, estado y tipo de usuario.', side: 'top', align: 'start' },
      { selector: '[data-tour="admin-user-actions"]', title: 'Acciones críticas', description: 'Permite ver detalles, activar/desactivar, resetear contraseña, promover o eliminar usuarios.', side: 'left', align: 'center' },
      { selector: '[data-tour="admin-activity-list"]', title: 'Registro completo', description: 'En la pestaña Actividad se consulta el historial de eventos del sistema.', side: 'top', align: 'start' },
    ],
  },
  {
    match: (pathname) => pathname === '/tasks',
    title: 'Tareas',
    description: 'Este recorrido cubre tableros, miembros, fases y creación de tareas.',
    steps: [
      { selector: '[data-tour="tasks-header"]', title: 'Gestión de tareas', description: 'La cabecera contiene el tablero activo y las acciones principales.', side: 'bottom', align: 'start' },
      { selector: '[data-tour="tasks-board-selector"]', title: 'Selector de tablero', description: 'Elige el tablero en el que quieres trabajar.', side: 'bottom', align: 'start' },
      { selector: '[data-tour="tasks-create-board"]', title: 'Crear tablero', description: 'Crea un tablero nuevo para organizar tareas por proyecto o equipo.', side: 'bottom', align: 'center' },
      { selector: '[data-tour="tasks-join-board"]', title: 'Unirse a tablero', description: 'Busca tableros públicos o compartidos para incorporarte.', side: 'bottom', align: 'center' },
      { selector: '[data-tour="tasks-members"]', title: 'Miembros', description: 'Gestiona quién participa en el tablero seleccionado.', side: 'bottom', align: 'center' },
      { selector: '[data-tour="tasks-phases"]', title: 'Fases', description: 'Configura columnas o etapas del flujo de trabajo.', side: 'bottom', align: 'center' },
      { selector: '[data-tour="tasks-new-task"]', title: 'Nueva tarea', description: 'Crea tareas con prioridad, fecha, asignados y adjuntos.', side: 'bottom', align: 'end' },
      { selector: '[data-tour="tasks-board"]', title: 'Tablero de trabajo', description: 'Arrastra y actualiza tareas dentro de las fases del tablero.', side: 'top', align: 'start' },
      { selector: '[data-tour="tasks-empty"]', title: 'Estado inicial', description: 'Si no hay tablero seleccionado, usa estas acciones para empezar.', side: 'top', align: 'center' },
    ],
  },
  {
    match: (pathname) => pathname === '/work-hours',
    title: 'Horas',
    description: 'Este módulo registra jornadas, ausencias, recuperaciones y aprobaciones.',
    steps: [
      { selector: '[data-tour="work-hours-header"]', title: 'Registro de jornada', description: 'La cabecera cambia según el rol: profesionales registran, empresas revisan y aprueban.', side: 'bottom', align: 'start' },
      { selector: '[data-tour="work-hours-actions"]', title: 'Acciones principales', description: 'Registra el día, recupera horas o exporta reportes según permisos.', side: 'bottom', align: 'end' },
      { selector: '[data-tour="work-hours-alert"]', title: 'Alertas de recuperación', description: 'Cuando existen ausencias pendientes, el sistema muestra el saldo a recuperar.', side: 'bottom', align: 'start' },
      { selector: '[data-tour="work-hours-stats"]', title: 'Resumen de horas', description: 'Indicadores de jornada, semana, aprobaciones y ausencias.', side: 'bottom', align: 'center' },
      { selector: '[data-tour="work-hours-calendar"]', title: 'Calendario', description: 'Selecciona días para filtrar registros y revisar ausencias o jornadas completas.', side: 'right', align: 'start' },
      { selector: '[data-tour="work-hours-list"]', title: 'Listado de registros', description: 'Abre registros, revisa detalles y aprueba horas cuando tu rol lo permite.', side: 'left', align: 'start' },
    ],
  },
  {
    match: (pathname) => pathname === '/reports',
    title: 'Reportes',
    description: 'Este módulo permite analizar horas y tareas por periodo, empleado y tipo de reporte.',
    steps: [
      { selector: '[data-tour="reports-header"]', title: 'Reportes', description: 'Aquí se consultan análisis de productividad y rendimiento.', side: 'bottom', align: 'start' },
      { selector: '[data-tour="reports-filters"]', title: 'Filtros', description: 'Filtra por empleado, mes y tipo de información.', side: 'bottom', align: 'start' },
      { selector: '[data-tour="reports-stats"]', title: 'Indicadores', description: 'Resumen de horas o tareas según la pestaña seleccionada.', side: 'bottom', align: 'center' },
      { selector: '[data-tour="reports-charts"]', title: 'Gráficos', description: 'Visualización de tendencias y distribución de registros.', side: 'top', align: 'center' },
      { selector: '[data-tour="reports-detail"]', title: 'Detalle', description: 'Tabla con registros recientes o tareas críticas para revisar casos puntuales.', side: 'top', align: 'start' },
    ],
  },
  {
    match: (pathname) => pathname === '/chat',
    title: 'Chat',
    description: 'Este módulo centraliza la comunicación interna con usuarios del sistema.',
    steps: [
      { selector: '[data-tour="chat-sidebar"]', title: 'Usuarios', description: 'Busca y revisa usuarios disponibles para conversar.', side: 'right', align: 'start' },
      { selector: '[data-tour="chat-search"]', title: 'Buscar usuario', description: 'Filtra la lista para encontrar rápidamente a alguien del equipo.', side: 'right', align: 'center' },
      { selector: '[data-tour="chat-users"]', title: 'Lista de usuarios', description: 'Muestra estado de conexión y contactos disponibles.', side: 'right', align: 'start' },
      { selector: '[data-tour="chat-messages"]', title: 'Mensajes', description: 'Aquí aparece el historial de conversación con separadores por fecha.', side: 'left', align: 'start' },
      { selector: '[data-tour="chat-input"]', title: 'Enviar mensaje', description: 'Escribe y envía mensajes. Enter envía cuando hay contenido.', side: 'top', align: 'center' },
    ],
  },
  {
    match: (pathname) => pathname === '/profile',
    title: 'Perfil',
    description: 'Este módulo reúne datos personales, seguridad, estadísticas y paneles asociados al rol.',
    steps: [
      { selector: '[data-tour="profile-header"]', title: 'Datos principales', description: 'Foto, nombre y rol de la cuenta activa.', side: 'bottom', align: 'start' },
      { selector: '[data-tour="profile-form"]', title: 'Información personal', description: 'Edita tus datos cuando actives el modo de edición.', side: 'right', align: 'start' },
      { selector: '[data-tour="profile-options"]', title: 'Opciones', description: 'Cambia contraseña o alterna la edición del perfil.', side: 'left', align: 'start' },
      { selector: '[data-tour="profile-stats"]', title: 'Estadísticas de cuenta', description: 'Estado actual y antigüedad de la cuenta.', side: 'left', align: 'start' },
      { selector: '[data-tour="profile-account"]', title: 'Cuenta', description: 'Acción para cerrar sesión desde el perfil.', side: 'left', align: 'start' },
    ],
  },
  {
    match: (pathname) => pathname === '/admin/tenants',
    title: 'Empresas',
    description: 'Este módulo permite administrar empresas y responsables.',
    steps: [
      { selector: '[data-tour="tenants-header"]', title: 'Empresas', description: 'Vista principal para crear y consultar clientes de la plataforma.', side: 'bottom', align: 'start' },
      { selector: '[data-tour="tenants-create"]', title: 'Nueva empresa', description: 'Crea una empresa asociando un responsable existente.', side: 'bottom', align: 'end' },
      { selector: '[data-tour="tenants-kpis"]', title: 'Resumen', description: 'Indicadores rápidos de empresas, activas y usuarios asociados.', side: 'bottom', align: 'center' },
      { selector: '[data-tour="tenants-search"]', title: 'Buscar', description: 'Filtra empresas por nombre o correo del responsable.', side: 'bottom', align: 'start' },
      { selector: '[data-tour="tenants-list"]', title: 'Listado', description: 'Abre una empresa para ver detalle o cambia su estado cuando corresponda.', side: 'top', align: 'start' },
    ],
  },
  {
    match: (pathname) => pathname === '/tickets',
    title: 'Tickets',
    description: 'Este módulo organiza solicitudes de soporte por etapas.',
    steps: [
      { selector: '[data-tour="tickets-header"]', title: 'Bandeja de tickets', description: 'Cabecera del tablero y acciones de prueba o integración.', side: 'bottom', align: 'start' },
      { selector: '[data-tour="tickets-action"]', title: 'Acción de prueba', description: 'Permite simular un mensaje para validar el flujo de soporte.', side: 'bottom', align: 'end' },
      { selector: '[data-tour="tickets-board"]', title: 'Columnas de atención', description: 'Los tickets se agrupan por estado: nuevo, en progreso, esperando y cerrado.', side: 'top', align: 'start' },
    ],
  },
  {
    match: (pathname) => pathname === '/admin/metrics',
    title: 'Métricas',
    description: 'Este módulo muestra métricas globales de engagement y rendimiento.',
    steps: [
      { selector: '[data-tour="metrics-header"]', title: 'Métricas globales', description: 'Resumen de engagement basado en eventos registrados.', side: 'bottom', align: 'start' },
      { selector: '[data-tour="metrics-period"]', title: 'Periodo', description: 'Cambia el rango de días para recalcular la información.', side: 'bottom', align: 'end' },
      { selector: '[data-tour="metrics-tabs"]', title: 'Categorías', description: 'Alterna entre emails, encuestas y análisis avanzado.', side: 'bottom', align: 'start' },
      { selector: '[data-tour="metrics-content"]', title: 'Contenido', description: 'Aquí se renderiza el análisis de la categoría seleccionada.', side: 'top', align: 'start' },
    ],
  },
  {
    match: (pathname) => pathname === '/admin/tools',
    title: 'Admin Tools',
    description: 'Este módulo reúne herramientas administrativas avanzadas.',
    steps: [
      { selector: '[data-tour="tools-header"]', title: 'Herramientas', description: 'Cabecera del módulo de herramientas administrativas.', side: 'bottom', align: 'start' },
      { selector: '[data-tour="tools-tabs"]', title: 'Tipos de herramienta', description: 'Alterna entre email marketing y encuestas.', side: 'bottom', align: 'start' },
      { selector: '[data-tour="tools-extra-action"]', title: 'Acción contextual', description: 'Algunas herramientas muestran aquí acciones específicas como crear campañas o encuestas.', side: 'bottom', align: 'end' },
      { selector: '[data-tour="tools-content"]', title: 'Área de trabajo', description: 'Aquí se usa la herramienta seleccionada.', side: 'top', align: 'start' },
    ],
  },
  {
    match: (pathname) => pathname === '/tutoriales',
    title: 'Tutoriales',
    description: 'Aquí se consultan guías publicadas y se administra la biblioteca de videos.',
    steps: [
      { selector: '[data-tour="tutoriales-header"]', title: 'Biblioteca de tutoriales', description: 'La pantalla agrupa guías rápidas para aprender el funcionamiento de Obertrack.', side: 'bottom', align: 'start' },
      { selector: '[data-tour="tutoriales-current-tour"]', title: 'Recorrido contextual', description: 'Este botón inicia el recorrido específico de la pantalla actual.', side: 'bottom', align: 'center' },
      { selector: '[data-tour="tutoriales-create"]', title: 'Nuevo tutorial', description: 'Crea una nueva guía para publicarla en la biblioteca de tutoriales.', side: 'bottom', align: 'end' },
      { selector: '[data-tour="tutoriales-tabs"]', title: 'Categorías', description: 'Filtra tutoriales por categoría o vuelve a verlos todos.', side: 'bottom', align: 'start' },
      { selector: '[data-tour="tutoriales-search"]', title: 'Buscador', description: 'Encuentra tutoriales por título o descripción.', side: 'bottom', align: 'end' },
      { selector: '[data-tour="tutoriales-grid"]', title: 'Tarjetas de tutorial', description: 'Abre un video haciendo clic en su tarjeta. El check indica que ya fue visto.', side: 'top', align: 'start' },
    ],
  },
]

function isVisible(el: Element | null): el is HTMLElement {
  return !!el && (el as HTMLElement).offsetWidth > 0 && (el as HTMLElement).offsetHeight > 0
}

function createDriver(steps: DriveStep[]) {
  return driver({
    showProgress: true,
    allowClose: true,
    overlayColor: '#060b23',
    overlayOpacity: 0.55,
    stagePadding: 6,
    stageRadius: 12,
    popoverClass: 'obertrack-tour',
    nextBtnText: 'Siguiente',
    prevBtnText: 'Anterior',
    doneBtnText: 'Finalizar',
    progressText: '{{current}} de {{total}}',
    steps,
  })
}

function getVisibleModuleSteps(moduleTour: ModuleTour): DriveStep[] {
  const steps: DriveStep[] = [
    {
      popover: {
        title: moduleTour.title,
        description: moduleTour.description,
      },
    },
  ]

  for (const step of moduleTour.steps) {
    if (!step.selector || isVisible(document.querySelector(step.selector))) {
      steps.push({
        element: step.selector,
        popover: {
          title: step.title,
          description: step.description,
          side: step.side ?? 'bottom',
          align: step.align ?? 'start',
        },
      })
    }
  }

  return steps
}

export function startSystemTour(role?: string) {
  const sections = getSections(role)

  const steps: DriveStep[] = [
    {
      popover: {
        title: '¡Bienvenido a Obertrack!',
        description: 'Te mostramos las secciones principales del sistema. Usa "Siguiente" para avanzar o cierra cuando quieras.',
      },
    },
  ]

  for (const section of sections) {
    const selector = `[data-tour="${section.path}"]`
    if (isVisible(document.querySelector(selector))) {
      steps.push({
        element: selector,
        popover: {
          title: section.title,
          description: section.description,
          side: 'right',
          align: 'start',
        },
      })
    }
  }

  createDriver(steps).drive()
}

export function startCurrentPageTour(pathname = window.location.pathname) {
  const moduleTour = MODULE_TOURS.find(tour => tour.match(pathname))

  if (!moduleTour) {
    startSystemTour()
    return
  }

  createDriver(getVisibleModuleSteps(moduleTour)).drive()
}
