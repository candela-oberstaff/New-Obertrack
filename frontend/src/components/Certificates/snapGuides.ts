// Guías inteligentes del editor de certificados, como las de Canva: al
// arrastrar un campo cerca del centro de la página o a la altura/columna de
// otro campo, se "pega" a esa línea y la línea se dibuja. Todo va en
// porcentaje de la página, igual que las posiciones de los campos.

/** Una guía que se está mostrando: dónde y si es el centro de la página. */
export interface SnapGuide {
  /** Posición en % (x para una guía vertical, y para una horizontal). */
  at: number
  /** true = centro de la página; false = alineado con otro campo. */
  center: boolean
}

export interface SnapResult {
  x: number
  y: number
  /** Guía vertical (alinea la X), si la hay. */
  vertical: SnapGuide | null
  /** Guía horizontal (alinea la Y), si la hay. */
  horizontal: SnapGuide | null
}

interface Point {
  x: number
  y: number
}

/** La candidata más cercana dentro del umbral, o null. */
function nearest(value: number, candidates: SnapGuide[], threshold: number): SnapGuide | null {
  let best: SnapGuide | null = null
  let bestDist = threshold
  for (const c of candidates) {
    const d = Math.abs(value - c.at)
    // A igual distancia gana el centro de la página: es la guía más útil.
    if (d < bestDist || (d === bestDist && best && c.center && !best.center)) {
      best = c
      bestDist = d
    }
  }
  return best
}

/**
 * Ajusta (x, y) a la guía más cercana en cada eje: el centro de la página
 * (50 %) o la posición de otro campo. Los umbrales van en % (el llamador los
 * calcula a partir de unos píxeles fijos, así el imán se siente igual a
 * cualquier tamaño de pantalla).
 */
export function snapPosition(pos: Point, others: Point[], thresholdX: number, thresholdY: number): SnapResult {
  const xs: SnapGuide[] = [{ at: 50, center: true }, ...others.map((o) => ({ at: o.x, center: false }))]
  const ys: SnapGuide[] = [{ at: 50, center: true }, ...others.map((o) => ({ at: o.y, center: false }))]
  const vertical = nearest(pos.x, xs, thresholdX)
  const horizontal = nearest(pos.y, ys, thresholdY)
  return {
    x: vertical ? vertical.at : pos.x,
    y: horizontal ? horizontal.at : pos.y,
    vertical,
    horizontal,
  }
}
