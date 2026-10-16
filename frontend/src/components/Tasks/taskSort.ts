import type { Task } from '../../types'
import { parseDateOnly } from '../../utils/date'

/**
 * Orden de las tarjetas dentro de una columna: primero el orden manual
 * (task.order, fijado al arrastrar), y para empates (tableros donde nunca se
 * reordenó, todas con order 0) la fecha límite como antes, con el id como
 * desempate estable. Lo usan el kanban y el cálculo de reorden al soltar:
 * deben coincidir o la tarjeta "salta" al soltarla.
 */
export function compareTasks(a: Task, b: Task): number {
  const ao = a.order ?? 0
  const bo = b.order ?? 0
  if (ao !== bo) return ao - bo
  if (a.end_date || b.end_date) {
    if (!a.end_date) return 1
    if (!b.end_date) return -1
    const diff = parseDateOnly(a.end_date).getTime() - parseDateOnly(b.end_date).getTime()
    if (diff !== 0) return diff
  }
  return a.id - b.id
}

/**
 * Cómo se ordena una columna. "manual" es el orden de siempre (compareTasks);
 * los otros dos son una VISTA por fecha de entrega: no cambian task.order, así
 * que al volver a "manual" cada tarjeta sigue donde la dejó quien la arrastró.
 */
export type TaskSortMode = 'manual' | 'due_asc' | 'due_desc'

/** Comparador de una columna según el modo elegido. */
export function comparatorFor(mode: TaskSortMode): (a: Task, b: Task) => number {
  if (mode === 'manual') return compareTasks
  const dir = mode === 'due_asc' ? 1 : -1
  return (a, b) => {
    // Sin fecha de entrega, siempre al final: no son "lo próximo a entregar"
    // ni "lo más lejano", simplemente no tienen fecha.
    if (!a.end_date && !b.end_date) return compareTasks(a, b)
    if (!a.end_date) return 1
    if (!b.end_date) return -1
    const diff = parseDateOnly(a.end_date).getTime() - parseDateOnly(b.end_date).getTime()
    return diff !== 0 ? diff * dir : compareTasks(a, b)
  }
}
