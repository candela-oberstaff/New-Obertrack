import { columnInViewOrder, moveCard } from './boardDrag'
import type { Task } from '../../../types'

// La lista llega del servidor por fecha de creación (la más nueva primero), que
// NO es el orden en que se ven las columnas. Ahí estaba el fallo.
function task(id: number, status: string, order: number): Task {
  return { id, status, order, board_id: 1, title: `T${id}` } as unknown as Task
}

const ids = (list: Task[]) => list.map((t) => t.id)

describe('moveCard', () => {
  // "En proceso" se ve 10, 20, 30, 40 (por order), pero llega desordenada.
  const server = [task(40, 'proceso', 3), task(30, 'proceso', 2), task(5, 'hacer', 0), task(20, 'proceso', 1), task(10, 'proceso', 0)]

  it('al pasarla a otra columna queda justo donde se soltó', () => {
    const moved = moveCard(server, 5, 1, 'proceso', 30)
    expect(ids(columnInViewOrder(moved, 1, 'proceso'))).toEqual([10, 20, 5, 30, 40])
  })

  it('soltada sobre la columna, va al final', () => {
    const moved = moveCard(server, 5, 1, 'proceso', null)
    expect(ids(columnInViewOrder(moved, 1, 'proceso'))).toEqual([10, 20, 30, 40, 5])
  })

  it('el order guardado coincide con lo que se ve (no salta al recargar)', () => {
    const moved = moveCard(server, 5, 1, 'proceso', 20)
    const column = columnInViewOrder(moved, 1, 'proceso')
    expect(column.map((t) => t.order)).toEqual([0, 1, 2, 3, 4])
    expect(ids(column)).toEqual([10, 5, 20, 30, 40])
  })

  it('dentro de la misma columna toma el lugar de la de debajo', () => {
    const moved = moveCard(server, 40, 1, 'proceso', 20)
    expect(ids(columnInViewOrder(moved, 1, 'proceso'))).toEqual([10, 40, 20, 30])
  })

  it('no toca otras columnas ni otros tableros', () => {
    const other = { ...task(99, 'proceso', 0), board_id: 2 } as Task
    const moved = moveCard([...server, other], 5, 1, 'proceso', 30)
    expect(moved.find((t) => t.id === 99)).toBe(other)
    expect(columnInViewOrder(moved, 1, 'hacer')).toEqual([])
  })
})
