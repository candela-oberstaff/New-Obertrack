import { comparatorFor, compareTasks } from './taskSort'
import type { Task } from '../../types'

const t = (id: number, end_date: string | null, order = 0) => ({ id, end_date, order } as unknown as Task)

describe('comparatorFor', () => {
  // Orden manual ya tocado: 3, 1, 2 (la del 20-oct quedó arriba al arrastrar).
  const tasks = [t(1, '2026-10-25', 1), t(2, null, 2), t(3, '2026-10-20', 0), t(4, '2026-10-18', 3)]
  const ids = (list: Task[]) => list.map((x) => x.id)

  it('manual respeta el orden arrastrado', () => {
    expect(ids([...tasks].sort(comparatorFor('manual')))).toEqual(ids([...tasks].sort(compareTasks)))
    expect(ids([...tasks].sort(comparatorFor('manual')))).toEqual([3, 1, 2, 4])
  })

  it('entrega más próxima primero, sin fecha al final, ignorando el orden manual', () => {
    expect(ids([...tasks].sort(comparatorFor('due_asc')))).toEqual([4, 3, 1, 2])
  })

  it('entrega más lejana primero, sin fecha igualmente al final', () => {
    expect(ids([...tasks].sort(comparatorFor('due_desc')))).toEqual([1, 3, 4, 2])
  })

  it('empates de fecha: decide el orden manual', () => {
    const same = [t(10, '2026-10-20', 5), t(11, '2026-10-20', 1)]
    expect(ids(same.sort(comparatorFor('due_asc')))).toEqual([11, 10])
  })
})
