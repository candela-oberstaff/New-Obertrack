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
