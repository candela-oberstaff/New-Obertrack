import type confettiLib from 'canvas-confetti'

/**
 * Confeti de celebración (paquete canvas-confetti), cargado bajo demanda: solo
 * se descarga cuando alguien aprueba (import dinámico, chunk aparte), así que no
 * pesa en el resto de la app. Se sirve desde nuestro propio build, no desde una
 * CDN: la CSP de producción (nginx.conf.template) solo permite scripts propios.
 *
 * Si el módulo no carga (sin red al pedir el chunk), `celebrate` no hace nada:
 * la celebración es un adorno y nunca debe romper la pantalla. Todo corre en el
 * hilo principal (sin Worker ni blob:) para cumplir la CSP de producción.
 */

type ConfettiOptions = confettiLib.Options
type ConfettiFn = confettiLib.CreateTypes

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

/**
 * Carga el paquete una sola vez y devuelve su lanzador; null si no se pudo.
 *
 * `useWorker: false` es imprescindible: la instancia por defecto del paquete
 * dibuja en un Worker creado desde una URL `blob:`, y la CSP de producción
 * (script-src sin blob:, sin worker-src) lo bloquea. En el hilo principal un
 * efecto corto como este va igual de fluido. Sin `canvas`, el paquete crea su
 * propio lienzo a pantalla completa y lo retira al terminar.
 */
export function loadConfetti(): Promise<ConfettiFn | null> {
  loading ??= import('canvas-confetti')
    .then((module) =>
      module.default.create(undefined, { resize: true, useWorker: false, disableForReducedMotion: true })
    )
    .catch(() => {
      // Se permite reintentar en la siguiente celebración.
      loading = null
      return null
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
