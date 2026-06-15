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

// Jerarquía de soporte: solo superadmins y Customer Success Managers gestionan
// al equipo (transferencias de tickets y reporte de rechazos).
export function isSupportManager(user: User | null | undefined): boolean {
  if (!user) return false
  return user.is_superadmin || (user.user_type === 'customer_success' && user.is_manager)
}
