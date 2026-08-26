import type { User } from '../types'

// Permisos efectivos por rol (espejo del backend handlers.RequirePermission):
// superadmins y cuentas empresa nunca se restringen; un usuario sin roles
// asignados conserva el comportamiento histórico de su tipo de cuenta.

export function canViewModule(user: User | null | undefined, module: string): boolean {
  if (!user) return false
  if (user.is_superadmin || user.user_type === 'empleador') return true
  if (!user.permissions) return true
  const level = user.permissions[module]
  return level === 'view' || level === 'edit'
}

export function canEditModule(user: User | null | undefined, module: string): boolean {
  if (!user) return false
  if (user.is_superadmin || user.user_type === 'empleador') return true
  if (!user.permissions) return true
  return user.permissions[module] === 'edit'
}

// Nivel en la cadena de mando, en UNA sola función.
//
// Un supervisor es TAMBIÉN manager (el flag va encima del otro), así que el
// orden importa: se muestra el nivel más alto. Vive aquí y no repetido en cada
// pantalla porque justo esa duplicación hacía que la misma persona apareciera
// como "Supervisor" en el menú y como "Manager" en su ficha.
//
// Devuelve null para quien no tiene gente a cargo: cada pantalla decide si
// entonces no pinta nada o cae a otra etiqueta.
export function hierarchyLabel(
  user: { is_manager?: boolean; is_supervisor?: boolean } | null | undefined,
): 'Supervisor' | 'Manager' | null {
  if (!user) return null
  if (user.is_supervisor) return 'Supervisor'
  if (user.is_manager) return 'Manager'
  return null
}

// Etiqueta COMPLETA del rol, para las listas donde se muestra una sola cosa por
// persona: el nivel si lo tiene, y si no el tipo de cuenta con su nombre de pantalla.
//
// Manager y supervisor no son tipos, son niveles encima de un profesional, así que
// pintar user_type a secas hacía que un supervisor apareciera como "Profesional" en el
// listado general. Varias pantallas ya lo resolvían a mano con `is_manager ? 'manager'
// : user_type`, que no contempla supervisor: ahí un supervisor salía como "manager".
export function roleLabel(
  user: { user_type?: string; is_manager?: boolean; is_supervisor?: boolean } | null | undefined,
): string {
  if (!user) return '—'
  const tipo = user.user_type ?? ''
  if (tipo === 'profesional' || tipo === 'empleado') {
    const nivel = hierarchyLabel(user)
    if (nivel) return nivel
  }
  return ROLE_LABELS[tipo] ?? tipo
}

const ROLE_LABELS: Record<string, string> = {
  superadmin: 'Superadmin',
  empleador: 'Empresa',
  customer_success: 'Customer Success',
  analista_it: 'Analista IT',
  profesional: 'Profesional',
  empleado: 'Profesional',
}

// Clase del distintivo. Se separa de la etiqueta porque el color AGRUPA —manager y
// supervisor comparten el ámbar de "tiene gente a cargo"— mientras que el texto
// distingue. Tres tonos para tres niveles obligaría a aprenderse una leyenda.
export function roleBadgeClass(
  user: { user_type?: string; is_manager?: boolean; is_supervisor?: boolean } | null | undefined,
): string {
  if (!user) return ''
  const tipo = user.user_type ?? ''
  if ((tipo === 'profesional' || tipo === 'empleado') && hierarchyLabel(user)) {
    return 'manager'
  }
  return tipo
}

// Jerarquía de soporte: solo superadmins y Customer Success Managers gestionan
// al equipo (transferencias de tickets y reporte de rechazos).
export function isSupportManager(user: User | null | undefined): boolean {
  if (!user) return false
  return user.is_superadmin || (user.user_type === 'customer_success' && user.is_manager)
}
