import { describe, it, expect, beforeEach } from 'vitest'
import { setRecordNav, getRecordNav, locateRecord, removeFromRecordNav } from './recordNav'

describe('recordNav', () => {
  beforeEach(() => {
    sessionStorage.clear()
  })

  it('devuelve la secuencia que anotó el listado', () => {
    setRecordNav('tenants', [5, 2, 9])
    expect(getRecordNav('tenants')).toEqual([5, 2, 9])
  })

  it('mantiene separadas las secuencias de cada ámbito', () => {
    setRecordNav('tenants', [1, 2])
    setRecordNav('admin-users', [7, 8])
    expect(getRecordNav('tenants')).toEqual([1, 2])
    expect(getRecordNav('admin-users')).toEqual([7, 8])
  })

  it('cada empresa recorre su propia plantilla', () => {
    setRecordNav('tenant-employees:1', [10, 11])
    setRecordNav('tenant-employees:2', [20, 21])
    expect(locateRecord('tenant-employees:2', 21).position).toBe(2)
    expect(locateRecord('tenant-employees:1', 21).position).toBe(0)
  })

  it('sitúa un registro intermedio con sus dos vecinos', () => {
    setRecordNav('tenants', [5, 2, 9])
    expect(locateRecord('tenants', 2)).toEqual({ prevId: 5, nextId: 9, position: 2, total: 3 })
  })

  it('no ofrece anterior en el primero ni siguiente en el último', () => {
    setRecordNav('tenants', [5, 2, 9])
    expect(locateRecord('tenants', 5)).toMatchObject({ prevId: null, nextId: 2, position: 1 })
    expect(locateRecord('tenants', 9)).toMatchObject({ prevId: 2, nextId: null, position: 3 })
  })

  // Entrar por enlace directo o desde otro listado: sin posición no se pinta el
  // paginador, en vez de mover al usuario por una lista que no tenía delante.
  it('deja fuera al registro que no está en la secuencia', () => {
    setRecordNav('tenants', [5, 2, 9])
    expect(locateRecord('tenants', 404)).toEqual({ prevId: null, nextId: null, position: 0, total: 3 })
  })

  it('sin secuencia anotada no hay nada que recorrer', () => {
    expect(locateRecord('tenants', 5)).toEqual({ prevId: null, nextId: null, position: 0, total: 0 })
  })

  it('ignora lo que haya guardado si no es una lista de ids', () => {
    sessionStorage.setItem('obertrack:nav:tenants', '{"roto":true}')
    expect(getRecordNav('tenants')).toEqual([])
    sessionStorage.setItem('obertrack:nav:tenants', 'no es json')
    expect(getRecordNav('tenants')).toEqual([])
  })

  it('descarta entradas que no sean numéricas', () => {
    sessionStorage.setItem('obertrack:nav:tenants', '[1,"2",null,3]')
    expect(getRecordNav('tenants')).toEqual([1, 3])
  })

  it('al eliminar un registro lo saca de la secuencia y recoloca a los demás', () => {
    setRecordNav('empresa-employees', [5, 2, 9])
    removeFromRecordNav('empresa-employees', 2)
    expect(getRecordNav('empresa-employees')).toEqual([5, 9])
    expect(locateRecord('empresa-employees', 5)).toMatchObject({ nextId: 9, position: 1, total: 2 })
  })
})
