import {
  filterTasksByScope,
  memberPendingOptions,
  scopeCount,
} from './taskScopeFilter'
import type { Task, User } from '../../../types'

// El filtro por integrante es la pieza que pidieron los clientes: ver qué le
// queda a alguien sin recorrer las columnas a ojo. Si dejara fuera una tarea de
// más nadie lo notaría, salvo la persona a la que le falta su tarea.

const laura = { id: 1, name: 'Laura Méndez' } as User
const diego = { id: 2, name: 'Diego Ramírez' } as User

const task = (id: number, completed: boolean, assignees: User[]) =>
  ({ id, completed, assignees } as Task)

const tasks = [
  task(10, false, [laura]),
  task(11, true, [laura]),
  task(12, false, [diego]),
  task(13, false, [laura, diego]), // compartida
  task(14, false, []), // sin responsable
]

const ids = (list: Task[]) => list.map((t) => t.id)

describe('filterTasksByScope', () => {
  it('sin filtro devuelve el tablero entero', () => {
    expect(ids(filterTasksByScope(tasks, { scope: 'all', onlyPending: false }))).toEqual(
      [10, 11, 12, 13, 14],
    )
  })

  it('deja solo las de un integrante, incluidas las compartidas', () => {
    expect(
      ids(filterTasksByScope(tasks, { scope: 'member', memberId: laura.id, onlyPending: false })),
    ).toEqual([10, 11, 13])
  })

  // Lo que de verdad se pidió: "las pendientes de x integrante".
  it('deja solo lo pendiente de un integrante', () => {
    expect(
      ids(filterTasksByScope(tasks, { scope: 'member', memberId: laura.id, onlyPending: true })),
    ).toEqual([10, 13])
  })

  it('esconde lo terminado también en Todas y en Mis tareas', () => {
    expect(ids(filterTasksByScope(tasks, { scope: 'all', onlyPending: true }))).toEqual(
      [10, 12, 13, 14],
    )
    expect(
      ids(filterTasksByScope(tasks, { scope: 'mine', currentUserId: laura.id, onlyPending: true })),
    ).toEqual([10, 13])
  })

  // Si el integrante ya no está en el tablero, la pantalla cae a 'all': mejor
  // enseñarlo todo que una columna vacía sin explicación.
  it('sin integrante no filtra por persona', () => {
    expect(
      ids(filterTasksByScope(tasks, { scope: 'member', memberId: null, onlyPending: false })),
    ).toEqual([10, 11, 12, 13, 14])
  })

  it('no se rompe con una tarea sin responsables', () => {
    const huerfana = [task(20, false, undefined as unknown as User[])]
    expect(
      ids(filterTasksByScope(huerfana, { scope: 'member', memberId: laura.id, onlyPending: false })),
    ).toEqual([])
  })
})

describe('memberPendingOptions', () => {
  it('cuenta lo que le queda sin terminar a cada quien', () => {
    expect(memberPendingOptions([laura, diego], tasks)).toEqual([
      { id: 1, name: 'Laura Méndez', pending: 2 },
      { id: 2, name: 'Diego Ramírez', pending: 2 },
    ])
  })

  it('devuelve lista vacía si el tablero no tiene integrantes', () => {
    expect(memberPendingOptions(undefined, tasks)).toEqual([])
  })
})

describe('scopeCount', () => {
  // El contador tiene que cuadrar con lo que se ve: "Todas 5" sobre un tablero
  // que muestra 4 tarjetas hace dudar de si falta algo.
  it('sigue al interruptor de pendientes', () => {
    expect(scopeCount(tasks, false)).toBe(5)
    expect(scopeCount(tasks, true)).toBe(4)
  })
})
