/**
 * Conversión de color para el selector propio.
 *
 * El navegador solo entiende hex en `<input type="color">`, pero para elegir un
 * color a mano hace falta HSV: el cuadro de saturación/brillo y la barra de
 * tono son literalmente los tres ejes de ese modelo. Estas funciones son el
 * puente, y están aparte porque son lo único del selector que se puede
 * equivocar en silencio.
 */

/** Paleta de colores de tablero: tonos que se distinguen entre sí y sobre fondo claro. */
export const BOARD_COLORS = [
  '#cc33cc', // orquídea (marca)
  '#8a2be2', // violeta
  '#3b82f6', // azul (el que traía por defecto)
  '#06b6d4', // cian
  '#10b981', // verde
  '#f59e0b', // ámbar
  '#ef4444', // rojo
  '#ec4899', // rosa
  '#64748b', // pizarra
  '#060b23', // navy (marca)
]

export interface HSV {
  /** Tono, 0-360 */
  h: number
  /** Saturación, 0-1 */
  s: number
  /** Brillo (value), 0-1 */
  v: number
}

/** Deja el hex en la forma larga y en minúsculas: '#ABC' → '#aabbcc'. */
export function normalizeHex(hex: string): string | null {
  const raw = (hex || '').trim().replace(/^#/, '')
  if (/^[0-9a-fA-F]{3}$/.test(raw)) {
    const [r, g, b] = raw.split('')
    return `#${r}${r}${g}${g}${b}${b}`.toLowerCase()
  }
  if (/^[0-9a-fA-F]{6}$/.test(raw)) {
    return `#${raw}`.toLowerCase()
  }
  return null
}

export function hexToHsv(hex: string): HSV | null {
  const normalized = normalizeHex(hex)
  if (!normalized) return null

  const r = parseInt(normalized.slice(1, 3), 16) / 255
  const g = parseInt(normalized.slice(3, 5), 16) / 255
  const b = parseInt(normalized.slice(5, 7), 16) / 255

  const max = Math.max(r, g, b)
  const min = Math.min(r, g, b)
  const delta = max - min

  let h = 0
  if (delta !== 0) {
    if (max === r) h = 60 * (((g - b) / delta) % 6)
    else if (max === g) h = 60 * ((b - r) / delta + 2)
    else h = 60 * ((r - g) / delta + 4)
  }
  if (h < 0) h += 360

  return { h, s: max === 0 ? 0 : delta / max, v: max }
}

export function hsvToHex({ h, s, v }: HSV): string {
  const c = v * s
  const x = c * (1 - Math.abs(((h / 60) % 2) - 1))
  const m = v - c

  let rgb: [number, number, number]
  if (h < 60) rgb = [c, x, 0]
  else if (h < 120) rgb = [x, c, 0]
  else if (h < 180) rgb = [0, c, x]
  else if (h < 240) rgb = [0, x, c]
  else if (h < 300) rgb = [x, 0, c]
  else rgb = [c, 0, x]

  const toHex = (n: number) =>
    Math.round((n + m) * 255)
      .toString(16)
      .padStart(2, '0')

  return `#${toHex(rgb[0])}${toHex(rgb[1])}${toHex(rgb[2])}`
}

/**
 * Decide si sobre ese color se lee mejor texto blanco o negro. Se usa para el
 * tic del color elegido: sobre un amarillo claro, un tic blanco desaparece.
 *
 * Es la fórmula de luminancia relativa de la WCAG, no un promedio de R/G/B: el
 * ojo no pesa igual los tres canales y con el promedio el verde salía mal.
 */
export function isLightColor(hex: string): boolean {
  const normalized = normalizeHex(hex)
  if (!normalized) return false

  const channel = (v: number) => {
    const c = v / 255
    return c <= 0.03928 ? c / 12.92 : Math.pow((c + 0.055) / 1.055, 2.4)
  }
  const r = channel(parseInt(normalized.slice(1, 3), 16))
  const g = channel(parseInt(normalized.slice(3, 5), 16))
  const b = channel(parseInt(normalized.slice(5, 7), 16))

  // 0.4 y no 0.5: en el ámbar (#f59e0b), que cae en 0.43, un tic blanco casi no
  // se distingue. El corte está puesto donde el tic negro empieza a leerse
  // mejor, no en la mitad de la escala.
  return 0.2126 * r + 0.7152 * g + 0.0722 * b > 0.4
}
