/**
 * Confeti de celebración (canvas-confetti), cargado bajo demanda desde la CDN:
 * solo se descarga cuando alguien aprueba, no pesa en el resto de la app.
 *
 * Si el script no carga (sin red, bloqueado por la CSP), `celebrate` no hace
 * nada: la celebración es un adorno y nunca debe romper la pantalla.
 *
 * OJO: la CSP de producción (nginx.conf.template) no permite cdn.jsdelivr.net
 * en script-src. Antes de publicarlo hay que sumarlo ahí o instalar el paquete
 * `canvas-confetti` y cambiar solo `loadConfetti`.
 */

const CONFETTI_SRC = 'https://cdn.jsdelivr.net/npm/canvas-confetti@1.9.3/dist/confetti.browser.min.js'
const LOAD_TIMEOUT_MS = 6000

/** Lo que usamos de canvas-confetti (https://github.com/catdad/canvas-confetti). */
interface ConfettiOptions {
  particleCount?: number
  angle?: number
  spread?: number
  startVelocity?: number
  decay?: number
  gravity?: number
  drift?: number
  ticks?: number
  origin?: { x?: number; y?: number }
  colors?: string[]
  scalar?: number
  zIndex?: number
  disableForReducedMotion?: boolean
}

type ConfettiFn = (options?: ConfettiOptions) => unknown

declare global {
  interface Window {
    confetti?: ConfettiFn
  }
}

/** Valores que se afinan en la vista de pruebas (/dev/induccion). */
export interface CelebrationConfig {
  particleCount: number
  spread: number
  startVelocity: number
  gravity: number
  scalar: number
  ticks: number
  /** Altura de la que sale, de 0 (arriba) a 1 (abajo). */
  originY: number
  colors: string[]
}

/** Colores de la marca más un dorado para que se vea de fiesta. */
export const DEFAULT_CELEBRATION: CelebrationConfig = {
  particleCount: 120,
  spread: 70,
  startVelocity: 45,
  gravity: 1,
  scalar: 1,
  ticks: 200,
  originY: 0.65,
  colors: ['#fa3ab4', '#cc33cc', '#a855f7', '#f5b301', '#ffffff'],
}

let loading: Promise<ConfettiFn | null> | null = null

/** Carga el script una sola vez; devuelve null si no se pudo. */
export function loadConfetti(): Promise<ConfettiFn | null> {
  if (typeof window === 'undefined') return Promise.resolve(null)
  if (window.confetti) return Promise.resolve(window.confetti)
  if (loading) return loading

  loading = new Promise<ConfettiFn | null>((resolve) => {
    const script = document.createElement('script')
    const finish = (fn: ConfettiFn | null) => {
      window.clearTimeout(timer)
      // Si falló, se permite reintentar en la siguiente celebración.
      if (!fn) loading = null
      resolve(fn)
    }
    const timer = window.setTimeout(() => finish(null), LOAD_TIMEOUT_MS)
    script.src = CONFETTI_SRC
    script.async = true
    script.onload = () => finish(window.confetti ?? null)
    script.onerror = () => finish(null)
    document.head.appendChild(script)
  })
  return loading
}

/**
 * Lanza el confeti. `completed` (se aprobó el programa entero) añade dos
 * cañones laterales durante un par de segundos; un bloque suelto solo lleva la
 * ráfaga central.
 */
export async function celebrate(config: CelebrationConfig = DEFAULT_CELEBRATION, completed = false): Promise<void> {
  const confetti = await loadConfetti()
  if (!confetti) return

  const base: ConfettiOptions = {
    spread: config.spread,
    startVelocity: config.startVelocity,
    gravity: config.gravity,
    scalar: config.scalar,
    ticks: config.ticks,
    colors: config.colors,
    zIndex: 10000,
    disableForReducedMotion: true,
  }

  confetti({ ...base, particleCount: config.particleCount, origin: { x: 0.5, y: config.originY } })
  if (!completed) return

  const end = Date.now() + 1800
  const cannons = () => {
    const each = Math.max(2, Math.round(config.particleCount / 30))
    confetti({ ...base, particleCount: each, angle: 60, origin: { x: 0, y: 0.8 } })
    confetti({ ...base, particleCount: each, angle: 120, origin: { x: 1, y: 0.8 } })
    if (Date.now() < end) requestAnimationFrame(cannons)
  }
  cannons()
}
