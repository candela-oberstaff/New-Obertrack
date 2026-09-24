import type { Board } from '../../../types'

/**
 * Orden de la lista de tableros.
 *
 * Hasta ahora el orden lo fijaba el servidor —`created_at DESC`, siempre— y no
 * había forma de cambiarlo: con cinco tableros da igual, con veinte hay que
 * recorrerlos con la vista cada vez.
 *
 * Se ordena en el cliente a propósito, aunque el servidor podría hacerlo: la
 * lista ya llega entera y es corta, así que reordenar es instantáneo y no gasta
 * una petición por cada clic. La preferencia se guarda por navegador, igual que
 * la empresa elegida.
 */

export type BoardSortKey =
  | 'recent'
  | 'oldest'
  | 'name_asc'
  | 'name_desc'
  | 'tasks_desc'
  | 'tasks_asc'

export const DEFAULT_BOARD_SORT: BoardSortKey = 'recent'

export const BOARD_SORT_OPTIONS: { value: BoardSortKey; label: string }[] = [
  { value: 'recent', label: 'Más recientes' },
  { value: 'oldest', label: 'Más antiguos' },
  { value: 'name_asc', label: 'Nombre (A-Z)' },
  { value: 'name_desc', label: 'Nombre (Z-A)' },
  { value: 'tasks_desc', label: 'Más tareas' },
  { value: 'tasks_asc', label: 'Menos tareas' },
]

const STORAGE_KEY = 'preferred_board_sort'

export function loadBoardSort(): BoardSortKey {
  try {
    const stored = localStorage.getItem(STORAGE_KEY)
    if (stored && BOARD_SORT_OPTIONS.some((o) => o.value === stored)) {
      return stored as BoardSortKey
    }
  } catch {
    // Navegador con el almacenamiento bloqueado: se sigue con el orden normal.
  }
  return DEFAULT_BOARD_SORT
}

export function saveBoardSort(key: BoardSortKey) {
  try {
    localStorage.setItem(STORAGE_KEY, key)
  } catch {
    // Igual que arriba: no poder recordarlo no debe romper la pantalla.
  }
}

/** Cuántas tareas tiene un tablero, sumando todas sus columnas. */
export function totalTasks(
  boardId: number,
  counts: Record<number, Record<string, number>>,
): number {
  const byStatus = counts?.[boardId]
  if (!byStatus) return 0
  return Object.values(byStatus).reduce((sum, n) => sum + (n || 0), 0)
}

/**
 * Devuelve los tableros ordenados. No modifica el array recibido: viene del
 * estado y ordenarlo en el sitio haría que React no viera el cambio.
 */
export function sortBoards(
  boards: Board[],
  key: BoardSortKey,
  counts: Record<number, Record<string, number>> = {},
): Board[] {
  // El nombre decide los empates en todos los criterios. Sin ese desempate,
  // dos tableros creados el mismo segundo —o con las mismas tareas— se
  // intercambiaban de sitio en cada render.
  const byName = (a: Board, b: Board) =>
    (a.name || '').localeCompare(b.name || '', 'es', { sensitivity: 'base' })

  const time = (board: Board) => {
    const ms = new Date(board.created_at ?? '').getTime()
    // Una fecha que no se entiende no debe mandar el tablero al principio de la
    // lista: se trata como el más antiguo posible.
    return Number.isNaN(ms) ? 0 : ms
  }

  const sorted = [...boards]
  switch (key) {
    case 'oldest':
      return sorted.sort((a, b) => time(a) - time(b) || byName(a, b))
    case 'name_asc':
      return sorted.sort(byName)
    case 'name_desc':
      return sorted.sort((a, b) => byName(b, a))
    case 'tasks_desc':
      return sorted.sort(
        (a, b) => totalTasks(b.id, counts) - totalTasks(a.id, counts) || byName(a, b),
      )
    case 'tasks_asc':
      return sorted.sort(
        (a, b) => totalTasks(a.id, counts) - totalTasks(b.id, counts) || byName(a, b),
      )
    case 'recent':
    default:
      return sorted.sort((a, b) => time(b) - time(a) || byName(a, b))
  }
}
