import { describe, it, expect } from 'vitest'
import { ageAt, emergencyContacts } from './person'

describe('ageAt', () => {
  const hoy = new Date(2026, 8, 11) // 11 sep 2026, hora local

  it('cuenta los años cumplidos, no los que va a cumplir', () => {
    expect(ageAt('2002-10-09', hoy)).toBe(23) // cumple en octubre: todavía 23
    expect(ageAt('2002-09-11', hoy)).toBe(24) // cumple hoy
    expect(ageAt('2002-09-10', hoy)).toBe(24) // cumplió ayer
  })

  it('lee la fecha como calendario, no como instante UTC', () => {
    // Medianoche UTC del día 9 es la noche del 8 en Caracas; leído mal, el
    // cumpleaños se movería un día. Es el mismo fallo que ya tuvo last_active.
    expect(ageAt('2002-10-09T00:00:00Z', hoy)).toBe(23)
  })

  it('no inventa una edad para lo que no vale', () => {
    expect(ageAt(null, hoy)).toBeNull()
    expect(ageAt('', hoy)).toBeNull()
    expect(ageAt('no es fecha', hoy)).toBeNull()
    expect(ageAt('2030-01-01', hoy)).toBeNull() // futura
  })
})

describe('emergencyContacts', () => {
  it('parte por coma y limpia lo vacío', () => {
    expect(emergencyContacts('+58 412 1234567 mamá, 0414 5556677 Juan (hermano)'))
      .toEqual(['+58 412 1234567 mamá', '0414 5556677 Juan (hermano)'])
    expect(emergencyContacts(' , 111 ,, ')).toEqual(['111'])
  })

  it('sin dato devuelve lista vacía, no una entrada en blanco', () => {
    expect(emergencyContacts('')).toEqual([])
    expect(emergencyContacts(null)).toEqual([])
    expect(emergencyContacts(undefined)).toEqual([])
  })
})
