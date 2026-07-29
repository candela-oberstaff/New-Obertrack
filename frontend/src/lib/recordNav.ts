/**
 * Navegación entre registros ("‹ anterior / siguiente ›") desde la vista de
 * detalle, sin tener que volver al listado para abrir el siguiente.
 *
 * El listado es el único que sabe QUÉ orden está viendo el usuario (con sus
 * filtros y su ordenación aplicados), así que es él quien deja esa secuencia
 * anotada al abrir una ficha; el detalle solo la recorre.
 *
 * Va en sessionStorage y no en un estado en memoria por dos motivos: sobrevive
 * a un F5 sobre la ficha (que en este panel es habitual tras editar), y es por
 * pestaña, así que dos pestañas abiertas en listados distintos no se pisan.
 */

/** Ámbito de una secuencia. Los que dependen de un padre lo incluyen. */
export type RecordScope =
  | 'tenants'
  | 'admin-users'
  | `tenant-employees:${number}`
  | 'empresa-employees'

const storageKey = (scope: RecordScope) => `obertrack:nav:${scope}`

/** Anota el orden visible. Llamar justo antes de navegar al detalle. */
export function setRecordNav(scope: RecordScope, ids: number[]): void {
  try {
    sessionStorage.setItem(storageKey(scope), JSON.stringify(ids))
  } catch {
    /* sessionStorage lleno o bloqueado: el detalle simplemente no paginará */
  }
}

/** Lee el orden anotado. Devuelve [] si no hay nada usable. */
export function getRecordNav(scope: RecordScope): number[] {
  try {
    const raw = sessionStorage.getItem(storageKey(scope))
    if (!raw) return []
    const parsed = JSON.parse(raw)
    return Array.isArray(parsed) ? parsed.filter((id): id is number => typeof id === 'number') : []
  } catch {
    return []
  }
}

export interface RecordNeighbours {
  prevId: number | null
  nextId: number | null
  /** Posición 1-based dentro de la secuencia, o 0 si el registro no está en ella. */
  position: number
  total: number
}

/**
 * Sitúa un registro dentro de su secuencia. Si no aparece en ella (se llegó por
 * enlace directo, desde otro listado, o el registro se filtró fuera después de
 * abrirlo) devuelve una posición 0 y sin vecinos: el paginador se oculta en vez
 * de mover al usuario por una lista que no es la que tenía delante.
 */
export function locateRecord(scope: RecordScope, currentId: number): RecordNeighbours {
  const ids = getRecordNav(scope)
  const index = ids.indexOf(currentId)
  if (index === -1) return { prevId: null, nextId: null, position: 0, total: ids.length }
  return {
    prevId: index > 0 ? ids[index - 1] : null,
    nextId: index < ids.length - 1 ? ids[index + 1] : null,
    position: index + 1,
    total: ids.length,
  }
}

/**
 * Quita un registro de la secuencia (p. ej. tras eliminarlo desde su ficha) para
 * que el paginador no ofrezca saltar a algo que ya no existe.
 */
export function removeFromRecordNav(scope: RecordScope, id: number): void {
  const ids = getRecordNav(scope)
  if (!ids.includes(id)) return
  setRecordNav(scope, ids.filter(x => x !== id))
}
