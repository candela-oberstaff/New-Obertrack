// Human-friendly (Spanish) descriptions for audit log entries, so non-technical
// users understand what happened without seeing raw actions/routes/tables.
import type { AuditLog } from '../services/audit.service'

const norm = (s?: string) => (s || '').replace(/-/g, '_')

// Singular noun per entity/table (lowercase, for inline phrases).
const ENTITY_ES: Record<string, string> = {
  tickets: 'ticket',
  ticket_messages: 'mensaje de ticket',
  ticket_transfers: 'traspaso de ticket',
  work_hours: 'registro de horas',
  notifications: 'notificación',
  users: 'usuario',
  tasks: 'tarea',
  boards: 'tablero',
  comments: 'comentario',
  channels: 'canal',
  channel_messages: 'mensaje de canal',
  message_reactions: 'reacción',
  surveys: 'encuesta',
  survey_responses: 'respuesta de encuesta',
  tutorials: 'tutorial',
  contacts: 'contacto',
  auth: 'sesión',
  admin: 'administración',
}

// Friendly module label (chip).
const MODULE_ES: Record<string, string> = {
  tickets: 'Tickets',
  ticket_messages: 'Tickets',
  ticket_transfers: 'Tickets',
  work_hours: 'Horas',
  notifications: 'Notificaciones',
  users: 'Usuarios',
  admin: 'Administración',
  tasks: 'Tareas',
  boards: 'Tableros',
  channels: 'Chat',
  channel_messages: 'Chat',
  auth: 'Autenticación',
  surveys: 'Encuestas',
  tutorials: 'Tutoriales',
  email: 'Email',
}

export function moduleLabel(module?: string): string {
  return MODULE_ES[norm(module)] || (module || '—')
}

function entityNoun(entityType?: string): string {
  return ENTITY_ES[norm(entityType)] || (entityType || 'registro')
}

// Returns a human sentence describing the event.
export function describeAudit(log: AuditLog): string {
  const verb = (log.action || '').split('.').pop() || ''
  const entity = entityNoun(log.entity_type || log.module)
  const idTxt = log.entity_id ? ` #${log.entity_id}` : ''

  // Authentication events read naturally on their own.
  switch (log.action) {
    case 'auth.login': return 'Inició sesión'
    case 'auth.login_failed': return 'Intento de inicio de sesión fallido'
    case 'auth.logout': return 'Cerró sesión'
    case 'auth.register': return 'Creó una cuenta'
    case 'auth.password_reset': return 'Restableció una contraseña'
  }

  if (log.kind === 'data') {
    // Automatic data change — gender-safe noun phrasing.
    switch (verb) {
      case 'created': return `Creación de ${entity}${idTxt}`
      case 'updated': return `Actualización de ${entity}${idTxt}`
      case 'deleted': return `Eliminación de ${entity}${idTxt}`
      default: return `Cambio en ${entity}${idTxt}`
    }
  }

  // User action.
  switch (verb) {
    case 'created': return `Creó ${entity}${idTxt}`
    case 'updated': return `Editó ${entity}${idTxt}`
    case 'deleted': return `Eliminó ${entity}${idTxt}`
    case 'approve': return `Aprobó ${entity === 'registro de horas' ? 'horas' : entity}${idTxt}`
    case 'reject': return `Rechazó ${entity === 'registro de horas' ? 'horas' : entity}${idTxt}`
    case 'transfer': return `Traspasó ${entity}${idTxt}`
    case 'send': return `Envió ${entity}${idTxt}`
    case 'notes': return `Agregó una nota${idTxt}`
    case 'suspend': return `Suspendió ${entity}${idTxt}`
    case 'activate': return `Activó ${entity}${idTxt}`
    case 'promote_manager': return `Promovió a manager${idTxt}`
    case 'toggle_status': return `Cambió el estado de ${entity}${idTxt}`
    case 'reset_password': return `Restableció contraseña${idTxt}`
    default: return `${verb || 'Acción'} ${entity}${idTxt}`.trim()
  }
}

// "Acción de usuario" vs "Cambio automático del sistema".
export function originLabel(log: AuditLog): string {
  return log.kind === 'data' ? 'Sistema (automático)' : 'Acción de usuario'
}
