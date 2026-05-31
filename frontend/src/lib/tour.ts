import { driver, type DriveStep } from 'driver.js'
import 'driver.js/dist/driver.css'
import './tour.css'

interface TourSection {
  path: string
  title: string
  description: string
}

const SECTIONS: TourSection[] = [
  { path: '/dashboard', title: 'Dashboard', description: 'Tu punto de partida: un resumen general de la actividad y los indicadores clave.' },
  { path: '/admin', title: 'Admin', description: 'Panel de superadmin para gestionar usuarios de la plataforma y ver la actividad.' },
  { path: '/admin/tenants', title: 'Empresas', description: 'Administra las empresas (tenants), sus empleados y el seguimiento de cada una.' },
  { path: '/tasks', title: 'Tareas', description: 'Tableros y tareas para organizar el trabajo del equipo por fases.' },
  { path: '/tickets', title: 'Tickets', description: 'Bandeja de soporte para atender y dar seguimiento a las solicitudes.' },
  { path: '/work-hours', title: 'Horas', description: 'Registra tus jornadas y revisa o aprueba las horas trabajadas.' },
  { path: '/reports', title: 'Reportes', description: 'Informes y exportaciones de horas y productividad.' },
  { path: '/chat', title: 'Chat', description: 'Mensajería interna por canales y mensajes directos con tu equipo.' },
  { path: '/admin/tools', title: 'Tools', description: 'Herramientas administrativas avanzadas de la plataforma.' },
  { path: '/admin/metrics', title: 'Métricas', description: 'Métricas globales del sistema para superadmins.' },
  { path: '/tutoriales', title: 'Tutoriales', description: 'Guías en video para aprender a usar cada sección de Obertrack.' },
  { path: '/profile', title: 'Perfil', description: 'Tu cuenta: datos personales, preferencias y configuración.' },
]

function isVisible(el: Element | null): el is HTMLElement {
  return !!el && (el as HTMLElement).offsetWidth > 0 && (el as HTMLElement).offsetHeight > 0
}

export function startSystemTour() {
  const steps: DriveStep[] = [
    {
      popover: {
        title: '¡Bienvenido a Obertrack!',
        description: 'Te mostramos las secciones principales del sistema. Usa "Siguiente" para avanzar o cierra cuando quieras.',
      },
    },
  ]

  for (const section of SECTIONS) {
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

  const tour = driver({
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

  tour.drive()
}
