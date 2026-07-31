/**
 * Señales de salud de una cuenta de empresa, para la ficha y el listado.
 *
 * Son dos preguntas distintas y a propósito no se mezclan en un solo número:
 * "¿cuándo la contactamos por última vez?" (lo que hicimos nosotros) y "¿cuándo
 * dio señales de vida?" (lo que hicieron ellos). Una empresa a la que llamamos
 * ayer pero que no registra jornadas hace dos meses es justo el caso que hay
 * que ver, y un indicador único lo escondería.
 */

/** Nivel de atención que pide una señal. Ordena el color, no el texto. */
export type HealthLevel = 'ok' | 'warn' | 'stale' | 'never'

export interface HealthSignal {
  level: HealthLevel
  /** Días transcurridos; null cuando nunca ocurrió. */
  days: number | null
  /** Texto listo para pintar: "hace 3 días", "hoy", "nunca". */
  label: string
}

/** A partir de cuántos días una señal pasa a ámbar y a rojo. */
const WARN_DAYS = 30
const STALE_DAYS = 90

/**
 * daysSince cuenta días de CALENDARIO, no periodos de 24 horas: algo de ayer a
 * las 23:00 es "ayer" aunque hayan pasado dos horas. Es como lo lee una persona
 * que mira la ficha.
 */
export function daysSince(iso: string | null | undefined, now: Date = new Date()): number | null {
  if (!iso) return null
  const then = new Date(iso)
  if (isNaN(then.getTime())) return null
  const a = Date.UTC(then.getFullYear(), then.getMonth(), then.getDate())
  const b = Date.UTC(now.getFullYear(), now.getMonth(), now.getDate())
  const diff = Math.floor((b - a) / 86_400_000)
  // Una fecha futura (relojes desincronizados, datos sembrados) se trata como
  // hoy: decir "hace -3 días" sería peor que redondear.
  return diff < 0 ? 0 : diff
}

export function healthSignal(iso: string | null | undefined, now: Date = new Date()): HealthSignal {
  const days = daysSince(iso, now)
  if (days === null) return { level: 'never', days: null, label: 'nunca' }
  if (days === 0) return { level: 'ok', days, label: 'hoy' }
  if (days === 1) return { level: 'ok', days, label: 'ayer' }

  const level: HealthLevel = days >= STALE_DAYS ? 'stale' : days >= WARN_DAYS ? 'warn' : 'ok'
  if (days < 30) return { level, days, label: `hace ${days} días` }

  const months = Math.floor(days / 30)
  if (months < 12) return { level, days, label: `hace ${months} ${months === 1 ? 'mes' : 'meses'}` }

  const years = Math.floor(days / 365)
  return { level, days, label: `hace más de ${years} ${years === 1 ? 'año' : 'años'}` }
}

/** Color del nivel. "never" y "stale" comparten tono: ambos piden lo mismo. */
export const HEALTH_COLOR: Record<HealthLevel, string> = {
  ok: '#059669',
  warn: '#d97706',
  stale: '#dc2626',
  never: '#dc2626',
}
