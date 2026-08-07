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
