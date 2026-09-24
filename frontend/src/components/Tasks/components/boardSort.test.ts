import { sortBoards, totalTasks } from './boardSort'
import type { Board } from '../../../types'

// El orden de los tableros lo fijaba el servidor y no se podía cambiar. Estas
// pruebas sostienen los criterios y, sobre todo, que ordenar no reordene el
// array original ni baile entre renders.

const board = (id: number, name: string, created_at: string) =>
  ({ id, name, created_at } as Board)

const boards = [
  board(1, 'Marketing', '2026-01-10T10:00:00Z'),
  board(2, 'Ventas', '2026-03-05T10:00:00Z'),
  board(3, 'diseño', '2026-02-01T10:00:00Z'),
]

const counts = {
  1: { todo: 3, done: 2 }, // 5
  2: { todo: 1 }, // 1
  3: { todo: 4, doing: 4 }, // 8
}

const names = (list: Board[]) => list.map((b) => b.name)

describe('sortBoards', () => {
  it('ordena del más reciente al más antiguo', () => {
    expect(names(sortBoards(boards, 'recent'))).toEqual(['Ventas', 'diseño', 'Marketing'])
  })

  it('ordena del más antiguo al más reciente', () => {
    expect(names(sortBoards(boards, 'oldest'))).toEqual(['Marketing', 'diseño', 'Ventas'])
  })

  // 'diseño' va antes que 'Marketing' pese a la minúscula: se compara como lo
  // leería una persona, no por código de carácter.
  it('ordena por nombre sin que la mayúscula ni el acento manden', () => {
    expect(names(sortBoards(boards, 'name_asc'))).toEqual(['diseño', 'Marketing', 'Ventas'])
    expect(names(sortBoards(boards, 'name_desc'))).toEqual(['Ventas', 'Marketing', 'diseño'])
  })

  it('ordena por cantidad de tareas', () => {
    expect(names(sortBoards(boards, 'tasks_desc', counts))).toEqual(['diseño', 'Marketing', 'Ventas'])
    expect(names(sortBoards(boards, 'tasks_asc', counts))).toEqual(['Ventas', 'Marketing', 'diseño'])
  })

  // Con un tablero abierto los conteos pueden no haber llegado todavía. Los que
  // faltan valen cero y entre ellos manda el desempate por nombre.
  it('trata como cero el tablero del que no hay conteo', () => {
    expect(names(sortBoards(boards, 'tasks_desc', { 2: { todo: 9 } }))).toEqual(
      ['Ventas', 'diseño', 'Marketing'],
    )
  })

  // Sin desempate, dos tableros iguales en el criterio se intercambiaban de
  // sitio en cada render y la lista parecía moverse sola.
  it('desempata por nombre', () => {
    const empatados = [
      board(1, 'Zeta', '2026-01-01T00:00:00Z'),
      board(2, 'Alfa', '2026-01-01T00:00:00Z'),
    ]
    expect(names(sortBoards(empatados, 'recent'))).toEqual(['Alfa', 'Zeta'])
  })

  it('no toca el array que recibe', () => {
    const original = [...boards]
    sortBoards(boards, 'name_asc')
    expect(boards).toEqual(original)
  })

  it('aguanta una fecha ilegible sin mandarla al principio', () => {
    const rotos = [board(1, 'Bueno', '2026-05-01T00:00:00Z'), board(2, 'Roto', 'no-es-fecha')]
    expect(names(sortBoards(rotos, 'recent'))).toEqual(['Bueno', 'Roto'])
  })
})

describe('totalTasks', () => {
  it('suma todas las columnas del tablero', () => {
    expect(totalTasks(1, counts)).toBe(5)
    expect(totalTasks(3, counts)).toBe(8)
  })

  it('devuelve cero si no hay conteo', () => {
    expect(totalTasks(99, counts)).toBe(0)
    expect(totalTasks(1, {})).toBe(0)
  })
})
