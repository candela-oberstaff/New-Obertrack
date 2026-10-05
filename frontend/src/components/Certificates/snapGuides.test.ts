import { describe, expect, it } from 'vitest'
import { snapPosition } from './snapGuides'

describe('snapPosition', () => {
  it('se pega al centro de la página en los dos ejes', () => {
    const r = snapPosition({ x: 49.4, y: 50.6 }, [], 1, 1)
    expect(r).toMatchObject({ x: 50, y: 50 })
    expect(r.vertical).toEqual({ at: 50, center: true })
    expect(r.horizontal).toEqual({ at: 50, center: true })
  })

  it('fuera del umbral no toca la posición ni muestra guías', () => {
    const r = snapPosition({ x: 42, y: 30 }, [{ x: 70, y: 80 }], 1, 1)
    expect(r).toEqual({ x: 42, y: 30, vertical: null, horizontal: null })
  })

  it('se alinea con otro campo (misma columna o misma altura)', () => {
    const r = snapPosition({ x: 69.6, y: 79.5 }, [{ x: 69, y: 80 }, { x: 31, y: 80 }], 1, 1)
    expect(r).toMatchObject({ x: 69, y: 80 })
    expect(r.vertical).toEqual({ at: 69, center: false })
    expect(r.horizontal).toEqual({ at: 80, center: false })
  })

  it('elige la guía más cercana', () => {
    const r = snapPosition({ x: 51, y: 10 }, [{ x: 51.6, y: 90 }], 2, 1)
    expect(r.x).toBe(51.6)
    expect(r.vertical?.center).toBe(false)
  })

  it('a la misma distancia gana el centro de la página', () => {
    const r = snapPosition({ x: 50.5, y: 10 }, [{ x: 51, y: 90 }], 1, 1)
    expect(r.x).toBe(50)
    expect(r.vertical?.center).toBe(true)
  })
})
