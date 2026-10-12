import type { Task } from '../../../types'
import { compareTasks } from '../taskSort'

/**
 * Las tarjetas de una columna en el orden en que se VEN (el de Column.tsx).
 *
 * El arrastre tiene que calcular las posiciones sobre esta misma lista. Antes
 * las calculaba sobre la lista tal como llega del servidor (por fecha de
 * creación) y la tarjeta se guardaba en otro sitio del que se soltó: saltaba a
 * una posición fuera de la vista de la columna y parecía que se había borrado.
 */
export function columnInViewOrder(list: Task[], boardId: number, status: string): Task[] {
  return list.filter((t) => t.board_id === boardId && t.status === status).sort(compareTasks)
}

/**
 * Mueve una tarjeta a `targetStatus`, delante de `overTaskId` (o al final si se
 * suelta sobre la columna vacía), y renumera `order` en la columna destino
 * para que lo que se ve, lo que se guarda y lo que vuelve del servidor sean la
 * misma secuencia.
 */
export function moveCard(
  list: Task[],
  activeId: number,
  boardId: number,
  targetStatus: string,
  overTaskId: number | null,
): Task[] {
  const active = list.find((t) => t.id === activeId)
  if (!active) return list

  const target = columnInViewOrder(list, boardId, targetStatus)
  let sequence: Task[]

  if (active.status === targetStatus) {
    // Dentro de la misma columna: la tarjeta toma el lugar de la que está debajo.
    const from = target.findIndex((t) => t.id === activeId)
    const to = overTaskId == null ? target.length - 1 : target.findIndex((t) => t.id === overTaskId)
    if (from === -1 || to === -1 || from === to) return list
    sequence = [...target]
    const [moved] = sequence.splice(from, 1)
    sequence.splice(to, 0, moved)
  } else {
    // A otra columna: se inserta delante de la tarjeta sobre la que se soltó.
    const rest = target.filter((t) => t.id !== activeId)
    const at = overTaskId == null ? -1 : rest.findIndex((t) => t.id === overTaskId)
    sequence = [...rest]
    sequence.splice(at === -1 ? rest.length : at, 0, { ...active, status: targetStatus as Task['status'] })
  }

  const updated = new Map<number, Task>()
  sequence.forEach((t, index) => updated.set(t.id, { ...t, order: index }))
  return list.map((t) => updated.get(t.id) ?? t)
}
