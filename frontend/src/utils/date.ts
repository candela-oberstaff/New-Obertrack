/**
 * Parse an ISO date string as local midnight, avoiding timezone shifts.
 * "2026-06-20T00:00:00Z" or "2026-06-20" → local Date(2026, 5, 20).
 */
export function parseDateOnly(str: string | null | undefined): Date {
  if (!str) return new Date(NaN)
  const datePart = str.split('T')[0]
  const parts = datePart.split('-').map(Number)
  if (parts.length !== 3 || parts.some(isNaN)) return new Date(str)
  return new Date(parts[0], parts[1] - 1, parts[2])
}

/**
 * Format a date-only ISO string for display in es-ES locale.
 */
/** Today at local midnight, for date-only comparisons. */
export function todayMidnight(): Date {
  const d = new Date()
  d.setHours(0, 0, 0, 0)
  return d
}

/**
 * Días enteros transcurridos desde un instante ISO hasta ahora, contando por días
 * de calendario y no por múltiplos de 24 h: algo de anoche debe leerse como "ayer"
 * aunque hayan pasado 14 horas. Devuelve null si la fecha falta o no se entiende.
 */
export function daysSince(iso: string | null | undefined): number | null {
  if (!iso) return null
  const then = new Date(iso)
  if (isNaN(then.getTime())) return null
  then.setHours(0, 0, 0, 0)
  const diff = todayMidnight().getTime() - then.getTime()
  if (diff < 0) return 0
  return Math.floor(diff / 86400000)
}

/**
 * Antigüedad en lenguaje natural: "hoy", "ayer", "hace 4 días".
 * Devuelve '' cuando no hay fecha, para poder incrustarlo sin condicionales.
 */
export function formatDaysSince(iso: string | null | undefined): string {
  const days = daysSince(iso)
  if (days === null) return ''
  if (days === 0) return 'hoy'
  if (days === 1) return 'ayer'
  return `hace ${days} días`
}

/**
 * Antigüedad corta para bitácoras: "ahora", "hace 5 min", "hace 3 h", "hace 2 d".
 * Por encima de una semana pasa a fecha, porque "hace 34 d" ya no lo lee nadie.
 */
export function formatRelative(iso: string | null | undefined): string {
  if (!iso) return ''
  const then = new Date(iso)
  if (isNaN(then.getTime())) return ''
  const seconds = Math.floor((Date.now() - then.getTime()) / 1000)
  if (seconds < 0) return 'ahora'
  if (seconds < 60) return 'ahora'
  const minutes = Math.floor(seconds / 60)
  if (minutes < 60) return `hace ${minutes} min`
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return `hace ${hours} h`
  const days = Math.floor(hours / 24)
  if (days <= 7) return `hace ${days} d`
  return then.toLocaleDateString('es-ES', { day: 'numeric', month: 'short' })
}

export function formatDateOnly(
  str: string | null | undefined,
  options?: Intl.DateTimeFormatOptions,
): string {
  if (!str) return ''
  const d = parseDateOnly(str)
  if (isNaN(d.getTime())) return str.split('T')[0] || str
  return d.toLocaleDateString(
    'es-ES',
    options ?? { day: 'numeric', month: 'short', year: 'numeric' },
  )
}
