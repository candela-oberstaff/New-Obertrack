/**
 * Resolución de la empresa de un destinatario, para los selectores de campañas
 * y encuestas.
 *
 * Hace falta porque la empresa de una persona NO está en su propia fila: solo
 * las cuentas empleador llevan `company_name`, y un profesional únicamente
 * guarda el `empleador_id` que apunta a ellas. Sin cruzar las dos cosas, un
 * selector no puede ni filtrar por empresa ni enseñar a qué empresa pertenece
 * cada quien — que es justo lo que hacía imposible acotar un envío a un cliente
 * concreto y obligaba a apoyarse en el botón de "Todos".
 */

export interface CompanyAwareUser {
  id: number
  name?: string
  email?: string
  user_type?: string
  company_name?: string
  empleador_id?: number
}

/** Id de empresa de una persona: la cuenta empleador es su propia empresa. */
export function companyIdOf(u: CompanyAwareUser): number {
  if (u.user_type === 'empleador') return u.id
  return u.empleador_id ?? 0
}

/**
 * Índice id de empresa → nombre, construido con las cuentas empleador que ya
 * vienen en la misma lista. No hace falta pedir nada más al servidor: el
 * selector carga a todo el mundo, y las empresas están ahí dentro.
 */
export function buildCompanyIndex(users: CompanyAwareUser[]): Map<number, string> {
  const index = new Map<number, string>()
  for (const u of users) {
    if (u.user_type !== 'empleador') continue
    const name = (u.company_name || '').trim() || (u.name || '').trim()
    if (name) index.set(u.id, name)
  }
  return index
}

/** Nombre de la empresa de una persona, o "" si no pertenece a ninguna. */
export function companyNameOf(u: CompanyAwareUser, index: Map<number, string>): string {
  const id = companyIdOf(u)
  if (!id) return ''
  return index.get(id) ?? (u.user_type === 'empleador' ? (u.company_name || u.name || '').trim() : '')
}

/** Valor del filtro para quienes no cuelgan de ninguna empresa (superadmins, CS sin asignar). */
export const NO_COMPANY = '__sin_empresa__'

/**
 * Opciones del desplegable de empresa, ordenadas por cuánta gente tienen: la
 * empresa grande es casi siempre la que se busca. Los contadores viajan en la
 * etiqueta porque antes de lanzar un envío lo primero que se quiere saber es a
 * cuánta gente va.
 *
 * `selected` se conserva aunque el filtro de rol la deje en cero: sin eso el
 * desplegable saltaría solo a otro valor y el listado cambiaría sin que nadie
 * lo hubiera tocado.
 */
export function companyOptions(
  users: CompanyAwareUser[],
  index: Map<number, string>,
  selected: string,
): Array<{ value: string; label: string }> {
  const counts = new Map<string, number>()
  let sinEmpresa = 0
  for (const u of users) {
    const name = companyNameOf(u, index)
    if (name) counts.set(name, (counts.get(name) ?? 0) + 1)
    else sinEmpresa++
  }
  if (selected !== 'all' && selected !== NO_COMPANY && !counts.has(selected)) {
    counts.set(selected, 0)
  }

  const options = Array.from(counts.entries())
    .sort((a, b) => b[1] - a[1] || a[0].localeCompare(b[0], 'es'))
    .map(([name, count]) => ({ value: name, label: `${name} (${count})` }))

  if (sinEmpresa > 0 || selected === NO_COMPANY) {
    options.push({ value: NO_COMPANY, label: `Sin empresa (${sinEmpresa})` })
  }
  return options
}

/** ¿Pasa esta persona el filtro de empresa elegido? */
export function matchesCompany(
  u: CompanyAwareUser,
  index: Map<number, string>,
  filter: string,
): boolean {
  if (filter === 'all') return true
  const name = companyNameOf(u, index)
  if (filter === NO_COMPANY) return name === ''
  return name === filter
}
