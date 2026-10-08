import { DEFAULT_ROLES, rolesOf } from '../../types'
import type { TargetRole, Tutorial, TutorialAudience, TutorialTarget } from '../../types'

const ROLE_LABELS: Record<TargetRole, string> = {
  empresa: 'Empresas',
  profesional: 'Profesionales',
  manager: 'Managers',
  supervisor: 'Supervisores',
  superadmin: 'Superadmins',
}

/** Va a una lista de personas elegidas a mano, no a un perfil. */
export function isPeopleTarget(t: Pick<Tutorial, 'target'>): boolean {
  return t.target?.mode === 'personas'
}

/**
 * A quién va dirigida una novedad, en pocas palabras, para la tarjeta y la
 * tabla del panel: «Todos», «Managers, Supervisores» o «12 personas».
 * Las novedades anteriores a los roles se leen desde su audiencia.
 */
export function audienceLabel(t: Pick<Tutorial, 'audience' | 'target'>): string {
  if (isPeopleTarget(t)) {
    const people = t.target.user_ids?.length ?? 0
    const groups = t.target.group_ids?.length ?? 0
    const parts: string[] = []
    if (people) parts.push(`${people} ${people === 1 ? 'persona' : 'personas'}`)
    if (groups) parts.push(`${groups} ${groups === 1 ? 'grupo' : 'grupos'}`)
    return parts.join(' y ') || 'Nadie'
  }
  const roles = rolesOf(t.audience, t.target)
  const everyone = DEFAULT_ROLES.every((r) => roles.includes(r))
  if (everyone) return roles.includes('superadmin') ? 'Todos y superadmins' : 'Todos'
  // Profesionales, managers y supervisores juntos se leen como un solo perfil.
  const pros: TargetRole[] = ['profesional', 'manager', 'supervisor']
  if (pros.every((r) => roles.includes(r))) {
    const rest = roles.filter((r) => !pros.includes(r)).map((r) => ROLE_LABELS[r])
    return ['Todos los profesionales', ...rest].join(', ')
  }
  return roles.map((r) => ROLE_LABELS[r]).join(', ') || 'Nadie'
}

/**
 * Audiencia de la columna que corresponde al público. El servidor la vuelve a
 * calcular al guardar; aquí solo mantiene coherente el formulario.
 */
export function audienceForTarget(target: TutorialTarget): TutorialAudience {
  if (target.mode === 'personas') return 'all'
  const roles = target.roles ?? []
  const employer = roles.includes('empresa')
  const professional = roles.some(r => r === 'profesional' || r === 'manager' || r === 'supervisor')
  const admins = roles.includes('superadmin')
  if (employer && !professional && !admins) return 'empleador'
  if (professional && !employer && !admins) return 'profesional'
  return 'all'
}
