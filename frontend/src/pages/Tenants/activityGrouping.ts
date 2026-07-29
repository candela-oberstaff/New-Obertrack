import type { TenantActivity } from '../../hooks'

export interface ActivityDayGroup {
  /** Clave estable del día (para el key de React). */
  key: string
  label: string
  items: TenantActivity[]
}

/**
 * Agrupa los movimientos del expediente por día conservando el orden que trae
 * el servidor (más reciente primero). La fecha pasa a la cabecera del grupo y
 * cada fila se queda solo con la hora: en un expediente con veinte líneas del
 * mismo día, repetir la fecha completa en cada una es puro ruido.
 *
 * Agrupa por días CONSECUTIVOS, no por día absoluto: si el orden viniera roto,
 * se verían dos grupos del mismo día en vez de una cronología mentirosa que
 * junta movimientos separados por el resto de la lista.
 */
export function groupByDay(items: TenantActivity[], now: Date = new Date()): ActivityDayGroup[] {
  const groups: ActivityDayGroup[] = []
  for (const item of items) {
    const date = new Date(item.timestamp)
    const valid = !isNaN(date.getTime())
    const key = valid ? date.toDateString() : 'sin-fecha'
    let group = groups[groups.length - 1]
    if (!group || group.key !== key) {
      group = { key, label: valid ? dayLabel(date, now) : 'Fecha no disponible', items: [] }
      groups.push(group)
    }
    group.items.push(item)
  }
  return groups
}

/** "Hoy" / "Ayer" para lo reciente; fecha con día de la semana para el resto. */
export function dayLabel(date: Date, now: Date = new Date()): string {
  const today = new Date(now)
  today.setHours(0, 0, 0, 0)
  const day = new Date(date)
  day.setHours(0, 0, 0, 0)
  const diffDays = Math.round((today.getTime() - day.getTime()) / 86_400_000)
  if (diffDays === 0) return 'Hoy'
  if (diffDays === 1) return 'Ayer'
  return day.toLocaleDateString('es-ES', {
    weekday: 'long',
    day: 'numeric',
    month: 'long',
    // El año solo cuando no es el actual: en un expediente reciente estorba.
    year: day.getFullYear() === today.getFullYear() ? undefined : 'numeric',
  })
}
