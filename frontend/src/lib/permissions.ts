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

// Jerarquía de soporte: solo superadmins y Customer Success Managers gestionan
// al equipo (transferencias de tickets y reporte de rechazos).
export function isSupportManager(user: User | null | undefined): boolean {
  if (!user) return false
  return user.is_superadmin || (user.user_type === 'customer_success' && user.is_manager)
}
