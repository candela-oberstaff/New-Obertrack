import type { Task, User } from '../../../types'
import type { TaskScope } from './TaskScopeToggle'
import type { MemberOption } from './TaskMemberMenu'

/**
 * De quién son las tareas que se ven, y si se esconden las terminadas.
 *
 * Está fuera de la pantalla porque lo usan las tres vistas —tablero, timeline y
 * calendario— y porque es la clase de regla que se rompe en silencio: si el
 * filtro por persona dejara fuera una tarea de más, nadie lo notaría salvo la
 * persona a la que le falta su tarea.
 */

/** Una tarea está pendiente mientras no esté terminada.
 *
 * Se mira `completed` y no el nombre de la columna porque el servidor y el
 * navegador mantienen ese campo en sincronía con Finalizado en ambos sentidos
 * (entrar la completa, salir la reabre), mientras que las fases las renombra
 * cualquiera. */
export function isPending(task: Task): boolean {
  return !task.completed
}

export function assignedTo(task: Task, userId: number): boolean {
  return !!task.assignees?.some((a) => a.id === userId)
}

export interface ScopeFilter {
  scope: TaskScope
  /** Solo se usa con scope 'mine'. */
  currentUserId?: number
  /** Solo se usa con scope 'member'. */
  memberId?: number | null
  onlyPending: boolean
}

export function filterTasksByScope(tasks: Task[], filter: ScopeFilter): Task[] {
  const { scope, currentUserId, memberId, onlyPending } = filter

  let list = tasks
  if (scope === 'mine' && currentUserId != null) {
    list = tasks.filter((t) => assignedTo(t, currentUserId))
  } else if (scope === 'member' && memberId != null) {
    list = tasks.filter((t) => assignedTo(t, memberId))
  }

  return onlyPending ? list.filter(isPending) : list
}

/**
 * Los integrantes del tablero con lo que le queda a cada uno sin terminar. Ese
 * número es la respuesta a "¿qué tiene pendiente fulano?", que hasta ahora se
 * averiguaba recorriendo las columnas a ojo.
 */
export function memberPendingOptions(members: User[] | undefined, tasks: Task[]): MemberOption[] {
  return (members ?? []).map((member) => ({
    id: member.id,
    name: member.name,
    pending: tasks.filter((t) => isPending(t) && assignedTo(t, member.id)).length,
  }))
}

/** Cuántas caben en el contador de un ámbito, respetando "solo pendientes". */
export function scopeCount(tasks: Task[], onlyPending: boolean): number {
  return onlyPending ? tasks.filter(isPending).length : tasks.length
}
